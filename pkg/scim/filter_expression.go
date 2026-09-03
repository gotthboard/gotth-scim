package scim

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
)

const (
	maximumFilterBytes  = 4096
	maximumFilterTokens = 512
	maximumFilterDepth  = 32
)

// FilterOperator is one RFC 7644 filter or logical operator.
type FilterOperator string

const (
	FilterEqual        FilterOperator = "eq"
	FilterNotEqual     FilterOperator = "ne"
	FilterContains     FilterOperator = "co"
	FilterStartsWith   FilterOperator = "sw"
	FilterEndsWith     FilterOperator = "ew"
	FilterPresent      FilterOperator = "pr"
	FilterGreaterThan  FilterOperator = "gt"
	FilterGreaterEqual FilterOperator = "ge"
	FilterLessThan     FilterOperator = "lt"
	FilterLessEqual    FilterOperator = "le"
	FilterAnd          FilterOperator = "and"
	FilterOr           FilterOperator = "or"
	FilterNot          FilterOperator = "not"
	FilterValuePath    FilterOperator = "valuePath"
)

// FilterExpression is a validated, immutable RFC 7644 filter syntax tree.
// Value is string, json.Number, bool, or nil for comparison expressions.
type FilterExpression struct {
	Operator  FilterOperator
	Attribute string
	Value     any
	Left      *FilterExpression
	Right     *FilterExpression
	Child     *FilterExpression
	contract  SchemaAttribute
	path      []string
}

type filterTokenKind uint8

const (
	filterEOF filterTokenKind = iota
	filterWord
	filterString
	filterNumberToken
	filterTrue
	filterFalse
	filterNull
	filterLeftParen
	filterRightParen
	filterLeftBracket
	filterRightBracket
)

type filterToken struct {
	kind  filterTokenKind
	text  string
	value any
}

type filterParser struct {
	tokens []filterToken
	index  int
}

// ParseFilter parses and schema-validates the complete RFC 7644 filter ABNF.
func ParseFilter(raw string, definition ResourceDefinition) (*FilterExpression, error) {
	return parseFilter(raw, definition, nil)
}

func parseValueFilter(raw string, attributes []SchemaAttribute) (*FilterExpression, error) {
	return parseFilter(raw, ResourceDefinition{}, attributes)
}

func parseFilter(raw string, definition ResourceDefinition, relative []SchemaAttribute) (*FilterExpression, error) {
	tokens, err := lexFilter(raw)
	if err != nil {
		return nil, err
	}
	parser := filterParser{tokens: tokens}
	expression, err := parser.parseOr(0)
	if err != nil || parser.peek().kind != filterEOF {
		return nil, fmt.Errorf("SCIM filter is invalid")
	}
	if err := validateFilterExpression(expression, definition, relative); err != nil {
		return nil, err
	}
	return expression, nil
}

func lexFilter(raw string) ([]filterToken, error) {
	if raw == "" || len(raw) > maximumFilterBytes || !utf8.ValidString(raw) || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return nil, fmt.Errorf("SCIM filter boundary is invalid")
	}
	tokens := make([]filterToken, 0, 32)
	for position := 0; position < len(raw); {
		for position < len(raw) && (raw[position] == ' ' || raw[position] == '\t') {
			position++
		}
		if position == len(raw) {
			break
		}
		if len(tokens) >= maximumFilterTokens {
			return nil, fmt.Errorf("SCIM filter token boundary exceeded")
		}
		switch raw[position] {
		case '(':
			tokens = append(tokens, filterToken{kind: filterLeftParen, text: "("})
			position++
		case ')':
			tokens = append(tokens, filterToken{kind: filterRightParen, text: ")"})
			position++
		case '[':
			tokens = append(tokens, filterToken{kind: filterLeftBracket, text: "["})
			position++
		case ']':
			tokens = append(tokens, filterToken{kind: filterRightBracket, text: "]"})
			position++
		case '"':
			start := position
			position++
			escaped := false
			for position < len(raw) {
				if !escaped && raw[position] == '"' {
					position++
					break
				}
				if !escaped && raw[position] == '\\' {
					escaped = true
				} else {
					escaped = false
				}
				position++
			}
			if position > len(raw) || position == len(raw) && raw[position-1] != '"' {
				return nil, fmt.Errorf("SCIM filter string is unterminated")
			}
			var value string
			if err := json.Unmarshal([]byte(raw[start:position]), &value); err != nil || !validString(value, maximumStringBytes) {
				return nil, fmt.Errorf("SCIM filter string is invalid")
			}
			tokens = append(tokens, filterToken{kind: filterString, text: raw[start:position], value: value})
		default:
			start := position
			for position < len(raw) && !strings.ContainsRune(" \t()[]", rune(raw[position])) {
				position++
			}
			word := raw[start:position]
			lower := strings.ToLower(word)
			token := filterToken{kind: filterWord, text: word, value: word}
			switch lower {
			case "true":
				token.kind, token.value = filterTrue, true
			case "false":
				token.kind, token.value = filterFalse, false
			case "null":
				token.kind, token.value = filterNull, nil
			default:
				if _, err := strconv.ParseFloat(word, 64); err == nil && validJSONNumber(word) {
					token.kind, token.value = filterNumberToken, json.Number(word)
				}
			}
			tokens = append(tokens, token)
		}
	}
	tokens = append(tokens, filterToken{kind: filterEOF})
	return tokens, nil
}

