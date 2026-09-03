package scim

import (
	"encoding/base64"
	"fmt"
	"net/url"
	pathpkg "path"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/secure/precis"
)

const (
	maximumStringBytes = 64 << 10
	maximumValues      = 1000
)

// WriteMode distinguishes create from replacement validation.
type WriteMode uint8

const (
	CreateMode WriteMode = iota + 1
	ReplaceMode
)

// Extension describes one admitted schema extension. Validate receives the
// extension object only and may enforce consumer-specific wire constraints.
type Extension struct {
	Schema      string
	Name        string
	Description string
	Attributes  []SchemaAttribute
	Required    bool
	Validate    func(Document) error
}

// SchemaAttribute is the discovery representation of one supported schema
// attribute. SubAttributes applies only to complex values.
type SchemaAttribute struct {
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	MultiValued     bool              `json:"multiValued"`
	Description     string            `json:"description,omitempty"`
	Required        bool              `json:"required"`
	CanonicalValues []string          `json:"canonicalValues,omitempty"`
	CaseExact       bool              `json:"caseExact"`
	Mutability      string            `json:"mutability"`
	Returned        string            `json:"returned"`
	Uniqueness      string            `json:"uniqueness"`
	ReferenceTypes  []string          `json:"referenceTypes,omitempty"`
	SubAttributes   []SchemaAttribute `json:"subAttributes,omitempty"`
}

// ResourceDefinition describes one SCIM collection and its storage lookup
// surface. Name, endpoint, and schema are immutable after registry creation.
type ResourceDefinition struct {
	Name             string
	Endpoint         string
	Schema           string
	Description      string
	Attributes       []SchemaAttribute
	UniqueAttribute  string
	FilterAttributes []string
	Extensions       []Extension
	Validate         func(Document, WriteMode) error
}

// Registry is an immutable set of resource definitions.
type Registry struct {
	byEndpoint map[string]ResourceDefinition
	byName     map[string]ResourceDefinition
	ordered    []ResourceDefinition
}

// DefaultDefinitions returns standard User and Group definitions.
func DefaultDefinitions() []ResourceDefinition {
	return []ResourceDefinition{
		{Name: "User", Endpoint: "Users", Schema: UserSchema, Description: "User Account", Attributes: standardUserAttributes(), UniqueAttribute: "userName", FilterAttributes: []string{"userName", "externalId"}, Extensions: []Extension{EnterpriseUserExtension()}},
		{Name: "Group", Endpoint: "Groups", Schema: GroupSchema, Description: "Group", Attributes: standardGroupAttributes(), UniqueAttribute: "displayName", FilterAttributes: []string{"displayName", "externalId"}},
	}
}

