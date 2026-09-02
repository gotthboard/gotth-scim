package scim

import (
	"encoding/base64"
	"fmt"
	"io"
)

const resourceIDBytes = 16

// NewResourceID returns one opaque 128-bit base64url identifier. The caller
// must persist it; regenerating IDs from mutable names violates SCIM identity.
func NewResourceID(entropy io.Reader) (string, error) {
	if entropy == nil {
		return "", fmt.Errorf("resource ID entropy is required")
	}
	var raw [resourceIDBytes]byte
	defer clear(raw[:])
	if _, err := io.ReadFull(entropy, raw[:]); err != nil {
		return "", fmt.Errorf("read resource ID entropy: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
