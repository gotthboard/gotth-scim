package scim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const defaultMaximumSearchCandidates = 10000

// SearchRequest is the RFC 7644 POST .search request body.
type SearchRequest struct {
	Schemas            []string `json:"schemas"`
	Attributes         []string `json:"attributes,omitempty"`
	ExcludedAttributes []string `json:"excludedAttributes,omitempty"`
	Filter             string   `json:"filter,omitempty"`
	SortBy             string   `json:"sortBy,omitempty"`
	SortOrder          string   `json:"sortOrder,omitempty"`
	StartIndex         int      `json:"startIndex,omitempty"`
	Count              *int     `json:"count,omitempty"`
}

type searchPlan struct {
	filter     *FilterExpression
	sortPath   []string
	sortAttr   SchemaAttribute
	descending bool
	start      int
	count      int
	include    [][]string
	exclude    [][]string
}

func decodeSearchRequest(raw []byte) (SearchRequest, error) {
	if len(raw) == 0 || len(raw) > MaximumResourceBytes {
		return SearchRequest{}, fmt.Errorf("search request size is invalid")
	}
	document, err := DecodeDocument(raw)
	if err != nil {
		return SearchRequest{}, fmt.Errorf("search request JSON is invalid")
	}
	if value, exists := document["count"]; exists && value == nil {
		return SearchRequest{}, fmt.Errorf("search count is invalid")
	}
	if value, exists := document["startIndex"]; exists && value == nil {
		return SearchRequest{}, fmt.Errorf("search startIndex is invalid")
	}
	var request SearchRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return SearchRequest{}, fmt.Errorf("search request JSON is invalid")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return SearchRequest{}, fmt.Errorf("search request JSON is invalid")
	}
	if err := ValidateSchemas(request.Schemas, []string{SearchRequestSchema}); err != nil {
		return SearchRequest{}, err
	}
	return request, nil
}

func searchRequestFromQuery(query url.Values) (SearchRequest, error) {
	admitted := map[string]bool{"attributes": true, "excludedAttributes": true, "filter": true, "sortBy": true, "sortOrder": true, "startIndex": true, "count": true}
	if err := rejectQuery(query, admitted); err != nil {
		return SearchRequest{}, err
	}
	start, err := queryInteger(query, "startIndex", 1)
	if err != nil {
		return SearchRequest{}, fmt.Errorf("startIndex is invalid")
	}
	count, err := queryInteger(query, "count", -1)
	if err != nil {
		return SearchRequest{}, fmt.Errorf("count is invalid")
	}
	request := SearchRequest{Filter: query.Get("filter"), SortBy: query.Get("sortBy"), SortOrder: query.Get("sortOrder"), StartIndex: start}
	if _, present := query["count"]; present {
		if count < 0 {
			return SearchRequest{}, fmt.Errorf("count is invalid")
		}
		request.Count = &count
	}
	request.Attributes = splitAttributeParameter(query.Get("attributes"))
	request.ExcludedAttributes = splitAttributeParameter(query.Get("excludedAttributes"))
	return request, nil
}

func splitAttributeParameter(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts
}

func compileSearch(request SearchRequest, definition ResourceDefinition, maximumPageSize int) (searchPlan, error) {
	plan := searchPlan{start: request.StartIndex, count: maximumPageSize}
	if plan.start == 0 {
		plan.start = 1
	}
	if plan.start < 1 {
		plan.start = 1
	}
	if request.Count != nil {
		if *request.Count < 0 {
			return searchPlan{}, fmt.Errorf("count is invalid")
		}
		plan.count = *request.Count
		if plan.count > maximumPageSize {
			plan.count = maximumPageSize
		}
	}
	if request.Filter != "" {
		expression, err := ParseFilter(request.Filter, definition)
		if err != nil {
			return searchPlan{}, fmt.Errorf("filter: %w", err)
		}
		plan.filter = expression
	}
	if request.SortOrder != "" && !strings.EqualFold(request.SortOrder, "ascending") && !strings.EqualFold(request.SortOrder, "descending") {
		return searchPlan{}, fmt.Errorf("sortOrder is invalid")
	}
	if request.SortOrder != "" && request.SortBy == "" {
		return searchPlan{}, fmt.Errorf("sortOrder requires sortBy")
	}
	plan.descending = strings.EqualFold(request.SortOrder, "descending")
	if request.SortBy != "" {
		path, attribute, ok := resolveAttributeContract(definition, nil, request.SortBy)
		if !ok || attribute.Type == "complex" {
			return searchPlan{}, fmt.Errorf("sortBy is invalid")
		}
		plan.sortPath, plan.sortAttr = path, attribute
	}
	if len(request.Attributes) > 0 && len(request.ExcludedAttributes) > 0 {
		return searchPlan{}, fmt.Errorf("attributes and excludedAttributes are mutually exclusive")
	}
	var err error
	plan.include, err = compileProjectionPaths(request.Attributes, definition)
	if err != nil {
		return searchPlan{}, err
	}
	plan.exclude, err = compileProjectionPaths(request.ExcludedAttributes, definition)
	return plan, err
}