func standardUserAttributes() []SchemaAttribute {
	stringAttribute := func(name string) SchemaAttribute {
		return SchemaAttribute{Name: name, Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "none"}
	}
	readOnlyDisplay := SchemaAttribute{Name: "display", Type: "string", Mutability: "readOnly", Returned: "default", Uniqueness: "none"}
	primary := SchemaAttribute{Name: "primary", Type: "boolean", Mutability: "readWrite", Returned: "default", Uniqueness: "none"}
	typeAttribute := func(values ...string) SchemaAttribute {
		result := stringAttribute("type")
		result.CanonicalValues = values
		return result
	}
	multiAttribute := func(name string, value SchemaAttribute, kind SchemaAttribute, extras ...SchemaAttribute) SchemaAttribute {
		attributes := []SchemaAttribute{value, readOnlyDisplay, kind, primary}
		attributes = append(attributes, extras...)
		return SchemaAttribute{Name: name, Type: "complex", MultiValued: true, Mutability: "readWrite", Returned: "default", Uniqueness: "none", SubAttributes: attributes}
	}
	value := stringAttribute("value")
	return []SchemaAttribute{
		{Name: "userName", Type: "string", Required: true, Mutability: "readWrite", Returned: "default", Uniqueness: "server"},
		{Name: "name", Type: "complex", Mutability: "readWrite", Returned: "default", Uniqueness: "none", SubAttributes: []SchemaAttribute{stringAttribute("formatted"), stringAttribute("familyName"), stringAttribute("givenName"), stringAttribute("middleName"), stringAttribute("honorificPrefix"), stringAttribute("honorificSuffix")}},
		stringAttribute("displayName"), stringAttribute("nickName"),
		{Name: "profileUrl", Type: "reference", ReferenceTypes: []string{"external"}, Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
		stringAttribute("title"), stringAttribute("userType"), stringAttribute("preferredLanguage"), stringAttribute("locale"), stringAttribute("timezone"),
		{Name: "active", Type: "boolean", Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
		{Name: "password", Type: "string", Mutability: "writeOnly", Returned: "never", Uniqueness: "none"},
		multiAttribute("emails", value, typeAttribute("work", "home", "other")),
		multiAttribute("phoneNumbers", value, typeAttribute("work", "home", "mobile", "fax", "pager", "other")),
		multiAttribute("ims", value, typeAttribute("aim", "gtalk", "icq", "xmpp", "msn", "skype", "qq", "yahoo")),
		multiAttribute("photos", SchemaAttribute{Name: "value", Type: "reference", ReferenceTypes: []string{"external"}, Mutability: "readWrite", Returned: "default", Uniqueness: "none"}, typeAttribute("photo", "thumbnail")),
		{Name: "addresses", Type: "complex", MultiValued: true, Mutability: "readWrite", Returned: "default", Uniqueness: "none", SubAttributes: []SchemaAttribute{stringAttribute("formatted"), stringAttribute("streetAddress"), stringAttribute("locality"), stringAttribute("region"), stringAttribute("postalCode"), stringAttribute("country"), typeAttribute("work", "home", "other"), primary}},
		multiAttribute("entitlements", value, typeAttribute()),
		multiAttribute("roles", value, typeAttribute()),
		multiAttribute("x509Certificates", SchemaAttribute{Name: "value", Type: "binary", Mutability: "readWrite", Returned: "default", Uniqueness: "none"}, typeAttribute()),
		{Name: "groups", Type: "complex", MultiValued: true, Mutability: "readOnly", Returned: "default", Uniqueness: "none", SubAttributes: []SchemaAttribute{
			{Name: "value", Type: "string", Mutability: "readOnly", Returned: "default", Uniqueness: "none"},
			{Name: "$ref", Type: "reference", Mutability: "readOnly", Returned: "default", Uniqueness: "none", ReferenceTypes: []string{"User", "Group"}},
			{Name: "display", Type: "string", Mutability: "readOnly", Returned: "default", Uniqueness: "none"},
			{Name: "type", Type: "string", Mutability: "readOnly", Returned: "default", Uniqueness: "none", CanonicalValues: []string{"direct", "indirect"}},
		}},
	}
}

func standardGroupAttributes() []SchemaAttribute {
	return []SchemaAttribute{
		{Name: "displayName", Type: "string", Required: true, Mutability: "readWrite", Returned: "default", Uniqueness: "server"},
		{Name: "members", Type: "complex", MultiValued: true, Mutability: "readWrite", Returned: "default", Uniqueness: "none", SubAttributes: []SchemaAttribute{
			{Name: "value", Type: "string", Required: true, Mutability: "immutable", Returned: "default", Uniqueness: "none"},
			{Name: "$ref", Type: "reference", Mutability: "immutable", Returned: "default", Uniqueness: "none", ReferenceTypes: []string{"User", "Group"}},
			{Name: "type", Type: "string", Mutability: "immutable", Returned: "default", Uniqueness: "none", CanonicalValues: []string{"User", "Group"}},
			{Name: "display", Type: "string", Mutability: "readOnly", Returned: "default", Uniqueness: "none"},
		}},
	}
}

// EnterpriseUserExtension returns the optional standard RFC 7643 enterprise
// User extension. It is included, but not required, by DefaultDefinitions.
func EnterpriseUserExtension() Extension {
	stringAttribute := func(name string) SchemaAttribute {
		return SchemaAttribute{Name: name, Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "none"}
	}
	return Extension{
		Schema: EnterpriseUserSchema, Name: "EnterpriseUser", Description: "Enterprise User",
		Attributes: []SchemaAttribute{
			stringAttribute("employeeNumber"), stringAttribute("costCenter"), stringAttribute("organization"), stringAttribute("division"), stringAttribute("department"),
			{Name: "manager", Type: "complex", Mutability: "readWrite", Returned: "default", Uniqueness: "none", SubAttributes: []SchemaAttribute{
				{Name: "value", Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "none"},
				{Name: "$ref", Type: "reference", Mutability: "readWrite", Returned: "default", Uniqueness: "none", ReferenceTypes: []string{"User"}},
				{Name: "displayName", Type: "string", Mutability: "readOnly", Returned: "default", Uniqueness: "none"},
			}},
		},
		Validate: validateEnterpriseUser,
	}
}

// NewRegistry validates and freezes resource definitions.
func NewRegistry(definitions []ResourceDefinition) (*Registry, error) {
	if len(definitions) == 0 || len(definitions) > 32 {
		return nil, fmt.Errorf("resource definition count is invalid")
	}
	registry := &Registry{byEndpoint: make(map[string]ResourceDefinition), byName: make(map[string]ResourceDefinition)}
	for _, definition := range definitions {
		if !validName(definition.Name) || !validName(definition.Endpoint) || !validSchemaURI(definition.Schema) {
			return nil, fmt.Errorf("resource definition identity is invalid")
		}
		if _, exists := registry.byEndpoint[definition.Endpoint]; exists {
			return nil, fmt.Errorf("resource endpoint %q is duplicated", definition.Endpoint)
		}
		if _, exists := registry.byName[definition.Name]; exists {
			return nil, fmt.Errorf("resource name %q is duplicated", definition.Name)
		}
		if definition.Name != "User" && definition.Name != "Group" && definition.Validate == nil {
			return nil, fmt.Errorf("custom resource definition requires validation")
		}
		if !validString(definition.Description, maximumStringBytes) || validateSchemaAttributes(definition.Attributes, 0) != nil {
			return nil, fmt.Errorf("resource discovery metadata is invalid")
		}
		if containsWriteOnly(definition.Attributes, definition.Name == "User") {
			return nil, fmt.Errorf("unsupported write-only resource attribute")
		}
		seenFilters := make(map[string]struct{})
		for _, attribute := range definition.FilterAttributes {
			folded := strings.ToLower(attribute)
			if !validAttributeName(attribute) || folded == "" {
				return nil, fmt.Errorf("filter attribute is invalid")
			}
			if _, exists := seenFilters[folded]; exists {
				return nil, fmt.Errorf("filter attribute is duplicated")
			}
			seenFilters[folded] = struct{}{}
		}
		if definition.UniqueAttribute != "" {
			if !validAttributeName(definition.UniqueAttribute) {
				return nil, fmt.Errorf("unique attribute is invalid")
			}
			if _, exists := seenFilters[strings.ToLower(definition.UniqueAttribute)]; !exists {
				return nil, fmt.Errorf("unique attribute must be a filter attribute")
			}
		}
		seenExtensions := make(map[string]struct{})
		for _, extension := range definition.Extensions {
			folded := strings.ToLower(extension.Schema)
			if !validSchemaURI(extension.Schema) || strings.EqualFold(extension.Schema, definition.Schema) {
				return nil, fmt.Errorf("extension schema is invalid")
			}
			if _, exists := seenExtensions[folded]; exists {
				return nil, fmt.Errorf("extension schema is duplicated")
			}
			if extension.Name != "" && !validName(extension.Name) || !validString(extension.Description, maximumStringBytes) || validateSchemaAttributes(extension.Attributes, 0) != nil {
				return nil, fmt.Errorf("extension discovery metadata is invalid")
			}
			if containsWriteOnly(extension.Attributes, false) {
				return nil, fmt.Errorf("write-only extension attributes require a consumer adapter")
			}
			seenExtensions[folded] = struct{}{}
		}
		for _, name := range definition.FilterAttributes {
			_, attribute, exists := resolveAttributeContract(definition, nil, name)
			if !exists || attribute.MultiValued || attribute.Type != "string" && attribute.Type != "reference" {
				return nil, fmt.Errorf("storage index attribute is not a singular string")
			}
		}
		definition.FilterAttributes = append([]string(nil), definition.FilterAttributes...)
		definition.Attributes = cloneSchemaAttributes(definition.Attributes)
		definition.Extensions = append([]Extension(nil), definition.Extensions...)
		for index := range definition.Extensions {
			definition.Extensions[index].Attributes = cloneSchemaAttributes(definition.Extensions[index].Attributes)
		}
		registry.byEndpoint[definition.Endpoint] = definition
		registry.byName[definition.Name] = definition
		registry.ordered = append(registry.ordered, definition)
	}
	sort.Slice(registry.ordered, func(i, j int) bool { return registry.ordered[i].Name < registry.ordered[j].Name })
	return registry, nil
}

func containsWriteOnly(attributes []SchemaAttribute, permitPassword bool) bool {
	for _, attribute := range attributes {
		if attribute.Mutability == "writeOnly" && !(permitPassword && attribute.Name == "password") {
			return true
		}
		if containsWriteOnly(attribute.SubAttributes, false) {
			return true
		}
	}
	return false
}

func validSchemaURI(value string) bool {
	if !validString(value, 1024) || value == "" {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.IsAbs() && parsed.Fragment == ""
}

func validateSchemaAttributes(attributes []SchemaAttribute, depth int) error {
	if depth > 4 || len(attributes) > 256 {
		return fmt.Errorf("schema attribute boundary is invalid")
	}
	seen := make(map[string]struct{}, len(attributes))
	for _, attribute := range attributes {
		folded := strings.ToLower(attribute.Name)
		if !validSchemaAttributeName(attribute.Name) {
			return fmt.Errorf("schema attribute name is invalid")
		}
		if _, exists := seen[folded]; exists {
			return fmt.Errorf("schema attribute is duplicated")
		}
		seen[folded] = struct{}{}
		if !oneOf(attribute.Type, "binary", "boolean", "complex", "dateTime", "decimal", "integer", "reference", "string") || !oneOf(attribute.Mutability, "immutable", "readOnly", "readWrite", "writeOnly") || !oneOf(attribute.Returned, "always", "default", "never", "request") || !oneOf(attribute.Uniqueness, "global", "none", "server") || !validString(attribute.Description, maximumStringBytes) || !validStringList(attribute.CanonicalValues, 256) || !validStringList(attribute.ReferenceTypes, 64) {
			return fmt.Errorf("schema attribute contract is invalid")
		}
		if attribute.Type != "complex" && len(attribute.SubAttributes) != 0 {
			return fmt.Errorf("non-complex attribute has sub-attributes")
		}
		if attribute.Mutability == "writeOnly" && attribute.Returned != "never" {
			return fmt.Errorf("write-only attribute must never be returned")
		}
		if attribute.Type != "reference" && len(attribute.ReferenceTypes) != 0 {
			return fmt.Errorf("only reference attributes have reference types")
		}
		if err := validateSchemaAttributes(attribute.SubAttributes, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func cloneSchemaAttributes(attributes []SchemaAttribute) []SchemaAttribute {
	result := append([]SchemaAttribute(nil), attributes...)
	for index := range result {
		result[index].CanonicalValues = append([]string(nil), result[index].CanonicalValues...)
		result[index].ReferenceTypes = append([]string(nil), result[index].ReferenceTypes...)
		result[index].SubAttributes = cloneSchemaAttributes(result[index].SubAttributes)
	}
	return result
}

func validStringList(values []string, maximum int) bool {
	if len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" || !validString(value, 1024) {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func oneOf(value string, admitted ...string) bool {
	for _, candidate := range admitted {
		if value == candidate {
			return true
		}
	}
	return false
}

func validSchemaAttributeName(value string) bool { return value == "$ref" || validName(value) }

func validName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		letter := character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z'
		digit := character >= '0' && character <= '9'
		if !letter && !(index > 0 && (digit || character == '-' || character == '_')) {
			return false
		}
	}
	return true
}

func validAttributeName(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return false
	}
	for _, part := range parts {
		if !validName(part) {
			return false
		}
	}
	return true
}

func (registry *Registry) definitionByEndpoint(endpoint string) (ResourceDefinition, bool) {
	if registry == nil {
		return ResourceDefinition{}, false
	}
	definition, exists := registry.byEndpoint[endpoint]
	return definition, exists
}

func (registry *Registry) definitionByName(name string) (ResourceDefinition, bool) {
	if registry == nil {
		return ResourceDefinition{}, false
	}
	definition, exists := registry.byName[name]
	return definition, exists
}

func (registry *Registry) definitions() []ResourceDefinition {
	if registry == nil {
		return nil
	}
	result := make([]ResourceDefinition, len(registry.ordered))
	for index, definition := range registry.ordered {
		result[index] = cloneDefinition(definition)
	}
	return result
}

func cloneDefinition(definition ResourceDefinition) ResourceDefinition {
	definition.FilterAttributes = append([]string(nil), definition.FilterAttributes...)
	definition.Attributes = cloneSchemaAttributes(definition.Attributes)
	definition.Extensions = append([]Extension(nil), definition.Extensions...)
	for index := range definition.Extensions {
		definition.Extensions[index].Attributes = cloneSchemaAttributes(definition.Extensions[index].Attributes)
	}
	return definition
}

func prepareResource(definition ResourceDefinition, document Document, mode WriteMode, requestID string) (Document, []IndexKey, string, error) {
	copy, err := cloneDocument(document)
	if err != nil {
		return nil, nil, "", err
	}
	if mode != CreateMode && mode != ReplaceMode {
		return nil, nil, "", fmt.Errorf("write mode is invalid")
	}
	delete(copy, "id")
	delete(copy, "meta")
	if _, exists := copy["password"]; exists {
		return nil, nil, "", fmt.Errorf("password is not supported")
	}
	delete(copy, "groups")
	stripReadOnly(definition, copy)
	removeUnassigned(copy, true)
	normalizePrimaryValues(copy)
	if err := validateSchemaSet(definition, copy); err != nil {
		return nil, nil, "", err
	}
	if definition.Name == "User" {
		if err := validateUser(definition, copy); err != nil {
			return nil, nil, "", err
		}
	} else if definition.Name == "Group" {
		if err := validateGroup(definition, copy); err != nil {
			return nil, nil, "", err
		}
	}
	if definition.Validate != nil {
		if err := definition.Validate(copy, mode); err != nil {
			return nil, nil, "", fmt.Errorf("resource validation failed: %w", err)
		}
	}
	if err := validateAttributeValues(map[string]any(copy), definition.Attributes); err != nil {
		return nil, nil, "", err
	}
	for _, extension := range definition.Extensions {
		if object, ok := copy[extension.Schema].(map[string]any); ok {
			if err := validateAttributeValuesExact(object, extension.Attributes); err != nil {
				return nil, nil, "", fmt.Errorf("extension %q: %w", extension.Schema, err)
			}
		}
	}
	externalID, err := optionalString(copy, "externalId", maximumStringBytes)
	if err != nil {
		return nil, nil, "", err
	}
	indexes := make([]IndexKey, 0, len(definition.FilterAttributes))
	for _, attribute := range definition.FilterAttributes {
		value, present, err := stringPath(copy, attribute)
		if err != nil {
			return nil, nil, "", err
		}
		if present {
			indexes = append(indexes, IndexKey{Name: attribute, Value: value, CaseExact: strings.EqualFold(attribute, "externalId"), Unique: strings.EqualFold(attribute, definition.UniqueAttribute)})
		}
	}
	return copy, indexes, externalID, nil
}

func removeUnassigned(object map[string]any, topLevel bool) {
	for key, value := range object {
		if topLevel && key == "schemas" {
			continue
		}
		switch typed := value.(type) {
		case nil:
			delete(object, key)
		case string:
			if typed == "" {
				delete(object, key)
			}
		case []any:
			if len(typed) == 0 {
				delete(object, key)
				continue
			}
			kept := typed[:0]
			for _, item := range typed {
				if item == nil {
					continue
				}
				if text, ok := item.(string); ok && text == "" {
					continue
				}
				if nested, ok := item.(map[string]any); ok {
					removeUnassigned(nested, false)
				}
				kept = append(kept, item)
			}
			if len(kept) == 0 {
				delete(object, key)
			} else {
				object[key] = kept
			}
		case map[string]any:
			removeUnassigned(typed, false)
		}
	}
}

func validateAttributeValues(object map[string]any, attributes []SchemaAttribute) error {
	for _, attribute := range attributes {
		value, exists := object[attribute.Name]
		if attribute.Required && !exists {
			return fmt.Errorf("required attribute %s is missing", attribute.Name)
		}
		if !exists {
			continue
		}
		values := []any{value}
		if attribute.MultiValued {
			array, ok := value.([]any)
			if !ok {
				return fmt.Errorf("attribute %s must be multi-valued", attribute.Name)
			}
			values = array
		} else if _, array := value.([]any); array {
			return fmt.Errorf("attribute %s must be singular", attribute.Name)
		}
		for _, candidate := range values {
			if err := validateAttributeValue(candidate, attribute); err != nil {
				return fmt.Errorf("attribute %s: %w", attribute.Name, err)
			}
		}
	}
	return nil
}

func validateAttributeValuesExact(object map[string]any, attributes []SchemaAttribute) error {
	admitted := make(map[string]struct{}, len(attributes))
	for _, attribute := range attributes {
		admitted[attribute.Name] = struct{}{}
	}
	for name := range object {
		if _, ok := admitted[name]; !ok {
			return fmt.Errorf("attribute %s is not in the schema", name)
		}
	}
	return validateAttributeValues(object, attributes)
}

func validateAttributeValue(value any, attribute SchemaAttribute) error {
	switch attribute.Type {
	case "string":
		if text, ok := value.(string); !ok || !validString(text, maximumStringBytes) {
			return fmt.Errorf("must be a string")
		}
	case "reference":
		text, ok := value.(string)
		if !ok || !validString(text, maximumStringBytes) {
			return fmt.Errorf("must be a reference")
		}
		parsed, err := url.Parse(text)
		if err != nil || parsed.Scheme == "" && !strings.HasPrefix(text, "/") || parsed.User != nil {
			return fmt.Errorf("reference is invalid")
		}
		for _, kind := range attribute.ReferenceTypes {
			if kind == "external" && (!parsed.IsAbs() || parsed.Scheme != "https" && parsed.Scheme != "http") {
				return fmt.Errorf("external reference is invalid")
			}
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("must be boolean")
		}
	case "integer":
		number, ok := exactFilterNumber(value)
		if !ok || !number.IsInt() {
			return fmt.Errorf("must be an integer")
		}
	case "decimal":
		if _, ok := exactFilterNumber(value); !ok {
			return fmt.Errorf("must be decimal")
		}
	case "dateTime":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("must be a dateTime")
		}
		if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
			return fmt.Errorf("dateTime is invalid")
		}
	case "binary":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("must be binary")
		}
		if _, err := base64.StdEncoding.DecodeString(text); err != nil {
			return fmt.Errorf("binary is invalid")
		}
	case "complex":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("must be complex")
		}
		if err := validateAttributeValuesExact(object, attribute.SubAttributes); err != nil {
			return err
		}
	default:
		return fmt.Errorf("type is unsupported")
	}
	return nil
}

