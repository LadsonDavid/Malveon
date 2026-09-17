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

- If a check can't be resolved with real confidence (a dynamic URL, a wrapped API client, a path built from a variable), the result is **`NO PROOF — couldn't resolve statically`**. Never guess PASS or FAIL. (Renamed from `NOT TESTED` 2026-09-17 — a real tester kept reading "NOT TESTED" as "hasn't been tested yet, implying a test exists," when the actual meaning is "we looked and found nothing solid, so we refuse to guess." `NO PROOF` says that directly.)
- If a test wasn't actually run and its real output captured, it is **not** `TESTED`. A test file existing, or a test being merely discoverable by a runner, is not the same as it having been run.
- The whole reason this tool exists is that AI agents claim things are done without proof. This tool must never do the same thing to its own users. When in doubt, the answer is `NO PROOF`, not a guess dressed up as a result.

**Static-only is a deliberate choice, not a shortcut.** `Feeling_Sun_6436` (our committed real tester) separates verification into three distinct states he never lets blur together: `code changed` → `checks passed` → `verified on the real host or device`. This tool's PASS only ever claims the first two — proof read directly from the code graph, never from the agent's own word. It must never claim the third state (an actual live run) without a real captured command result behind it; that band stays `NO PROOF`, always. Adding a live server/browser to close that gap would reintroduce the exact human-loop dependency this tool exists to remove — if the tester has to stand up their own app to get a real answer, we haven't removed friction, we've relocated it. Push harder on static/graph logic before ever reaching for live execution.

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

1. `features.Detect(root)` — looks for files whose *name* contains "plan"/"feature"/"checklist"/"todo" (case-insensitive) with a supported extension. Fast, precise, the common case.
2. If that finds nothing (the plan could be named anything — confirmed 2026-09-16), `features.DetectByContent(root)` falls back to checking file *content*: a `.json` file is a candidate only if it parses as an array of objects each with a non-empty `name` field (the real shape this tool expects, not just "is it JSON"); a `.md`/`.markdown` file is a candidate only if it has at least 2 real checklist-syntax lines. `.txt` is deliberately excluded from this fallback — a bare text file with no name hint and no structural markers has no reliable content signal, and guessing there would break the tool's own rule.

**Both tiers recurse the whole tree now (fixed 2026-09-16, was top-level-only).** A real test run against a project keeping its plan in `docs/PLAN.md` found nothing at the root, fell through to the content tier, and a README.md that merely had checklist-looking lines won by default — a confidently wrong result from the tool's own plan detection, the exact failure mode this whole tool exists to catch elsewhere. Recursion skips the same noise directories the extractor already does (`.git`, `node_modules`, `vendor`, `dist`, `build`, `.next`, `.malveon`).

**No assumptions, ever (tightened again 2026-09-16, same day).** The founder's first reaction to the fix above was still "no — directly ask which file is the plan file, there shouldn't be any assumptions anymore," even for a single confident name match. So the earlier "exactly one name match → auto-use and announce" shortcut is gone too. Every candidate this resolves — one or many, found by name or only by content — now gets shown to the user and requires an explicit answer:

- **Exactly one candidate** → shown and confirmed (`found a possible plan file (by name): docs/PLAN.md — use it? [Y/n]`, or `(by content, not by name)` for a weaker match) before use. Saying no prompts for the real path directly.
- More than one candidate, from either tier → the tool asks which one, interactively (numbered list, bare Enter picks the first).
- No candidates (from either tier) → the tool asks for a path directly.
- Not running in a real terminal (stdin isn't a TTY — scripts/CI) → fails immediately with a clear error naming the candidate(s), rather than hanging waiting for input that will never come or silently assuming an answer.

Same rule as everywhere else in this tool, now applied to its own plan-detection step without exception: never guess, and when unsure — which, per the founder's correction, means *always*, unless `--features` was passed explicitly — ask instead of picking silently. `--features <path>` remains the only way to skip being asked, which is what scripts and CI should use.

### 3.2 The nine checks

Most read the same in-memory code graph, built once per run. Frontend overlap risk (3.2.8) scans CSS/JSX directly rather than the route/call graph, and incompleteness (3.2.10) scans raw file text for markers — both answering a different kind of question than the graph-based checks. No live server, no browser, no spinning up the tested app for any of them.

#### 3.2.1 Wiring check — does a frontend action actually reach a real backend

The original, validated check. For each feature: find candidate frontend action nodes (click handlers, form submits, fetch calls) and candidate backend nodes (route handlers) via loose token match on the feature's words. Cross-check with three possible outcomes:

**Noise-word filtering, three layers (all added 2026-09-16 from the same real external test run).** Word-overlap matching is only as good as its notion of "this word doesn't count" — the real test surfaced three distinct ways a shared word can be meaningless, each needing a different fix:

1. **Reserved filenames** (`internal/extractor`'s `reservedBaseNames`). A feature description containing the ordinary word "page" ("Build `/circles` directory page...") matched 75 unrelated files, because in a Next.js App Router app nearly every route segment's UI file is literally named `page.tsx` — that filename carried no real identifying signal, just noise that happened to overlap. `wordsFor` now skips contributing a file's own name as a match word when that name is a framework-reserved/near-universal filename with no topical meaning of its own: Next.js's `page`/`layout`/`loading`/`error`/`template`/`default`/`not-found`/`route`/`global-error`/`middleware`, plus `index` (JS/TS), `main` (Go), `__init__` (Python).
2. **Codebase-specific over-common words** (`internal/graph`'s `commonWordSet`, `commonWordMinCount`/`commonWordFraction`). "Restore `/admin/circles`" matched an unrelated `/api/admin/announcements` route via the shared word "admin" — real, on-topic in general, but appearing in 19 of 292 nodes (6.5%) in this specific app because of its large `/admin/*` section, carrying no more signal there than "page" did. `FindNodesMatching` now computes, per graph, which words exceed both an absolute floor (>15 nodes) *and* a relative share (>5% of all nodes) and excludes them from matching — chosen from the real data (`api` sat at 47.9%, `admin` at 6.5%, both false-positive sources; a genuinely specific word like `circles` sat at 3.4%, correctly unaffected). The absolute floor exists specifically so this never fires on a small graph (a young project, a test fixture) where a real shared word between a genuine pair can easily be 100% of a tiny node count.
3. **General English stopwords** (`internal/graph`'s `stopWords`, a small fixed list — `a`/`the`/`and`/`all`/`for`/`with`/etc.). "All 6 Settings tabs..." matched an unrelated `/api/notifications/read-all` route via "all" — too rare in that graph (one node) to trip the frequency filter, but still not a real signal; it's a coincidence between an ordinary English word and a URL path fragment, not a codebase-frequency problem. A small, fixed, never-needs-tuning stopword list closes the gap the frequency filter structurally can't reach. After this fix, the same feature correctly matched instead via the real word "settings."

- A path exists between a matched frontend and backend node, and the edge is tagged `EXTRACTED` (seen directly in source) → **PASS**.
- No path exists between any matched frontend and backend node → **FAIL**, with the reason stated plainly.
- Only one side has a matching node at all → **`NO PROOF`**, and the reason names exactly which side is missing (`"backend route found, but no matching frontend call"` / `"frontend call found, but no matching backend route"` / `"no matching frontend or backend node found"`, added 2026-09-16 — a reviewer asked "how do we identify the backend exists with no frontend for it," and the old one-size-fits-all "frontend and/or backend" message genuinely couldn't answer that).
- A path only exists via an `INFERRED` edge (a guess, not read directly from source) → **`NO PROOF`**. Never round an inference up to a PASS.

#### 3.2.2 Contract match — does the connection actually agree on shape

Answers "the wiring exists, but do both sides agree on what's being sent/returned" — still shape-level, not business-logic-level, and must never be reported as more than that. Two independent sub-checks, reported and verdicted separately — a pair can match on method and mismatch on fields, or the reverse:

**Method agreement (v1 original scope).** Built on `graph.MatchedPairs` (the same pairs the wiring check proved are connected) — does the call's HTTP method (explicit, or GET by the real Fetch API default when `fetch()` has no options object) equal the route's registered method? Agree → **CONTRACT MATCH**. Disagree, and both sides resolvable → **CONTRACT MISMATCH**, naming both methods. Either side unresolvable, or no wired pair to check in the first place → **NO PROOF**.

**Request-field agreement (added 2026-09-16).** The real, scoped slice of "full body-field comparison": does the call actually send every field the route handler reads off `req.body`? **JS/TS only**, and only when both sides resolve to a plain literal object — see `internal/extractor/bodyshape.go`. A spread (`...x`) anywhere in an object literal marks the *whole* object unresolved rather than guessing at a possibly-incomplete field set; a handler that never touches `req.body` at all resolves to zero fields (a real, provable fact), but any *indirect* read (assigned to a variable first, read via a spread) is silently under-detected — that's the deliberately safe direction of error, since under-detecting a read can only miss a real mismatch, never fabricate a false one. Handler reads a field the call never sends → **CONTRACT MISMATCH**, naming the missing field(s). Every read field is sent → **CONTRACT MATCH**. Either side isn't a literal object, or the language isn't JS/TS → **NO PROOF**. The reverse direction (call sends a field the handler ignores) is never flagged — that's not a bug.

**Still not built, real future work:** Python/Go field-shape coverage (kwargs and struct literals need a different, larger extraction approach than JS/TS's text-scanning); response-shape comparison (fields returned vs. fields the frontend reads afterward); whether the frontend branches on an error status at all.

Label this result `CONTRACT MATCH`, never `CORRECT` — it proves shape consistency, not that the business result is right. State that limit plainly in any report or doc that mentions this check.

#### 3.2.3 Not-in-plan flag — does the code do something the plan never asked for

Treats `features.json` as the locked authority. For every new or changed frontend action / route / backend node in the current diff, check whether it matches any plan entry (same loose token match as 3.2.1).

- No matching plan entry → **NOT IN PLAN — human review**, reported as its own list, separate from PASS/FAIL/NO PROOF.

This doesn't decide whether the thing should exist — it turns "quietly discover a surprise button" into "review a short flagged list," which is a real reduction in review effort, not a claim that the human step is removed.

#### 3.2.4 Hero-act detection — known bug patterns this session's own code introduced, zero self-report

**Rebuilt 2026-09-16.** v1 originally shipped a manual variant: a plain-text self-report from the agent (`--bugs-reported <path>`, one reported bug per line) cross-checked against the session's changed-files set. That manual path is now **removed**. The founder's standing rule for this whole tool is that a self-report is never evidence, only ever a claim — and a hand-typed bug list is exactly that, the same trust problem this check exists to catch elsewhere. It's kept as design history here only because the reasoning that led to the real replacement below built directly on it.

**Corrected scope, 2026-09-16 (second pass):** the founder corrected the framing — the point isn't "find bugs that were created *and* fixed," it's "find bugs that were created in the session," full stop, whether they're still there or not. That reframing surfaced a real gap: the original build only ever looked at the "introduced-then-fixed" half. **Two independent signals now answer this, reported as one check** (confirmed via `/software`'s `evolutionary-architectures` routing: both protect the exact same fitness-function concern — "no known bad pattern introduced this session" — just at different cadence/availability, not two separate checks):

1. **Git baseline (new, always available once a session started, no watcher needed).** Compares each session-changed file's content at the session-start commit (`gitutil.FileAtRef`, `git show ref:path`) against its current on-disk version. A known, named pattern present now and absent at session start → **INTRODUCED THIS SESSION — STILL PRESENT**, a live, unfixed bug. Present at session start too → pre-existing, never flagged, regardless of whether the file was touched again for something unrelated.
2. **Watch history (the original mechanism, needs `malveon watch` running).** For each file that changed this session, checks whether a **known, named bug pattern** (a small catalog — see `internal/checks/heropatterns`, e.g. `if (x = 5)` assignment-in-condition in JS/TS, `if err != nil {}` empty error-handling in Go, bare `except:` in Python) was present in an *earlier* captured snapshot and is **absent from the current version** → **INTRODUCED & FIXED SAME SESSION**. This is the only signal that can see a pattern that appeared and disappeared entirely within the session — invisible to any single before/after diff, which is exactly why `malveon watch` (3.2.9) exists.

- Neither signal available (no session started, and `malveon watch` never run) → reports itself unavailable, same as every other session-scoped check.
- Session started but watch wasn't running → still runs, using the git-baseline signal alone; the report notes plainly that "introduced and fixed" detection specifically didn't run.
- Watcher's heartbeat went stale (crashed) → its signal is unavailable, with the reason stated — an incomplete recording must never be used as if it were the whole session; the git-baseline signal is unaffected by this.
- Deliberately narrow: a small, named pattern catalog, not a general "was this a real bug" judgment. That general case isn't resolvable from static snapshots alone (see the reasoning trail in `progress.md`'s log) — it needs either a self-report (the removed manual path) or actual test execution (the fully rigorous version, still deferred — see section 4).

**Considered and rejected (2026-09-16): sourcing a bug list automatically from commit messages, the way 3.2.7's confidence check sources its claims.** Rejected because it's circular here specifically: it would mean trusting the commit message's own claim of being a "fix" to decide whether something is a bug — the exact self-report this check exists to not trust, just relocated from chat into git. (Confidence-check's automatic sourcing is legitimately different — see 3.2.7's own note on why.)

#### 3.2.5 Session lifecycle (supports 3.2.4)

Two commands, automatic capture — the tester never manually looks up a commit hash:

- `malveon session start` — run once, right before the agent's task begins. Captures the current git ref into local state (e.g. `.malveon/session.json`).
- `malveon check` — runs every check. Not-in-plan, hero-act, confidence, and incompleteness each need a session to have been started. Any that can't run report themselves `SKIPPED`/unavailable with a plain reason — never silently omitted, never run against a guessed baseline.

#### 3.2.6 Overlap — two or more route registrations claiming the same method+path

Not Fowler's "Duplicated Code" smell (two chunks of logic computing the same thing) — this is route *collision*: two different registrations both claiming the same URL space, where only one will ever actually run and which one depends on framework/registration-order internals this tool has no visibility into. Scoped narrow to avoid false positives (routed through `refactoring`'s smell-scoping guidance, 2026-09-16):

- Only `Extracted` (literal) paths are compared against each other — a dynamic path is never guessed into a collision.
- Only routes with a *known* method are compared — same path, different method (`GET /refund` vs. `POST /refund`) is two legitimate routes, not a conflict.
- Path comparison reuses `graph.PathsMatch` (the same wildcard-aware equality wiring/contract already use) — so a literal route and a parameterized one claiming overlapping space (`/profile/123` vs. `/profile/:id`) correctly counts as a collision, not just byte-identical paths.
- **Reachability (added 2026-09-16): a best-effort heuristic, not a proof.** Each extractor now tracks which named function (if any) a route registration's line falls inside — Go gets this free from `go/ast`; JS/TS and Python get it from a brace/indentation-based scan (`internal/extractor`'s `funcRange`/`enclosingFuncFor`, shared machinery also used by the contract check's handler-body scanning). For a colliding node inside a named function, `internal/checks/overlap/reachability.go` counts how many times that function's name appears anywhere else in the codebase (word-boundary text search) — a count of exactly one (just its own declaration) gets it a `"possibly unreachable"` note next to that node's evidence line. Anonymous functions and top-level registrations can't be judged this way and are never flagged. Any scan failure or an unresolved name defaults to "assume referenced" — this signal must never turn an uncertainty into a false unreachable claim. A route inside genuinely dead code this heuristic can't identify (e.g. an anonymous callback, or a named function that's referenced only in a comment) still just counts as a plain registration, same as before.
- **Route-precedence awareness (added 2026-09-17 — a real false positive, then researched before generalizing).** A real test run flagged `/api/circles/[id]` and `/api/circles/mine` (two separate Next.js `route.ts` files) as a collision — but in Next.js App Router, a static segment always beats a dynamic one, deterministically, so that pair was never actually ambiguous. Before assuming "static always wins" applies everywhere, websearched how Express, Flask, FastAPI, Go's stdlib `net/http.ServeMux`/chi/gorilla-mux, and gin each resolve route matching: **Next.js App Router and Go's stdlib `net/http.ServeMux` (1.22+)/chi resolve by *specificity*** — static beats dynamic, order-independent, never ambiguous. **Express, Flask, FastAPI, and gorilla/mux resolve by *registration order*** — a dynamic route registered *before* a static one genuinely makes the static route permanently unreachable, a real bug that must stay flagged. `internal/checks/overlap`'s `isSafePair` only suppresses two confirmed-safe shapes: both nodes are Next.js route files (`extractor.IsNextRouteFile`, exported for this reuse), or same file with the static route registered before the dynamic one (`graph.IsFullyStatic`, exported) — the one ordering safe under every researched router regardless of resolution strategy. Every other static/dynamic pair — different files, not confirmed Next.js — stays flagged, since malveon can't yet tell gorilla/mux from chi or Express from a hypothetical order-independent router, and a false "safe" verdict there would hide a real, order-dependent bug.

No session required — this is a property of the current codebase, not something scoped to "this session's changes."

#### 3.2.7 Confidence check — does the agent's own claim match what was actually verified

Cross-references whatever the agent claimed about a feature against the wiring check's already-computed, evidence-based verdict for that same feature. Same discipline as hero-act: the agent's language is never the proof, only ever a claim to check.

**Claim source is automatic by default (confirmed 2026-09-16 — no manual "ask the agent and paste it" step required):** reads commit messages since session start via `gitutil.CommitMessagesSince`, the same "read what already exists, ask for nothing new" approach as `ChangedFilesSince`. Requires a session to have been started; if not, or if there are no commits yet, the check either reports itself unavailable (no session) or reports a clean, honest empty result (session exists, nothing committed yet — not an error). `--claimed-summary <path>` remains available as an explicit override for anyone whose commit messages are too terse to carry a real claim, or who wants to feed it something else.

**Why this is legitimately different from hero-act's rejected commit-message idea (see 3.2.4's history):** hero-act's commit-message approach was circular — it used the commit message's own self-description ("this is a fix") as *evidence for an objective fact*, which the agent could simply evade by not labeling something a fix. This check doesn't need the commit message to be honestly self-labeled as anything: it only captures whatever confident language the agent actually wrote, if any, then checks that claim against a verdict computed entirely independently (the wiring check). An agent that writes modest, non-confident commit messages doesn't evade anything — it just produces more `NOT CLAIMED` (no claim found) results, the same honest fallback this check has always had.

- A line mentioning a feature's words alongside a confidence phrase ("works," "done," "fully functional," "ready," etc.) counts as a claim for that feature.
- Claimed, and wiring says PASS → **CONFIRMED**.
- Claimed, and wiring says FAIL or NO PROOF → **CONFIDENCE MISMATCH**, quoting the claim line and the real verdict.
- Feature never mentioned anywhere in the claim text → **NOT CLAIMED** (no claim to check) — same as everywhere else, absence of evidence isn't treated as a result. (Renamed from `NOT TESTED` 2026-09-17, deliberately distinct from wiring/contract's `NO PROOF` — the two labels used to be the same word for two different things: `NO PROOF` means "we looked for evidence and found none," `NOT CLAIMED` means the agent never said anything about this feature at all, so there was never a claim to test in the first place.)

This intentionally does **not** parse confidence language as a standalone signal (it's never used as evidence on its own) — it exists purely to catch the gap between what the agent *said* and what was actually *verified* elsewhere in the report.

#### 3.2.8 Frontend overlap risk — positioned elements with no positioning context

Distinct from 3.2.6 (backend route collision) — this one is about CSS, specifically whether a visually overlapping layout is *likely*. This is deliberately never reported as "these two elements overlap on screen" — that fact depends on real rendered geometry (actual content size, viewport), which cannot be known without a browser. See CLAUDE.md's core rule (section 2): this stays a structural-risk signal, not a confirmed-overlap claim.

**v1 scope, "Tailwind first" (confirmed 2026-09-16):** scans `.jsx`/`.tsx`/`.html`/`.vue` files for elements using Tailwind's `absolute` utility class. A file where **no** element anywhere uses `relative`/`sticky`/`fixed` gets every `absolute` element in it flagged — an absolutely-positioned element with no positioning context anywhere in the file will position against the page itself instead of its intended container, a well-known real bug, not a guess.

**`fixed` is never flagged, and counts as valid context (fixed 2026-09-17 — a real false positive, found by reading a real project's `Modal.tsx` and `admin/layout.tsx`).** The original version treated `fixed` identically to `absolute`, requiring a `relative`/`sticky` ancestor for both — CSS-incorrect: `position: fixed` always resolves against the *viewport*, so it never needs an ancestor to anchor to, the same way `absolute` does. It also flagged `Modal.tsx`'s standard `<div className="fixed inset-0">` wrapping `<div className="absolute inset-0">` backdrop pattern, because `absolute`/`fixed` were both excluded from counting as context for anything — but CSS gives any non-static positioned element (`relative`/`sticky`/`fixed`, same as `absolute`) its own containing block, so a `fixed` ancestor **is** valid context for a nested `absolute` child. `escapingClasses` (what gets checked) is now `absolute` only; `positioningContextClasses` (what counts as valid context) is `relative`/`sticky`/`fixed`. `absolute` deliberately still doesn't count as its own context — this check is file-scoped, not ancestor-precise, so one `absolute` anywhere in a file would silently suppress every other `absolute` element's real missing-context bug.

**Plain CSS/SCSS/LESS (added 2026-09-16, same `fixed` fix applied 2026-09-17).** The same file-scoped heuristic applied to an actual `position:` declaration instead of a utility class name: a `.css`/`.scss`/`.less` file with `position: absolute` anywhere and no `position: relative`/`sticky`/`fixed` anywhere gets flagged; `position: fixed` itself is never flagged. Context detection deliberately scans the whole file as **flat text**, not brace-paired to each declaration's enclosing selector — a brace-matching scan would miss a parent selector's own `relative` when a child selector nested inside the same block declares `absolute` (real, common SCSS nesting), producing a false positive; a flat "does this value appear anywhere in the file" search can't undercount a real context declaration that way, at the cost of the same file-scoped/not-ancestor-precise trade-off already accepted below.

**Stated limitation:** file-scoped, not ancestor-precise, for both Tailwind and plain-CSS paths. Neither traces the real JSX/CSS-cascade ancestor chain (that needs full tag-tree/selector-specificity resolution — a real future addition); each only knows whether *any* positioning context exists anywhere in the file.

#### 3.2.9 `malveon watch` — the background capture that makes 3.2.4 possible

A separate, optional, long-running command (`internal/watch`, built 2026-09-16), not a check itself — it's the "camera" 3.2.4 reads from. Run it before the agent's task begins; it watches the repo recursively and, on a **continual** cadence (debounced file-save events, not a fixed timer and not only-when-`check`-runs — see the fitness-function cadence reasoning in `progress.md`'s log for why), copies changed source files into `.malveon/history/` as they're saved. Independent of git entirely — no commits, no session-start ref needed for this specific mechanism.

- **Library:** `github.com/fsnotify/fsnotify` — the standard, actively maintained, pure-Go cross-platform watcher (inotify/kqueue/ReadDirectoryChangesW). Verified this doesn't reintroduce cgo or break cross-compilation (linux/amd64, darwin/arm64, darwin/amd64 all still build clean from this Windows machine after adding it) — it's genuinely the first and only external dependency in `go.mod`, and it earns that spot because building a cross-platform file-watcher from scratch would be reinventing exactly what this library already does correctly.
- **Recursive watching is handled manually** — fsnotify only watches one directory natively; new subdirectories created mid-session are detected and added dynamically. `Chmod` events are filtered as noise. Same `skipDirs` as the extractor (`.git`, `node_modules`, `vendor`, `dist`, `build`, `.malveon`).
- **Honesty mechanism:** the watcher writes its own liveness heartbeat to `.malveon/watch-status.json` throughout, and marks itself cleanly stopped on exit. Anything reading captured history (3.2.4) checks this *first* — a stale heartbeat (crashed process) or a missing status file means the history may be incomplete, and gets reported as unavailable rather than silently trusted as if it were the whole session. A background process that can die silently is exactly the kind of unverifiable claim this tool exists to refuse to make, and that discipline had to apply to the watcher itself, not just the checks reading from it.
- **Stated limitations:** no NFS/SMB support (fsnotify's own constraint — those protocols don't provide filesystem-level change notifications); Linux inotify watch-count and macOS file-descriptor limits can make startup fail on very large repos, and that failure is surfaced, never swallowed.

#### 3.2.10 Incompleteness check — does the code itself admit it's unfinished

**Built 2026-09-16**, in direct response to the founder pushing the same "no self-report, read from code" standard onto the confidence check (3.2.7) that had already been applied to hero-act. Confidence-check's core question — "did the agent claim X works" — is inherently linguistic; it cannot be reduced to pure code structure the way hero-act's "pattern present, then absent" question could. Rather than force a fake structural answer onto a linguistic question, this ships as a genuinely different, additional, narrower check instead, while `confidence.Run` stays exactly as it is (it never trusted its input as evidence in the first place — see 3.2.7).

Scans every source file (`.js`/`.jsx`/`.ts`/`.tsx`/`.py`/`.go`) changed since session start for `TODO`/`FIXME`/`HACK`/`XXX` markers or the phrase "not implemented" (case-insensitive). Confirmed via research that this is real, established static-analysis scope — tools like SonarQube already treat these markers as a legitimate quality signal — and that there is no general way to detect "complete"/"confident-worthy" from code structure alone, only specific, named markers like these.

- Marker found → **FLAGGED**, with file, line, and the matching line's text — real, structural proof the code documents its own gap.
- No markers found → reported clean, but this proves nothing on its own: most finished code has no markers either. **Deliberately one-directional** — a marker's presence is evidence; its absence is not, and this check must never be read as a substitute for confidence-check. They ask different questions: "does the code admit it's unfinished" vs. "does the agent's claim match what was verified."
- No session started → unavailable, same as every other session-scoped check.

### 3.3 Output

One report, sectioned by check type (wiring / contract / overlap / frontend-overlap-risk / not-in-plan / hero-act / incompleteness / confidence). Within each section: one row per feature or finding — the name, the result, the specific reason, and the file/line evidence where relevant. No fixed report format imposed beyond that — keep it plain and readable in a terminal.

**Redesigned 2026-09-17 — a real user on a real 43-feature codebase said the output was unreadable.** The original layout printed every feature in plan order regardless of verdict, repeating the same multi-line explanation after each one — a real run showed the same "no matching frontend or backend node found for this feature" sentence 22 times in a row, each with its own blank-line-separated block, before the reader ever saw a usable summary. Fixed by applying established usability/design principles (`dont-make-me-think`'s scanning/hierarchy guidance, `universal-principles-of-design`'s chunking/progressive-disclosure) rather than guessing at a new layout:

- **An `OVERVIEW` block prints first** — one line per check with its counts (`7 PASS · 0 FAIL · 36 NO PROOF`), before any per-check detail. The reader sees the whole shape of the result — is anything actually broken — without scrolling through it. `internal/report.WriteOverview` takes the `CheckSummary` every other `Write*` function now returns.
- **Within a check, findings are grouped by what they mean, not left in plan order.** Proven-broken (`FAIL`/`CONTRACT MISMATCH`/`CONFIDENCE MISMATCH`) prints in full, first — rare, and the only bucket that needs a human's full attention. Proven-fine (`PASS`/`CONTRACT MATCH`/`CONFIRMED`) compacts to one line plus its evidence — common, just needs confirming, not re-explaining. `NO PROOF`/`NOT CLAIMED` — usually the largest bucket — groups by its *reason* instead of repeating the same sentence per feature: a real run had 36 NO PROOF results sharing 4 reasons, which collapsed ~150 lines of repetition into 4 short headers plus a bullet list of names each.
- **A contract result with a `MATCH` method but a `MISMATCH` on request fields still lands in the full-detail bucket, not the compact one** — the fields disagreement is the real problem there, and bucketing by method verdict alone would have buried it.
- **Findings that repeat per file (frontend overlap risk, incompleteness markers) group under the file once**, instead of repeating the risk sentence or file path for every line in it — a file with 4 flagged lines now prints one header and 4 one-line entries instead of 4 full repeated blocks.
- `cmd/malveon/main.go` renders each check's section into its own buffer first (so every section's summary is known before printing), prints the overview, then flushes the sections in the original order — decouples compute order from print order without duplicating any counting logic.

**Plain-language labels, same day, same feedback round.** The founder's next reaction: "the overview contents are vague! newbie cant [understand] terms like hero act, contract" — `WIRING`/`CONTRACT`/`OVERLAP`/`HERO-ACT` are this tool's own internal names, meaningless to someone reading a report for the first time. Every overview label and section headline now leads with a plain-language phrase instead of the internal name — `"New bugs this session"` instead of `HERO-ACT`, `"Backend route conflicts"` instead of `OVERLAP`, `"Agent's claims vs reality"` instead of `CONFIDENCE` — with the original internal name kept in parentheses right after (`"Backend route conflicts (overlap check)"`), so it's still greppable and still matches the vocabulary this file and `progress.md` already use throughout. The overview label and the section headline below it use the *identical* phrase, so scanning from the summary down to detail is a literal text match, not a translation.

### 3.4 Install / usage (what gets sent to the tester)

- Install the `malveon` binary — a one-line install script per OS (`install.sh` for macOS/Linux, `install.ps1` for Windows, added 2026-09-16) downloads the right release asset, puts it on PATH, and (macOS) clears the Gatekeeper quarantine flag, removing the download/chmod/PATH/Gatekeeper friction of doing it by hand. Manual download from Releases still works too. Single binary — no external tool prerequisite, the extractor is built in, not shelled out. Cross-compiles clean for linux/amd64, darwin/arm64, and windows/amd64 with no cgo, verified 2026-09-11.
- `malveon session start` — captures the current git state before the agent's task begins.
- `malveon watch` (optional, but recommended) — run in the background for the richer hero-act signal, zero self-report (see 3.2.9). Ctrl-C when the agent's task is done. Without it, hero-act still runs on the git-baseline signal alone (catches a bug introduced this session that's still present); only the "introduced and fixed within the session" signal specifically needs the watcher.
- Agent does its implementation work, committing along the way like normal. Nothing manual required from here — no bug list to hand-type, no confidence summary to paste.
- `malveon check` — no flags required. Auto-detects the plan file (see 3.1), reads confidence claims from commit messages automatically (see 3.2.7), scans session-changed files for incompleteness markers automatically (see 3.2.10), and reads known-bug-pattern findings from `malveon watch`'s captured history automatically if it was running (see 3.2.4). `--features`/`--claimed-summary` remain available to skip the automatic behavior or for scripts/CI.
- A small example repo + expected output, so the tester knows what a working run looks like before pointing it at their own real one. `testdata/fixture` in this repo doubles as that example today.

## 4. What's explicitly deferred (do not build yet)

- Recurring-failure memory across sessions (a local history file flagging when the same category of failure shows up again).
- Exit code that blocks a git commit on FAIL.
- Full CSS cascade/specificity resolution for frontend overlap risk (3.2.8's plain-CSS path, added 2026-09-16, covers a flat "does this file have the value anywhere" heuristic, not real selector-specificity or ancestor tracing).
- Full request/response body-shape comparison beyond contract's 2026-09-16 request-field slice: Python/Go field-shape coverage, response-shape comparison, and error-branch checking (3.2.2).
- Live-rendered visual overlap confirmation (an optional, clearly-separate mode that would actually check real geometry) — explicitly not folded into the static checks above; see the founder discussion in `session-context-full.md` section 7 for why this stays a separate, later decision rather than a quiet addition to v1.
- The fully rigorous version of automatic hero-act detection: running the project's own test suite against each `malveon watch` snapshot and watching for fail→pass transitions, instead of (or alongside) the named bad-pattern catalog 3.2.4 actually uses. Needs live execution of the tester's own tests, which is why it stays deferred rather than built quietly around the edge of the static-only rule — the pattern-catalog version (built) is the honest, narrower slice; this is the general case.

These are real, grounded in specific pain points (see `session-context-full.md` section 7 for the full mapping) — they come after v1 (all nine checks in section 3) proves itself with a real person, not before. (Backend overlap detection was originally on this list too — pulled forward into v1 on 2026-09-16, see 3.2.6.)

## 5. Tech choices

- **Go.** Chosen explicitly over the original Python-plus-shell-out-to-graphify plan: `malveon` owns its own static analysis instead of depending on the graphify CLI as a runtime dependency, so a stranger installs one binary, not a Python toolchain plus a separate `graphify extract` step.
- **Own parser, pure Go, no cgo.** Graphify's *model* — tag every edge `EXTRACTED` (seen directly in source) vs. `INFERRED` (a resolved guess), never round an inference up to proof — is what this tool's extractor follows. The actual parsing does not use tree-sitter: `go-tree-sitter` requires cgo (a C compiler), which isn't guaranteed on a tester's machine any more than it was on the build machine, and a cgo binary breaks the "one clean binary, no toolchain" promise this tool exists to keep. Instead: `go/parser`/`go/ast` (stdlib) for Go source, and a hand-rolled literal-argument scanner (regex for the call site, a manual balanced-paren scan for the argument, then a strict literal-vs-dynamic classifier) for JS/TS/Python. Graphify itself is never installed, imported, or shelled out to at runtime — reference only.
- **v1 language coverage: Python, TypeScript, JavaScript, Go.** Covers the common frontend (JS/TS) + backend (Python/Go/Node) stack combinations. Confirmed 2026-09-11; revisit only once Feeling_Sun_6436's actual stack is confirmed, in case it falls outside this set.
- **Next.js App Router route handlers (added 2026-09-16, `internal/extractor/nextjs.go`).** The first real external test run (a real Next.js 16 project) found every one of its 64+ genuinely built backend routes silently reading as missing — `export async function GET(request) {...}` in a file literally named `route.ts` is a completely different registration mechanism than the `app.get(path, handler)` pattern `jsRoutePattern` looks for, and the extractor had never learned it. Fixed: a file matching Next.js's reserved `route.ts`/`.tsx`/`.js`/`.jsx` filename gets scanned for one exported HTTP-method function per verb (`GET`/`POST`/`PUT`/`DELETE`/`PATCH`/`HEAD`/`OPTIONS`, both `export function X` and `export const X =` styles); the URL path is derived from the file's own location under `app/` (or `src/app/`), stripping route-group folders like `(app)` (organizational only, never in the real URL) and normalizing `[id]`/`[...slug]`/`[[...slug]]` to the same `:id` wildcard syntax `graph.PathsMatch` already understands — so these routes plug straight into wiring/contract/overlap with zero changes elsewhere. Confidence is always `Extracted`: the path comes from the file's real location on disk, not a guess. Proven against the real project that found the gap: went from `0 PASS, 43 NO PROOF` to `10 PASS, 0 FAIL, 33 NO PROOF` on the same plan file. **Stated limitation:** App Router only — the older Next.js Pages Router (`pages/api/*.ts`, a single default-exported handler that branches on `req.method` internally) uses a different convention and isn't covered.
- **No live server, no browser, no spinning up the tested app.** Everything in section 3 is static analysis of source files and git history only. (Running the tester's own existing test suite for real captured results is a later addition — not v1.)
- **One external dependency, deliberately: `github.com/fsnotify/fsnotify` (added 2026-09-16, for `malveon watch`, 3.2.9).** Everything else in `go.mod` is stdlib. This is the one case where writing it ourselves would mean reinventing a cross-platform OS-notification wrapper (inotify/kqueue/ReadDirectoryChangesW) that a widely-used, actively maintained library already does correctly — pure Go, no cgo, verified it doesn't break cross-compilation to linux/darwin from this Windows machine.
- **One shared `internal/skipdirs` package for every directory-walking check (added 2026-09-16).** Five packages (`extractor`, `features`, `watch`, `checks/overlap`, `checks/uioverlap`) each used to keep their own independent copy of the same base skip-list (`.git`/`node_modules`/`vendor`/`dist`/`build`/`.malveon`). That duplication caused a real bug: `.next` (Next.js's build output) was added to `features`'s copy but not `extractor`'s, so the extractor spent real time walking Next.js's *compiled, bundled* JS chunks as if they were source — citing generated files as "evidence" instead of the real code that produced them, and diluting the common-word frequency filter (3.2.1) with thousands of generated nodes. Consolidated into one list, `skipdirs.Names`, that every one of those 5 packages imports — adding a new entry now fixes all of them at once. Also broadened past just `.next`: JS/TS framework build output (`.nuxt`, `.svelte-kit`, `.turbo`), Python bytecode cache/virtualenvs (`__pycache__`, `.venv`, `venv`), and test coverage output (`coverage`), applying the same "cover the equivalent for every supported language, not just the one case found" reasoning already used for the reserved-filename fix.

## 6. If more context is needed

`D:\Customer talk\session-context-full.md` has the complete history: the investor rejection that started this, the market research, every Reddit pain point and who said it, the mistakes caught and corrected along the way, the 2026-09-11 Go/own-parser/four-check pivot and why, and why this scope was chosen over the alternatives that were considered and rejected. Read it before making a scope decision that isn't already covered above.

Track day-to-day build progress in `progress.md` in this same folder.
