// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "v0.0.0-dev"

func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the agent version",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(version)
		},
	}

	return cmd
}