func validJSONNumber(raw string) bool {
	var number json.Number
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err != nil {
		return false
	}
	return decoder.Decode(&struct{}{}) == io.EOF
}

func (parser *filterParser) peek() filterToken { return parser.tokens[parser.index] }

func (parser *filterParser) take() filterToken {
	token := parser.peek()
	if parser.index < len(parser.tokens)-1 {
		parser.index++
	}
	return token
}

func (parser *filterParser) word(value string) bool {
	return parser.peek().kind == filterWord && strings.EqualFold(parser.peek().text, value)
}

func (parser *filterParser) parseOr(depth int) (*FilterExpression, error) {
	left, err := parser.parseAnd(depth + 1)
	if err != nil {
		return nil, err
	}
	for parser.word("or") {
		parser.take()
		right, err := parser.parseAnd(depth + 1)
		if err != nil {
			return nil, err
		}
		left = &FilterExpression{Operator: FilterOr, Left: left, Right: right}
	}
	return left, nil
}

func (parser *filterParser) parseAnd(depth int) (*FilterExpression, error) {
	left, err := parser.parseUnary(depth + 1)
	if err != nil {
		return nil, err
	}
	for parser.word("and") {
		parser.take()
		right, err := parser.parseUnary(depth + 1)
		if err != nil {
			return nil, err
		}
		left = &FilterExpression{Operator: FilterAnd, Left: left, Right: right}
	}
	return left, nil
}

func (parser *filterParser) parseUnary(depth int) (*FilterExpression, error) {
	if depth > maximumFilterDepth {
		return nil, fmt.Errorf("SCIM filter depth exceeded")
	}
	if parser.word("not") {
		parser.take()
		if parser.take().kind != filterLeftParen {
			return nil, fmt.Errorf("SCIM not expression requires parentheses")
		}
		child, err := parser.parseOr(depth + 1)
		if err != nil || parser.take().kind != filterRightParen {
			return nil, fmt.Errorf("SCIM not expression is invalid")
		}
		return &FilterExpression{Operator: FilterNot, Child: child}, nil
	}
	if parser.peek().kind == filterLeftParen {
		parser.take()
		child, err := parser.parseOr(depth + 1)
		if err != nil || parser.take().kind != filterRightParen {
			return nil, fmt.Errorf("SCIM grouped filter is invalid")
		}
		return child, nil
	}
	return parser.parseAttribute(depth + 1)
}

