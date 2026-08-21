// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/lifecycle"
	"go.patchbase.net/server/internal/utils"
)

type LifecycleService interface {
	ListHosts(ctx context.Context) ([]HostLifecycleInfo, error)
	CatalogSource(ctx context.Context) (lifecycle.CatalogSource, error)
}

type HostLifecycleInfo struct {
	HostInfo
	Lifecycle lifecycle.Status `json:"lifecycle"`
}

type lifecycleService struct {
	hosts   Hosts
	catalog *lifecycle.Catalog
}

func NewLifecycleService(i do.Injector) (LifecycleService, error) {
	hostsService, err := do.Invoke[Hosts](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get Hosts service: %w", err)
	}
	catalog, err := lifecycle.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load lifecycle catalog: %w", err)
	}
	return &lifecycleService{
		hosts:   hostsService,
		catalog: catalog,
	}, nil
}

func (s *lifecycleService) ListHosts(ctx context.Context) ([]HostLifecycleInfo, error) {
	hosts, err := s.hosts.ListHosts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}
	now := time.Now().UTC()

	return utils.Map(hosts, func(host HostInfo) HostLifecycleInfo {
		return HostLifecycleInfo{
			HostInfo:  host,
			Lifecycle: s.catalog.Status(now, 0, lifecycle.NewStatusInput(host.OSName, host.OSVersion, host.OSMajor)),
		}
	}), nil
}

func (s *lifecycleService) CatalogSource(_ context.Context) (lifecycle.CatalogSource, error) {
	return s.catalog.Source(), nil
}