func compileProjectionPaths(raw []string, definition ResourceDefinition) ([][]string, error) {
	if len(raw) > 100 {
		return nil, fmt.Errorf("attribute projection is too large")
	}
	result := make([][]string, 0, len(raw))
	seen := make(map[string]struct{})
	for _, candidate := range raw {
		path, _, ok := resolveAttributeContract(definition, nil, candidate)
		if !ok {
			return nil, fmt.Errorf("projection attribute is invalid")
		}
		key := strings.ToLower(strings.Join(path, "\x00"))
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, path)
	}
	return result, nil
}

func applySearch(plan searchPlan, resources []Document) ([]Document, int, error) {
	filtered := resources[:0]
	for _, resource := range resources {
		matched, err := MatchFilter(plan.filter, resource)
		if err != nil {
			return nil, 0, err
		}
		if matched {
			filtered = append(filtered, resource)
		}
	}
	if len(plan.sortPath) != 0 {
		sort.SliceStable(filtered, func(left, right int) bool {
			comparison := compareSortValues(filtered[left], filtered[right], plan.sortPath, plan.sortAttr)
			if comparison == 0 {
				comparison = strings.Compare(documentID(filtered[left]), documentID(filtered[right]))
			}
			if plan.descending {
				return comparison > 0
			}
			return comparison < 0
		})
	}
	total := len(filtered)
	begin := plan.start - 1
	if begin > total {
		begin = total
	}
	end := begin + plan.count
	if end > total {
		end = total
	}
	result := make([]Document, 0, end-begin)
	for _, resource := range filtered[begin:end] {
		result = append(result, resource)
	}
	return result, total, nil
}

func compareSortValues(left, right Document, path []string, contract SchemaAttribute) int {
	leftValues, rightValues := sortValues(map[string]any(left), path), sortValues(map[string]any(right), path)
	if len(leftValues) == 0 && len(rightValues) == 0 {
		return 0
	}
	if len(leftValues) == 0 {
		return 1
	}
	if len(rightValues) == 0 {
		return -1
	}
	comparison, ok := orderFilterValues(leftValues[0], rightValues[0], contract)
	if !ok {
		return 0
	}
	return comparison
}

func sortValues(document map[string]any, path []string) []any {
	if len(path) >= 2 {
		if values, ok := document[path[0]].([]any); ok {
			selected := -1
			for index, raw := range values {
				object, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				if primary, _ := object["primary"].(bool); primary {
					selected = index
					break
				}
				if selected < 0 {
					selected = index
				}
			}
			if selected >= 0 {
				if object, ok := values[selected].(map[string]any); ok {
					return lookupFilterValues(object, path[1:])
				}
			}
		}
	}
	return lookupFilterValues(document, path)
}

func documentID(document Document) string { value, _ := document["id"].(string); return value }

