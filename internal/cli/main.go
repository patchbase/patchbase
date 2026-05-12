package cli

import "github.com/spf13/cobra"

func MainCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "patchbase-server",
		Short: "PatchBase server is the backend for self-hosted PatchBase",
	}

	root.AddCommand(NewServeCmd())
	root.AddCommand(NewMigrateCmd())
	root.AddCommand(NewVersionCmd())

	return root
}
