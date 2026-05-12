package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "v0.0.0-dev"

func runVersion(cmd *cobra.Command, args []string) {
	fmt.Println(version)
}

func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run:   runVersion,
	}
}
