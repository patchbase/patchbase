// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package webutil

import (
	"fmt"
	"strconv"
	"time"

	"go.patchbase.net/server/internal/utils"
)

// ParseInt parses a non-negative integer from a query string value.
// An empty input yields the supplied default. Negative inputs are rejected so callers
// don't have to repeat the bounds check for pagination parameters.
func ParseInt(raw string, fieldName string, defaultValue int) (int, error) {
	if raw == "" {
		return defaultValue, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", fieldName, err)
	}
	if v < 0 {
		return 0, fmt.Errorf("%s must not be negative", fieldName)
	}
	return v, nil
}

// ParseIntOpt parses a non-negative integer from a query string value.
// An empty input yields nil. Negative inputs are rejected so callers
// don't have to repeat the bounds check for pagination parameters.
func ParseIntOpt(raw string, fieldName string) (utils.Option[int], error) {
	if raw == "" {
		return utils.None[int](), nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return utils.None[int](), fmt.Errorf("invalid %s: %w", fieldName, err)
	}
	if v < 0 {
		return utils.None[int](), fmt.Errorf("%s must not be negative", fieldName)
	}
	return utils.Some(v), nil
}

func ParseInt32Opt(raw string, fieldName string) (utils.Option[int32], error) {
	value, err := ParseIntOpt(raw, fieldName)
	if err != nil {
		return utils.None[int32](), err
	}
	if value.IsNone() {
		return utils.None[int32](), nil
	}
	v := value.Unwrap()
	if v > int(^uint32(0)>>1) {
		return utils.None[int32](), fmt.Errorf("%s must not be greater than %d", fieldName, int(^uint32(0)>>1))
	}
	return utils.Some(int32(v)), nil
}

// ParseTimestamp accepts RFC3339 / RFC3339Nano and bare date (YYYY-MM-DD)
// inputs. An empty string yields the zero time.Time and no error so callers
// can use it directly on optional query parameters.
//
// RFC3339 inputs are returned with their time-of-day intact.
// Bare dates are widened to the START of that day in UTC (00:00:00.000000000) so a
// `created_at >= from` bound is inclusive of the user's selected day.
// Pair this with ParseTimestampEnd on the upper bound of a date range
// filter so the user-selected day is fully included.
func ParseTimestamp(raw string) (time.Time, error) {
	t, dateOnly, err := parseTimestamp(raw)
	if err != nil || t.IsZero() {
		return t, err
	}
	if dateOnly {
		return startOfDayUTC(t), nil
	}
	return t, nil
}

// ParseTimestampEnd accepts RFC3339 / RFC3339Nano and bare date (YYYY-MM-DD) inputs.
// RFC3339 inputs are returned with their time-of-day intact;
// bare dates are widened to the END of that day in UTC (23:59:59.999999999) so that a
// SQL `<=` bound on a user-typed date includes every event recorded during that calendar day
// not just events at midnight. An empty string yields the zero time.Time and no error.
func ParseTimestampEnd(raw string) (time.Time, error) {
	t, dateOnly, err := parseTimestamp(raw)
	if err != nil || t.IsZero() {
		return t, err
	}
	if dateOnly {
		return endOfDayUTC(t), nil
	}
	return t, nil
}

// parseTimestamp parses raw as RFC3339 (preferred) or YYYY-MM-DD and
// returns whether the input was date-only, so callers can apply
// day-boundary normalization only when the user actually supplied a bare
// date rather than a full timestamp.
func parseTimestamp(raw string) (time.Time, bool, error) {
	if raw == "" {
		return time.Time{}, false, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, false, nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t, true, nil
	}
	return time.Time{}, false, fmt.Errorf("invalid timestamp %q: expected RFC3339 or YYYY-MM-DD", raw)
}

// startOfDayUTC returns midnight (00:00:00.000000000) on the same calendar day as t, in UTC.
func startOfDayUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// endOfDayUTC returns the last representable nanosecond on the same calendar day as t, in UTC.
// SQL `<=` comparisons against this value will include every row whose timestamp falls within that day.
func endOfDayUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
}
