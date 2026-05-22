package cli

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"go.patchbase.net/agent/internal/client"
	"go.patchbase.net/agent/internal/config"
	agent "go.patchbase.net/proto/agent"
)

func newEnrollCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enroll <server-url> <token>",
		Short: "Write agent config to disk for later sync runs",
		Args:  cobra.ExactArgs(2),
		RunE:  enroll,
	}

	cmd.Flags().StringP("config", "c", config.DefaultPath, "config file path")
	cmd.Flags().String("ca-cert", "", "CA certificate path")
	cmd.Flags().BoolP("allow-insecure-http", "k", false, "allow plain HTTP")

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
		HostToken:         "",
		CACert:            caCert,
		AllowInsecureHTTP: allowInsecureHTTP,
	}

	httpClient, err := client.NewHTTPClient(args[0], caCert, allowInsecureHTTP)
	if err != nil {
		return fmt.Errorf("create http client: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("read hostname: %w", err)
	}

	registerResult, err := httpClient.RegisterHost(
		cmd.Context(),
		&agent.RegisterHostRequest{
			RegistrationToken: args[1],
			Hostname:          hostname,
			MachineId:         readMachineID(),
			Metadata:          collectRegistrationMetadata(),
		},
	)
	if err != nil {
		return fmt.Errorf("register host: %w", err)
	}
	if registerResult.Response == nil || registerResult.Response.HostAccessToken == "" {
		return fmt.Errorf("register host: invalid response")
	}

	cfg.HostToken = registerResult.Response.HostAccessToken

	fs := config.DefaultFS()
	if err := config.Save(fs, configPath, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	slog.Info("Successfully enrolled host",
		"config_path", configPath,
		"server_url", cfg.ServerURL,
		"host_id", registerResult.Response.HostId,
		"approval_status", registerResult.Response.ApprovalStatus,
	)
	return nil
}

func readMachineID() string {
	data, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func collectRegistrationMetadata() *agent.RegisterHostMetadata {
	return &agent.RegisterHostMetadata{
		IpAddress:    firstNonLoopbackIP(),
		OsName:       runtime.GOOS,
		OsVersion:    readOSVersion(),
		Architecture: runtime.GOARCH,
	}
}

func readOSVersion() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown"
	}

	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "VERSION_ID=") {
			continue
		}
		value := strings.TrimPrefix(line, "VERSION_ID=")
		value = strings.TrimSpace(strings.Trim(value, `"`))
		if value != "" {
			return value
		}
	}

	return "unknown"
}

func firstNonLoopbackIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP == nil || ipNet.IP.IsLoopback() {
			continue
		}
		ip4 := ipNet.IP.To4()
		if ip4 != nil {
			return ip4.String()
		}
	}

	return ""
}