func (parser *filterParser) parseAttribute(depth int) (*FilterExpression, error) {
	if depth > maximumFilterDepth || parser.peek().kind != filterWord {
		return nil, fmt.Errorf("SCIM attribute expression is invalid")
	}
	attribute := parser.take().text
	if parser.peek().kind == filterLeftBracket {
		parser.take()
		child, err := parser.parseOr(depth + 1)
		if err != nil || parser.take().kind != filterRightBracket {
			return nil, fmt.Errorf("SCIM value-path filter is invalid")
		}
		return &FilterExpression{Operator: FilterValuePath, Attribute: attribute, Child: child}, nil
	}
	if parser.peek().kind != filterWord {
		return nil, fmt.Errorf("SCIM comparison operator is missing")
	}
	operator := FilterOperator(strings.ToLower(parser.take().text))
	if operator == FilterPresent {
		return &FilterExpression{Operator: operator, Attribute: attribute}, nil
	}
	if !oneOf(string(operator), "eq", "ne", "co", "sw", "ew", "gt", "ge", "lt", "le") {
		return nil, fmt.Errorf("SCIM comparison operator is invalid")
	}
	value := parser.take()
	if value.kind != filterString && value.kind != filterNumberToken && value.kind != filterTrue && value.kind != filterFalse && value.kind != filterNull {
		return nil, fmt.Errorf("SCIM comparison value is invalid")
	}
	return &FilterExpression{Operator: operator, Attribute: attribute, Value: value.value}, nil
}

func validateFilterExpression(expression *FilterExpression, definition ResourceDefinition, relative []SchemaAttribute) error {
	if expression == nil {
		return fmt.Errorf("SCIM filter expression is missing")
	}
	switch expression.Operator {
	case FilterAnd, FilterOr:
		if err := validateFilterExpression(expression.Left, definition, relative); err != nil {
			return err
		}
		return validateFilterExpression(expression.Right, definition, relative)
	case FilterNot:
		return validateFilterExpression(expression.Child, definition, relative)
	case FilterValuePath:
		path, contract, ok := resolveAttributeContract(definition, relative, expression.Attribute)
		if !ok || !contract.MultiValued || contract.Type != "complex" {
			return fmt.Errorf("SCIM value-path attribute is invalid")
		}
		expression.path, expression.contract = path, contract
		return validateFilterExpression(expression.Child, definition, contract.SubAttributes)
	default:
		path, contract, ok := resolveAttributeContract(definition, relative, expression.Attribute)
		if !ok {
			return fmt.Errorf("SCIM filter attribute is unsupported")
		}
		if expression.Operator != FilterPresent && !compatibleFilterValue(contract, expression.Operator, expression.Value) {
			return fmt.Errorf("SCIM filter comparison is incompatible with its attribute")
		}
		expression.path, expression.contract = path, contract
		return nil
	}
}

func compatibleFilterValue(contract SchemaAttribute, operator FilterOperator, value any) bool {
	if value == nil {
		return operator == FilterEqual || operator == FilterNotEqual
	}
	if operator == FilterContains || operator == FilterStartsWith || operator == FilterEndsWith {
		_, ok := value.(string)
		return ok && (contract.Type == "string" || contract.Type == "reference")
	}
	switch contract.Type {
	case "boolean":
		_, ok := value.(bool)
		return ok && (operator == FilterEqual || operator == FilterNotEqual)
	case "integer", "decimal":
		number, ok := exactFilterNumber(value)
		return ok && (contract.Type != "integer" || number.Denom().Cmp(big.NewInt(1)) == 0)
	case "dateTime", "string", "reference", "binary":
		_, ok := value.(string)
		if !ok || contract.Type == "binary" && operator != FilterEqual && operator != FilterNotEqual {
			return false
		}
		if contract.Type == "dateTime" {
			_, err := time.Parse(time.RFC3339Nano, value.(string))
			return err == nil
		}
		return true
	default:
		return value == nil && (operator == FilterEqual || operator == FilterNotEqual)
	}
}

func resolveAttributeContract(definition ResourceDefinition, relative []SchemaAttribute, raw string) ([]string, SchemaAttribute, bool) {
	if relative != nil {
		return resolveFromAttributes(relative, raw, nil)
	}
	remaining := raw
	prefix := ""
	if len(remaining) > len(definition.Schema)+1 && strings.EqualFold(remaining[:len(definition.Schema)+1], definition.Schema+":") {
		remaining = remaining[len(definition.Schema)+1:]
	}
	for _, extension := range definition.Extensions {
		candidate := extension.Schema + ":"
		if len(remaining) > len(candidate) && strings.EqualFold(remaining[:len(candidate)], candidate) {
			prefix, remaining = extension.Schema, remaining[len(candidate):]
			prefixPath := []string{extension.Schema}
			if extension.Name == "$core" {
				prefixPath = nil
			}
			path, contract, ok := resolveFromAttributes(extension.Attributes, remaining, prefixPath)
			return path, contract, ok
		}
	}
	if prefix == "" {
		if path, contract, ok := resolveCommonAttribute(remaining); ok {
			return path, contract, true
		}
		return resolveFromAttributes(definition.Attributes, remaining, nil)
	}
	return nil, SchemaAttribute{}, false
}

