package cli

import (
	"strings"
	"unicode"
)

// parseRepoFlag flattens the values of a repeated --repo flag into repository
// names, preserving the order they were first given and dropping duplicates.
//
// Every plausible way of writing a list is accepted — repeated flags
// (--repo api --repo web), commas (--repo api,web), and a quoted run of names
// (--repo "api web") — because Cobra hands back the same slice either way and
// guessing wrong is a confusing error rather than a useful one.
func parseRepoFlag(values []string) []string {
	var (
		names []string
		seen  = map[string]bool{}
	)
	for _, value := range values {
		for _, name := range strings.FieldsFunc(value, isNameSeparator) {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names
}

func isNameSeparator(r rune) bool { return r == ',' || unicode.IsSpace(r) }
