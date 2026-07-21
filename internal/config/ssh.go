// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package config

import "time"

type SSH struct {
	PullJobTimeout time.Duration `mapstructure:"pull_job_timeout" yaml:"pull_job_timeout"`
}

func (s *SSH) Validate() error {
	if s.PullJobTimeout <= 0 {
		return ErrInvalidConfig("ssh.pull_job_timeout", "must be greater than zero")
	}
	return nil
}

const (
	DefaultSSHPullJobTimeout = 5 * time.Minute
)

func init() {
	SetDefault("ssh.pull_job_timeout", DefaultSSHPullJobTimeout)
}