func normalizePrimaryValues(object map[string]any) {
	for _, value := range object {
		switch typed := value.(type) {
		case []any:
			lastPrimary := -1
			for index, raw := range typed {
				if item, ok := raw.(map[string]any); ok {
					normalizePrimaryValues(item)
					if primary, _ := item["primary"].(bool); primary {
						lastPrimary = index
					}
				}
			}
			if lastPrimary >= 0 {
				for index, raw := range typed {
					if item, ok := raw.(map[string]any); ok && index != lastPrimary {
						if _, exists := item["primary"]; exists {
							item["primary"] = false
						}
					}
				}
			}
		case map[string]any:
			normalizePrimaryValues(typed)
		}
	}
}

func validateSchemaSet(definition ResourceDefinition, document Document) error {
	rawSchemas, exists := document["schemas"]
	values, ok := rawSchemas.([]any)
	if !exists || !ok || len(values) == 0 || len(values) > len(definition.Extensions)+1 {
		return fmt.Errorf("schemas do not match the resource definition")
	}
	admitted := map[string]Extension{}
	for _, extension := range definition.Extensions {
		admitted[strings.ToLower(extension.Schema)] = extension
	}
	seen := make(map[string]struct{}, len(values))
	base := false
	for index, raw := range values {
		schema, ok := raw.(string)
		folded := strings.ToLower(schema)
		if !ok || schema == "" {
			return fmt.Errorf("schemas contains a non-string value")
		}
		if _, exists := seen[folded]; exists {
			return fmt.Errorf("schemas contains a case-equivalent duplicate")
		}
		seen[folded] = struct{}{}
		if strings.EqualFold(schema, definition.Schema) {
			base = true
			values[index] = definition.Schema
			continue
		}
		extension, exists := admitted[folded]
		if !exists {
			return fmt.Errorf("schemas contains an unsupported extension")
		}
		values[index] = extension.Schema
		object, exists := document[extension.Schema]
		if !exists {
			for key, candidate := range document {
				if strings.EqualFold(key, extension.Schema) {
					object, exists = candidate, true
					if key != extension.Schema {
						delete(document, key)
						document[extension.Schema] = candidate
					}
					break
				}
			}
		}
		extensionObject, ok := object.(map[string]any)
		if !exists || !ok {
			return fmt.Errorf("extension %q must be an object", extension.Schema)
		}
		if extension.Validate != nil {
			if err := extension.Validate(Document(extensionObject)); err != nil {
				return fmt.Errorf("extension %q is invalid: %w", extension.Schema, err)
			}
		}
	}
	if !base {
		return fmt.Errorf("base resource schema is missing")
	}
	for _, extension := range definition.Extensions {
		_, present := seen[strings.ToLower(extension.Schema)]
		if extension.Required && !present {
			return fmt.Errorf("required extension %q is missing", extension.Schema)
		}
	}
	return nil
}

