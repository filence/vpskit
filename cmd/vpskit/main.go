package main

import (
	"fmt"
	"os"

	"vpskit.local/vpskit/internal/app"
)

var (
	version                = "dev"
	releasePublicKeyBase64 = ""
)

func main() {
	if err := app.Run(os.Args[1:], version, releasePublicKeyBase64); err != nil {
		fmt.Fprintln(os.Stderr, "vpskit:", err)
		os.Exit(1)
	}
}
