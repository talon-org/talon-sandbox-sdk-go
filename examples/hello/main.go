package main

import (
	"context"
	"fmt"
	"log"
	"os"

	sandbox "x.xgit.pro/dark/talon-sandbox-sdk-go"
)

func main() {
	ctx := context.Background()

	sb, err := sandbox.Create(ctx, sandbox.Opts{
		Image:   "talon-alpine",
		Resources: sandbox.Resources{CPU: 1, Memory: "2GiB"},
		Network: "allowlist",
		TTL:     "1h",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer sb.Kill(ctx)

	fmt.Println("sandbox:", sb.ID())

	result, err := sb.Run(ctx, "node -e 'console.log(\"hello from sandbox\")'")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(result.Combined)

	url, err := sb.Expose(ctx, 3000)
	if err != nil {
		// Expose endpoint may not be available yet (Spec 50 pending).
		fmt.Fprintln(os.Stderr, "expose:", err)
	} else {
		fmt.Println("Preview URL:", url)
	}
}
