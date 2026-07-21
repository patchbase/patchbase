// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
package webutil

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func ClientIP(r *http.Request) string {
	trusted := isRemoteAddrLocal(r)
	if xff := r.Header.Get("X-Forwarded-For"); trusted && xff != "" {
		if before, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(before)
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); trusted && xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isRemoteAddrLocal(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	return IsLocalAddr(host)
}

func IsLocalAddr(s string) bool {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return false
	}
	return addr.IsLoopback() || addr.IsPrivate()
}