func validateUser(definition ResourceDefinition, document Document) error {
	allowed := map[string]string{
		"schemas": "array", "externalId": "string", "userName": "string", "name": "object",
		"displayName": "string", "nickName": "string", "profileUrl": "string", "title": "string",
		"userType": "string", "preferredLanguage": "string", "locale": "string", "timezone": "string",
		"active": "boolean", "emails": "array", "phoneNumbers": "array", "ims": "array", "photos": "array",
		"addresses": "array", "entitlements": "array", "roles": "array", "x509Certificates": "array",
	}
	if err := validateTopLevel(definition, document, allowed); err != nil {
		return err
	}
	userName, err := requiredString(document, "userName", 1024)
	if err != nil || strings.TrimSpace(userName) != userName {
		return fmt.Errorf("userName is required and invalid")
	}
	userName, err = precis.UsernameCasePreserved.String(userName)
	if err != nil || userName == "" {
		return fmt.Errorf("userName fails international string preparation")
	}
	document["userName"] = userName
	for _, name := range []string{"externalId", "displayName", "nickName", "profileUrl", "title", "userType", "preferredLanguage", "locale", "timezone"} {
		if _, err := optionalString(document, name, maximumStringBytes); err != nil {
			return err
		}
	}
	if value, exists := document["active"]; exists {
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("active must be boolean")
		}
	}
	if value, exists := document["name"]; exists {
		if err := validateStringObject(value, []string{"formatted", "familyName", "givenName", "middleName", "honorificPrefix", "honorificSuffix"}); err != nil {
			return fmt.Errorf("name is invalid: %w", err)
		}
	}
	for _, name := range []string{"emails", "phoneNumbers", "ims", "photos", "entitlements", "roles", "x509Certificates"} {
		if value, exists := document[name]; exists {
			if err := validateMultiValue(value, []string{"value", "display", "type", "$ref"}); err != nil {
				return fmt.Errorf("%s is invalid: %w", name, err)
			}
		}
	}
	if value, exists := document["addresses"]; exists {
		if err := validateMultiValue(value, []string{"formatted", "streetAddress", "locality", "region", "postalCode", "country", "type"}); err != nil {
			return fmt.Errorf("addresses is invalid: %w", err)
		}
	}
	return nil
}

