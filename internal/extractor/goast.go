package extractor

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/LadsonDavid/beta-test/internal/graph"
)

// routeMethodNames covers net/http, gorilla/mux, gin, and echo style
// registration calls: mux.HandleFunc, router.GET, e.POST, etc.
var routeMethodNames = map[string]string{
	"handlefunc": "",
	"get":        "GET",
	"post":       "POST",
	"put":        "PUT",
	"delete":     "DELETE",
	"patch":      "PATCH",
}

// callMethodNames covers outbound http.Get/http.Post-style calls.
var callMethodNames = map[string]string{
	"get":  "GET",
	"post": "POST",
	"put":  "PUT",
	"do":   "",
}

// scanGo extracts route/network call sites from Go source using the
// standard library's real parser (go/parser, go/ast) — no regex, no
// hand-rolled scanning, because Go gives us this for free.
func scanGo(src string, fset *token.FileSet, file *ast.File) []callSite {
	var sites []callSite

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		name := strings.ToLower(sel.Sel.Name)

		line := fset.Position(call.Pos()).Line

		// Disambiguate receiver first: a receiver named like an HTTP
		// client (http, client, httpClient, ...) making a .Get/.Post/...
		// call is an outbound network call, not a route registration —
		// check that before falling back to the route-name table, since
		// the method-name vocabularies overlap (both have "Get"/"Post").
		if ident, ok := sel.X.(*ast.Ident); ok {
			recv := strings.ToLower(ident.Name)
			if strings.Contains(recv, "http") || strings.Contains(recv, "client") {
				if method, isCall := callMethodNames[name]; isCall {
					if lit, ok := firstStringArg(call); ok {
						sites = append(sites, callSite{
							kind:   graph.NetworkCall,
							method: method,
							line:   line,
							argRaw: lit,
						})
					}
					return true
				}
			}
		}
		if method, isRoute := routeMethodNames[name]; isRoute {
			if lit, ok := firstStringArg(call); ok {
				sites = append(sites, callSite{
					kind:   graph.RouteHandler,
					method: method,
					line:   line,
					argRaw: lit,
				})
			}
			return true
		}
		return true
	})

	return sites
}

// firstStringArg returns the raw quoted-literal text of a call's first
// argument if it's a plain string literal, formatted the same way
// readFirstArg's output would be so literalPath() can classify it.
func firstStringArg(call *ast.CallExpr) (string, bool) {
	if len(call.Args) == 0 {
		return "", false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	// lit.Value already includes the surrounding quotes, e.g. "\"/refund\"".
	unquoted, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return `"` + unquoted + `"`, true
}

func parseGoFile(path, src string) (*token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, err
	}
	return fset, f, nil
}
