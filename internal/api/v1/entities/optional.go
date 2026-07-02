package entities

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.patchbase.net/server/internal/utils"
)

// NewOptionalFormattedTime converts a nullable *time.Time into a formatted
// Option[string] using apiTimeLayout, or None when the pointer is nil.
func NewOptionalFormattedTime(value *time.Time) utils.Option[string] {
	if value == nil {
		return utils.None[string]()
	}
	return utils.Some(value.UTC().Format(apiTimeLayout))
}

// NewOptionalPgText converts a pgtype.Text into an Option[string],
// or None when the value is invalid/null.
func NewOptionalPgText(value pgtype.Text) utils.Option[string] {
	if !value.Valid {
		return utils.None[string]()
	}
	return utils.Some(value.String)
}
