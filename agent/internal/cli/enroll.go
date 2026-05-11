package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.patchbase.net/agent/internal/config"
)

func newEnrollCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enroll <server-url> <token>",
		Short: "Write agent config to disk for later sync runs",
		Args:  cobra.ExactArgs(2),
		RunE:  enroll,
	}

	cmd.Flags().StringP("config", "-c", config.DefaultPath, "config file path")
	cmd.Flags().String("ca-cert", "", "CA certificate path")
	cmd.Flags().BoolP("allow-insecure-http", "-k", false, "allow plain HTTP")

	return cmd
}

func enroll(cmd *cobra.Command, args []string) error {
	configPath, err := cmd.Flags().GetString("config")
	if err != nil {
		return fmt.Errorf("get config path: %w", err)
	}

	caCert, err := cmd.Flags().GetString("ca-cert")
	if err != nil {
		return fmt.Errorf("get CA cert path: %w", err)
	}

	allowInsecureHTTP, err := cmd.Flags().GetBool("allow-insecure-http")
	if err != nil {
		return fmt.Errorf("get allow-insecure-http flag: %w", err)
	}

	cfg := config.File{
		ServerURL:         args[0],
		HostToken:         args[1],
		CACert:            caCert,
		AllowInsecureHTTP: allowInsecureHTTP,
	}

	fs := config.DefaultFS()
	if err := config.Save(fs, configPath, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("wrote config to %s\nserver_url=%s\n", configPath, cfg.ServerURL)
	return nil
}
