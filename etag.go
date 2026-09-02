package scim

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Version returns a strong quoted SHA-256 entity tag for one canonical resource
// representation. Callers must include every state field whose change should
// invalidate conditional requests.
func Version(canonical []byte) (string, error) {
	if len(canonical) == 0 || len(canonical) > 1<<20 {
		return "", fmt.Errorf("canonical resource state has an invalid size")
	}
	digest := sha256.Sum256(canonical)
	return `"` + hex.EncodeToString(digest[:]) + `"`, nil
}

type entityTag struct {
	opaque string
	weak   bool
}

type entityTags struct {
	star bool
	tags []entityTag
}

func parseEntityTags(raw string) (entityTags, error) {
	value := strings.Trim(raw, " \t")
	if value == "*" {
		return entityTags{star: true}, nil
	}
	if value == "" {
		return entityTags{}, fmt.Errorf("entity-tag list is empty")
	}
	var result entityTags
	for position := 0; position < len(value); {
		for position < len(value) && (value[position] == ' ' || value[position] == '\t' || value[position] == ',') {
			position++
		}
		if position == len(value) {
			break
		}
		weak := strings.HasPrefix(value[position:], "W/")
		if weak {
			position += 2
		}
		if position >= len(value) || value[position] != '"' {
			return entityTags{}, fmt.Errorf("entity-tag is malformed")
		}
		position++
		start := position
		for position < len(value) && value[position] != '"' {
			character := value[position]
			if character != 0x21 && (character < 0x23 || character > 0x7e) && character < 0x80 {
				return entityTags{}, fmt.Errorf("entity-tag contains an invalid character")
			}
			position++
		}
		if position == len(value) {
			return entityTags{}, fmt.Errorf("entity-tag is unterminated")
		}
		result.tags = append(result.tags, entityTag{opaque: value[start:position], weak: weak})
		position++
		for position < len(value) && (value[position] == ' ' || value[position] == '\t') {
			position++
		}
		if position < len(value) && value[position] != ',' {
			return entityTags{}, fmt.Errorf("entity-tags must be comma separated")
		}
	}
	if len(result.tags) == 0 {
		return entityTags{}, fmt.Errorf("entity-tag list is empty")
	}
	return result, nil
}

// IfMatch reports whether an If-Match value admits mutation of a resource with
// the current strong version. An empty header means no precondition.
func IfMatch(header, current string, exists bool) (bool, error) {
	if header == "" {
		return true, nil
	}
	candidates, err := parseEntityTags(header)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	currentTag, err := parseSingleStrongTag(current)
	if err != nil {
		return false, err
	}
	if candidates.star {
		return true, nil
	}
	for _, candidate := range candidates.tags {
		if !candidate.weak && candidate.opaque == currentTag {
			return true, nil
		}
	}
	return false, nil
}

// IfNoneMatch reports whether a request may proceed. For reads, false maps to
// 304. For writes, false maps to 412. Weak comparison is intentional.
func IfNoneMatch(header, current string, exists bool) (bool, error) {
	if header == "" || !exists {
		return true, nil
	}
	candidates, err := parseEntityTags(header)
	if err != nil {
		return false, err
	}
	currentTag, err := parseSingleStrongTag(current)
	if err != nil {
		return false, err
	}
	if candidates.star {
		return false, nil
	}
	for _, candidate := range candidates.tags {
		if candidate.opaque == currentTag {
			return false, nil
		}
	}
	return true, nil
}

func parseSingleStrongTag(current string) (string, error) {
	parsed, err := parseEntityTags(current)
	if err != nil || parsed.star || len(parsed.tags) != 1 || parsed.tags[0].weak {
		return "", fmt.Errorf("current resource version is not one strong entity-tag")
	}
	return parsed.tags[0].opaque, nil
}
