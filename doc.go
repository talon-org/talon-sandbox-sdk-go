// Package talonsandbox is the Go SDK for Talon Sandbox — isolated container
// environments for AI agents.
//
// # Quick start
//
//	import sandbox "x.xgit.pro/dark/talon-sandbox-sdk-go"
//
//	sb, err := sandbox.Create(ctx, sandbox.Opts{
//	    Image:     "talon-alpine",
//	    Resources: sandbox.Resources{CPU: 2, Memory: "4GiB"},
//	    Network:   "allowlist",
//	    Timeout:   "30m",
//	    TTL:       "6h",
//	    Labels:    map[string]string{"project": "agent-x"},
//	})
//	if err != nil { log.Fatal(err) }
//	defer sb.Kill(ctx)
//
//	// Synchronous command
//	result, err := sb.Run(ctx, "npm install")
//
//	// Async long-running process
//	proc, err := sb.Spawn(ctx, "npm run dev")
//	proc.OnStdout(func(chunk []byte) { fmt.Printf("[stdout] %s", chunk) })
//
//	// Interactive terminal
//	pty, err := sb.Terminal().Open(ctx)
//	pty.OnData(func(chunk []byte) { os.Stdout.Write(chunk) })
//	pty.Write(ctx, []byte("ls /\n"))
//
//	// Expose port
//	url, err := sb.Expose(ctx, 5173)
//
//	// Filesystem
//	data, err := sb.FS().Read(ctx, "/workspace/app.py")
//	err = sb.FS().Write(ctx, "/workspace/app.py", []byte("print('hi')"))
//
// # Configuration
//
// Set TALON_SANDBOX_SERVER and TALON_SANDBOX_API_KEY environment variables, or
// call Configure before first use:
//
//	sandbox.Configure("https://api.example.com", "ask_...")
//
// To use a custom client for a single call:
//
//	sb, err := sandbox.Create(ctx, opts, sandbox.WithAPIKey("ask_abc"))
package talonsandbox
