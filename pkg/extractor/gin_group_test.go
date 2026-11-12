package extractor

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/LadsonDavid/Malveon/pkg/graph"
)

func routesIn(t *testing.T, files map[string]string) []string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	g, err := Extract(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, n := range g.Nodes {
		if n.Kind == graph.RouteHandler && n.Path != "" {
			out = append(out, n.Method+" "+n.Path)
		}
	}
	sort.Strings(out)
	return out
}

// Found on webbrain-systray: `api := e.Group("/companion/api", guard)` then `api.POST("/accounts/login")`
// serves /companion/api/accounts/login. Groups nest.
func TestGinRouterGroupPrefixIsPartOfTheRoute(t *testing.T) {
	got := routesIn(t, map[string]string{"routes.go": `package app

func routes(e *gin.Engine) {
	e.GET("/health", h)
	api := e.Group("/companion/api", guard)
	api.POST("/accounts/login", h)
	admin := api.Group("/admin")
	admin.DELETE("/users", h)
}
`})
	want := map[string]bool{"GET /health": true, "POST /companion/api/accounts/login": true, "DELETE /companion/api/admin/users": true}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Fatalf("got %v", got)
		}
	}
}
