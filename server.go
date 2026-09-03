package scim

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultMaximumPageSize = 100

// ScopeResolver returns the opaque provisioning scope for an already
// authenticated request. Returning an error denies the request.
type ScopeResolver func(*http.Request) (string, error)

// AuthenticationScheme documents authentication performed before Server.
// Server never parses credentials itself.
type AuthenticationScheme struct {
	Type             string `json:"type"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	SpecURI          string `json:"specUri,omitempty"`
	DocumentationURI string `json:"documentationUri,omitempty"`
}

// ServerConfig defines one SCIM HTTP handler.
type ServerConfig struct {
	Store                   Store
	Registry                *Registry
	ExternalURL             string
	ResolveScope            ScopeResolver
	Clock                   func() time.Time
	Entropy                 io.Reader
	MaximumPageSize         int
	MaximumPatchOperations  int
	MaximumSearchCandidates int
	AuthenticationSchemes   []AuthenticationScheme
	DocumentationURI        string
	PublicDiscovery         bool
	WeakETags               bool
	ChangePasswordSupported bool
}

// Server is a concurrency-safe SCIM HTTP handler.
type Server struct {
	store                   Store
	registry                *Registry
	externalURL             *url.URL
	resolveScope            ScopeResolver
	clock                   func() time.Time
	entropy                 io.Reader
	maximumPageSize         int
	maximumPatchOperations  int
	maximumSearchCandidates int
	authenticationSchemes   []AuthenticationScheme
	documentationURI        string
	publicDiscovery         bool
	weakETags               bool
	changePasswordSupported bool
	mu                      sync.Mutex
}

// NewServer validates configuration and returns a ready HTTP handler.
func NewServer(config ServerConfig) (*Server, error) {
	if config.Store == nil || config.ResolveScope == nil {
		return nil, fmt.Errorf("SCIM store and scope resolver are required")
	}
	if config.ChangePasswordSupported {
		if _, ok := config.Store.(PasswordStore); !ok {
			return nil, fmt.Errorf("advertised password support requires PasswordStore")
		}
	}
	if len(config.AuthenticationSchemes) == 0 || len(config.AuthenticationSchemes) > 16 {
		return nil, fmt.Errorf("at least one authentication scheme is required")
	}
	for _, scheme := range config.AuthenticationSchemes {
		if !validName(scheme.Type) || scheme.Name == "" || !validString(scheme.Name, 1024) || scheme.Description == "" || !validString(scheme.Description, maximumStringBytes) {
			return nil, fmt.Errorf("authentication scheme is invalid")
		}
		for _, raw := range []string{scheme.SpecURI, scheme.DocumentationURI} {
			if raw != "" {
				parsed, err := url.Parse(raw)
				if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
					return nil, fmt.Errorf("authentication scheme URL is invalid")
				}
			}
		}
	}
	registry := config.Registry
	if registry == nil {
		var err error
		registry, err = NewRegistry(DefaultDefinitions())
		if err != nil {
			return nil, err
		}
	}
	externalURL, err := validateExternalURL(config.ExternalURL)
	if err != nil {
		return nil, err
	}
	if config.Clock == nil {
		config.Clock = time.Now
	}
	if config.Entropy == nil {
		config.Entropy = rand.Reader
	}
	if config.MaximumPageSize == 0 {
		config.MaximumPageSize = defaultMaximumPageSize
	}
	if config.MaximumPatchOperations == 0 {
		config.MaximumPatchOperations = 100
	}
	if config.MaximumSearchCandidates == 0 {
		config.MaximumSearchCandidates = defaultMaximumSearchCandidates
	}
	if config.DocumentationURI != "" {
		parsed, err := url.Parse(config.DocumentationURI)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			return nil, fmt.Errorf("SCIM documentation URI is invalid")
		}
	}
	if config.MaximumPageSize < 1 || config.MaximumPageSize > 10000 || config.MaximumPatchOperations < 1 || config.MaximumPatchOperations > 1000 || config.MaximumSearchCandidates < 1 || config.MaximumSearchCandidates > 100000 {
		return nil, fmt.Errorf("SCIM server limits are invalid")
	}
	return &Server{
		store: config.Store, registry: registry, externalURL: externalURL, resolveScope: config.ResolveScope,
		clock: config.Clock, entropy: config.Entropy, maximumPageSize: config.MaximumPageSize,
		maximumPatchOperations: config.MaximumPatchOperations, maximumSearchCandidates: config.MaximumSearchCandidates,
		authenticationSchemes: append([]AuthenticationScheme(nil), config.AuthenticationSchemes...), documentationURI: config.DocumentationURI,
		publicDiscovery: config.PublicDiscovery, weakETags: config.WeakETags, changePasswordSupported: config.ChangePasswordSupported,
	}, nil
}

// ServeHTTP serves discovery, resource, and Bulk endpoints below ExternalURL's
// path. Authentication must run before this handler.
func (server *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if server == nil || request == nil {
		writeProtocolError(writer, clientError(500, "", "server is unavailable"))
		return
	}
	segments, err := server.routeSegments(request.URL)
	if err != nil {
		writeProtocolError(writer, clientError(404, "", "SCIM endpoint was not found"))
		return
	}
	discovery := len(segments) > 0 && (segments[0] == "ServiceProviderConfig" || segments[0] == "ResourceTypes" || segments[0] == "Schemas")
	scope := ""
	if !discovery || !server.publicDiscovery {
		scope, err = server.resolveScope(request)
		if err != nil || scope == "" || !validString(scope, 1024) {
			writeProtocolError(writer, clientError(401, "", "authentication is required"))
			return
		}
	}
	if len(segments) == 1 && segments[0] == ".search" {
		server.serveSearch(writer, request, scope, nil)
		return
	}
	if len(segments) == 0 {
		if request.Method != http.MethodGet {
			methodNotAllowed(writer, "GET")
			return
		}
		search, err := searchRequestFromQuery(request.URL.Query())
		if err != nil {
			writeProtocolError(writer, clientError(400, "invalidValue", err.Error()))
			return
		}
		server.executeSearch(writer, request, scope, server.registry.definitions(), search)
		return
	}
	if len(segments) == 1 {
		switch segments[0] {
		case "ServiceProviderConfig":
			server.serveServiceProviderConfig(writer, request)
			return
		case "ResourceTypes":
			server.serveResourceTypes(writer, request, "")
			return
		case "Schemas":
			server.serveSchemas(writer, request, "")
			return
		case "Bulk":
			server.serveBulk(writer, request, scope)
			return
		}
	}
	if len(segments) == 2 && segments[0] == "ResourceTypes" {
		server.serveResourceTypes(writer, request, segments[1])
		return
	}
	if len(segments) == 2 && segments[0] == "Schemas" {
		server.serveSchemas(writer, request, segments[1])
		return
	}
	if len(segments) < 1 || len(segments) > 2 {
		writeProtocolError(writer, clientError(404, "", "SCIM endpoint was not found"))
		return
	}
	definition, exists := server.registry.definitionByEndpoint(segments[0])
	if !exists {
		writeProtocolError(writer, clientError(404, "", "SCIM endpoint was not found"))
		return
	}
	if len(segments) == 1 {
		server.serveCollection(writer, request, scope, definition)
		return
	}
	if segments[1] == ".search" {
		server.serveSearch(writer, request, scope, &definition)
		return
	}
	server.serveResource(writer, request, scope, definition, segments[1])
}

func (server *Server) routeSegments(requestURL *url.URL) ([]string, error) {
	base := strings.TrimSuffix(server.externalURL.EscapedPath(), "/")
	path := requestURL.EscapedPath()
	if path == base {
		return []string{}, nil
	}
	if !strings.HasPrefix(path, base+"/") {
		return nil, fmt.Errorf("path is outside SCIM base")
	}
	relative := strings.TrimPrefix(path, base+"/")
	rawSegments := strings.Split(relative, "/")
	segments := make([]string, len(rawSegments))
	for index, raw := range rawSegments {
		decoded, err := url.PathUnescape(raw)
		if err != nil || decoded == "" || decoded == "." || decoded == ".." {
			return nil, fmt.Errorf("path segment is invalid")
		}
		segments[index] = decoded
	}
	return segments, nil
}

func (server *Server) serveCollection(writer http.ResponseWriter, request *http.Request, scope string, definition ResourceDefinition) {
	switch request.Method {
	case http.MethodGet:
		server.listResources(writer, request, scope, definition)
	case http.MethodPost:
		if err := rejectQuery(request.URL.Query(), map[string]bool{"attributes": true, "excludedAttributes": true}); err != nil {
			writeProtocolError(writer, err)
			return
		}
		if err := validateResourceProjection(request.URL.Query(), definition, server.maximumPageSize); err != nil {
			writeProtocolError(writer, err)
			return
		}
		server.createResource(writer, request, scope, definition)
	default:
		methodNotAllowed(writer, "GET, POST")
	}
}

func (server *Server) serveResource(writer http.ResponseWriter, request *http.Request, scope string, definition ResourceDefinition, id string) {
	if !validResourceID(id) {
		writeProtocolError(writer, clientError(404, "", "SCIM resource was not found"))
		return
	}
	if err := rejectQuery(request.URL.Query(), map[string]bool{"attributes": true, "excludedAttributes": true}); err != nil {
		writeProtocolError(writer, err)
		return
	}
	if err := validateResourceProjection(request.URL.Query(), definition, server.maximumPageSize); err != nil {
		writeProtocolError(writer, err)
		return
	}
	switch request.Method {
	case http.MethodGet:
		server.getResource(writer, request, scope, definition, id)
	case http.MethodPut:
		server.replaceResource(writer, request, scope, definition, id)
	case http.MethodPatch:
		server.patchResource(writer, request, scope, definition, id)
	case http.MethodDelete:
		server.deleteResource(writer, request, scope, definition, id)
	default:
		methodNotAllowed(writer, "GET, PUT, PATCH, DELETE")
	}
}

func validateResourceProjection(query url.Values, definition ResourceDefinition, maximumPageSize int) error {
	request, err := searchRequestFromQuery(query)
	if err != nil {
		return clientError(400, "invalidValue", err.Error())
	}
	if _, err := compileSearch(request, definition, maximumPageSize); err != nil {
		return clientError(400, "invalidValue", err.Error())
	}
	return nil
}

func (server *Server) getResource(writer http.ResponseWriter, request *http.Request, scope string, definition ResourceDefinition, id string) {
	var record Record
	err := server.store.Transact(request.Context(), func(transaction Transaction) error {
		var err error
		record, err = transaction.Get(scope, definition.Name, id)
		return err
	})
	if err != nil {
		writeStoreError(writer, err)
		return
	}
	proceed, err := IfNoneMatch(request.Header.Get("If-None-Match"), record.Version, true)
	if err != nil {
		writeProtocolError(writer, clientError(400, "invalidValue", "If-None-Match is malformed"))
		return
	}
	if !proceed {
		writer.Header().Set("ETag", server.entityTag(record.Version))
		writer.WriteHeader(http.StatusNotModified)
		return
	}
	server.writeRecordProjected(writer, http.StatusOK, definition, record, request.URL.Query())
}

func (server *Server) listResources(writer http.ResponseWriter, request *http.Request, scope string, definition ResourceDefinition) {
	search, err := searchRequestFromQuery(request.URL.Query())
	if err != nil {
		writeProtocolError(writer, clientError(400, "invalidValue", err.Error()))
		return
	}
	server.executeSearch(writer, request, scope, []ResourceDefinition{definition}, search)
}

func (server *Server) createResource(writer http.ResponseWriter, request *http.Request, scope string, definition ResourceDefinition) {
	raw, err := readBody(request)
	if err != nil {
		writeProtocolError(writer, err)
		return
	}
	record, err := server.create(request.Context(), scope, "", definition, raw)
	if err != nil {
		writeStoreError(writer, err)
		return
	}
	server.writeRecordProjected(writer, http.StatusCreated, definition, record, request.URL.Query())
}

func (server *Server) create(ctx context.Context, scope, manager string, definition ResourceDefinition, raw []byte) (Record, error) {
	defer clearSecret(raw)
	document, err := DecodeDocument(raw)
	if err != nil {
		return Record{}, clientError(400, "invalidSyntax", "resource JSON is invalid")
	}
	password, err := extractPassword(document, server.changePasswordSupported, definition.Name)
	if err != nil {
		return Record{}, err
	}
	defer clearSecret(password)
	document, indexes, externalID, err := prepareResource(definition, document, CreateMode, "")
	if err != nil {
		return Record{}, clientError(400, "invalidValue", "resource is invalid")
	}
	id, now, err := server.identityAndTime()
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err := server.store.Transact(ctx, func(transaction Transaction) error {
		credentialVersion, err := writePassword(transaction, scope, definition.Name, id, password)
		if err != nil {
			return err
		}
		record, err = newRecord(scope, manager, definition.Name, id, externalID, document, indexes, now, credentialVersion)
		if err != nil {
			return err
		}
		return transaction.Create(record)
	}); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (server *Server) replaceResource(writer http.ResponseWriter, request *http.Request, scope string, definition ResourceDefinition, id string) {
	raw, err := readBody(request)
	if err != nil {
		writeProtocolError(writer, err)
		return
	}
	record, err := server.replace(request.Context(), scope, definition, id, raw, request.Header.Get("If-Match"), request.Header.Get("If-None-Match"))
	if err != nil {
		writeStoreError(writer, err)
		return
	}
	server.writeRecordProjected(writer, http.StatusOK, definition, record, request.URL.Query())
}

func (server *Server) replace(ctx context.Context, scope string, definition ResourceDefinition, id string, raw []byte, ifMatch, ifNoneMatch string) (Record, error) {
	defer clearSecret(raw)
	document, err := DecodeDocument(raw)
	if err != nil {
		return Record{}, clientError(400, "invalidSyntax", "resource JSON is invalid")
	}
	password, err := extractPassword(document, server.changePasswordSupported, definition.Name)
	if err != nil {
		return Record{}, err
	}
	defer clearSecret(password)
	now, err := server.currentTime()
	if err != nil {
		return Record{}, err
	}
	var result Record
	err = server.store.Transact(ctx, func(transaction Transaction) error {
		current, err := transaction.Get(scope, definition.Name, id)
		if err != nil {
			return err
		}
		if err := server.evaluateWritePreconditions(ifMatch, ifNoneMatch, current); err != nil {
			return err
		}
		currentDocument, err := DecodeDocument(current.Data)
		if err != nil {
			return err
		}
		if err := enforceReplacementMutability(definition, currentDocument, document); err != nil {
			return err
		}
		prepared, indexes, externalID, err := prepareResource(definition, document, ReplaceMode, id)
		if err != nil {
			return clientError(400, "invalidValue", "resource is invalid")
		}
		credentialVersion := current.CredentialVersion
		if len(password) != 0 {
			credentialVersion, err = writePassword(transaction, scope, definition.Name, id, password)
			if err != nil {
				return err
			}
		}
		var changed bool
		result, changed, err = replacementRecord(current, externalID, prepared, indexes, now, credentialVersion)
		if err != nil || !changed {
			return err
		}
		return transaction.Replace(result, current.Version)
	})
	return result, err
}

func (server *Server) patchResource(writer http.ResponseWriter, request *http.Request, scope string, definition ResourceDefinition, id string) {
	raw, err := readBody(request)
	if err != nil {
		writeProtocolError(writer, err)
		return
	}
	defer clearSecret(raw)
	patch, err := DecodePatch(raw, server.maximumPatchOperations)
	if err != nil {
		writeProtocolError(writer, clientError(400, "invalidSyntax", "PATCH request is invalid"))
		return
	}
	record, err := server.patch(request.Context(), scope, definition, id, patch, request.Header.Get("If-Match"), request.Header.Get("If-None-Match"))
	if err != nil {
		writeStoreError(writer, err)
		return
	}
	server.writeRecordProjected(writer, http.StatusOK, definition, record, request.URL.Query())
}

func (server *Server) patch(ctx context.Context, scope string, definition ResourceDefinition, id string, patch PatchRequest, ifMatch, ifNoneMatch string) (Record, error) {
	now, err := server.currentTime()
	if err != nil {
		return Record{}, err
	}
	var result Record
	err = server.store.Transact(ctx, func(transaction Transaction) error {
		current, err := transaction.Get(scope, definition.Name, id)
		if err != nil {
			return err
		}
		if err := server.evaluateWritePreconditions(ifMatch, ifNoneMatch, current); err != nil {
			return err
		}
		document, err := DecodeDocument(current.Data)
		if err != nil {
			return err
		}
		document, err = applyPatchDocument(definition, document, patch)
		if err != nil {
			return err
		}
		password, err := extractPassword(document, server.changePasswordSupported, definition.Name)
		if err != nil {
			return err
		}
		defer clearSecret(password)
		currentDocument, err := DecodeDocument(current.Data)
		if err != nil {
			return err
		}
		if err := enforceReplacementMutability(definition, currentDocument, document); err != nil {
			return err
		}
		document, indexes, externalID, err := prepareResource(definition, document, ReplaceMode, id)
		if err != nil {
			return clientError(400, "invalidValue", "patched resource is invalid")
		}
		credentialVersion := current.CredentialVersion
		if len(password) != 0 {
			credentialVersion, err = writePassword(transaction, scope, definition.Name, id, password)
			if err != nil {
				return err
			}
		}
		var changed bool
		result, changed, err = replacementRecord(current, externalID, document, indexes, now, credentialVersion)
		if err != nil || !changed {
			return err
		}
		return transaction.Replace(result, current.Version)
	})
	return result, err
}

func (server *Server) deleteResource(writer http.ResponseWriter, request *http.Request, scope string, definition ResourceDefinition, id string) {
	err := server.remove(request.Context(), scope, definition, id, request.Header.Get("If-Match"), request.Header.Get("If-None-Match"))
	if err != nil {
		writeStoreError(writer, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (server *Server) remove(ctx context.Context, scope string, definition ResourceDefinition, id, ifMatch, ifNoneMatch string) error {
	now, err := server.currentTime()
	if err != nil {
		return err
	}
	return server.store.Transact(ctx, func(transaction Transaction) error {
		current, err := transaction.Get(scope, definition.Name, id)
		if err != nil {
			return err
		}
		if err := server.evaluateWritePreconditions(ifMatch, ifNoneMatch, current); err != nil {
			return err
		}
		tombstone := Tombstone{Scope: current.Scope, ResourceType: current.ResourceType, ID: current.ID, ExternalID: current.ExternalID, Manager: current.Manager, Version: current.Version, DeletedAt: now}
		return transaction.Delete(scope, definition.Name, id, current.Version, tombstone)
	})
}

func evaluateWritePreconditions(ifMatch, ifNoneMatch string, current Record) error {
	allowed, err := IfMatch(ifMatch, current.Version, true)
	if err != nil {
		return clientError(400, "invalidValue", "If-Match is malformed")
	}
	if !allowed {
		return ErrPrecondition
	}
	allowed, err = IfNoneMatch(ifNoneMatch, current.Version, true)
	if err != nil {
		return clientError(400, "invalidValue", "If-None-Match is malformed")
	}
	if !allowed {
		return ErrPrecondition
	}
	return nil
}

func (server *Server) evaluateWritePreconditions(ifMatch, ifNoneMatch string, current Record) error {
	if server.weakETags && ifMatch != "" {
		candidates, err := parseEntityTags(ifMatch)
		if err != nil {
			return clientError(400, "invalidValue", "If-Match is malformed")
		}
		currentTag, err := parseSingleStrongTag(current.Version)
		if err != nil {
			return err
		}
		matched := candidates.star
		for _, candidate := range candidates.tags {
			if candidate.opaque == currentTag {
				matched = true
			}
		}
		if !matched {
			return ErrPrecondition
		}
		ifMatch = ""
	}
	return evaluateWritePreconditions(ifMatch, ifNoneMatch, current)
}

func (server *Server) identityAndTime() (string, time.Time, error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	id, err := NewResourceID(server.entropy)
	if err != nil {
		return "", time.Time{}, err
	}
	now := server.clock().UTC()
	if now.IsZero() {
		return "", time.Time{}, fmt.Errorf("SCIM clock returned zero time")
	}
	return id, now, nil
}

func (server *Server) currentTime() (time.Time, error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	now := server.clock().UTC()
	if now.IsZero() {
		return time.Time{}, fmt.Errorf("SCIM clock returned zero time")
	}
	return now, nil
}

func (server *Server) writeRecord(writer http.ResponseWriter, status int, definition ResourceDefinition, record Record) {
	server.writeRecordProjected(writer, status, definition, record, nil)
}

func (server *Server) writeRecordProjected(writer http.ResponseWriter, status int, definition ResourceDefinition, record Record, query url.Values) {
	document, err := server.renderRecord(definition, record)
	if err != nil {
		writeProtocolError(writer, clientError(500, "", "stored resource is invalid"))
		return
	}
	if query != nil {
		search, err := searchRequestFromQuery(query)
		if err != nil {
			writeProtocolError(writer, clientError(400, "invalidValue", err.Error()))
			return
		}
		plan, err := compileSearch(search, definition, server.maximumPageSize)
		if err != nil {
			writeProtocolError(writer, clientError(400, "invalidValue", err.Error()))
			return
		}
		document, err = projectDocument(document, plan.include, plan.exclude, definition)
		if err != nil {
			writeProtocolError(writer, clientError(500, "", "resource projection failed"))
			return
		}
	}
	location := server.resourceLocation(definition, record.ID)
	writer.Header().Set("ETag", server.entityTag(record.Version))
	writer.Header().Set("Location", location)
	writer.Header().Set("Content-Location", location)
	writeJSON(writer, status, document)
}

func (server *Server) renderRecord(definition ResourceDefinition, record Record) (Document, error) {
	if err := validateRecord(record); err != nil || record.ResourceType != definition.Name {
		return nil, fmt.Errorf("stored record is invalid")
	}
	document, err := DecodeDocument(record.Data)
	if err != nil {
		return nil, err
	}
	prepared, indexes, externalID, err := prepareResource(definition, document, ReplaceMode, record.ID)
	if err != nil || externalID != record.ExternalID || !reflect.DeepEqual(indexes, record.Indexes) {
		return nil, fmt.Errorf("stored resource violates its schema")
	}
	canonical, err := canonicalDocument(prepared)
	if err != nil || !bytes.Equal(canonical, record.Data) {
		return nil, fmt.Errorf("stored resource is not canonical")
	}
	document = prepared
	document["id"] = record.ID
	document["meta"] = map[string]any{
		"resourceType": record.ResourceType,
		"created":      record.Created.UTC().Format(time.RFC3339Nano),
		"lastModified": record.LastModified.UTC().Format(time.RFC3339Nano),
		"version":      server.entityTag(record.Version),
		"location":     server.resourceLocation(definition, record.ID),
	}
	return document, nil
}

func (server *Server) entityTag(strong string) string {
	if server.weakETags {
		return "W/" + strong
	}
	return strong
}

func (server *Server) resourceLocation(definition ResourceDefinition, id string) string {
	result := *server.externalURL
	base := strings.TrimSuffix(server.externalURL.Path, "/")
	result.Path = base + "/" + definition.Endpoint + "/" + id
	result.RawPath = server.externalURL.EscapedPath() + "/" + definition.Endpoint + "/" + url.PathEscape(id)
	return result.String()
}

func readBody(request *http.Request) ([]byte, error) {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(request.Header.Get("Content-Type"), ";")[0]))
	if mediaType != "application/scim+json" && mediaType != "application/json" {
		return nil, clientError(415, "", "Content-Type must be application/scim+json")
	}
	reader := io.LimitReader(request.Body, MaximumResourceBytes+1)
	raw, err := io.ReadAll(reader)
	if err != nil || len(raw) == 0 || len(raw) > MaximumResourceBytes {
		return nil, clientError(413, "tooLarge", "request body is invalid or too large")
	}
	return raw, nil
}

func rejectQuery(query url.Values, admitted map[string]bool) error {
	for key, values := range query {
		if !admitted[key] || len(values) != 1 {
			return clientError(400, "invalidValue", "query parameter is unsupported or repeated")
		}
	}
	return nil
}

func queryInteger(query url.Values, name string, fallback int) (int, error) {
	value := query.Get(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func methodNotAllowed(writer http.ResponseWriter, allow string) {
	writer.Header().Set("Allow", allow)
	writeProtocolError(writer, clientError(405, "", "HTTP method is not supported"))
}

func writeStoreError(writer http.ResponseWriter, err error) {
	var protocol *ProtocolError
	switch {
	case errors.As(err, &protocol):
		writeProtocolError(writer, protocol)
	case errors.Is(err, ErrNotFound):
		writeProtocolError(writer, clientError(404, "", "SCIM resource was not found"))
	case errors.Is(err, ErrConflict), errors.Is(err, ErrTombstoned):
		writeProtocolError(writer, clientError(409, "uniqueness", "SCIM resource conflicts with existing state"))
	case errors.Is(err, ErrPrecondition):
		writeProtocolError(writer, clientError(412, "", "SCIM resource precondition failed"))
	default:
		writeProtocolError(writer, clientError(500, "", "SCIM storage operation failed"))
	}
}

func writeProtocolError(writer http.ResponseWriter, err error) {
	failure := &ProtocolError{Status: 500, Detail: "SCIM operation failed"}
	var typed *ProtocolError
	if errors.As(err, &typed) && typed.Status >= 400 && typed.Status <= 599 && typed.Detail != "" {
		failure = typed
	}
	response, buildErr := NewError(failure.Status, failure.SCIMType, failure.Detail)
	if buildErr != nil {
		response = ErrorResponse{Schemas: []string{ErrorSchema}, Status: "500", Detail: "SCIM operation failed"}
		failure.Status = 500
	}
	writeJSON(writer, failure.Status, response)
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	encoded, err := json.Marshal(value)
	if err != nil {
		status = http.StatusInternalServerError
		encoded = []byte(`{"schemas":["urn:ietf:params:scim:api:messages:2.0:Error"],"status":"500","detail":"SCIM response encoding failed"}`)
	}
	writer.Header().Set("Content-Type", "application/scim+json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_, _ = writer.Write(encoded)
}
