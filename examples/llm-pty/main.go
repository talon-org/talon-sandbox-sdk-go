// llm-pty demonstrates streaming PTY output suitable for feeding an LLM.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	sandbox "x.xgit.pro/dark/talon-sandbox-sdk-go"
)

func main() {
	ctx := context.Background()

	sb, err := sandbox.Create(ctx, sandbox.Opts{
		Image:   "talon-alpine",
		Network: "open",
		TTL:     "30m",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer sb.Kill(ctx)

	pty, err := sb.Terminal().Open(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer pty.Close(ctx)

	pty.OnData(func(chunk []byte) {
		// In production: feed chunk to your LLM context window.
		fmt.Printf("[pty] %s", chunk)
	})
	pty.OnClose(func() {
		fmt.Println("[pty] session closed")
	})

	pty.Write(ctx, []byte("ls /\n"))
	time.Sleep(500 * time.Millisecond)
	pty.Resize(ctx, 40, 120)
	pty.Write(ctx, []byte("echo done\n"))
	time.Sleep(200 * time.Millisecond)
}
