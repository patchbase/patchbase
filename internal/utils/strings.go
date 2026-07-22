package utils

import "strings"

func EmptySpaceString(s string) Option[string] {
	if strings.TrimSpace(s) == "" {
		return None[string]()
	}
	return Some(s)
}
