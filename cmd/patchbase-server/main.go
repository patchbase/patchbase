package main

import (
	"os"

	"go.patchbase.net/server/internal/cli"
	_ "golang.org/x/crypto/x509roots/fallback"
)

func main() {
	cmd := cli.MainCmd()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
