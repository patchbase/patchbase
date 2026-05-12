package entities

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.patchbase.net/server/internal/utils"
)

const dateTimeLayout = "2006-01-02T15:04:05.000000"

type DateTime time.Time

func NewDateTime(ts pgtype.Timestamp) utils.Option[DateTime] {
	if !ts.Valid {
		return utils.None[DateTime]()
	}
	return utils.Some(DateTime(ts.Time))
}

func NewDateTimeFromTimestamptz(ts pgtype.Timestamptz) utils.Option[DateTime] {
	if !ts.Valid {
		return utils.None[DateTime]()
	}
	return utils.Some(DateTime(ts.Time))
}

func (dt DateTime) MarshalJSON() ([]byte, error) {
	t := time.Time(dt).UTC()
	return json.Marshal(t.Format(dateTimeLayout))
}
