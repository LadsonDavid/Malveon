package extractor

import (
	"encoding/json"
	"path"
	"path/filepath"
	"strings"
)

// Which framework a file belongs to is decided by the package.json above it,
// never by a folder name alone: a `pages/` folder is a Next.js route in one
// project and a Nuxt page in another, and `routes/` means three different
// things to SvelteKit, Remix and Express.

// packageDeps is every dependency name a package.json lists.
func packageDeps(raw []byte) map[string]bool {
	var pkg struct {
		Dependencies         map[string]json.RawMessage `json:"dependencies"`
		DevDependencies      map[string]json.RawMessage `json:"devDependencies"`
		PeerDependencies     map[string]json.RawMessage `json:"peerDependencies"`
		OptionalDependencies map[string]json.RawMessage `json:"optionalDependencies"`
	}
	out := map[string]bool{}
	if json.Unmarshal(raw, &pkg) != nil {
		return out
	}
	for _, m := range []map[string]json.RawMessage{pkg.Dependencies, pkg.DevDependencies, pkg.PeerDependencies, pkg.OptionalDependencies} {
		for name := range m {
			out[name] = true
		}
	}
	return out
}

// addPackage records the dependencies of the package.json at dir (root-relative,
// slash form, "." for the root), and the package itself when it has a name: a
// workspace package another folder imports by that name.
func (in *jsInfo) addPackage(dir string, raw []byte) {
	if in.deps == nil {
		in.deps = map[string]map[string]bool{}
	}
	in.deps[dir] = packageDeps(raw)

	var pkg struct {
		Name   string `json:"name"`
		Main   string `json:"main"`
		Module string `json:"module"`
		Source string `json:"source"`
		Types  string `json:"types"`
	}
	if json.Unmarshal(raw, &pkg) != nil || pkg.Name == "" || dir == "." {
		return // the root manifest names the project, not a package inside it
	}
	if in.workspace == nil {
		in.workspace = map[string]workspacePackage{}
	}
	ws := workspacePackage{dir: dir}
	for _, entry := range []string{pkg.Source, pkg.Module, pkg.Main, pkg.Types} {
		if entry != "" {
			ws.entries = append(ws.entries, strings.TrimSuffix(path.Join(dir, entry), path.Ext(entry)))
		}
	}
	in.workspace[pkg.Name] = ws
}

// workspacePackage is a package that lives in this repo and is imported by its
// name (`import { db } from "@acme/db"`).
type workspacePackage struct {
	dir     string   // root-relative folder, slashes
	entries []string // where its package.json says the code starts, extension removed
}

// workspaceImport turns a bare import ("@acme/db", "@acme/db/schema") of a
// package in this repo into the places its file may be.
func (in *jsInfo) workspaceImport(spec string) (bases []string, ok bool) {
	if in == nil || len(in.workspace) == 0 {
		return nil, false
	}
	name, sub := spec, ""
	parts := strings.Split(spec, "/")
	switch {
	case strings.HasPrefix(spec, "@") && len(parts) >= 2:
		name, sub = parts[0]+"/"+parts[1], strings.Join(parts[2:], "/")
	case len(parts) >= 2:
		name, sub = parts[0], strings.Join(parts[1:], "/")
	}
	ws, found := in.workspace[name]
	if !found {
		return nil, false
	}
	if sub == "" {
		bases = append(bases, ws.entries...)
		return append(bases, path.Join(ws.dir, "src/index"), path.Join(ws.dir, "index"), path.Join(ws.dir, "src/main")), true
	}
	return []string{path.Join(ws.dir, sub), path.Join(ws.dir, "src", sub)}, true
}

// hasDep reports whether any package.json at or above file's folder lists a
// dependency whose name is one of names, or starts with one of the names that
// end in "/" (a scope: "@remix-run/").
func (in *jsInfo) hasDep(file string, names ...string) bool {
	if in == nil {
		return false
	}
	dir := path.Dir(filepath.ToSlash(file))
	for {
		for dep := range in.deps[dir] {
			for _, n := range names {
				if dep == n || (strings.HasSuffix(n, "/") && strings.HasPrefix(dep, n)) {
					return true
				}
			}
		}
		if dir == "." || dir == "/" || dir == "" {
			return false
		}
		dir = path.Dir(dir)
	}
}

// depDir is the folder of the nearest package.json at or above file that lists
// one of names — the root of that framework's project.
func (in *jsInfo) depDir(file string, names ...string) (string, bool) {
	if in == nil {
		return "", false
	}
	dir := path.Dir(filepath.ToSlash(file))
	for {
		for dep := range in.deps[dir] {
			for _, n := range names {
				if dep == n || (strings.HasSuffix(n, "/") && strings.HasPrefix(dep, n)) {
					return dir, true
				}
			}
		}
		if dir == "." || dir == "/" || dir == "" {
			return "", false
		}
		dir = path.Dir(dir)
	}
}

// projectHasDep is hasDep for the whole project: any package.json lists one of names.
func (in *jsInfo) projectHasDep(names ...string) bool {
	if in == nil {
		return false
	}
	for _, deps := range in.deps {
		for dep := range deps {
			for _, n := range names {
				if dep == n || (strings.HasSuffix(n, "/") && strings.HasPrefix(dep, n)) {
					return true
				}
			}
		}
	}
	return false
}
