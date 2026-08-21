// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package lifecycle_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.patchbase.net/server/internal/lifecycle"
)

func mustCatalog(t *testing.T) *lifecycle.Catalog {
	t.Helper()
	c, err := lifecycle.Load()
	require.NoError(t, err)
	return c
}

func TestLoadValidatesCatalog(t *testing.T) {
	c, err := lifecycle.Load()
	require.NoError(t, err)
	assert.NotEmpty(t, c.Products)
	assert.NotEmpty(t, c.GeneratedAt)
	assert.NotEmpty(t, c.SourceURLs)
}

func TestStatusUnknownForUnrecognizedOS(t *testing.T) {
	c := mustCatalog(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	s := c.Status(now, 0, lifecycle.StatusInput{OSName: "Solaris", OSVersion: "11"})
	assert.Equal(t, lifecycle.StatusUnknown, s.State)
	assert.Empty(t, s.ProductKey)
	assert.True(t, s.Product.IsNone())
}

func TestStatusUnknownWhenVersionMissing(t *testing.T) {
	c := mustCatalog(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	s := c.Status(now, 0, lifecycle.StatusInput{OSName: "Ubuntu"})
	assert.Equal(t, lifecycle.StatusUnknown, s.State)
}

func TestStatusUnknownForUnmatchedCycle(t *testing.T) {
	c := mustCatalog(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	s := c.Status(now, 0, lifecycle.StatusInput{OSName: "Ubuntu", OSVersion: "99.04"})
	assert.Equal(t, lifecycle.StatusUnknown, s.State)
}

func TestStatusSupportedForUbuntu2404(t *testing.T) {
	c := mustCatalog(t)
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)

	s := c.Status(now, 0, lifecycle.StatusInput{OSName: "Ubuntu", OSVersion: "24.04"})
	assert.Equal(t, lifecycle.StatusSupported, s.State)
	assert.Equal(t, "ubuntu", s.ProductKey)
	assert.Equal(t, "Ubuntu", s.ProductDisplayName)
	assert.Equal(t, "24.04", s.Cycle)
	assert.Equal(t, "2029-04-25", s.StandardSupportEnd)
	assert.True(t, s.DaysRemaining.IsPresent() && s.DaysRemaining.Unwrap() > 0)
	assert.NotEmpty(t, s.SourceURL)
}

func TestStatusApproachingEndOfSupport(t *testing.T) {
	c := mustCatalog(t)
	// Ubuntu 22.04 standard support ends 2027-04-26. Pick a "now" 60 days
	// before that, well within the default 365-day approaching window.
	now := time.Date(2027, 2, 26, 0, 0, 0, 0, time.UTC)

	s := c.Status(now, 0, lifecycle.StatusInput{OSName: "Ubuntu", OSVersion: "22.04"})
	assert.Equal(t, lifecycle.StatusApproachingEndOfSupport, s.State)
	assert.InDelta(t, 59, s.DaysRemaining.Unwrap(), 1)
}

func TestStatusSupportedJustOutsideApproachingWindow(t *testing.T) {
	c := mustCatalog(t)
	// Ubuntu 24.04 ends 2029-04-25. 400 days before is ~2028-03-21,
	// outside the default 365-day approaching window.
	now := time.Date(2028, 3, 21, 0, 0, 0, 0, time.UTC)

	s := c.Status(now, 0, lifecycle.StatusInput{OSName: "Ubuntu", OSVersion: "24.04"})
	assert.Equal(t, lifecycle.StatusSupported, s.State)
}

func TestStatusStandardSupportEndedNoExtendedCoverage(t *testing.T) {
	c := mustCatalog(t)
	// Rocky 9 standard support ends 2032-05-31 and there is no extended
	// coverage recorded. Pick a "now" after that date.
	now := time.Date(2032, 6, 1, 0, 0, 0, 0, time.UTC)

	s := c.Status(now, 0, lifecycle.StatusInput{OSName: "Rocky Linux", OSVersion: "9"})
	assert.Equal(t, lifecycle.StatusStandardSupportEnded, s.State)
	assert.True(t, s.DaysRemaining.IsPresent() && s.DaysRemaining.Unwrap() < 0)
}

func TestStatusExtendedCoverageAvailableForUbuntu2004(t *testing.T) {
	c := mustCatalog(t)
	// Ubuntu 20.04 standard support ended 2025-04-23;
	// extended coverage runs to 2030-04-23.
	// A date in between must surface extended coverage availability (the host subscription is not verified).
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	s := c.Status(now, 0, lifecycle.StatusInput{OSName: "Ubuntu", OSVersion: "20.04"})
	assert.Equal(t, lifecycle.StatusExtendedCoverageAvailable, s.State)
	assert.Equal(t, "2030-04-23", s.ExtendedCoverageEnd)
	assert.NotEmpty(t, s.ExtendedCoverageNote)
}

func TestStatusStandardSupportEndedAfterExtendedCoverageExpires(t *testing.T) {
	c := mustCatalog(t)
	// Ubuntu 20.04 extended coverage ends 2030-04-23.
	// After that date the host is past standard support with no remaining coverage.
	now := time.Date(2030, 5, 1, 0, 0, 0, 0, time.UTC)

	s := c.Status(now, 0, lifecycle.StatusInput{OSName: "Ubuntu", OSVersion: "20.04"})
	assert.Equal(t, lifecycle.StatusStandardSupportEnded, s.State)
}

func TestStatusOnEndDateIsApproachingWithZeroDaysRemaining(t *testing.T) {
	c := mustCatalog(t)
	// Ubuntu 24.04 ends 2029-04-25. On the end date inclusive, the host
	// is within the approaching window (0 days <= 365) and days
	// remaining is exactly 0 (not negative).
	now := time.Date(2029, 4, 25, 12, 0, 0, 0, time.UTC)

	s := c.Status(now, 0, lifecycle.StatusInput{OSName: "Ubuntu", OSVersion: "24.04"})
	assert.Equal(t, lifecycle.StatusApproachingEndOfSupport, s.State)
	assert.Equal(t, 0, s.DaysRemaining.Unwrap())
}

func TestStatusDayAfterEndIsStandardSupportEnded(t *testing.T) {
	c := mustCatalog(t)
	// Rocky 9 has no extended coverage; standard support ends 2032-05-31.
	// The day after, the host is past standard support.
	now := time.Date(2032, 6, 1, 0, 0, 0, 0, time.UTC)

	s := c.Status(now, 0, lifecycle.StatusInput{OSName: "Rocky Linux", OSVersion: "9"})
	assert.Equal(t, lifecycle.StatusStandardSupportEnded, s.State)
	assert.Equal(t, -1, s.DaysRemaining.Unwrap())
}

func TestStatusFallsBackToOSMajorWhenVersionMissing(t *testing.T) {
	c := mustCatalog(t)
	// Debian 12 standard support ends 2026-06-10.
	// Use a date well outside the 365-day approaching window so the status is "supported".
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	s := c.Status(now, 0, lifecycle.StatusInput{OSName: "Debian GNU/Linux", OSMajor: 12})
	assert.Equal(t, lifecycle.StatusSupported, s.State)
	assert.Equal(t, "12", s.Cycle)
}

func TestStatusResolvesMinorVersionToMajorCycle(t *testing.T) {
	c := mustCatalog(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// A host reporting a point release like "9.2" must still match the "9" cycle in the catalog.
	// The matcher first tries the raw version and, when no exact match exists, falls back to the major version.
	s := c.Status(now, 0, lifecycle.StatusInput{OSName: "Rocky Linux", OSVersion: "9.2"})
	assert.Equal(t, lifecycle.StatusSupported, s.State)
	assert.Equal(t, "9", s.Cycle)
}

func TestStatusHonorsAliasesCaseInsensitive(t *testing.T) {
	c := mustCatalog(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	cases := []string{"Ubuntu", "ubuntu", " UBUNTU "}
	for _, name := range cases {
		s := c.Status(now, 0, lifecycle.StatusInput{OSName: name, OSVersion: "24.04"})
		assert.Equal(t, lifecycle.StatusSupported, s.State, "alias %q should match", name)
	}
}

func TestStatusResolvesEquivalentNameAliases(t *testing.T) {
	c := mustCatalog(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// "Red Hat Enterprise Linux Server" is an alias declared in the catalog for the rhel product and must resolve like the canonical name.
	s := c.Status(now, 0, lifecycle.StatusInput{OSName: "Red Hat Enterprise Linux Server", OSVersion: "9"})
	assert.Equal(t, "rhel", s.ProductKey)
	assert.Equal(t, lifecycle.StatusSupported, s.State)
}

func TestStatusCustomApproachingThreshold(t *testing.T) {
	c := mustCatalog(t)
	// Ubuntu 24.04 ends 2029-04-25.
	// With a 30-day threshold, a date 60 days before should NOT be flagged as approaching.
	now := time.Date(2029, 2, 24, 0, 0, 0, 0, time.UTC)

	s := c.Status(now, 30*24*time.Hour, lifecycle.StatusInput{OSName: "Ubuntu", OSVersion: "24.04"})
	assert.Equal(t, lifecycle.StatusSupported, s.State)

	// With a 90-day threshold the same date should be approaching.
	s = c.Status(now, 90*24*time.Hour, lifecycle.StatusInput{OSName: "Ubuntu", OSVersion: "24.04"})
	assert.Equal(t, lifecycle.StatusApproachingEndOfSupport, s.State)
}

func TestSourceReturnsCatalogProvenance(t *testing.T) {
	c := mustCatalog(t)
	src := c.Source()
	assert.NotEmpty(t, src.GeneratedAt)
	assert.NotEmpty(t, src.SourceURLs)
}

func TestProductByAliasReturnsNilForUnknown(t *testing.T) {
	c := mustCatalog(t)
	assert.True(t, c.ProductByAlias("nonexistent").IsNone())
}

func TestProductReleaseReturnsNilForUnknownCycle(t *testing.T) {
	c := mustCatalog(t)
	p, ok := c.Product("ubuntu").Get()
	require.True(t, ok)
	assert.True(t, p.Release("99.04").IsNone())
}

func TestStatusNonLTSUbuntuRelease(t *testing.T) {
	c := mustCatalog(t)
	// Ubuntu 25.04 (non-LTS) has standard support ending 2026-01-05 and no extended coverage.
	// Use a date well outside the 365-day approaching window so the status is "supported".
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	s := c.Status(now, 0, lifecycle.StatusInput{OSName: "Ubuntu", OSVersion: "25.04"})
	assert.Equal(t, lifecycle.StatusSupported, s.State)
	assert.Equal(t, "25.04", s.Cycle)
	assert.Empty(t, s.ExtendedCoverageEnd)
}

func TestStatusNonLTSUbuntuEnded(t *testing.T) {
	c := mustCatalog(t)
	// After 2026-01-05, Ubuntu 25.04 has no extended coverage, so the
	// status must be standard-support-ended (not extended-coverage).
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	s := c.Status(now, 0, lifecycle.StatusInput{OSName: "Ubuntu", OSVersion: "25.04"})
	assert.Equal(t, lifecycle.StatusStandardSupportEnded, s.State)
	assert.Empty(t, s.ExtendedCoverageEnd)
}
