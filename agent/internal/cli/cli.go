// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package cli

import (
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	root := &cobra.Command{
		Use:   "patchbase-agent",
		Short: "PatchBase agent",
	}

	root.AddCommand(
		newEnrollCmd(),
		newSyncCmd(),
		newVersionCmd(),
	)

	return root
}