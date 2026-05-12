package collector

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/spf13/afero"
	agent "go.patchbase.net/proto/agent"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

func CollectSnapshot(ctx context.Context, fs afero.Fs, runner ExecRunner, version string) (*agent.AgentSnapshot, error) {
	now := time.Now().UTC()
	sentAt := timestamppb.New(now)

	osRelease, err := ReadOsRelease(fs)
	if err != nil {
		return nil, fmt.Errorf("read os-release: %w", err)
	}

	osFamily, err := NormalizeOsFamily(osRelease.ID)
	if err != nil {
		return nil, fmt.Errorf("normalize os family: %w", err)
	}

	osMajor, err := ParseMajorVersion(osRelease.VersionID)
	if err != nil {
		return nil, fmt.Errorf("parse major version: %w", err)
	}

	machineID, err := ReadMachineID(fs)
	if err != nil {
		return nil, fmt.Errorf("read machine-id: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("read hostname: %w", err)
	}

	unameMachine, unameRelease, err := getUnameInfo()
	if err != nil {
		return nil, fmt.Errorf("detect architecture: %w", err)
	}

	arch, err := DetectArchitecture(unameMachine)
	if err != nil {
		return nil, fmt.Errorf("detect architecture: %w", err)
	}

	uptimeSeconds, err := ReadUptime(fs)
	if err != nil {
		return nil, fmt.Errorf("read uptime: %w", err)
	}

	bootTime := now.Add(-time.Duration(uptimeSeconds) * time.Second)
	bootTimePb := timestamppb.New(bootTime)

	var availableUpdateCount int32
	count, err := CollectAvailablePackageUpdateCount(ctx, runner, osFamily)
	if err == nil {
		availableUpdateCount = count
	}

	hostnameStr := hostname

	host := &agent.Host{
		MachineId:                   machineID,
		Hostname:                    hostnameStr,
		OsFamily:                    osFamily,
		OsName:                      osRelease.Name,
		OsVersion:                   osRelease.VersionID,
		OsMajor:                     osMajor,
		Architecture:                arch,
		AvailablePackageUpdateCount: availableUpdateCount,
		AgentVersion:                version,
		BootTime:                    bootTimePb,
		UptimeSeconds:               uptimeSeconds,
	}

	kernelNEVRA, err := RunningKernelNEVRA(ctx, runner, osFamily, unameRelease)
	if err != nil {
		return nil, fmt.Errorf("detect kernel: %w", err)
	}

	runtime := &agent.Runtime{
		KernelRunning: kernelNEVRA,
	}

	pkgs, err := CollectInstalledPackages(ctx, runner, osFamily)
	if err != nil {
		return nil, fmt.Errorf("collect packages: %w", err)
	}

	repos, err := CollectEnabledRepos(fs, osFamily)
	if err != nil {
		return nil, fmt.Errorf("collect repos: %w", err)
	}

	return &agent.AgentSnapshot{
		SchemaVersion: "v0",
		SentAt:        sentAt,
		Host:          host,
		Repos:         repos,
		Packages:      pkgs,
		Runtime:       runtime,
	}, nil
}

func getUnameInfo() (machine string, release string, err error) {
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err != nil {
		return "", "", fmt.Errorf("uname: %w", err)
	}

	var machineBytes []byte
	for _, c := range uname.Machine {
		if c == 0 {
			break
		}
		machineBytes = append(machineBytes, byte(c))
	}

	var releaseBytes []byte
	for _, c := range uname.Release {
		if c == 0 {
			break
		}
		releaseBytes = append(releaseBytes, byte(c))
	}

	return string(machineBytes), string(releaseBytes), nil
}