func resolveFromAttributes(attributes []SchemaAttribute, raw string, prefix []string) ([]string, SchemaAttribute, bool) {
	parts := strings.Split(raw, ".")
	if len(parts) < 1 || len(parts) > 2 {
		return nil, SchemaAttribute{}, false
	}
	for _, attribute := range attributes {
		if !strings.EqualFold(attribute.Name, parts[0]) {
			continue
		}
		path := append(append([]string(nil), prefix...), attribute.Name)
		if len(parts) == 1 {
			return path, attribute, true
		}
		for _, sub := range attribute.SubAttributes {
			if strings.EqualFold(sub.Name, parts[1]) {
				return append(path, sub.Name), sub, true
			}
		}
	}
	return nil, SchemaAttribute{}, false
}

func resolveCommonAttribute(raw string) ([]string, SchemaAttribute, bool) {
	common := []SchemaAttribute{
		{Name: "schemas", Type: "string", MultiValued: true, Mutability: "readOnly", Returned: "always", Uniqueness: "none", CaseExact: true},
		{Name: "id", Type: "string", Mutability: "readOnly", Returned: "always", Uniqueness: "global", CaseExact: true},
		{Name: "externalId", Type: "string", Mutability: "readWrite", Returned: "default", Uniqueness: "none", CaseExact: true},
		{Name: "meta", Type: "complex", Mutability: "readOnly", Returned: "default", Uniqueness: "none", SubAttributes: []SchemaAttribute{
			{Name: "resourceType", Type: "string", Mutability: "readOnly", Returned: "default", Uniqueness: "none", CaseExact: true},
			{Name: "created", Type: "dateTime", Mutability: "readOnly", Returned: "default", Uniqueness: "none"},
			{Name: "lastModified", Type: "dateTime", Mutability: "readOnly", Returned: "default", Uniqueness: "none"},
			{Name: "version", Type: "string", Mutability: "readOnly", Returned: "default", Uniqueness: "none", CaseExact: true},
			{Name: "location", Type: "reference", Mutability: "readOnly", Returned: "default", Uniqueness: "none", CaseExact: true},
		}},
	}
	return resolveFromAttributes(common, raw, nil)
}

// MatchFilter evaluates a validated filter against one canonical resource.
func MatchFilter(expression *FilterExpression, document Document) (bool, error) {
	return matchFilter(expression, map[string]any(document))
}

func matchFilter(expression *FilterExpression, document map[string]any) (bool, error) {
	if expression == nil {
		return true, nil
	}
	switch expression.Operator {
	case FilterAnd:
		left, err := matchFilter(expression.Left, document)
		if err != nil || !left {
			return left, err
		}
		return matchFilter(expression.Right, document)
	case FilterOr:
		left, err := matchFilter(expression.Left, document)
		if err != nil || left {
			return left, err
		}
		return matchFilter(expression.Right, document)
	case FilterNot:
		matched, err := matchFilter(expression.Child, document)
		return !matched, err
	case FilterValuePath:
		values := lookupFilterValues(document, expression.path)
		for _, value := range values {
			if object, ok := value.(map[string]any); ok {
				matched, err := matchFilter(expression.Child, object)
				if err != nil || matched {
					return matched, err
				}
			}
		}
		return false, nil
	default:
		values := lookupFilterValues(document, expression.path)
		if expression.Operator == FilterPresent {
			for _, value := range values {
				if filterPresent(value) {
					return true, nil
				}
			}
			return false, nil
		}
		for _, value := range values {
			matched, err := compareFilterValue(value, expression.Value, expression.contract, expression.Operator)
			if err != nil || matched {
				return matched, err
			}
		}
		return false, nil
	}
}

