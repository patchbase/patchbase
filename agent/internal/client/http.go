package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	agent "go.patchbase.net/proto/agent"
	"google.golang.org/protobuf/proto"
)

type SyncResult struct {
	Endpoint  string
	Status    int
	RequestID string
	Body      []byte
	Response  *SyncResponse
}

type SyncResponse struct {
	Accepted           bool   `json:"accepted"`
	HostID             string `json:"host_id"`
	SnapshotID         string `json:"snapshot_id"`
	NextCheckInSeconds int32  `json:"next_check_in_seconds"`
}

type HTTPClient struct {
	Client *http.Client
}

func NewHTTPClient(caCertPath string, allowInsecureHTTP bool) (*HTTPClient, error) {
	transport := &http.Transport{}

	if caCertPath != "" {
		data, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("read CA cert: %w", err)
		}

		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("failed to append CA cert")
		}

		transport.TLSClientConfig = &tls.Config{
			RootCAs: pool,
		}
	}

	if allowInsecureHTTP {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	return &HTTPClient{
		Client: &http.Client{Transport: transport},
	}, nil
}

func (c *HTTPClient) PostSnapshot(ctx context.Context, serverURL, hostToken string, snapshot *agent.AgentSnapshot) (*SyncResult, error) {
	endpoint := snapshotEndpoint(serverURL)

	body, err := proto.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal snapshot: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+hostToken)
	req.Header.Set("Content-Type", "application/x-protobuf")

	isHTTP := strings.HasPrefix(endpoint, "http://")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("post snapshot: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if isHTTP && !isLoopback(endpoint) {
		return nil, fmt.Errorf("http only allowed for loopback addresses")
	}

	result := &SyncResult{
		Endpoint:  endpoint,
		Status:    resp.StatusCode,
		RequestID: resp.Header.Get("X-Request-Id"),
		Body:      respBody,
	}

	var syncResp SyncResponse
	if err := json.Unmarshal(respBody, &syncResp); err == nil {
		result.Response = &syncResp
	}

	return result, nil
}

func snapshotEndpoint(serverURL string) string {
	trimmed := strings.TrimRight(serverURL, "/")
	suffix := "/api/v1/agent/snapshots"
	if strings.HasSuffix(trimmed, suffix) {
		return trimmed
	}
	return trimmed + suffix
}

func isLoopback(endpoint string) bool {
	return strings.Contains(endpoint, "://localhost") ||
		strings.Contains(endpoint, "://127.0.0.1") ||
		strings.Contains(endpoint, "://[::1]")
}
