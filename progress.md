# Progress — malveon check v1

> Update this file as implementation continues. See `CLAUDE.md` in this folder for the full spec behind every item here.

## Status

**Phase:** All 4 checks (wiring, contract, not-in-plan, hero-act) + session lifecycle built, tested, and proven end-to-end together through the real CLI. Packaging/distribution and the actual demo message to Feeling_Sun_6436 are what's left.
**Last updated:** 2026-09-11

## Locked decisions (quick reference)

| Decision | Answer |
|---|---|
| Language | Go |
| Parser | Own, pure Go — no cgo, no tree-sitter (see note below), graphify used as a design reference only |
| v1 language coverage | Python, TypeScript, JavaScript, Go |
| Checks in v1 | Wiring, Contract match, Not-in-plan flag, Hero-act detection (4 total) — **all 4 built** |
| Contract match v1 scope | HTTP method agreement only (real, extracted, tested) — full request/response body-shape comparison is a later feature, not built |
| Hero-act granularity | File-level, not line-level (git diff parsed for changed files, not changed line ranges) — a stated simplification, not hidden |
| Hero-act self-report input | Plain text, one reported bug per line |
| Session-start snapshot | Automatic (`malveon session start` captures `git rev-parse HEAD`) |
| Deferred (not in v1) | Overlap/duplicate detection, recurring-failure memory, commit-blocking exit code |
| First real tester | Feeling_Sun_6436 (Reddit) — committed to running it against their own real repo |

**Architecture correction made during implementation (2026-09-11):** the original plan called for `go-tree-sitter` (cgo-based) bindings. This machine has no C compiler, and more importantly a cgo binary would have quietly broken the "one clean binary, no toolchain" promise for the tester too. Switched to a pure-Go extractor: `go/parser`/`go/ast` (stdlib) for Go source, and a hand-rolled literal-argument scanner (regex for the call site + manual balanced-paren scan for the argument) for JS/TS/Python. `go.mod` has zero external dependencies. Verified this actually pays off: cross-compiled clean binaries for linux/amd64, darwin/arm64, and windows/amd64 from this one Windows dev machine with no extra toolchain — that would not have been possible with cgo. `CLAUDE.md` section 5 updated to match.

## What's actually built and tested right now

