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

SDK 默认连接官方托管端点 `https://api.sandbox.talon.net.cn`。
只需设置 API Key 即可直接使用：

```
TALON_SANDBOX_API_KEY=ask_...
```

自部署用户可通过环境变量覆盖服务地址：

```
TALON_SANDBOX_SERVER=https://your-self-hosted.example.com
TALON_SANDBOX_API_KEY=ask_...
```

或在代码中显式配置：

```go
// 托管端（只需 API Key，无需设置 server）
sandbox.Configure("", "ask_...")

// 自部署端
sandbox.Configure("https://your-self-hosted.example.com", "ask_...")
```

## License

Proprietary — All rights reserved.
