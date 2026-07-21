// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	agent "go.patchbase.net/proto/agent"
	"google.golang.org/protobuf/proto"
)

type HTTPClient struct {
	serverURL string
	client    *http.Client
}

func NewHTTPClient(serverURL string, caCertPath string, allowInsecureHTTP bool) (Client, error) {
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

	if strings.HasPrefix(serverURL, "http://") && !isLoopback(serverURL) {
		return nil, fmt.Errorf("http only allowed for loopback addresses")
	}

	return &HTTPClient{
		serverURL: strings.TrimRight(serverURL, "/"),
		client:    &http.Client{Transport: transport, Timeout: 60 * time.Second},
	}, nil
}

func (c *HTTPClient) RegisterHost(ctx context.Context, r *agent.RegisterHostRequest) (*RegisterResult, error) {
	return httpPost[agent.RegisterHostResponse](c, ctx, c.registerEndpoint(), r, "")
}

func (c *HTTPClient) PostSnapshot(ctx context.Context, hostToken string, snapshot *agent.AgentSnapshot) (*SyncResult, error) {
	return httpPost[agent.SyncResponse](c, ctx, c.snapshotEndpoint(), snapshot, hostToken)
}

func (c *HTTPClient) snapshotEndpoint() string {
	return c.serverURL + "/api/v1/agent/snapshots"
}

func (c *HTTPClient) registerEndpoint() string {
	return c.serverURL + "/api/v1/agent/register"
}

func (c *HTTPClient) doRequest(ctx context.Context, method string, endpoint string, body []byte, authToken string) (*http.Response, []byte, error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Accept", "application/x-protobuf")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close() // nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read response: %w", err)
	}

	return resp, respBody, nil
}

func httpPost[R any](c *HTTPClient, ctx context.Context, endpoint string, payload proto.Message, authToken string) (*Result[R], error) {
	body, err := proto.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, respBody, err := c.doRequest(ctx, http.MethodPost, endpoint, body, authToken)
	if err != nil {
		return nil, err
	}

	result := &Result[R]{
		Endpoint:     endpoint,
		Status:       resp.StatusCode,
		RequestID:    resp.Header.Get("X-Request-Id"),
		Body:         respBody,
		Response:     nil,
		ErrorMessage: "",
	}

	if resp.StatusCode >= 400 {
		var apiErr agent.APIError
		if err := proto.Unmarshal(respBody, &apiErr); err == nil {
			result.ErrorCode = apiErr.GetCode()
			result.ErrorMessage = apiErr.GetMessage()
		}
		if result.ErrorMessage == "" {
			if result.ErrorCode == "" {
				result.ErrorMessage = string(respBody)
			} else {
				result.ErrorMessage = result.ErrorCode
			}
		}
	} else {
		var r R
		pm, ok := any(&r).(proto.Message)
		if !ok {
			return nil, fmt.Errorf("internal error: type %T does not implement proto.Message", &r)
		}
		if err := proto.Unmarshal(respBody, pm); err == nil {
			result.Response = &r
		}
	}

	return result, nil
}

func isLoopback(endpoint string) bool {
	return strings.Contains(endpoint, "://localhost") ||
		strings.Contains(endpoint, "://127.0.0.1") ||
		strings.Contains(endpoint, "://[::1]")
}