func projectDocument(document Document, include, exclude [][]string, definition ResourceDefinition) (Document, error) {
	source, err := cloneDocument(document)
	if err != nil {
		return nil, err
	}
	always, never, requested := returnabilityPaths(definition)
	for _, path := range never {
		deleteProjectedPath(source, path)
	}
	for _, path := range requested {
		if !pathRequested(path, include) {
			deleteProjectedPath(source, path)
		}
	}
	returnSource, err := cloneDocument(source)
	if err != nil {
		return nil, err
	}
	if len(include) != 0 {
		result := Document{}
		copyProjectedPath(result, source, []string{"schemas"})
		copyProjectedPath(result, source, []string{"id"})
		for _, path := range always {
			copyProjectedPath(result, source, path)
		}
		for _, path := range include {
			copyProjectedPath(result, source, path)
		}
		return result, nil
	}
	for _, path := range exclude {
		if len(path) == 1 && (strings.EqualFold(path[0], "schemas") || strings.EqualFold(path[0], "id")) || pathRequested(path, always) {
			continue
		}
		deleteProjectedPath(source, path)
	}
	for _, path := range always {
		copyProjectedPath(source, returnSource, path)
	}
	return source, nil
}

func copyProjectedPath(target, source map[string]any, path []string) {
	if len(path) == 0 {
		return
	}
	value, ok := source[path[0]]
	if !ok {
		return
	}
	if len(path) == 1 {
		target[path[0]] = value
		return
	}
	switch object := value.(type) {
	case map[string]any:
		child, ok := target[path[0]].(map[string]any)
		if !ok {
			child = map[string]any{}
			target[path[0]] = child
		}
		copyProjectedPath(child, object, path[1:])
	case []any:
		children, ok := target[path[0]].([]any)
		if !ok || len(children) != len(object) {
			children = make([]any, len(object))
			for index := range children {
				children[index] = map[string]any{}
			}
			target[path[0]] = children
		}
		for index, raw := range object {
			sourceChild, sourceOK := raw.(map[string]any)
			targetChild, targetOK := children[index].(map[string]any)
			if sourceOK && targetOK {
				copyProjectedPath(targetChild, sourceChild, path[1:])
			}
		}
	}
}

func deleteProjectedPath(target map[string]any, path []string) {
	if len(path) == 0 {
		return
	}
	if len(path) == 1 {
		delete(target, path[0])
		return
	}
	switch object := target[path[0]].(type) {
	case map[string]any:
		deleteProjectedPath(object, path[1:])
		if len(object) == 0 {
			delete(target, path[0])
		}
	case []any:
		for _, raw := range object {
			if child, ok := raw.(map[string]any); ok {
				deleteProjectedPath(child, path[1:])
			}
		}
	}
}

func returnabilityPaths(definition ResourceDefinition) (always, never, requested [][]string) {
	var walk func([]SchemaAttribute, []string)
	walk = func(attributes []SchemaAttribute, prefix []string) {
		for _, attribute := range attributes {
			path := append(append([]string(nil), prefix...), attribute.Name)
			switch attribute.Returned {
			case "always":
				always = append(always, path)
			case "never":
				never = append(never, path)
			case "request":
				requested = append(requested, path)
			}
			if attribute.Type == "complex" {
				walk(attribute.SubAttributes, path)
			}
		}
	}
	walk(definition.Attributes, nil)
	for _, extension := range definition.Extensions {
		walk(extension.Attributes, []string{extension.Schema})
	}
	never = append(never, []string{"password"})
	return
}

