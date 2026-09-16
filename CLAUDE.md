# CLAUDE.md

> Operating instructions for working in this repo. Read this before touching anything.

## 0. Communication language

Reply in plain English. This repo is going to be shared with a real outside tester (a stranger from Reddit) and lives on a public-ish GitHub repo, so keep chat replies professional and clear — no Thanglish here (unlike the main Malveon project).

## 1. What this project is

A small, standalone CLI tool called `malveon check`, written in Go. It checks whether an AI coding agent's claimed-done work actually matches a plan — not by asking the agent, but by reading the real code.

This is **not** part of the Malveon monorepo and must never depend on it. A stranger should be able to clone this repo alone and run it, without touching anything else. Nothing from Malveon's internal apps, business logic, or credentials gets copied in here.

**Why this exists:** built in response to a real, confirmed pain point from a week of customer discovery on Reddit (r/cursor, r/AI_Agents, r/AgentsOfAI, r/SaaS). Full background is in `D:\Customer talk\session-context-full.md` if more context is ever needed — this file only has what's needed to actually build v1.

**Who the first real user is:** a Reddit user (`Feeling_Sun_6436`) has explicitly committed to installing this and running it against their own real repo (backend, UI, and tests), then reporting honestly where the evidence matches reality and where it overclaims. This is a real, external test — not a hypothetical. Build for them specifically, not for an imagined average user.

## 2. The core rule — never overclaim

This is the single most important rule in this codebase, more important than any feature: **never report a result the tool doesn't actually have proof for.**

- If a check can't be resolved with real confidence (a dynamic URL, a wrapped API client, a path built from a variable), the result is **`NOT TESTED — couldn't resolve statically`**. Never guess PASS or FAIL.
- If a test wasn't actually run and its real output captured, it is **not** `TESTED`. A test file existing, or a test being merely discoverable by a runner, is not the same as it having been run.
- The whole reason this tool exists is that AI agents claim things are done without proof. This tool must never do the same thing to its own users. When in doubt, the answer is `NOT TESTED`, not a guess dressed up as a result.

**Static-only is a deliberate choice, not a shortcut.** `Feeling_Sun_6436` (our committed real tester) separates verification into three distinct states he never lets blur together: `code changed` → `checks passed` → `verified on the real host or device`. This tool's PASS only ever claims the first two — proof read directly from the code graph, never from the agent's own word. It must never claim the third state (an actual live run) without a real captured command result behind it; that band stays `NOT TESTED`, always. Adding a live server/browser to close that gap would reintroduce the exact human-loop dependency this tool exists to remove — if the tester has to stand up their own app to get a real answer, we haven't removed friction, we've relocated it. Push harder on static/graph logic before ever reaching for live execution.

## 3. v1 scope — build this, nothing else yet

Expanded three times, deliberately, across 2026-09-11 and 2026-09-16 planning sessions — not silent scope creep. Original v1 was one check only (wiring); after working through the reasoning with the founder, six more checks were pulled forward from "deferred"/"not yet scoped" into v1 because each one directly answers a specific, named pain point (either from the original Reddit thread or from the founder's own follow-up review of what was still missing) and none of them requires live execution to implement honestly. What's still deferred (section 4) stays deferred for the same reason it always was: real ideas, but unproven until v1 holds up against a real stranger's repo.

### 3.1 Input

A features/plan file in whatever format the user already has it in — no fixed universal format forced on them. Three formats are supported, dispatched by file extension (see `internal/features`):

**JSON** (`.json`) — the original fixed shape:
```json
[
  { "id": "refund-button", "name": "refund button" }
]
```

**Markdown checklist** (`.md`/`.markdown`) — `- [ ] item`, `- [x] item`, `- item`, `* item`, or `1. item` lines; everything else (headings, prose) is skipped, not guessed at:
```markdown
- [ ] Refund button
- [x] Cancel order
```

**Plain text** (`.txt`, or any other/no extension) — one feature name per non-empty, non-`#`-comment line.

IDs are generated from the name (slugified, de-duplicated) for Markdown/text input, since those formats don't carry an explicit ID.

