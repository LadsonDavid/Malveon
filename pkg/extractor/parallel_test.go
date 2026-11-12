package extractor

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// The graph built with every core must be the graph built with one: the files are scanned
// side by side but merged in walk order, and nothing may depend on which finished first.
func TestExtractGivesTheSameGraphWhateverTheCoreCount(t *testing.T) {
	roots := []string{"../../testdata/fixture", "../../testdata/fixture-go", "../../testdata/fixture-python", "../../testdata/fixture-java", "../../testdata/fixture-ruby", "../../testdata/fixture-php", "../../testdata/fixture-rust", "../../testdata/fixture-dotnet", bigTree(t)}
	for _, root := range roots {
		prev := runtime.GOMAXPROCS(1)
		one, err1 := Extract(root)
		runtime.GOMAXPROCS(8)
		many, err2 := Extract(root)
		runtime.GOMAXPROCS(prev)
		if err1 != nil || err2 != nil {
			t.Fatalf("%s: %v %v", root, err1, err2)
		}
		if !reflect.DeepEqual(one, many) {
			t.Fatalf("%s: the graph differs between one core and eight", root)
		}
		if len(one.Files) == 0 {
			t.Fatalf("%s: nothing was read", root)
		}
	}
}

// bigTree writes a few hundred small files of several kinds, enough to keep every worker busy.
func bigTree(t *testing.T) string {
	dir := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", `{"name":"app","dependencies":{"express":"4","react":"18"}}`)
	for i := 0; i < 120; i++ {
		write(fmt.Sprintf("server/routes/r%d.js", i), fmt.Sprintf("const express = require('express')\nconst router = express.Router()\nrouter.get('/api/item%d', (req, res) => res.json({}))\nrouter.post('/api/item%d', (req, res) => res.json(req.body))\nmodule.exports = router\n", i, i))
		write(fmt.Sprintf("web/c%d.jsx", i), fmt.Sprintf("import { helper%d } from './h%d'\nexport function C%d() {\n  const go = () => fetch('/api/item%d', { method: 'POST', body: JSON.stringify({ a: 1 }) })\n  return <button onClick={go}>Save %d</button>\n}\n", i, i, i, i, i))
		write(fmt.Sprintf("web/h%d.js", i), fmt.Sprintf("export function helper%d(x) { return x + %d }\n", i, i))
		write(fmt.Sprintf("py/app%d.py", i), fmt.Sprintf("from flask import Flask\napp = Flask(__name__)\n@app.route('/py/%d', methods=['GET'])\ndef f%d():\n    return 'ok'\n", i, i))
		write(fmt.Sprintf("go/s%d.go", i), fmt.Sprintf("package s\nimport \"net/http\"\nfunc init() { http.HandleFunc(\"/go/%d\", func(w http.ResponseWriter, r *http.Request) {}) }\n", i))
	}
	write("web/broken.go", "package x\nfunc (") // a syntax error in one file must not stop the others
	return dir
}
