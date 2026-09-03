package scim

import (
	"errors"
	"reflect"
	"testing"
)

func TestRegistryAndResourceValidation(t *testing.T) {
	definitions := DefaultDefinitions()
	definitions[0].Extensions = []Extension{{Schema: "urn:example:employee", Name: "Employee", Required: true, Validate: func(document Document) error {
		_, err := requiredString(document, "employeeNumber", 32)
		return err
	}}}
	registry, err := NewRegistry(definitions)
	if err != nil {
		t.Fatal(err)
	}
	user, _ := registry.definitionByName("User")
	document, err := DecodeDocument([]byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User","urn:example:employee"],"userName":"member","externalId":"upstream-1","urn:example:employee":{"employeeNumber":"42"},"meta":{"ignored":true},"groups":[{"value":"ignored"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	prepared, indexes, externalID, err := prepareResource(user, document, CreateMode, "")
	if err != nil {
		t.Fatal(err)
	}
	if externalID != "upstream-1" || len(indexes) != 2 || prepared["meta"] != nil || prepared["groups"] != nil {
		t.Fatalf("prepared resource = %#v, %#v, %q", prepared, indexes, externalID)
	}
	invalid := []string{
		`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"],"userName":"member"}`,
		`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User","urn:example:employee"],"userName":"member","password":"secret","urn:example:employee":{"employeeNumber":"42"}}`,
		`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User","urn:example:employee"],"id":"client","userName":"member","urn:example:employee":{"employeeNumber":"42"}}`,
		`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User","urn:example:employee"],"userName":"member","unknown":true,"urn:example:employee":{"employeeNumber":"42"}}`,
		`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User","urn:example:employee"],"userName":"member","urn:ietf:params:scim:schemas:core:2.0:User":{"smuggled":true},"urn:example:employee":{"employeeNumber":"42"}}`,
	}
	for index, raw := range invalid {
		document, decodeErr := DecodeDocument([]byte(raw))
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if _, _, _, err := prepareResource(user, document, CreateMode, ""); err == nil {
			t.Errorf("invalid resource %d passed", index)
		}
	}
	if _, err := NewRegistry([]ResourceDefinition{{Name: "User", Endpoint: "Users", Schema: UserSchema}, {Name: "Other", Endpoint: "Users", Schema: "urn:other"}}); err == nil {
		t.Fatal("duplicate endpoint passed")
	}
}

func TestSchemaURIsCanonicalize(t *testing.T) {
	definition := DefaultDefinitions()[0]
	definition.Extensions = []Extension{{Schema: "urn:Example:Extension", Validate: func(Document) error { return nil }}}
	document, _ := DecodeDocument([]byte(`{"schemas":["URN:IETF:PARAMS:SCIM:SCHEMAS:CORE:2.0:USER","URN:EXAMPLE:EXTENSION"],"userName":"member","URN:EXAMPLE:EXTENSION":{}}`))
	prepared, _, _, err := prepareResource(definition, document, CreateMode, "")
	if err != nil {
		t.Fatal(err)
	}
	schemas := prepared["schemas"].([]any)
	if schemas[0] != UserSchema || schemas[1] != "urn:Example:Extension" {
		t.Fatalf("canonical schemas = %#v", schemas)
	}
}

func TestSchemaURIValidation(t *testing.T) {
	for _, value := range []string{"", "relative", "urn:bad#fragment", "urn:bad\nvalue"} {
		if validSchemaURI(value) {
			t.Errorf("invalid schema URI %q passed", value)
		}
	}
	if !validSchemaURI("urn:example:schema") || !validSchemaURI("https://example.test/schema") {
		t.Fatal("valid schema URI failed")
	}
}

func TestAttributeNamesUseSCIMASCII(t *testing.T) {
	if validName("Üser") || validName("1user") || !validName("user_name-2") || !validSchemaAttributeName("$ref") {
		t.Fatal("SCIM attribute-name ABNF is not enforced")
	}
}

func TestGroupAndHelpers(t *testing.T) {
	registry, err := NewRegistry(DefaultDefinitions())
	if err != nil {
		t.Fatal(err)
	}
	group, _ := registry.definitionByName("Group")
	document, _ := DecodeDocument([]byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"Operators","members":[{"value":"user-1","type":"User"}]}`))
	prepared, indexes, _, err := prepareResource(group, document, CreateMode, "")
	if err != nil || len(indexes) != 1 || !reflect.DeepEqual(prepared, document) {
		t.Fatalf("group validation = (%#v, %#v, %v)", prepared, indexes, err)
	}
	bad, _ := DecodeDocument([]byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:Group"],"displayName":"Operators","members":[{"display":"missing id"}]}`))
	if _, _, _, err := prepareResource(group, bad, CreateMode, ""); err == nil {
		t.Fatal("member without value passed")
	}
	if _, err := validateExternalURL("http://example.test/scim/v2"); err == nil {
		t.Fatal("plaintext external URL passed")
	}
}

func TestSchemaSetAndGroupFailures(t *testing.T) {
	extensionFailure := func(Document) error { return errors.New("rejected") }
	definition := DefaultDefinitions()[0]
	definition.Extensions = []Extension{{Schema: "urn:required", Required: true, Validate: extensionFailure}}
	invalid := []string{
		`{"schemas":"wrong","userName":"member"}`,
		`{"schemas":[1],"userName":"member"}`,
		`{"schemas":["` + UserSchema + `","` + UserSchema + `"],"userName":"member"}`,
		`{"schemas":["` + UserSchema + `","urn:unknown"],"userName":"member"}`,
		`{"schemas":["` + UserSchema + `","urn:required"],"userName":"member"}`,
		`{"schemas":["` + UserSchema + `","urn:required"],"userName":"member","urn:required":"wrong"}`,
		`{"schemas":["` + UserSchema + `","urn:required"],"userName":"member","urn:required":{}}`,
	}
	for index, raw := range invalid {
		document, err := DecodeDocument([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := prepareResource(definition, document, CreateMode, ""); err == nil {
			t.Errorf("invalid schema case %d passed", index)
		}
	}
	group := DefaultDefinitions()[1]
	for index, raw := range []string{
		`{"schemas":["` + GroupSchema + `"]}`,
		`{"schemas":["` + GroupSchema + `"],"displayName":true}`,
		`{"schemas":["` + GroupSchema + `"],"displayName":" bad "}`,
		`{"schemas":["` + GroupSchema + `"],"displayName":"good","members":"bad"}`,
		`{"schemas":["` + GroupSchema + `"],"displayName":"good","members":[{"value":true}]}`,
	} {
		document, err := DecodeDocument([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := prepareResource(group, document, CreateMode, ""); err == nil {
			t.Errorf("invalid group %d passed", index)
		}
	}
}
