package scim

import "fmt"

// ProtocolError is a safe client-facing SCIM failure. Detail must not contain
// implementation errors, credentials, or storage diagnostics.
type ProtocolError struct {
	Status   int
	SCIMType string
	Detail   string
}

func (failure *ProtocolError) Error() string {
	if failure == nil {
		return "<nil>"
	}
	return fmt.Sprintf("SCIM %d %s: %s", failure.Status, failure.SCIMType, failure.Detail)
}

func clientError(status int, scimType, detail string) error {
	return &ProtocolError{Status: status, SCIMType: scimType, Detail: detail}
}
