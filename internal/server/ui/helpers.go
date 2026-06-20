package ui

import (
	"encoding/json"
	"strings"

	"github.com/nebari-dev/nebari-catalog-pack/internal/registry"
)

// asset joins the base path with a relative asset/route path.
func asset(basePath, p string) string {
	return strings.TrimRight(basePath, "/") + p
}

// initials returns up to two uppercase initials for a fallback icon.
func initials(title string) string {
	fields := strings.Fields(strings.ReplaceAll(title, "-", " "))
	var b strings.Builder
	for _, f := range fields {
		if f == "" {
			continue
		}
		b.WriteString(strings.ToUpper(f[:1]))
		if b.Len() >= 2 {
			break
		}
	}
	if b.Len() == 0 {
		return "?"
	}
	return b.String()
}

// installVals renders the hx-vals JSON for a pack install button.
func installVals(p registry.Pack) string {
	b, _ := json.Marshal(map[string]string{
		"pack":    p.Name,
		"version": p.Latest,
	})
	return string(b)
}

// short truncates a commit hash for display.
func short(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}