func lookupFilterValues(document map[string]any, path []string) []any {
	if len(path) == 0 {
		return nil
	}
	value, exists := document[path[0]]
	if !exists {
		return nil
	}
	values := []any{value}
	for _, part := range path[1:] {
		next := make([]any, 0)
		for _, current := range values {
			switch typed := current.(type) {
			case map[string]any:
				if child, ok := typed[part]; ok {
					next = append(next, child)
				}
			case []any:
				for _, item := range typed {
					if object, ok := item.(map[string]any); ok {
						if child, exists := object[part]; exists {
							next = append(next, child)
						}
					}
				}
			}
		}
		values = next
	}
	flattened := make([]any, 0, len(values))
	for _, value := range values {
		if array, ok := value.([]any); ok {
			flattened = append(flattened, array...)
		} else {
			flattened = append(flattened, value)
		}
	}
	return flattened
}

func filterPresent(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return typed != ""
	case []any:
		return len(typed) != 0
	case map[string]any:
		return len(typed) != 0
	default:
		return true
	}
}

func compareFilterValue(actual, expected any, contract SchemaAttribute, operator FilterOperator) (bool, error) {
	if expected == nil {
		if operator == FilterEqual {
			return actual == nil, nil
		}
		if operator == FilterNotEqual {
			return actual != nil, nil
		}
	}
	if actual == nil {
		return false, nil
	}
	if operator == FilterContains || operator == FilterStartsWith || operator == FilterEndsWith {
		left, leftOK := actual.(string)
		right, rightOK := expected.(string)
		if !leftOK || !rightOK {
			return false, nil
		}
		if !contract.CaseExact {
			left, right = cases.Fold().String(left), cases.Fold().String(right)
		}
		switch operator {
		case FilterContains:
			return strings.Contains(left, right), nil
		case FilterStartsWith:
			return strings.HasPrefix(left, right), nil
		default:
			return strings.HasSuffix(left, right), nil
		}
	}
	comparison, comparable := orderFilterValues(actual, expected, contract)
	if !comparable {
		return false, nil
	}
	switch operator {
	case FilterEqual:
		return comparison == 0, nil
	case FilterNotEqual:
		return comparison != 0, nil
	case FilterGreaterThan:
		return comparison > 0, nil
	case FilterGreaterEqual:
		return comparison >= 0, nil
	case FilterLessThan:
		return comparison < 0, nil
	case FilterLessEqual:
		return comparison <= 0, nil
	default:
		return false, fmt.Errorf("unsupported filter comparison")
	}
}

func orderFilterValues(actual, expected any, contract SchemaAttribute) (int, bool) {
	switch contract.Type {
	case "string", "reference", "binary":
		left, leftOK := actual.(string)
		right, rightOK := expected.(string)
		if !leftOK || !rightOK {
			return 0, false
		}
		if !contract.CaseExact {
			left, right = cases.Fold().String(left), cases.Fold().String(right)
		}
		return strings.Compare(left, right), true
	case "dateTime":
		leftRaw, leftOK := actual.(string)
		rightRaw, rightOK := expected.(string)
		left, leftErr := time.Parse(time.RFC3339Nano, leftRaw)
		right, rightErr := time.Parse(time.RFC3339Nano, rightRaw)
		if !leftOK || !rightOK || leftErr != nil || rightErr != nil {
			return 0, false
		}
		if left.Before(right) {
			return -1, true
		}
		if left.After(right) {
			return 1, true
		}
		return 0, true
	case "integer", "decimal":
		left, leftOK := exactFilterNumber(actual)
		right, rightOK := exactFilterNumber(expected)
		if !leftOK || !rightOK {
			return 0, false
		}
		if comparison := left.Cmp(right); comparison < 0 {
			return -1, true
		} else if comparison > 0 {
			return 1, true
		}
		return 0, true
	case "boolean":
		left, leftOK := actual.(bool)
		right, rightOK := expected.(bool)
		if !leftOK || !rightOK {
			return 0, false
		}
		if left == right {
			return 0, true
		}
		if !left && right {
			return -1, true
		}
		return 1, true
	default:
		return 0, false
	}
}

func exactFilterNumber(value any) (*big.Rat, bool) {
	var raw string
	switch typed := value.(type) {
	case json.Number:
		raw = typed.String()
	case float64:
		raw = strconv.FormatFloat(typed, 'g', -1, 64)
	default:
		return nil, false
	}
	result, ok := new(big.Rat).SetString(raw)
	return result, ok
}
