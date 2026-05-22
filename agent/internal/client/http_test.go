package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	agent "go.patchbase.net/proto/agent"
	"google.golang.org/protobuf/proto"
)

func TestIsLoopback(t *testing.T) {
	assert.True(t, isLoopback("http://localhost/api"))
	assert.True(t, isLoopback("http://127.0.0.1/api"))
	assert.True(t, isLoopback("http://[::1]/api"))
	assert.False(t, isLoopback("https://patchbase.local/api"))
}

func TestHTTPClient_RegisterHost(t *testing.T) {
	expectedReq := &agent.RegisterHostRequest{
		RegistrationToken: "tok-123",
		Hostname:          "test-host",
		MachineId:         "m-123",
		Metadata: &agent.RegisterHostMetadata{
			IpAddress:    "1.2.3.4",
			OsName:       "linux",
			OsVersion:    "22.04",
			Architecture: "amd64",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/agent/register", r.URL.Path)
		assert.Equal(t, "application/x-protobuf", r.Header.Get("Content-Type"))

		bodyBytes, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var req agent.RegisterHostRequest
		err = proto.Unmarshal(bodyBytes, &req)
		require.NoError(t, err)
		assert.True(t, proto.Equal(expectedReq, &req))

		resp := &agent.RegisterHostResponse{
			HostId:          "host-abc",
			HostAccessToken: "acc-xyz",
			ApprovalStatus:  "approved",
		}
		w.Header().Set("X-Request-Id", "req-123")
		w.WriteHeader(http.StatusOK)
		respBytes, err := proto.Marshal(resp)
		require.NoError(t, err)
		_, err = w.Write(respBytes)
		require.NoError(t, err)
	}))
	defer srv.Close()

	c, err := NewHTTPClient(srv.URL, "", false)
	require.NoError(t, err)

	res, err := c.RegisterHost(context.Background(), expectedReq)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, srv.URL+"/api/v1/agent/register", res.Endpoint)
	assert.Equal(t, http.StatusOK, res.Status)
	assert.Equal(t, "req-123", res.RequestID)
	require.NotNil(t, res.Response)
	assert.Equal(t, "host-abc", res.Response.HostId)
	assert.Equal(t, "acc-xyz", res.Response.HostAccessToken)
	assert.Equal(t, "approved", res.Response.ApprovalStatus)
}

func TestHTTPClient_PostSnapshot(t *testing.T) {
	expectedSnap := &agent.AgentSnapshot{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/agent/snapshots", r.URL.Path)
		assert.Equal(t, "application/x-protobuf", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer host-tok", r.Header.Get("Authorization"))

		bodyBytes, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		
		var snap agent.AgentSnapshot
		err = proto.Unmarshal(bodyBytes, &snap)
		require.NoError(t, err)

		w.Header().Set("X-Request-Id", "req-456")
		w.WriteHeader(http.StatusAccepted)
		resp := &agent.SyncResponse{
			Accepted:           true,
			HostId:             "host-123",
			SnapshotId:         "snap-123",
			NextCheckInSeconds: 60,
		}
		respBytes, err := proto.Marshal(resp)
		require.NoError(t, err)
		_, err = w.Write(respBytes)
		require.NoError(t, err)
	}))
	defer srv.Close()

	c, err := NewHTTPClient(srv.URL, "", false)
	require.NoError(t, err)

	res, err := c.PostSnapshot(context.Background(), "host-tok", expectedSnap)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, srv.URL+"/api/v1/agent/snapshots", res.Endpoint)
	assert.Equal(t, http.StatusAccepted, res.Status)
	assert.Equal(t, "req-456", res.RequestID)
	require.NotNil(t, res.Response)
	assert.True(t, res.Response.Accepted)
	assert.Equal(t, "host-123", res.Response.HostId)
	assert.Equal(t, "snap-123", res.Response.SnapshotId)
	assert.Equal(t, int32(60), res.Response.NextCheckInSeconds)
}