- **`internal/graph`** — Node/Graph model, `FindNodesMatching`, `PathsMatch` (`:id`/`{id}` wildcards), `MatchedPairs` (the one shared definition of "these two are connected" that both wiring and contract build on, so they can't quietly drift apart). Unit tested.
- **`internal/extractor`** — pure-Go, all 4 languages:
  - JS/TS: route registrations, `fetch`/`axios` calls, and now `fetch`'s actual HTTP method (parses the options object's `method:` field, defaults to GET per the real Fetch API spec when there's no options object — a documented default, not a guess).
  - Python: `@app.route`/`@app.get`-style decorators, `requests.*` calls.
  - Go: real AST via `go/ast` — route registrations (`router.GET`, `http.HandleFunc`) vs. outbound calls (`httpClient.Get`), disambiguated by receiver name so overlapping method vocabularies (`Get`/`Post` mean different things depending on the receiver) don't collide. A real misclassification bug here was caught by the test suite and fixed.
  - Never rounds a dynamic/interpolated path up to a literal — proven via the fixture's `profile-update` NOT TESTED case.
- **`internal/gitutil`** — shells out to the system `git` (not vendored — every repo this runs against already has git). `CurrentRef`, `ChangedFilesSince` (tracked + untracked files). Tested against real temp git repos.
- **`internal/session`** — `Start`/`Load` for `.malveon/session.json`. Tested, including the "no commits yet" failure case.
- **`internal/checks/wiring`** — PASS/FAIL/NOT TESTED. Proven end-to-end.
- **`internal/checks/contract`** — CONTRACT MATCH/MISMATCH/NOT TESTED, HTTP-method-agreement scope. Proven end-to-end, including a real MISMATCH case (frontend calls PUT, backend registered POST) added to the fixture.
- **`internal/checks/planauthority`** — NOT IN PLAN findings, scoped to files actually changed since session start (never flags the whole pre-existing repo). Tested against a real temp git repo: correctly flagged an unplanned route added after session start, correctly left the pre-existing planned route alone.
- **`internal/checks/heroact`** — SELF-INTRODUCED / PRE-EXISTING / NOT RESOLVED, proved via `git diff <session-start>..HEAD`, never inferred. Tested against a real temp git repo covering all three verdicts (including a made-up filename correctly landing on NOT RESOLVED instead of a guess).
- **`internal/report`** — all 4 sections rendered.
- **`cmd/malveon`** — `malveon session start [--root]` and `malveon check --features <path> [--root] [--bugs-reported <path>]`, both real, both run.

**Full end-to-end run** (session start → agent adds an unplanned route + self-reports a bug it just introduced → check) — proved manually against a throwaway git repo, exact CLI a tester would use:

```
WIRING CHECK: 1 PASS
CONTRACT CHECK: 1 MATCH
NOT-IN-PLAN CHECK: [NOT IN PLAN] route /debug-panel
HERO-ACT CHECK: [SELF-INTRODUCED, FOUND & FIXED SAME SESSION]
```

All four checks, working together, correctly, through the real CLI a stranger would run.

## Build checklist

- [x] `extractor` — pure-Go parser for Python/TS/JS/Go, in-memory code graph, EXTRACTED/INFERRED tagging
- [x] `graph` — query layer (`FindNodesMatching`, `PathsMatch`, `MatchedPairs`)
- [x] Prove `extractor` + `graph` against a fixture repo (JS/TS side — see note below on Python/Go)
- [x] `wiring` check — proven end-to-end
- [x] `contract` check — proven end-to-end (method-agreement scope)
- [x] `planauthority` check — proven end-to-end against a real git repo
- [x] `heroact` check — proven end-to-end against a real git repo
- [x] `session` lifecycle — `malveon session start`, tested
- [x] `report` — all 4 checks covered
- [x] `cli` — `session start` and `check` both wired and run for real
- [ ] Packaging — cross-compilation itself is verified working (linux/amd64, darwin/arm64, windows/amd64 all built clean from this machine); still need actual release binaries + a distribution/install story written down
- [ ] Python/Go fixture repos — those two extractors are still unit-tested in isolation only (`extractor_test.go`), not proven end-to-end through a full multi-file fixture + all 4 checks the way JS/TS is. Real gap, stated plainly, not silently assumed to work.

## Known, stated limitations (not gaps hiding as bugs)

- **Contract match is method-agreement only.** Doesn't compare request/response body fields yet. Labeled `CONTRACT MATCH`, never `CORRECT`, specifically so this doesn't get over-read.
- **Hero-act is file-level, not line-level.** If a file was touched at all this session, any reported bug pointing at that file reads as self-introduced — even if the specific line the bug is actually in predates the session. Stated in `CLAUDE.md` and here, not hidden.
- **JS `axios({method: 'post', url: '/x'})` object-call style isn't parsed** — only `axios.get/post/put/delete/patch(...)` method-call style is. Same for Flask's bare `@app.route(...)` without an explicit method — deliberately left `NOT TESTED` rather than guessing GET.

## Demo readiness (before sending to Feeling_Sun_6436)

- [x] Small example repo with frontend + backend, a wired feature, a broken one, a dynamic-path one, and a method-mismatch one (`testdata/fixture`)
- [x] Sample `features.json` for the example repo
- [x] Sample plain-text hero-act self-report + expected result (proved in the manual end-to-end run above)
- [x] Full expected terminal output captured from a real run against the example repo
- [ ] Install steps written and tested from a clean clone (binary exists and works; written install doc doesn't yet)
- [ ] Message drafted to Feeling_Sun_6436 with install steps + example + expected output

## Open questions / blockers

None currently.

## Log

- **2026-09-11 (planning)** — Confirmed Go, own-parser-referencing-graphify, four checks in v1, language coverage, hero-act input format, session-snapshot capture. `CLAUDE.md` and `session-context-full.md` updated.
- **2026-09-11 (implementation, part 1)** — Hit and resolved a real blocker (no C compiler; pivoted from `go-tree-sitter` to pure Go). Built and proved the wiring check end-to-end. One real bug (Go outbound-call misclassification) caught by tests and fixed.
- **2026-09-11 (implementation, part 2)** — Built the remaining 3 checks (contract, not-in-plan, hero-act) plus session lifecycle. Added `internal/gitutil` (shells out to system git) and `internal/session` as shared foundations. Extended the extractor to resolve `fetch`'s actual HTTP method for the contract check. Refactored wiring's pairing logic into `graph.MatchedPairs` so wiring and contract share one definition of "connected" instead of two that could drift. All new packages have real tests against temp git repos, not just fixtures. Extended `testdata/fixture` with a method-mismatch case. Proved the full 4-check pipeline end-to-end manually against a throwaway git repo, matching the exact command sequence a real tester would run. Verified cross-compilation actually works (linux/amd64, darwin/arm64, windows/amd64) — the concrete payoff of going pure-Go instead of cgo.
- **2026-09-16 (manual verification + icon)** — Founder independently ran the full test suite and the real CLI against `testdata/fixture` on their own machine (PowerShell, not the bash used to build it), including the full session-start → not-in-plan → hero-act flow with a real fake "bad agent" scenario (unplanned route + self-reported bug). Every verdict matched what the automated tests assert — first fully independent verification, not just the founder trusting the earlier transcript. Two real PowerShell/bash syntax mismatches surfaced and got fixed in the process (`&&` doesn't work in Windows PowerShell 5.1; `session start` and `check` need the same `--root` or they look for `.malveon/session.json` in different places — both now documented). Also embedded the Malveon logo (`Malveon-logo/image.ico`) into the Windows binary via `rsrc` (`cmd/malveon/rsrc_windows_amd64.syso`, filename-suffixed so it only links into windows/amd64 builds — verified linux/darwin cross-compilation still unaffected, and verified via .NET's `Icon.ExtractAssociatedIcon` that the icon is genuinely embedded, not just assumed).
