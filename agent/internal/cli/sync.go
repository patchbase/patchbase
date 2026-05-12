package cli

import (
	"fmt"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"go.patchbase.net/agent/internal/client"
	"go.patchbase.net/agent/internal/collector"
	"go.patchbase.net/agent/internal/config"
	"google.golang.org/protobuf/encoding/protojson"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Collect a snapshot and POST it to the PatchBase backend",
		RunE:  runSync,
	}

	cmd.Flags().StringP("config", "c", config.DefaultPath, "config file path")
	cmd.Flags().StringP("server-url", "s", "", "server URL (overrides config)")
	cmd.Flags().StringP("token", "t", "", "host token (overrides config)")
	cmd.Flags().String("ca-cert", "", "CA certificate path")
	cmd.Flags().BoolP("allow-insecure-http", "k", false, "allow plain HTTP")
	cmd.Flags().Bool("debug", false, "print snapshot JSON to stdout")

	return cmd
}

type syncOpts struct {
	configPath        string
	serverURL         string
	hostToken         string
	caCert            string
	allowInsecureHTTP bool
	printPayload      bool
}

func runSync(cmd *cobra.Command, args []string) error {
	opts, err := parseSyncOpts(cmd, args)
	if err != nil {
		return err
	}

	fs := config.DefaultFS()
	fileConfig, err := loadSyncConfig(fs, opts)
	if err != nil {
		return err
	}

	snapshot, err := collector.CollectSnapshot(cmd.Context(), fs, collector.DefaultExecRunner, version)
	if err != nil {
		return fmt.Errorf("collect snapshot: %w", err)
	}

	if opts.printPayload {
		data, err := protojson.MarshalOptions{Multiline: true}.Marshal(snapshot)
		if err != nil {
			return fmt.Errorf("marshal snapshot: %w", err)
		}
		fmt.Println(string(data))
	}

	httpClient, err := client.NewHTTPClient(fileConfig.CACert, fileConfig.AllowInsecureHTTP)
	if err != nil {
		return fmt.Errorf("create http client: %w", err)
	}

	result, err := httpClient.PostSnapshot(cmd.Context(), fileConfig.ServerURL, fileConfig.HostToken, snapshot)
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	fmt.Printf("sync status=%d endpoint=%s\n", result.Status, result.Endpoint)
	if result.Status < 200 || result.Status >= 300 {
		if result.RequestID != "" {
			fmt.Printf("request_id=%s\n", result.RequestID)
		}
	}
	if result.Response != nil {
		fmt.Printf("accepted=%v\n", result.Response.Accepted)
		if result.Response.HostID != "" {
			fmt.Printf("host_id=%s\n", result.Response.HostID)
		}
		if result.Response.SnapshotID != "" {
			fmt.Printf("snapshot_id=%s\n", result.Response.SnapshotID)
		}
		if result.Response.NextCheckInSeconds != 0 {
			fmt.Printf("next_check_in_seconds=%d\n", result.Response.NextCheckInSeconds)
		}
	} else if len(result.Body) != 0 {
		fmt.Printf("response_body=%s\n", string(result.Body))
	}

	if result.Status < 200 || result.Status >= 300 {
		return fmt.Errorf("sync rejected with status %d", result.Status)
	}

	return nil
}

func loadSyncConfig(fs afero.Fs, opts syncOpts) (config.File, error) {
	fileConfig, err := config.Load(fs, opts.configPath)
	if err != nil {
		if opts.serverURL == "" && opts.hostToken == "" {
			return config.File{}, fmt.Errorf("config not found at %s; run `patchbase-agent enroll <server-url> <token>` or pass --server-url and --token", opts.configPath)
		}
		if opts.serverURL == "" {
			return config.File{}, fmt.Errorf("missing --server-url")
		}
		if opts.hostToken == "" {
			return config.File{}, fmt.Errorf("missing --token")
		}
		return config.File{
			ServerURL:         opts.serverURL,
			HostToken:         opts.hostToken,
			CACert:            opts.caCert,
			AllowInsecureHTTP: opts.allowInsecureHTTP,
		}, nil
	}

	if opts.serverURL != "" {
		fileConfig.ServerURL = opts.serverURL
	}
	if opts.hostToken != "" {
		fileConfig.HostToken = opts.hostToken
	}
	if opts.caCert != "" {
		fileConfig.CACert = opts.caCert
	}
	if opts.allowInsecureHTTP {
		fileConfig.AllowInsecureHTTP = true
	}

	return fileConfig, nil
}

func parseSyncOpts(cmd *cobra.Command, args []string) (syncOpts, error) {
	configPath, err := cmd.Flags().GetString("config")
	if err != nil {
		return syncOpts{}, fmt.Errorf("parse config flag: %w", err)
	}
	serverURL, err := cmd.Flags().GetString("server-url")
	if err != nil {
		return syncOpts{}, fmt.Errorf("parse server-url flag: %w", err)
	}
	hostToken, err := cmd.Flags().GetString("token")
	if err != nil {
		return syncOpts{}, fmt.Errorf("parse token flag: %w", err)
	}
	caCert, err := cmd.Flags().GetString("ca-cert")
	if err != nil {
		return syncOpts{}, fmt.Errorf("parse ca-cert flag: %w", err)
	}
	allowInsecureHTTP, err := cmd.Flags().GetBool("allow-insecure-http")
	if err != nil {
		return syncOpts{}, fmt.Errorf("parse allow-insecure-http flag: %w", err)
	}
	printPayload, err := cmd.Flags().GetBool("debug")
	if err != nil {
		return syncOpts{}, fmt.Errorf("parse debug flag: %w", err)
	}

	return syncOpts{
		configPath:        configPath,
		serverURL:         serverURL,
		hostToken:         hostToken,
		caCert:            caCert,
		allowInsecureHTTP: allowInsecureHTTP,
		printPayload:      printPayload,
	}, nil
}
