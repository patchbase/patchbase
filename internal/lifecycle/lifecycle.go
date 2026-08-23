// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only

package lifecycle

import (
	"embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.patchbase.net/server/internal/utils"
)

//go:embed catalog.json
var catalogFS embed.FS

const (
	// StatusStandardSupportEnded means standard vendor support for the matched release has ended.
	// If extended coverage is known to be available (the host subscription is not verified by PatchBase),
	// the [Status] will instead be StatusExtendedCoverageAvailable.
	StatusStandardSupportEnded = "standard-support-ended"

	// StatusApproachingEndOfSupport means the matched release is still within standard support but
	// standard support ends within the configured approaching threshold (default 365 days).
	StatusApproachingEndOfSupport = "approaching-end-of-support"

	// StatusSupported means the matched release is within standard support and standard support ends after the approaching threshold.
	StatusSupported = "supported"

	// StatusExtendedCoverageAvailable means standard support has ended but the catalog records an
	// extended coverage end date that is still in the future.
	// PatchBase does not verify whether the host is actually subscribed to the extended coverage program.
	StatusExtendedCoverageAvailable = "extended-coverage-available"

	// StatusUnknown means the catalog could not match the host's operating system to any product or release cycle.
	StatusUnknown = "unknown"
)

// DefaultApproachingThreshold is the default window used to flag releases approaching the end of standard support.
var DefaultApproachingThreshold = 365 * 24 * time.Hour

// Catalog is an in-memory, read-only view of the embedded lifecycle catalog.
type Catalog struct {
	GeneratedAt string    `json:"generated_at"`
	SourceURLs  []string  `json:"source_urls"`
	Products    []Product `json:"products"`
	once        sync.Once
	byID        map[string]*Product
	byAlias     map[string]*Product
}

// Product describes a vendor product and its known release cycles.
type Product struct {
	ProductKey  string    `json:"product_key"`
	DisplayName string    `json:"display_name"`
	Aliases     []string  `json:"aliases"`
	SourceURL   string    `json:"source_url"`
	Releases    []Release `json:"releases"`

	byCycle map[string]Release
}

// Release describes a single release cycle of a product.
type Release struct {
	Cycle                string `json:"cycle"`
	StandardSupportEnd   string `json:"standard_support_end"`
	EOL                  string `json:"eol"`
	ExtendedCoverageEnd  string `json:"extended_coverage_end,omitempty"`
	ExtendedCoverageNote string `json:"extended_coverage_note,omitempty"`
	SourceURL            string `json:"source_url"`
}

func Load() (*Catalog, error) {
	raw, err := catalogFS.ReadFile("catalog.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded lifecycle catalog: %w", err)
	}

	var c Catalog
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parse embedded lifecycle catalog: %w", err)
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("invalid lifecycle catalog: %w", err)
	}
	return &c, nil
}

// MustLoad is a convenience wrapper around [Load]
// For use in initialization paths where a malformed embedded catalog is a fatal error.
func MustLoad() *Catalog {
	c, err := Load()
	if err != nil {
		panic(fmt.Sprintf("lifecycle: %v", err))
	}
	return c
}

func (c *Catalog) validate() error {
	if len(c.Products) == 0 {
		return fmt.Errorf("catalog contains no products")
	}
	byID := make(map[string]*Product, len(c.Products))
	byAlias := make(map[string]*Product, len(c.Products))
	for i := range c.Products {
		p := &c.Products[i]
		if p.ProductKey == "" {
			return fmt.Errorf("product at index %d is missing product_key", i)
		}
		if len(p.Aliases) == 0 {
			return fmt.Errorf("product %q must declare at least one alias", p.ProductKey)
		}
		if len(p.Releases) == 0 {
			return fmt.Errorf("product %q must declare at least one release", p.ProductKey)
		}
		if _, dup := byID[p.ProductKey]; dup {
			return fmt.Errorf("duplicate product_key %q", p.ProductKey)
		}
		byID[p.ProductKey] = p

		byCycle := make(map[string]Release, len(p.Releases))
		for j := range p.Releases {
			r := p.Releases[j]
			if r.Cycle == "" {
				return fmt.Errorf("product %q has a release with an empty cycle", p.ProductKey)
			}
			if _, dup := byCycle[r.Cycle]; dup {
				return fmt.Errorf("product %q has duplicate cycle %q", p.ProductKey, r.Cycle)
			}
			if r.StandardSupportEnd == "" {
				return fmt.Errorf("product %q cycle %q is missing standard_support_end", p.ProductKey, r.Cycle)
			}
			if _, err := parseDate(r.StandardSupportEnd); err != nil {
				return fmt.Errorf("product %q cycle %q has invalid standard_support_end: %w", p.ProductKey, r.Cycle, err)
			}
			if r.EOL != "" {
				if _, err := parseDate(r.EOL); err != nil {
					return fmt.Errorf("product %q cycle %q has invalid eol: %w", p.ProductKey, r.Cycle, err)
				}
			}
			if r.ExtendedCoverageEnd != "" {
				if _, err := parseDate(r.ExtendedCoverageEnd); err != nil {
					return fmt.Errorf("product %q cycle %q has invalid extended_coverage_end: %w", p.ProductKey, r.Cycle, err)
				}
			}
			byCycle[r.Cycle] = r
		}
		p.byCycle = byCycle

		for _, alias := range p.Aliases {
			key := normalizeAlias(alias)
			if key == "" {
				continue
			}
			if owner, dup := byAlias[key]; dup && owner != p {
				return fmt.Errorf("alias %q is claimed by more than one product", key)
			}
			byAlias[key] = p
		}
	}
	c.byID = byID
	c.byAlias = byAlias
	return nil
}