func validateGroup(definition ResourceDefinition, document Document) error {
	if err := validateTopLevel(definition, document, map[string]string{"schemas": "array", "externalId": "string", "displayName": "string", "members": "array"}); err != nil {
		return err
	}
	displayName, err := requiredString(document, "displayName", maximumStringBytes)
	if err != nil || strings.TrimSpace(displayName) != displayName {
		return fmt.Errorf("displayName is required and invalid")
	}
	if _, err := optionalString(document, "externalId", maximumStringBytes); err != nil {
		return err
	}
	if value, exists := document["members"]; exists {
		for _, raw := range value.([]any) {
			if member, ok := raw.(map[string]any); ok {
				delete(member, "display")
			}
		}
		if err := validateMultiValue(value, []string{"value", "$ref", "display", "type"}); err != nil {
			return fmt.Errorf("members is invalid: %w", err)
		}
		for _, raw := range value.([]any) {
			member := raw.(map[string]any)
			if _, err := requiredString(Document(member), "value", 1024); err != nil {
				return fmt.Errorf("member value is required")
			}
		}
	}
	return nil
}

func validateEnterpriseUser(document Document) error {
	for _, name := range []string{"employeeNumber", "costCenter", "organization", "division", "department"} {
		if _, err := optionalString(document, name, maximumStringBytes); err != nil {
			return err
		}
	}
	for name := range document {
		if !oneOf(name, "employeeNumber", "costCenter", "organization", "division", "department", "manager") {
			return fmt.Errorf("enterprise attribute %q is unsupported", name)
		}
	}
	if raw, exists := document["manager"]; exists {
		manager, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("enterprise manager is invalid")
		}
		delete(manager, "displayName")
		if err := validateStringObject(manager, []string{"value", "$ref"}); err != nil {
			return fmt.Errorf("enterprise manager is invalid: %w", err)
		}
	}
	return nil
}

