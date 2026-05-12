package main

import (
	"os"

	"go.patchbase.net/server/internal/cli"
)

func main() {
	cmd := cli.MainCmd()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
