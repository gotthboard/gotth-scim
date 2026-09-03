package scim

import (
	"net/http"
	"strings"
)

func (server *Server) serveServiceProviderConfig(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, "GET")
		return
	}
	if err := rejectDiscoveryQuery(request); err != nil {
		writeProtocolError(writer, err)
		return
	}
	response := map[string]any{
		"schemas":               []string{ServiceProviderConfigSchema},
		"patch":                 map[string]any{"supported": true},
		"bulk":                  map[string]any{"supported": true, "maxOperations": MaximumBulkOperations, "maxPayloadSize": MaximumBulkBytes},
		"filter":                map[string]any{"supported": true, "maxResults": server.maximumPageSize},
		"changePassword":        map[string]any{"supported": server.changePasswordSupported},
		"sort":                  map[string]any{"supported": true},
		"etag":                  map[string]any{"supported": true},
		"authenticationSchemes": append([]AuthenticationScheme(nil), server.authenticationSchemes...),
		"meta":                  map[string]any{"resourceType": "ServiceProviderConfig", "location": strings.TrimSuffix(server.externalURL.String(), "/") + "/ServiceProviderConfig"},
	}
	if server.documentationURI != "" {
		response["documentationUri"] = server.documentationURI
	}
	writeJSON(writer, http.StatusOK, response)
}

func (server *Server) serveResourceTypes(writer http.ResponseWriter, request *http.Request, id string) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, "GET")
		return
	}
	if err := rejectDiscoveryQuery(request); err != nil {
		writeProtocolError(writer, err)
		return
	}
	resources := make([]map[string]any, 0)
	for _, definition := range server.registry.definitions() {
		if id != "" && id != definition.Name {
			continue
		}
		extensions := make([]map[string]any, 0, len(definition.Extensions))
		for _, extension := range definition.Extensions {
			extensions = append(extensions, map[string]any{"schema": extension.Schema, "required": extension.Required})
		}
		location := server.discoveryLocation("ResourceTypes", definition.Name)
		resource := map[string]any{
			"schemas": []string{ResourceTypeSchema}, "id": definition.Name, "name": definition.Name,
			"endpoint": "/" + definition.Endpoint, "schema": definition.Schema, "schemaExtensions": extensions,
			"meta": map[string]any{"resourceType": "ResourceType", "location": location},
		}
		if definition.Description != "" {
			resource["description"] = definition.Description
		}
		resources = append(resources, resource)
	}
	if id != "" {
		if len(resources) == 0 {
			writeProtocolError(writer, clientError(404, "", "resource type was not found"))
			return
		}
		writeJSON(writer, http.StatusOK, resources[0])
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"schemas": []string{ListResponseSchema}, "totalResults": len(resources), "startIndex": 1, "itemsPerPage": len(resources), "Resources": resources})
}

func (server *Server) serveSchemas(writer http.ResponseWriter, request *http.Request, id string) {
	if request.Method != http.MethodGet {
		methodNotAllowed(writer, "GET")
		return
	}
	if err := rejectDiscoveryQuery(request); err != nil {
		writeProtocolError(writer, err)
		return
	}
	resources := make([]map[string]any, 0)
	seen := make(map[string]struct{})
	for _, definition := range server.registry.definitions() {
		resources = appendSchemaResource(resources, seen, id, definition.Schema, definition.Name, definition.Description, definition.Attributes, server)
		for _, extension := range definition.Extensions {
			name := extension.Name
			if name == "" {
				name = extension.Schema
			}
			resources = appendSchemaResource(resources, seen, id, extension.Schema, name, extension.Description, extension.Attributes, server)
		}
	}
	if id != "" {
		if len(resources) == 0 {
			writeProtocolError(writer, clientError(404, "", "schema was not found"))
			return
		}
		writeJSON(writer, http.StatusOK, resources[0])
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"schemas": []string{ListResponseSchema}, "totalResults": len(resources), "startIndex": 1, "itemsPerPage": len(resources), "Resources": resources})
}

func rejectDiscoveryQuery(request *http.Request) error {
	if request.URL.Query().Get("filter") != "" {
		return clientError(http.StatusForbidden, "", "discovery filtering is not supported")
	}
	// RFC 7644 section 4 requires discovery endpoints to ignore the standard
	// query parameters. Unknown parameters are rejected so typos do not vanish.
	return rejectQuery(request.URL.Query(), map[string]bool{
		"attributes": true, "excludedAttributes": true, "sortBy": true,
		"sortOrder": true, "startIndex": true, "count": true, "filter": true,
	})
}

func appendSchemaResource(resources []map[string]any, seen map[string]struct{}, requested, schema, name, description string, attributes []SchemaAttribute, server *Server) []map[string]any {
	if requested != "" && requested != schema {
		return resources
	}
	if _, exists := seen[schema]; exists {
		return resources
	}
	seen[schema] = struct{}{}
	location := server.discoveryLocation("Schemas", schema)
	return append(resources, map[string]any{
		"schemas": []string{SchemaSchema}, "id": schema, "name": name, "description": description,
		"attributes": attributes, "meta": map[string]any{"resourceType": "Schema", "location": location},
	})
}

func (server *Server) discoveryLocation(collection, id string) string {
	return server.resourceLocation(ResourceDefinition{Endpoint: collection}, id)
}
