package client

import agent "go.patchbase.net/proto/agent"

type Result[R any] struct {
	Endpoint     string
	Status       int
	RequestID    string
	Body         []byte
	Response     *R
	ErrorMessage string
}

type SyncResult = Result[agent.SyncResponse]

type RegisterResult = Result[agent.RegisterHostResponse]
