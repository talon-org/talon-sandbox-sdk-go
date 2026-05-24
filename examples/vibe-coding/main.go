// vibe-coding demonstrates a full workflow: create sandbox, write files,
// install deps, spawn a dev server, expose port, and clean up.
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
		Image: "node:20-bookworm",
		Resources: sandbox.Resources{
			CPU:    2,
			Memory: "4GiB",
			Disk:   "10GiB",
		},
		Network: "allowlist",
		Env:     map[string]string{"NODE_ENV": "development"},
		Timeout: "30m",
		TTL:     "6h",
		Labels:  map[string]string{"project": "vibe-coding"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer sb.Kill(ctx)

	fmt.Println("Sandbox created:", sb.ID())

	// Write a simple Express server.
	appCode := []byte(`
const express = require('express')
const app = express()
app.get('/', (req, res) => res.send('hello from talon sandbox!'))
app.listen(3000, () => console.log('listening on :3000'))
`)
	if err := sb.FS().Write(ctx, "/workspace/index.js", appCode); err != nil {
		log.Fatal("write file:", err)
	}
	pkgJSON := []byte(`{"name":"demo","version":"1.0.0","dependencies":{"express":"^4"}}`)
	if err := sb.FS().Write(ctx, "/workspace/package.json", pkgJSON); err != nil {
		log.Fatal("write package.json:", err)
	}

	// Install dependencies synchronously.
	result, err := sb.Run(ctx, "cd /workspace && npm install")
	if err != nil {
		log.Fatal("npm install:", err)
	}
	fmt.Println("npm install exit:", result.ExitCode)

	// Spawn the dev server.
	proc, err := sb.Spawn(ctx, "node /workspace/index.js")
	if err != nil {
		log.Fatal("spawn:", err)
	}
	proc.OnStdout(func(line []byte) {
		fmt.Printf("[server] %s", line)
	})

	time.Sleep(time.Second)

	// Expose port 3000 for external access.
	url, err := sb.Expose(ctx, 3000)
	if err != nil {
		fmt.Println("expose not available:", err)
	} else {
		fmt.Println("Preview URL:", url)
	}

	// Run for 5 seconds then clean up.
	time.Sleep(5 * time.Second)
	proc.Kill(ctx)
}
