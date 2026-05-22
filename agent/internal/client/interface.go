package client

import (
	"context"

	agent "go.patchbase.net/proto/agent"
)

type Client interface {
	PostSnapshot(ctx context.Context, hostToken string, snapshot *agent.AgentSnapshot) (*SyncResult, error)
	RegisterHost(ctx context.Context, req *agent.RegisterHostRequest) (*RegisterResult, error)
}

type PostSnapshotRequest struct {
	HostToken string
	Snapshot  *agent.AgentSnapshot
}


