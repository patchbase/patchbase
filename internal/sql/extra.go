package sql

import (
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
