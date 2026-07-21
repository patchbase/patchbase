// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package client

import agent "go.patchbase.net/proto/agent"

type Result[R any] struct {
	Endpoint     string
	Status       int
	RequestID    string
	Body         []byte
	Response     *R
	ErrorCode    string
	ErrorMessage string
}

type SyncResult = Result[agent.SyncResponse]

type RegisterResult = Result[agent.RegisterHostResponse]