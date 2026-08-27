// Package slug turns display names into URL-friendly identifiers.
package slug

import (
	"regexp"
	"strings"
)

var (
	nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)
	trimHyphens     = regexp.MustCompile(`^-+|-+$`)
)

// Generate turns s into a lowercase, hyphen-separated slug, e.g.
// "My Konos Monaco Royal" → "my-konos-monaco-royal".
//
// It does NOT guarantee uniqueness within a table — two different names
// can collide (or the same name can be re-submitted). Callers that need
// a unique column value pair this with a database check-and-suffix loop;
// see products.Repository.uniqueSlug for that half.
func Generate(s string) string {
	lower := strings.ToLower(s)
	result := nonAlphanumeric.ReplaceAllString(lower, "-")
	result = trimHyphens.ReplaceAllString(result, "")
	if result == "" {
		return "item"
	}
	return result
}
