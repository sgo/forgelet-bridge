package steps

import (
	"fmt"
	"strings"
)

// localpartOf splits the local part out of a Matrix user id.
func localpartOf(userID string) (string, error) {
	trimmed := strings.TrimPrefix(userID, "@")
	localpart, _, found := strings.Cut(trimmed, ":")
	if !found || localpart == "" {
		return "", fmt.Errorf("not a matrix user id: %q", userID)
	}
	return localpart, nil
}

func appendUnique(list []string, value string) []string {
	for _, item := range list {
		if item == value {
			return list
		}
	}
	return append(list, value)
}

// forgeNames parses a Gherkin list like "forge-a, forge-b" or "forge-a".
func forgeNames(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ' '
	})
	var names []string
	for _, field := range fields {
		switch field {
		case "", "and":
			continue
		}
		names = append(names, field)
	}
	return names
}