**Finding the plan file (confirmed 2026-09-16 — the command shouldn't need memorizing):** `--features <path>` is optional, not required. When omitted, resolution runs in two tiers:

1. `features.Detect(root)` — top-level only, not recursive — looks for files whose *name* contains "plan"/"feature"/"checklist"/"todo" (case-insensitive) with a supported extension. Fast, precise, the common case.
2. If that finds nothing (the plan could be named anything — confirmed 2026-09-16), `features.DetectByContent(root)` falls back to checking file *content*: a `.json` file is a candidate only if it parses as an array of objects each with a non-empty `name` field (the real shape this tool expects, not just "is it JSON"); a `.md`/`.markdown` file is a candidate only if it has at least 2 real checklist-syntax lines. `.txt` is deliberately excluded from this fallback — a bare text file with no name hint and no structural markers has no reliable content signal, and guessing there would break the tool's own rule.

Whichever tier produced the candidates, the same resolution applies:

- Exactly one candidate → used automatically, and the choice is printed (`using plan file: PLAN.md`) — never silent.
- More than one candidate → the tool asks which one, interactively (numbered list, bare Enter picks the first).
- No candidates (from either tier) → the tool asks for a path directly.
- Not running in a real terminal (stdin isn't a TTY — scripts/CI) and the answer is ambiguous or missing → fails immediately with a clear error naming the candidates, rather than hanging waiting for input that will never come.

Same rule as everywhere else in this tool: never guess, and when unsure, ask instead of picking silently. `--features` still works exactly as before and skips the whole detection/prompt step, which is what scripts and CI should use.

### 3.2 The seven checks

The first six read the same in-memory code graph, built once per run. The seventh (3.2.8, frontend overlap risk) scans CSS/JSX directly rather than the route/call graph, since it's answering a different kind of question. No live server, no browser, no spinning up the tested app for any of them.

#### 3.2.1 Wiring check — does a frontend action actually reach a real backend

The original, validated check. For each feature: find candidate frontend action nodes (click handlers, form submits, fetch calls) and candidate backend nodes (route handlers) via loose token match on the feature's words. Cross-check with three possible outcomes:

- A path exists between a matched frontend and backend node, and the edge is tagged `EXTRACTED` (seen directly in source) → **PASS**.
- No path exists between any matched frontend and backend node → **FAIL**, with the reason stated plainly.
- A path only exists via an `INFERRED` edge (a guess, not read directly from source), or no matching nodes were found at all → **`NOT TESTED`**. Never round an inference up to a PASS.

#### 3.2.2 Contract match — does the connection actually agree on shape

Answers "the wiring exists, but do both sides agree on what's being sent/returned" — still shape-level, not business-logic-level, and must never be reported as more than that.

**v1 implemented scope: HTTP method agreement only.** Built on `graph.MatchedPairs` (the same pairs the wiring check proved are connected) — does the call's HTTP method (explicit, or GET by the real Fetch API default when `fetch()` has no options object) equal the route's registered method? Agree → **CONTRACT MATCH**. Disagree, and both sides resolvable → **CONTRACT MISMATCH**, naming both methods. Either side unresolvable, or no wired pair to check in the first place → **NOT TESTED**.

**Not yet built, real future work:** full request/response body-field comparison (extract the request shape the frontend call constructs vs. what the backend handler reads; the response shape returned vs. the fields the frontend actually reads afterward; whether the frontend branches on an error status at all). Method agreement was chosen as the v1 slice because it's a real, common bug class (wrong verb) and was extractable and provably correct with what the extractor already had — not because the fuller shape check isn't worth building.

Label this result `CONTRACT MATCH`, never `CORRECT` — it proves shape consistency, not that the business result is right. State that limit plainly in any report or doc that mentions this check.

#### 3.2.3 Not-in-plan flag — does the code do something the plan never asked for

Treats `features.json` as the locked authority. For every new or changed frontend action / route / backend node in the current diff, check whether it matches any plan entry (same loose token match as 3.2.1).

- No matching plan entry → **NOT IN PLAN — human review**, reported as its own list, separate from PASS/FAIL/NOT TESTED.

This doesn't decide whether the thing should exist — it turns "quietly discover a surprise button" into "review a short flagged list," which is a real reduction in review effort, not a claim that the human step is removed.

#### 3.2.4 Hero-act detection — did the agent "find" a bug it just created itself

Needs a session-start git reference (see 3.2.5) and a plain-text self-report from the agent describing what it fixed, one reported item per line.

- Parse each line of the plain-text report for a file path matching the repo tree.
- Check whether that **file** (v1 granularity is file-level, not line-level — see limitation note below) is part of the changed-files set since session-start (`git diff --name-only` plus untracked new files).
  - File is part of the session's own diff → **SELF-INTRODUCED, FOUND & FIXED SAME SESSION** (hero act) — the causality proof is the diff itself: remove the session's changes and the "bug" never existed.
  - File exists in the repo but wasn't touched this session → **PRE-EXISTING, GENUINELY FOUND**.
  - No file reference found in the line, or the referenced file doesn't exist in the repo → **NOT RESOLVED**. Never force a match against unclear free text — same "never guess" rule as everywhere else in this tool.

**Stated limitation:** file-level, not line-level. If a file was touched at all this session, any reported bug pointing at that file reads as self-introduced — even on the (presumably rare) chance the specific line predates the session. Precise line-range diff parsing is real future work, not built in v1.

#### 3.2.5 Session lifecycle (supports 3.2.4)

Two commands, automatic capture — the tester never manually looks up a commit hash:

- `malveon session start` — run once, right before the agent's task begins. Captures the current git ref into local state (e.g. `.malveon/session.json`).
- `malveon check --features features.json --bugs-reported <path>` — runs all four checks. Not-in-plan and hero-act each report themselves `SKIPPED`/unavailable, with a plain reason, if no session was started or (for hero-act specifically) no `--bugs-reported` file was given — never silently omitted, never run against a guessed baseline.

#### 3.2.6 Overlap — two or more route registrations claiming the same method+path

Not Fowler's "Duplicated Code" smell (two chunks of logic computing the same thing) — this is route *collision*: two different registrations both claiming the same URL space, where only one will ever actually run and which one depends on framework/registration-order internals this tool has no visibility into. Scoped narrow to avoid false positives (routed through `refactoring`'s smell-scoping guidance, 2026-09-16):

- Only `Extracted` (literal) paths are compared against each other — a dynamic path is never guessed into a collision.
- Only routes with a *known* method are compared — same path, different method (`GET /refund` vs. `POST /refund`) is two legitimate routes, not a conflict.
- Path comparison reuses `graph.PathsMatch` (the same wildcard-aware equality wiring/contract already use) — so a literal route and a parameterized one claiming overlapping space (`/profile/123` vs. `/profile/:id`) correctly counts as a collision, not just byte-identical paths.
- **Stated limitation:** no reachability analysis. A route registered inside code that's never actually called still counts as a registration here.

No session required — this is a property of the current codebase, not something scoped to "this session's changes."

#### 3.2.7 Confidence check — does the agent's own claim match what was actually verified

Takes a plain-text file of the agent's own summary (`--claimed-summary`) — whatever it said about what it built — and cross-references it against the wiring check's already-computed, evidence-based verdict for each feature the plan mentions. Same discipline as hero-act: the agent's language is never the proof, only ever a claim to check.

- A line mentioning a feature's words alongside a confidence phrase ("works," "done," "fully functional," "ready," etc.) counts as a claim for that feature.
- Claimed, and wiring says PASS → **CONFIRMED**.
- Claimed, and wiring says FAIL or NOT TESTED → **CONFIDENCE MISMATCH**, quoting the claim line and the real verdict.
- Feature never mentioned in the summary → **NOT TESTED** (no claim to check) — same as everywhere else, absence of evidence isn't treated as a result.

This intentionally does **not** parse confidence language as a standalone signal (it's never used as evidence on its own) — it exists purely to catch the gap between what the agent *said* and what was actually *verified* elsewhere in the report.

#### 3.2.8 Frontend overlap risk — positioned elements with no positioning context

Distinct from 3.2.6 (backend route collision) — this one is about CSS, specifically whether a visually overlapping layout is *likely*. This is deliberately never reported as "these two elements overlap on screen" — that fact depends on real rendered geometry (actual content size, viewport), which cannot be known without a browser. See CLAUDE.md's core rule (section 2): this stays a structural-risk signal, not a confirmed-overlap claim.

**v1 scope, "Tailwind first" (confirmed 2026-09-16):** scans `.jsx`/`.tsx`/`.html`/`.vue` files for elements using Tailwind's `absolute`/`fixed` utility classes. A file where **no** element anywhere uses `relative` or `sticky` gets every `absolute`/`fixed` element in it flagged — a positioned element with no positioning context anywhere in the file will position against the page itself instead of its intended container, a well-known real bug, not a guess. `absolute`/`fixed` are deliberately excluded from counting as a "context" for each other — an escaping element sitting near another escaping element isn't a fix, it's the same problem twice.

**Stated limitation:** file-scoped, not ancestor-precise. This does not trace the real JSX parent chain (that needs full tag-tree parsing — a real future addition); it only knows whether *any* positioning context exists anywhere in the file. Plain-CSS-file cascade resolution (the general case beyond Tailwind utility classes) is explicitly deferred, not built — see section 4.

### 3.3 Output

One report, sectioned by check type (wiring / contract / overlap / frontend-overlap-risk / not-in-plan / hero-act / confidence). Within each section: one row per feature or finding — the name, the result, the specific reason, and the file/line evidence where relevant. No fixed report format imposed beyond that — keep it plain and readable in a terminal.

### 3.4 Install / usage (what gets sent to the tester)

- Clone the repo.
- Download/install the `malveon` Go binary (single binary — no external tool prerequisite; the extractor is built in, not shelled out). Cross-compiles clean for linux/amd64, darwin/arm64, and windows/amd64 with no cgo, verified 2026-09-11.
- `malveon session start` — captures the current git state before the agent's task begins.
- (agent does its implementation work; ask it what bugs it fixed and save that to a plain-text file, one item per line)
- `malveon check` — no flags required. Auto-detects the plan file (see 3.1), asks if ambiguous. `--features`/`--bugs-reported`/`--claimed-summary` all remain available for scripts/CI or to skip the prompt; the hero-act and confidence sections report themselves skipped, with a plain reason, if their optional input isn't given.
- A small example repo + expected output, so the tester knows what a working run looks like before pointing it at their own real one. `testdata/fixture` in this repo doubles as that example today.

## 4. What's explicitly deferred (do not build yet)

- Recurring-failure memory across sessions (a local history file flagging when the same category of failure shows up again).
- Exit code that blocks a git commit on FAIL.
- Plain-CSS-file cascade resolution for frontend overlap risk (3.2.8 currently covers Tailwind utility classes only).
- Live-rendered visual overlap confirmation (an optional, clearly-separate mode that would actually check real geometry) — explicitly not folded into the static checks above; see the founder discussion in `session-context-full.md` section 7 for why this stays a separate, later decision rather than a quiet addition to v1.

These are real, grounded in specific pain points (see `session-context-full.md` section 7 for the full mapping) — they come after v1 (all seven checks in section 3) proves itself with a real person, not before. (Backend overlap detection was originally on this list too — pulled forward into v1 on 2026-09-16, see 3.2.6.)

## 5. Tech choices

- **Go.** Chosen explicitly over the original Python-plus-shell-out-to-graphify plan: `malveon` owns its own static analysis instead of depending on the graphify CLI as a runtime dependency, so a stranger installs one binary, not a Python toolchain plus a separate `graphify extract` step.
- **Own parser, pure Go, no cgo.** Graphify's *model* — tag every edge `EXTRACTED` (seen directly in source) vs. `INFERRED` (a resolved guess), never round an inference up to proof — is what this tool's extractor follows. The actual parsing does not use tree-sitter: `go-tree-sitter` requires cgo (a C compiler), which isn't guaranteed on a tester's machine any more than it was on the build machine, and a cgo binary breaks the "one clean binary, no toolchain" promise this tool exists to keep. Instead: `go/parser`/`go/ast` (stdlib) for Go source, and a hand-rolled literal-argument scanner (regex for the call site, a manual balanced-paren scan for the argument, then a strict literal-vs-dynamic classifier) for JS/TS/Python. Zero external dependencies in `go.mod`. Graphify itself is never installed, imported, or shelled out to at runtime — reference only.
- **v1 language coverage: Python, TypeScript, JavaScript, Go.** Covers the common frontend (JS/TS) + backend (Python/Go/Node) stack combinations. Confirmed 2026-09-11; revisit only once Feeling_Sun_6436's actual stack is confirmed, in case it falls outside this set.
- **No live server, no browser, no spinning up the tested app.** Everything in section 3 is static analysis of source files and git history only. (Running the tester's own existing test suite for real captured results is a later addition — not v1.)

## 6. If more context is needed

`D:\Customer talk\session-context-full.md` has the complete history: the investor rejection that started this, the market research, every Reddit pain point and who said it, the mistakes caught and corrected along the way, the 2026-09-11 Go/own-parser/four-check pivot and why, and why this scope was chosen over the alternatives that were considered and rejected. Read it before making a scope decision that isn't already covered above.

Track day-to-day build progress in `progress.md` in this same folder.
