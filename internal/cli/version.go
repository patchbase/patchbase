package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.patchbase.net/server/internal/buildinfo"
)

func runVersion(cmd *cobra.Command, args []string) {
	fmt.Println(buildinfo.Version)
}

func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run:   runVersion,
	}
}