func pathRequested(path []string, requested [][]string) bool {
	for _, candidate := range requested {
		if len(candidate) > len(path) {
			continue
		}
		matches := true
		for index := range candidate {
			if !strings.EqualFold(candidate[index], path[index]) {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

func (server *Server) serveSearch(writer http.ResponseWriter, request *http.Request, scope string, one *ResourceDefinition) {
	if request.Method != http.MethodPost {
		methodNotAllowed(writer, "POST")
		return
	}
	if err := rejectQuery(request.URL.Query(), nil); err != nil {
		writeProtocolError(writer, err)
		return
	}
	raw, err := readBody(request)
	if err != nil {
		writeProtocolError(writer, err)
		return
	}
	search, err := decodeSearchRequest(raw)
	if err != nil {
		writeProtocolError(writer, clientError(400, "invalidSyntax", "search request is invalid"))
		return
	}
	definitions := server.registry.definitions()
	if one != nil {
		definitions = []ResourceDefinition{*one}
	}
	server.executeSearch(writer, request, scope, definitions, search)
}

func (server *Server) executeSearch(writer http.ResponseWriter, request *http.Request, scope string, definitions []ResourceDefinition, search SearchRequest) {
	searchDefinition := combinedSearchDefinition(definitions)
	plan, err := compileSearch(search, searchDefinition, server.maximumPageSize)
	if err != nil {
		scimType := "invalidValue"
		if strings.Contains(err.Error(), "filter") {
			scimType = "invalidFilter"
		}
		writeProtocolError(writer, clientError(400, scimType, err.Error()))
		return
	}
	type searchCandidate struct {
		definition ResourceDefinition
		record     Record
	}
	candidates := make([]searchCandidate, 0)
	err = server.store.Transact(request.Context(), func(transaction Transaction) error {
		remaining := server.maximumSearchCandidates
		for _, definition := range definitions {
			limit := remaining
			if limit == 0 {
				limit = 1
			}
			records, err := transaction.List(Query{Scope: scope, ResourceType: definition.Name, Limit: limit})
			if err != nil {
				return err
			}
			if remaining == 0 && len(records) != 0 {
				return ErrTooMany
			}
			remaining -= len(records)
			for _, record := range records {
				candidates = append(candidates, searchCandidate{definition: definition, record: record})
			}
		}
		return nil
	})
	if err != nil {
		if err == ErrTooMany {
			writeProtocolError(writer, clientError(413, "tooLarge", "search candidate boundary exceeded"))
		} else {
			writeStoreError(writer, err)
		}
		return
	}
	all := make([]Document, 0, len(candidates))
	for _, candidate := range candidates {
		document, err := server.renderRecord(candidate.definition, candidate.record)
		if err != nil {
			writeProtocolError(writer, clientError(500, "", "stored resource is invalid"))
			return
		}
		matched, err := MatchFilter(plan.filter, document)
		if err != nil {
			writeProtocolError(writer, clientError(500, "", "search processing failed"))
			return
		}
		if matched {
			all = append(all, document)
		}
	}
	plan.filter = nil // filtering was performed per definition above
	resources, total, err := applySearchWithDefinition(plan, all, searchDefinition)
	if err != nil {
		writeProtocolError(writer, clientError(500, "", "search processing failed"))
		return
	}
	response := map[string]any{"schemas": []string{ListResponseSchema}, "totalResults": total, "startIndex": plan.start, "itemsPerPage": len(resources), "Resources": resources}
	writeJSON(writer, http.StatusOK, response)
}

func applySearchWithDefinition(plan searchPlan, resources []Document, definition ResourceDefinition) ([]Document, int, error) {
	filtered, total, err := applySearch(plan, resources)
	if err != nil {
		return nil, 0, err
	}
	for index, document := range filtered {
		filtered[index], err = projectDocument(document, plan.include, plan.exclude, definition)
		if err != nil {
			return nil, 0, err
		}
	}
	return filtered, total, nil
}

func combinedSearchDefinition(definitions []ResourceDefinition) ResourceDefinition {
	result := ResourceDefinition{Schema: definitions[0].Schema, Attributes: cloneSchemaAttributes(definitions[0].Attributes), Extensions: append([]Extension(nil), definitions[0].Extensions...)}
	seen := make(map[string]struct{})
	for _, attribute := range result.Attributes {
		seen[strings.ToLower(attribute.Name)] = struct{}{}
	}
	for index, definition := range definitions {
		if index > 0 {
			result.Extensions = append(result.Extensions, Extension{Schema: definition.Schema, Name: "$core", Attributes: cloneSchemaAttributes(definition.Attributes)})
		}
		for _, attribute := range definition.Attributes {
			if _, exists := seen[strings.ToLower(attribute.Name)]; !exists {
				result.Attributes = append(result.Attributes, attribute)
				seen[strings.ToLower(attribute.Name)] = struct{}{}
			}
		}
		if index > 0 {
			result.Extensions = append(result.Extensions, definition.Extensions...)
		}
	}
	return result
}
