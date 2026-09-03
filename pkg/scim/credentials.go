package scim

import (
	"fmt"

	"golang.org/x/text/secure/precis"
)

func extractPassword(document Document, supported bool, resourceType string) ([]byte, error) {
	raw, exists := document["password"]
	delete(document, "password")
	if !exists {
		return nil, nil
	}
	if !supported || resourceType != "User" {
		return nil, clientError(400, "mutability", "password changes are not supported")
	}
	value, ok := raw.(string)
	if !ok || value == "" || len(value) > 1024 {
		return nil, clientError(400, "invalidValue", "password is invalid")
	}
	prepared, err := precis.OpaqueString.String(value)
	if err != nil || prepared == "" || len(prepared) > 1024 {
		return nil, clientError(400, "invalidValue", "password fails international string preparation")
	}
	return []byte(prepared), nil
}

func clearSecret(secret []byte) {
	for index := range secret {
		secret[index] = 0
	}
}

func writePassword(transaction Transaction, scope, resourceType, id string, secret []byte) (string, error) {
	if len(secret) == 0 {
		return "", nil
	}
	writer, ok := transaction.(PasswordTransaction)
	if !ok {
		return "", fmt.Errorf("store does not implement advertised password capability")
	}
	copy := append([]byte(nil), secret...)
	defer clearSecret(copy)
	revision, err := writer.SetPassword(scope, resourceType, id, copy)
	if err != nil {
		return "", err
	}
	if revision == "" || !validString(revision, 1024) {
		return "", fmt.Errorf("password adapter returned an invalid revision")
	}
	return revision, nil
}
