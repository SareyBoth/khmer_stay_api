package http

import (
	"strings"
	"testing"

	"github.com/khmerstay/gin-api/internal/config"
)

func TestOpenAPIDocumentsEveryVersionedRoute(t *testing.T) {
	router := NewRouter(Dependencies{Config: config.Config{AppEnv: "test", UploadsDir: t.TempDir()}})
	document := openAPIDocument()
	paths, ok := document["paths"].(map[string]any)
	if !ok {
		t.Fatal("OpenAPI document does not contain a paths object")
	}

	for _, route := range router.Routes() {
		if !strings.HasPrefix(route.Path, "/api/v1") {
			continue
		}
		path := strings.TrimPrefix(route.Path, "/api/v1")
		path = strings.ReplaceAll(path, ":id", "{id}")
		pathItem, exists := paths[path].(map[string]any)
		if !exists {
			t.Errorf("route %s %s is missing from OpenAPI", route.Method, route.Path)
			continue
		}
		if _, exists = pathItem[strings.ToLower(route.Method)]; !exists {
			t.Errorf("method %s for %s is missing from OpenAPI", route.Method, path)
		}
	}
}

func TestOpenAPIPublicRoutesDoNotRequireBearerToken(t *testing.T) {
	document := openAPIDocument()
	paths := document["paths"].(map[string]any)
	for _, path := range []string{"/health", "/auth/login", "/auth/register", "/auth/google", "/locations/provinces"} {
		pathItem := paths[path].(map[string]any)
		for method, rawOperation := range pathItem {
			operation := rawOperation.(map[string]any)
			security, exists := operation["security"]
			if !exists {
				t.Errorf("public operation %s %s inherits bearer authentication", strings.ToUpper(method), path)
				continue
			}
			if len(security.([]any)) != 0 {
				t.Errorf("public operation %s %s has non-empty security", strings.ToUpper(method), path)
			}
		}
	}
}