// Product returns the product for the given product_key.
func (c *Catalog) Product(productKey string) utils.Option[Product] {
	c.ensureIndex()
	return utils.FromPtr(c.byID[productKey])
}

// ProductByAlias returns the product matching the given os_name (or any alias).
func (c *Catalog) ProductByAlias(osName string) utils.Option[Product] {
	c.ensureIndex()
	return utils.FromPtr(c.byAlias[normalizeAlias(osName)])
}

func (c *Catalog) ensureIndex() {
	c.once.Do(func() {
		if c.byID != nil && c.byAlias != nil {
			return
		}
		_ = c.validate()
	})
}

// Release returns the release for the given cycle key (e.g. "20.04" for Ubuntu, "12" for Debian).
func (p *Product) Release(cycle string) utils.Option[Release] {
	r, ok := p.byCycle[normalizeCycle(cycle)]
	if !ok {
		return utils.None[Release]()
	}
	return utils.Some(r)
}

// matchRelease resolves the catalog release for a host-reported version.
// It first tries the raw version string (covers exact cycle keys like "20.04" and "12"),
// then falls back to the major-version key (so "9.2" resolves to the "9" cycle for RPM distros
// and an empty os_version with os_major=12 resolves to Debian "12").
func (p *Product) matchRelease(osVersion string, osMajor int32) utils.Option[Release] {
	v := strings.TrimSpace(osVersion)
	if v != "" {
		if r := p.Release(v); r.IsPresent() {
			return r
		}
		// Fall back to the major-version portion of a point release
		// such as "9.2" or "20.04.1".
		if idx := strings.IndexByte(v, '.'); idx > 0 {
			majorPart := v[:idx]
			if r := p.Release(majorPart); r.IsPresent() {
				return r
			}
		}
	}
	if osMajor > 0 {
		if r := p.Release(strconv.Itoa(int(osMajor))); r.IsPresent() {
			return r
		}
	}
	return utils.None[Release]()
}

type StatusInput struct {
	OSName    string // friendly os_name as reported by the host, ex: Ubuntu
	OSVersion string // os_version as reported by the host (e.g. "20.04", "12", "9.2")
	OSMajor   int32  // os_major as reported by the host (used when OSVersion is empty)
}

func NewStatusInput(osName, osVersion string, osMajor int32) StatusInput {
	return StatusInput{
		OSName:    osName,
		OSVersion: osVersion,
		OSMajor:   osMajor,
	}
}

type Status struct {
	State string `json:"state"`
	// Product is the matched product, or None when State is StatusUnknown.
	Product utils.Option[Product] `json:"-"`
	// Release is the matched release, or None when State is StatusUnknown.
	Release utils.Option[Release] `json:"-"`
	// ProductKey echoes the matched product_key, or "" when unmatched.
	ProductKey string `json:"product_key,omitempty"`
	// ProductDisplayName is the friendly name of the matched product, or "" when unmatched.
	ProductDisplayName string `json:"product_display_name,omitempty"`
	// Cycle is the matched release cycle (e.g. "20.04"), or "" when unmatched.
	Cycle string `json:"cycle,omitempty"`
	// StandardSupportEnd is the standard-support end date of the matched release in YYYY-MM-DD form
	// or "" when unmatched.
	StandardSupportEnd string `json:"standard_support_end,omitempty"`
	// EOL is the final end-of-life date of the matched release in YYYY-MM-DD form
	// or "" when the catalog does not record one.
	EOL string `json:"eol,omitempty"`
	// ExtendedCoverageEnd is the extended coverage end date in YYYY-MM-DD form
	// or "" when no extended coverage is recorded.
	ExtendedCoverageEnd string `json:"extended_coverage_end,omitempty"`
	// ExtendedCoverageNote is the catalog's note about extended coverage
	// availability, or "" when none is recorded. PatchBase does not
	// verify whether the host is actually subscribed.
	ExtendedCoverageNote string `json:"extended_coverage_note,omitempty"`
	// SourceURL is the source URL for the matched release entry, or "" when unmatched.
	SourceURL string `json:"source_url,omitempty"`
	// DaysRemaining is the number of days until standard support ends, rounded down.
	// It is negative when standard support has already ended.
	// It is 0 when the end date is today. It is None when the host is unmatched.
	DaysRemaining utils.Option[int] `json:"days_remaining"`
}

