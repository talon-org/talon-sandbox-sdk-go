// Package terminal implements PTY sessions over WebSocket for Talon Sandbox.
//
// Usage:
//
//	term := sb.Terminal()
//	pty, err := term.Open(ctx)
//	pty.OnData(func(b []byte) { os.Stdout.Write(b) })
//	pty.Write(ctx, []byte("ls /\n"))
//	pty.Close(ctx)
package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/coder/websocket"

	"x.xgit.pro/dark/talon-sandbox-sdk-go/internal/httpx"
)

// Terminal manages PTY sessions for a single sandbox.
type Terminal struct {
	sandboxID string
	wsBase    string // 例如 "wss://api.sandbox.talon.net.cn"，自部署时为 "ws://localhost:18080"
	authHdr   string
	cookies   []*http.Cookie
}

// New creates a Terminal handle. Called by Sandbox.Terminal().
func New(sandboxID, wsBase, authHeader string, cookies []*http.Cookie) *Terminal {
	return &Terminal{
		sandboxID: sandboxID,
		wsBase:    wsBase,
		authHdr:   authHeader,
		cookies:   cookies,
	}
}

// Open dials the PTY WebSocket endpoint and returns a live PTYSession.
// The receive loop starts immediately in a goroutine.
func (t *Terminal) Open(ctx context.Context) (*PTYSession, error) {
	wsURL := toWS(t.wsBase + "/v1/sandboxes/" + t.sandboxID + "/pty")

	// WebSocket 握手头也带规范 User-Agent,与 HTTP 请求一致,保证来源归因为 sdk-go。
	opts := &websocket.DialOptions{
		HTTPHeader: http.Header{
			"User-Agent": {httpx.UserAgent()},
		},
	}
	if t.authHdr != "" {
		opts.HTTPHeader.Set("Authorization", t.authHdr)
	}
	if len(t.cookies) > 0 {
		parts := make([]string, 0, len(t.cookies))
		for _, ck := range t.cookies {
			parts = append(parts, ck.Name+"="+ck.Value)
		}
		opts.HTTPHeader.Set("Cookie", strings.Join(parts, "; "))
	}

	conn, _, err := websocket.Dial(ctx, wsURL, opts)
	if err != nil {
		return nil, fmt.Errorf("terminal open: %w", err)
	}
	conn.SetReadLimit(4 * 1024 * 1024)

	// Derive a session-scoped context from the parent. Cancelling sessCtx
	// (via Close, or parent ctx cancellation) unblocks conn.Read so the
	// recvLoop goroutine exits cleanly. Without this, recvLoop would call
	// conn.Read(context.Background()) which only returns on network error —
	// any caller that drops a *PTYSession reference without Close() leaks
	// the goroutine and the WebSocket connection.
	sessCtx, cancel := context.WithCancel(ctx)
	sess := &PTYSession{
		conn:    conn,
		stopCh:  make(chan struct{}),
		sessCtx: sessCtx,
		cancel:  cancel,
	}
	go sess.recvLoop()
	return sess, nil
}

func toWS(u string) string {
	u = strings.Replace(u, "https://", "wss://", 1)
	u = strings.Replace(u, "http://", "ws://", 1)
	return u
}

// ─── PTYSession ───────────────────────────────────────────────────────────────

// PTYSession is a live bidirectional PTY over WebSocket.
// Safe for concurrent use.
type PTYSession struct {
	conn    *websocket.Conn
	mu      sync.Mutex
	closed  bool
	stopCh  chan struct{}
	sessCtx context.Context
	cancel  context.CancelFunc

	cbMu     sync.RWMutex
	dataFns  []func([]byte)
	closeFns []func()
}

// OnData registers a callback for each data chunk received from the PTY.
// Callbacks are invoked sequentially from the receive goroutine.
func (p *PTYSession) OnData(fn func([]byte)) {
	p.cbMu.Lock()
	defer p.cbMu.Unlock()
	p.dataFns = append(p.dataFns, fn)
}

// OnClose registers a callback invoked when the session closes.
func (p *PTYSession) OnClose(fn func()) {
	p.cbMu.Lock()
	defer p.cbMu.Unlock()
	p.closeFns = append(p.closeFns, fn)
}

// Write sends bytes to the PTY stdin. Blocks until the frame is sent.
func (p *PTYSession) Write(ctx context.Context, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return fmt.Errorf("PTY session is closed")
	}
	return p.conn.Write(ctx, websocket.MessageBinary, data)
}

// Resize sends a terminal resize event to the server.
func (p *PTYSession) Resize(ctx context.Context, rows, cols int) error {
	msg, _ := json.Marshal(map[string]any{"type": "resize", "rows": rows, "cols": cols})
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return fmt.Errorf("PTY session is closed")
	}
	return p.conn.Write(ctx, websocket.MessageText, msg)
}

// Close closes the PTY session gracefully. Idempotent.
func (p *PTYSession) Close(ctx context.Context) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	close(p.stopCh)
	cancel := p.cancel
	p.mu.Unlock()
	// Cancel session ctx so recvLoop's pending Read returns immediately.
	cancel()
	return p.conn.Close(websocket.StatusNormalClosure, "")
}

func (p *PTYSession) recvLoop() {
	defer p.cancel() // release sessCtx resources on natural exit
	defer p.emitClose()
	for {
		// Reading with the session ctx instead of context.Background() so
		// Close() (or parent ctx cancel) unblocks this goroutine without
		// waiting for the server to close the WebSocket.
		_, data, err := p.conn.Read(p.sessCtx)
		if err != nil {
			return
		}
		p.emitData(data)
	}
}

func (p *PTYSession) emitData(data []byte) {
	p.cbMu.RLock()
	fns := make([]func([]byte), len(p.dataFns))
	copy(fns, p.dataFns)
	p.cbMu.RUnlock()
	for _, fn := range fns {
		safeCallData(fn, data)
	}
}

func (p *PTYSession) emitClose() {
	p.cbMu.RLock()
	fns := make([]func(), len(p.closeFns))
	copy(fns, p.closeFns)
	p.cbMu.RUnlock()
	for _, fn := range fns {
		safeCallClose(fn)
	}
}

func safeCallData(fn func([]byte), data []byte) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("PTY OnData callback panicked", "panic", r)
		}
	}()
	fn(data)
}

func safeCallClose(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("PTY OnClose callback panicked", "panic", r)
		}
	}()
	fn()
}