func validateTopLevel(definition ResourceDefinition, document Document, allowed map[string]string) error {
	extensions := make(map[string]struct{})
	for _, extension := range definition.Extensions {
		extensions[strings.ToLower(extension.Schema)] = struct{}{}
	}
	for key, value := range document {
		typeName, exists := allowed[key]
		if !exists {
			if _, extension := extensions[strings.ToLower(key)]; extension {
				continue
			}
			return fmt.Errorf("attribute %q is unsupported", key)
		}
		switch typeName {
		case "string":
			if _, ok := value.(string); !ok {
				return fmt.Errorf("attribute %q must be a string", key)
			}
		case "boolean":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("attribute %q must be boolean", key)
			}
		case "array":
			if _, ok := value.([]any); !ok {
				return fmt.Errorf("attribute %q must be an array", key)
			}
		case "object":
			if _, ok := value.(map[string]any); !ok {
				return fmt.Errorf("attribute %q must be an object", key)
			}
		}
	}
	return nil
}

func validateStringObject(value any, allowed []string) error {
	object, ok := value.(map[string]any)
	if !ok || len(object) > len(allowed) {
		return fmt.Errorf("complex value shape is invalid")
	}
	set := make(map[string]struct{}, len(allowed))
	for _, name := range allowed {
		set[name] = struct{}{}
	}
	for name, raw := range object {
		text, ok := raw.(string)
		if _, admitted := set[name]; !admitted || !ok || !validString(text, maximumStringBytes) {
			return fmt.Errorf("sub-attribute %q is invalid", name)
		}
	}
	return nil
}

