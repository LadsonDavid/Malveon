package wiring_test

import (
	"github.com/LadsonDavid/Malveon/pkg/extractor"
	"github.com/LadsonDavid/Malveon/pkg/features"
	"github.com/LadsonDavid/Malveon/pkg/wiring"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func incidentProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for f, s := range files {
		p := filepath.Join(root, f)
		if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(p, []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	return root
}
func TestIncidentLocalDelegationAndImportsAreNotBehaviorProof(t *testing.T) {
	for _, consumer := range []string{"unit.test.ts", "testdata/sample.ts", "app.ts"} {
		t.Run(consumer, func(t *testing.T) {
			root := incidentProject(t, map[string]string{"adapter.ts": `export class Adapter { run(){ return this.port.execute() } }`, consumer: `import { Adapter } from './adapter.js'; const a = new Adapter();`})
			g, e := extractor.Extract(root)
			if e != nil {
				t.Fatal(e)
			}
			for _, word := range []string{"server", "servers", "local operations"} {
				r := wiring.Run(g, []features.Feature{{ID: "x", Name: "`Adapter` handles " + word}})[0]
				if r.Verdict != wiring.Skipped || !r.Unchecked || !strings.Contains(r.Reason, "unverified") {
					t.Fatalf("%s: %+v", word, r)
				}
			}
		})
	}
}
func TestIncidentTypeStarAndBrokenCounterpart(t *testing.T) {
	for _, star := range []bool{true, false} {
		barrel := `export type {Other} from './port.js'`
		if star {
			barrel = `export type * from './port.js'`
		}
		root := incidentProject(t, map[string]string{"port.ts": `export interface AgentPort {run():void}; export interface Other {}`, "index.ts": barrel, "app.ts": `import type {AgentPort} from './index.js';`})
		g, e := extractor.Extract(root)
		if e != nil {
			t.Fatal(e)
		}
		if star && len(g.BrokenImports()) > 0 {
			t.Fatalf("valid type-star accused: %+v", g.BrokenImports())
		}
		if !star && len(g.BrokenImports()) == 0 {
			t.Fatal("deliberately missing named export was missed")
		}
	}
}
func TestIncidentCitationAndContinuationPreserveIntent(t *testing.T) {
	root := incidentProject(t, map[string]string{"plan.md": "# External References\n- Hermes uses `cron/monitor.py`\n\n# Requirements\n- `Adapter` handles a job\n  without network access and only after approval.\n", "adapter.ts": "export class Adapter {}"})
	fs, e := features.Load(filepath.Join(root, "plan.md"))
	if e != nil {
		t.Fatal(e)
	}
	if len(fs) != 2 || !fs[0].Reference || !strings.Contains(fs[1].Name, "only after approval") {
		t.Fatalf("lost context: %+v", fs)
	}
	g, e := extractor.Extract(root)
	if e != nil {
		t.Fatal(e)
	}
	r := wiring.Run(g, fs)
	if r[0].Verdict != wiring.Skipped || r[0].Unchecked {
		t.Fatalf("external citation demanded locally: %+v", r[0])
	}
	if r[1].Verdict != wiring.Skipped || !r[1].Unchecked {
		t.Fatalf("mere existence proved approval semantics: %+v", r[1])
	}
}

func TestFixtureRoutesCannotProveProductionConnection(t *testing.T) {
	root := incidentProject(t, map[string]string{"client.js": `fetch('/api/demo')`, "testdata/server.js": `app.get('/api/demo', (req,res)=>res.json({ok:true}));`})
	g, e := extractor.Extract(root)
	if e != nil {
		t.Fatal(e)
	}
	if len(g.RoutesServing("/api/demo")) != 0 {
		t.Fatal("fixture route counted as production server")
	}
	inner, e := extractor.Extract(filepath.Join(root, "testdata"))
	if e != nil {
		t.Fatal(e)
	}
	if len(inner.RoutesServing("/api/demo")) != 1 {
		t.Fatal("explicitly selected fixture root was suppressed")
	}
}
func TestOriginalMalvesSourceColumnIsCitationContext(t *testing.T) {
	root := incidentProject(t, map[string]string{"PLAN.md": "## 15. Ideas taken, and from where\n\n| Idea | Source |\n|---|---|\n| Monitors hashed before any model call | Hermes `cron/monitor.py`, `cron/suggestions.py` |\n"})
	fs, e := features.Load(filepath.Join(root, "PLAN.md"))
	if e != nil {
		t.Fatal(e)
	}
	g, e := extractor.Extract(root)
	if e != nil {
		t.Fatal(e)
	}
	r := wiring.Run(g, fs)[0]
	if r.Verdict == wiring.NotBuilt || r.Verdict == wiring.Working {
		t.Fatalf("citation is neither missing local implementation nor proof of behavior: %+v", r)
	}
}
func TestPortTypeReferenceDoesNotRequireRuntimeImport(t *testing.T) {
	root := incidentProject(t, map[string]string{"ports.ts": `export interface AgentSession {cancel():void}`, "tasks.ts": "import type {AgentSession} from './ports.js';\nexport class Tasks {constructor(private session:AgentSession){} steer(){this.session.cancel()}}"})
	g, e := extractor.Extract(root)
	if e != nil {
		t.Fatal(e)
	}
	for _, line := range []string{"`Tasks` in `tasks.ts` and `AgentSession` in `ports.ts`: messages reach the agent at tool boundaries.", "`Tasks` in `tasks.ts` uses `AgentSession` in `ports.ts` for steering."} {
		r := wiring.Run(g, []features.Feature{{ID: "steering", Name: line}})[0]
		if r.Verdict != wiring.Skipped || !r.Unchecked {
			t.Fatalf("a port's type-only relationship is not a missing runtime link: %+v", r)
		}
	}
}
