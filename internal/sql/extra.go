// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package sql

import (
	"errors"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"go.patchbase.net/server/internal/utils"
)

// IsUniqueViolation reports whether err is a PostgreSQL unique-violation on the
// given constraint or unique-index name. Callers must specify which constraint
// they care about so unrelated unique violations surface as 500s instead of
// being silently mapped to a domain error.
func IsUniqueViolation(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == pgerrcode.UniqueViolation &&
		pgErr.ConstraintName == constraintName
}

// IsForeignKeyViolation reports whether err is a PostgreSQL foreign-key
// violation on the given constraint name.
func IsForeignKeyViolation(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == pgerrcode.ForeignKeyViolation &&
		pgErr.ConstraintName == constraintName
}

func ToOption[T any](v T, err error) (utils.Option[T], error) {
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return utils.None[T](), nil
		}
		return utils.None[T](), err
	}
	return utils.Some(v), nil
}

func Required[T any](v T, err error) func(fallback error) (T, error) {
	return func(fallback error) (T, error) {
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return v, fallback
			}
			return v, err
		}
		return v, nil
	}
}

func TimestamptzNow() pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:             time.Now().UTC(),
		Valid:            true,
		InfinityModifier: pgtype.Finite,
	}
}

func TimestamptzFromTime(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{
			Time:             time.Time{},
			Valid:            false,
			InfinityModifier: pgtype.Finite,
		}
	}

	return pgtype.Timestamptz{
		Time:             t.UTC(),
		Valid:            true,
		InfinityModifier: pgtype.Finite,
	}
}

func NewTimeOption(ts pgtype.Timestamptz) utils.Option[time.Time] {
	if !ts.Valid {
		return utils.None[time.Time]()
	}
	return utils.Some(ts.Time.UTC())
}