func validateMultiValue(value any, allowedStrings []string) error {
	values, ok := value.([]any)
	if !ok || len(values) > maximumValues {
		return fmt.Errorf("multi-valued boundary is invalid")
	}
	set := make(map[string]struct{}, len(allowedStrings))
	for _, name := range allowedStrings {
		set[name] = struct{}{}
	}
	for _, raw := range values {
		object, ok := raw.(map[string]any)
		if !ok || len(object) > len(allowedStrings)+1 {
			return fmt.Errorf("multi-valued element is invalid")
		}
		for name, item := range object {
			if name == "primary" {
				if _, ok := item.(bool); !ok {
					return fmt.Errorf("primary must be boolean")
				}
				continue
			}
			text, ok := item.(string)
			if _, admitted := set[name]; !admitted || !ok || !validString(text, maximumStringBytes) {
				return fmt.Errorf("sub-attribute %q is invalid", name)
			}
		}
	}
	return nil
}

func requiredString(document Document, name string, maximum int) (string, error) {
	value, exists := document[name]
	text, ok := value.(string)
	if !exists || !ok || !validString(text, maximum) || text == "" {
		return "", fmt.Errorf("%s is required and invalid", name)
	}
	return text, nil
}

func optionalString(document Document, name string, maximum int) (string, error) {
	value, exists := document[name]
	if !exists {
		return "", nil
	}
	text, ok := value.(string)
	if !ok || !validString(text, maximum) {
		return "", fmt.Errorf("%s is invalid", name)
	}
	return text, nil
}

func validString(value string, maximum int) bool {
	return len(value) <= maximum && utf8.ValidString(value) && strings.IndexFunc(value, unicode.IsControl) < 0
}

func stringPath(document Document, path string) (string, bool, error) {
	parts := strings.Split(path, ".")
	var value any = map[string]any(document)
	for _, part := range parts {
		object, ok := value.(map[string]any)
		if !ok {
			return "", false, fmt.Errorf("lookup path %q is not singular", path)
		}
		value, ok = object[part]
		if !ok {
			return "", false, nil
		}
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return "", false, fmt.Errorf("lookup path %q is not a non-empty string", path)
	}
	return text, true, nil
}

func validateExternalURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.RawPath != "" {
		return nil, fmt.Errorf("external URL must be an absolute HTTPS URL")
	}
	if parsed.Path != "" {
		if cleaned := pathpkg.Clean(parsed.Path); cleaned != parsed.Path && strings.TrimSuffix(parsed.Path, "/") != cleaned {
			return nil, fmt.Errorf("external URL path is not canonical")
		}
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	parsed.RawPath = ""
	return parsed, nil
}
