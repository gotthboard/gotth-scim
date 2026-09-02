package scim

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ParseEqualityFilter accepts the deliberately bounded interoperable subset
// `<attribute> eq "<value>"` for one caller-selected attribute.
func ParseEqualityFilter(raw, supportedAttribute string) (string, error) {
	if len(raw) == 0 || len(raw) > 4096 || !utf8.ValidString(raw) || supportedAttribute == "" {
		return "", fmt.Errorf("SCIM filter boundary is invalid")
	}
	trimmed := strings.TrimSpace(raw)
	firstSpace := strings.IndexAny(trimmed, " \t")
	if firstSpace < 1 {
		return "", fmt.Errorf("only %s eq filters are supported", supportedAttribute)
	}
	attribute := trimmed[:firstSpace]
	remainder := strings.TrimLeft(trimmed[firstSpace:], " \t")
	secondSpace := strings.IndexAny(remainder, " \t")
	if secondSpace < 1 || !strings.EqualFold(attribute, supportedAttribute) || !strings.EqualFold(remainder[:secondSpace], "eq") {
		return "", fmt.Errorf("only %s eq filters are supported", supportedAttribute)
	}
	quoted := strings.TrimSpace(remainder[secondSpace:])
	if len(quoted) < 2 || quoted[0] != '"' || quoted[len(quoted)-1] != '"' {
		return "", fmt.Errorf("SCIM equality value must be quoted")
	}
	value := quoted[1 : len(quoted)-1]
	if strings.Contains(value, `"`) || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("SCIM equality value is invalid")
	}
	return value, nil
}
