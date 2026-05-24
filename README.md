# talon-sandbox-sdk-go

Go SDK v2 for [Talon Sandbox](https://talon-sandbox.dev) — isolated container environments for AI agents.

## Install

```sh
go get x.xgit.pro/dark/talon-sandbox-sdk-go
```

## Quick start

```go
import sandbox "x.xgit.pro/dark/talon-sandbox-sdk-go"

ctx := context.Background()

sb, err := sandbox.Create(ctx, sandbox.Opts{
    Image:     "node:20-bookworm",
    Resources: sandbox.Resources{CPU: 2, Memory: "4GiB"},
    Network:   "allowlist",
    Timeout:   "30m",
    TTL:       "6h",
    Labels:    map[string]string{"project": "agent-x"},
})
if err != nil { log.Fatal(err) }
defer sb.Kill(ctx)

// Run a command synchronously
result, err := sb.Run(ctx, "npm install")

// Spawn a long-running process
proc, err := sb.Spawn(ctx, "npm run dev")
proc.OnStdout(func(chunk []byte) { fmt.Printf("[dev] %s", chunk) })
proc.Wait(ctx)

// Interactive terminal
pty, _ := sb.Terminal().Open(ctx)
pty.OnData(func(chunk []byte) { os.Stdout.Write(chunk) })
pty.Write(ctx, []byte("ls /\n"))
pty.Resize(ctx, 40, 120)
pty.Close(ctx)

// Expose a port
url, _ := sb.Expose(ctx, 5173)
fmt.Println(url) // https://sb-xxx-5173.preview.talon-sandbox.dev

// Signed URL
signed, _ := sb.Expose(ctx, 5173, sandbox.ExposeOpts{Sign: true, TTL: "1h"})

// Filesystem
data, _ := sb.FS().Read(ctx, "/workspace/app.py")
sb.FS().Write(ctx, "/workspace/app.py", []byte("print('hi')"))
entries, _ := sb.FS().List(ctx, "/workspace")
sb.FS().Remove(ctx, "/workspace/old")

// Environment variables
val, _ := sb.Env().Get(ctx, "NODE_ENV")
sb.Env().Set(ctx, "API_KEY", "sk-...")

// Browser (CDP)
b, _ := sb.Browser().Start(ctx)
fmt.Println(b.CDPURL)

// Reattach to an existing sandbox
sb2, _ := sandbox.Get(ctx, "sb_abc123")
list, _ := sandbox.List(ctx, sandbox.ListOpts{Labels: map[string]string{"project": "agent-x"}})
```

## Configuration

Set environment variables:

```
TALON_SANDBOX_SERVER=https://api.example.com
TALON_SANDBOX_API_KEY=ask_...
```

Or configure programmatically:

```go
sandbox.Configure("https://api.example.com", "ask_...")
```

## License

Proprietary — All rights reserved.