// Status computes the lifecycle status for a single host.
// The approaching threshold controls when a release is flagged as StatusApproachingEndOfSupport
// pass zero to use [DefaultApproachingThreshold].
//
// Status is a pure, deterministic function of the catalog and the input: the only external input is the supplied [time.Time] (now).
// Callers should pass the request time so that tests are reproducible.
func (c *Catalog) Status(now time.Time, approaching time.Duration, in StatusInput) Status {
	if approaching <= 0 {
		approaching = DefaultApproachingThreshold
	}

	product, ok := c.ProductByAlias(in.OSName).Get()
	if !ok {
		return Status{State: StatusUnknown} // nolint: exhaustruct
	}

	release, ok := product.matchRelease(in.OSVersion, in.OSMajor).Get()
	if !ok {
		return Status{State: StatusUnknown} // nolint: exhaustruct
	}

	standardEnd, err := parseDate(release.StandardSupportEnd)
	if err != nil {
		// validate() should have caught this at load time; treat as
		// unknown rather than panicking.
		return Status{State: StatusUnknown} // nolint: exhaustruct
	}

	today := truncateToDay(now.UTC())
	endDay := truncateToDay(standardEnd)
	daysRemaining := int(endDay.Sub(today).Hours() / 24)

	state := StatusSupported
	switch {
	case today.Before(endDay) || today.Equal(endDay):
		if endDay.Sub(today) <= approaching {
			state = StatusApproachingEndOfSupport
		}
	default:
		// Standard support has ended.
		// If extended coverage is recorded and still in the future, surface that the availability is known
		if release.ExtendedCoverageEnd != "" {
			if extEnd, extErr := parseDate(release.ExtendedCoverageEnd); extErr == nil && today.Before(truncateToDay(extEnd)) {
				state = StatusExtendedCoverageAvailable
			} else {
				state = StatusStandardSupportEnded
			}
		} else {
			state = StatusStandardSupportEnded
		}
	}

	return Status{
		State:                state,
		Product:              utils.Some(product),
		Release:              utils.Some(release),
		ProductKey:           product.ProductKey,
		ProductDisplayName:   product.DisplayName,
		Cycle:                release.Cycle,
		StandardSupportEnd:   release.StandardSupportEnd,
		EOL:                  release.EOL,
		ExtendedCoverageEnd:  release.ExtendedCoverageEnd,
		ExtendedCoverageNote: release.ExtendedCoverageNote,
		SourceURL:            release.SourceURL,
		DaysRemaining:        utils.Some(daysRemaining),
	}
}

// CatalogSource describes provenance metadata for the catalog as a whole.
// It is exposed via the API so operators can see where the lifecycle data
// came from and when it was last refreshed (i.e. when the embedded catalog
// was generated).
type CatalogSource struct {
	GeneratedAt string   `json:"generated_at"`
	SourceURLs  []string `json:"source_urls"`
}

// Source returns provenance metadata for the catalog.
func (c *Catalog) Source() CatalogSource {
	urls := make([]string, len(c.SourceURLs))
	copy(urls, c.SourceURLs)
	return CatalogSource{
		GeneratedAt: c.GeneratedAt,
		SourceURLs:  urls,
	}
}

// normalizeAlias lowercases and trims an alias for case-insensitive lookup.
func normalizeAlias(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}

// normalizeCycle strips surrounding whitespace from a cycle identifier so
// that "9", " 9 " resolve to the same release.
func normalizeCycle(cycle string) string {
	return strings.TrimSpace(cycle)
}

// parseDate parses a YYYY-MM-DD date in UTC. The catalog only stores dates,
// not instants, so we discard the time component.
func parseDate(value string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("parse date %q: %w", value, err)
	}
	return t.UTC(), nil
}

// truncateToDay returns t with the hour, minute, second and nanosecond
// components zeroed out, in UTC.
func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
