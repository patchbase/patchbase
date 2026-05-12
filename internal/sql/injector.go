package sql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/do/v2"
	"go.patchbase.net/server/internal/config"
)

func NewWithInjector(i do.Injector) (Querier, error) {
	pgxpool, err := do.Invoke[*pgxpool.Pool](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get *pgxpool.Pool from injector: %w", err)
	}
	return New(pgxpool), nil
}

func NewPGXPool(i do.Injector) (*pgxpool.Pool, error) {
	config, err := do.Invoke[config.Config](i)
	if err != nil {
		return nil, fmt.Errorf("failed to get config.Config from injector: %w", err)
	}

	return pgxpool.New(context.Background(), config.Database.URL)
}
