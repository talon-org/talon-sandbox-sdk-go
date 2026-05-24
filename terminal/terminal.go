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
)

// Terminal manages PTY sessions for a single sandbox.
type Terminal struct {
	sandboxID string
	wsBase    string // e.g. "ws://localhost:18080"
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

	opts := &websocket.DialOptions{}
	if t.authHdr != "" {
		opts.HTTPHeader = http.Header{
			"Authorization": {t.authHdr},
		}
	}
	if len(t.cookies) > 0 {
		if opts.HTTPHeader == nil {
			opts.HTTPHeader = http.Header{}
		}
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

	sess := &PTYSession{
		conn:   conn,
		stopCh: make(chan struct{}),
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
	conn   *websocket.Conn
	mu     sync.Mutex
	closed bool
	stopCh chan struct{}

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

// Close closes the PTY session gracefully.
func (p *PTYSession) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	close(p.stopCh)
	return p.conn.Close(websocket.StatusNormalClosure, "")
}

func (p *PTYSession) recvLoop() {
	defer p.emitClose()
	for {
		select {
		case <-p.stopCh:
			return
		default:
		}
		_, data, err := p.conn.Read(context.Background())
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
