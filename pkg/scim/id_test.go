package scim

import (
	"bytes"
	"io"
	"testing"
)

func TestNewResourceID(t *testing.T) {
	id, err := NewResourceID(bytes.NewReader([]byte("0123456789abcdef")))
	if err != nil || id != "MDEyMzQ1Njc4OWFiY2RlZg" {
		t.Fatalf("NewResourceID() = (%q, %v)", id, err)
	}
	for _, reader := range []io.Reader{nil, bytes.NewReader([]byte("short"))} {
		if got, err := NewResourceID(reader); err == nil || got != "" {
			t.Errorf("NewResourceID(invalid) = (%q, %v)", got, err)
		}
	}
}
