# Progress — malveon check v1

> Update this file as implementation continues. See `CLAUDE.md` in this folder for the full spec behind every item here.

## Status

**Phase:** All 7 checks (wiring, contract, overlap, frontend-overlap-risk, not-in-plan, hero-act, confidence) + session lifecycle + multi-format plan input + plan-file auto-detection + automatic confidence-claim sourcing built, tested, and proven end-to-end through the real CLI. `malveon check` now runs with zero required manual steps except hero-act's `--bugs-reported` (kept manual deliberately — see decisions table). `beta-test/` is now its own real git repo, release binaries built for 4 platforms. Waiting on the founder's go-ahead before anything goes public (push + GitHub Release).
**Last updated:** 2026-09-16

## Locked decisions (quick reference)

| Decision | Answer |
|---|---|
| Language | Go |
| Parser | Own, pure Go — no cgo, no tree-sitter, graphify used as a design reference only |
| v1 language coverage | Python, TypeScript, JavaScript, Go |
| Checks in v1 | Wiring, Contract match, Overlap, Frontend-overlap-risk, Not-in-plan flag, Hero-act, Confidence (7 total) — **all 7 built** |
| Plan input formats | JSON (`.json`), Markdown checklist (`.md`), plain text (anything else) — all normalized to the same internal `Feature` list |
| Contract match v1 scope | HTTP method agreement only — full request/response body-shape comparison is later work, not built |
| Overlap v1 scope | Same-method+same-path route collisions only (via `graph.PathsMatch`, wildcard-aware); no reachability analysis |
| Frontend overlap v1 scope | Tailwind utility classes only ("Tailwind first," confirmed 2026-09-16) — flags `absolute`/`fixed` elements with no `relative`/`sticky` anywhere in the same file. Structural risk only, never a claim of confirmed visual overlap (that needs real rendering, deliberately not built). Plain-CSS cascade resolution and live-render confirmation both explicitly deferred. |
| Confidence check | Claim source is automatic by default — reads commit messages since session start (`gitutil.CommitMessagesSince`), no manual prompt needed. `--claimed-summary` still works as an explicit override. Cross-references whatever claim it finds against the wiring verdict — doesn't parse confidence language as evidence on its own |
| Hero-act automatic sourcing | Considered and rejected — commit-message sourcing would be circular here (trusting the commit's own "this is a fix" label), unlike confidence's automatic sourcing which doesn't need that label. Stays manual (`--bugs-reported`) until real automatic alternatives (background snapshotting + test-suite pass/fail signal) are built |
| Hero-act granularity | File-level, not line-level |
| Hero-act self-report input | Plain text, one reported bug per line |
| Session-start snapshot | Automatic (`malveon session start` captures `git rev-parse HEAD`) |
| Deferred (not in v1) | Recurring-failure memory, commit-blocking exit code, plain-CSS cascade resolution, live-render overlap confirmation |
| First real tester | Feeling_Sun_6436 (Reddit) — committed to running it against their own real repo |
| Distribution | Prebuilt binaries via GitHub Release (chosen over "clone and build," which needs Go installed on the tester's end) |

**Architecture correction (2026-09-11):** dropped `go-tree-sitter` (cgo) for a pure-Go extractor after finding no C compiler on the build machine — also the better call for binary portability generally. See `CLAUDE.md` section 5.

**Repo-structure fix (2026-09-16):** `beta-test/` was nested inside the private `D:\Customer talk` workspace repo, and that outer repo's `origin` remote was pointed at the *public* `github.com/LadsonDavid/beta-test.git` — one `git push` away from leaking all the private Reddit research and session notes. Fixed: `beta-test/` is now its own real git repo (own `.git`, one clean commit, only actual product files) with that remote; the outer private repo now has no remote at all. Nothing had been pushed before this was caught.

## What's actually built and tested right now

- **`internal/graph`** — Node/Graph model, `FindNodesMatching`, `PathsMatch` (`:id`/`{id}` wildcards), `MatchedPairs`. Unit tested.
- **`internal/extractor`** — pure-Go, all 4 languages (JS/TS route+call extraction incl. `fetch` method resolution, Python decorators/`requests.*`, Go via real `go/ast`). Never rounds a dynamic path up to a literal.
- **`internal/features`** — multi-format plan loader (JSON/Markdown/plain-text), one unchanged external signature (`Load(path)`) regardless of format — routed through `philosophy-of-software-design` to keep it a deep module instead of leaking format concepts to callers. Slug-based ID generation with de-dup. Tested against all 3 formats.
- **`internal/gitutil`** — shells out to system `git`. Tested against real temp repos.
- **`internal/session`** — `Start`/`Load` for `.malveon/session.json`. Tested.
- **`internal/checks/wiring`** — PASS/FAIL/NOT TESTED. Proven end-to-end.
- **`internal/checks/contract`** — CONTRACT MATCH/MISMATCH/NOT TESTED. Proven end-to-end.
- **`internal/checks/overlap`** — new. Groups `Extracted`, known-method routes by `graph.PathsMatch` equality; flags groups >1. Scoped via `refactoring` skill guidance to exclude same-path/different-method pairs and unresolved-method pairs from ever being compared. Tested with 4 scenario pairs (true literal collision, wildcard-vs-literal collision, different-method non-collision, unresolved-method non-collision, dynamic-path non-collision) — all correct.
- **`internal/checks/planauthority`** — NOT IN PLAN findings, scoped to session diff. Tested against a real temp git repo.
- **`internal/checks/heroact`** — SELF-INTRODUCED / PRE-EXISTING / NOT RESOLVED via git diff. Tested against a real temp git repo.
- **`internal/checks/confidence`** — new. Cross-references a plain-text agent summary against wiring verdicts per feature; CONFIRMED/CONFIDENCE MISMATCH/NOT TESTED. Tested for all three verdicts, including a real "claims it's fully functional but wiring says NOT TESTED" mismatch case.
- **`internal/report`** — all 6 sections rendered.
- **`cmd/malveon`** — `malveon session start [--root]` and `malveon check --features <path> [--root] [--bugs-reported <path>] [--claimed-summary <path>]`.

**Full end-to-end proof, this session:** a duplicate `/refund` POST registration correctly flagged by overlap; a false "cancel order is done and working perfectly" claim (against a feature with no wiring at all) correctly flagged as CONFIDENCE MISMATCH; the plan given as a `.md` checklist instead of JSON, parsed and matched correctly through the whole pipeline.

## Build checklist

- [x] `extractor`, `graph` — pure-Go, all 4 languages, tested
- [x] `features` — multi-format plan loader (JSON/Markdown/text), tested
- [x] `wiring`, `contract`, `overlap`, `planauthority`, `heroact`, `confidence` — all 6 checks, all proven end-to-end
- [x] `session` lifecycle
- [x] `report`, `cli` — all 6 sections wired, both commands run for real
- [x] Repo split — `beta-test/` is its own git repo, correct remote, private workspace repo's leak risk closed
- [x] Release binaries built (`dist/`: windows-amd64, darwin-arm64, darwin-amd64, linux-amd64) — cross-compilation reverified after the icon addition
- [x] Windows icon embedded (`rsrc_windows_amd64.syso`), verified genuinely present via .NET, verified it doesn't affect non-Windows builds
- [x] README.md written (public-facing entry point, distinct from `CLAUDE.md`)
- [ ] **Push to the public remote** — held pending explicit founder go-ahead (public, hard-to-reverse action)
- [ ] **Create the GitHub Release** with the 4 binaries attached — same, held pending go-ahead
- [ ] Python/Go fixture repos — still unit-tested in isolation only, not proven end-to-end through a full multi-file fixture the way JS/TS is
- [ ] Message drafted to Feeling_Sun_6436

## Known, stated limitations (not gaps hiding as bugs)

- **Contract match is method-agreement only** — no request/response body comparison yet.
- **Hero-act is file-level, not line-level.**
- **Overlap has no reachability analysis** — a duplicate registration inside dead code still counts.
- **Confidence check doesn't parse confidence language as standalone evidence** — it only ever cross-references a claim against the wiring verdict, by design.
- **JS `axios({method:'post', url:'/x'})` object-call style, and Flask's bare `@app.route(...)` without an explicit method, aren't parsed** — left `NOT TESTED` rather than guessed.

## Demo readiness (before sending to Feeling_Sun_6436)

- [x] Example repo with a wired feature, a broken one, a dynamic-path one, a method-mismatch one (`testdata/fixture`)
- [x] Sample plain-text hero-act and confidence inputs + expected results (proved manually this session)
- [x] Full expected terminal output captured from real runs
- [x] Release binaries built
- [ ] Actually pushed + released (blocked on founder go-ahead)
- [ ] Message drafted to Feeling_Sun_6436

## Open questions / blockers

Waiting on founder confirmation to push `beta-test` to the public remote and create the v0.1.0 GitHub Release.

## Log

- **2026-09-11 (planning)** — Confirmed Go, own-parser-referencing-graphify, four checks in v1, language coverage, hero-act input format, session-snapshot capture.
- **2026-09-11 (implementation, part 1)** — Pivoted from `go-tree-sitter` to pure Go (no C compiler on the build machine). Built and proved the wiring check end-to-end.
- **2026-09-11 (implementation, part 2)** — Built contract, not-in-plan, hero-act, and session lifecycle. Added `internal/gitutil` and `internal/session`. Proved the full 4-check pipeline end-to-end. Verified cross-compilation (linux/amd64, darwin/arm64, windows/amd64).
- **2026-09-16 (manual verification + icon)** — Founder independently ran the full test suite and CLI on their own machine (PowerShell), confirming every verdict matched. Embedded the Malveon logo into the Windows binary via `rsrc`, verified genuinely present and cross-platform-safe.
- **2026-09-16 (repo split + release prep)** — Found and fixed the private-repo-remote-pointed-at-public-repo leak risk. Split `beta-test/` into its own real git repo, one clean commit. Wrote `README.md`. Built 4-platform release binaries. Stopped before pushing/releasing pending explicit go-ahead.
- **2026-09-16 (three more checks)** — Founder listed 6 real pain points and asked which were built; 3 were gaps (overlap detection, any-format plan input, and no way to catch the agent's stated confidence contradicting reality). Routed through `/software` (`refactoring` for scoping overlap detection precisely enough to avoid false positives; `philosophy-of-software-design` for keeping the multi-format plan loader a clean deep module instead of a leaky one). Built and tested all three: `internal/features` now accepts JSON/Markdown/plain-text; `internal/checks/overlap` catches colliding route registrations; `internal/checks/confidence` catches the agent's own claim contradicting the wiring verdict. All proven end-to-end through the real CLI, not just unit tests.
- **2026-09-16 (frontend overlap — "overlap" clarified to mean visual UI overlap)** — Founder clarified pain point #3 meant *visual* frontend overlap (elements colliding on screen), not backend route collision. Worked through why that fundamentally needs real rendering (browser/layout engine) to confirm geometrically, which conflicts with the whole tool's no-live-testing identity. Walked through all 10 real CSS-overlap bug categories against what's actually statically knowable vs. render-dependent — the key finding: *stacking/positioning structure* is fully knowable from source, *actual geometric collision* is not. Designed a tiered plan (static structural-risk checks always-on; live confirmation kept separate, optional, and narrow) instead of an all-or-nothing choice. Founder chose "Tailwind first" over full plain-CSS cascade resolution for v1 (much smaller build, matches what most AI-agent-generated frontends actually use). Built `internal/checks/uioverlap`: flags `absolute`/`fixed` Tailwind classes with no `relative`/`sticky` context anywhere in the same file. Caught and fixed a real logic bug during testing (absolute/fixed were wrongly counted as their own "positioning context," silently canceling every finding) — found via the test suite failing, not assumed correct. Proven end-to-end. Now 7 checks total. Live-render confirmation and plain-CSS support both explicitly deferred, not built.
- **2026-09-16 (command UX — plan file auto-detection)** — Founder flagged that `malveon check --features ... --bugs-reported ... --claimed-summary ...` was too long to memorize, and proposed an interactive prompt for finding the plan file (like `AskUserQuestion`). Built `features.Detect(root)` (top-level scan for plan-looking filenames) plus `resolveFeaturesPath` in `cmd/malveon`: one candidate → auto-used and announced; multiple → asks interactively; none → asks for a path; non-interactive (scripted/CI) and ambiguous → fails immediately with a clear error instead of hanging on stdin forever. `--features` still works exactly as before for scripts. `malveon check` now runs with zero required flags in the common case. Proven end-to-end, including the non-interactive fail-fast path (verified it doesn't hang).
- **2026-09-16 (plan file auto-detection, part 2 — content fallback)** — Founder pointed out the real gap: name-based detection does nothing for a plan file with no name hint at all (`sprint3.json`, `notes.md`, anything). Built `features.DetectByContent(root)` as a fallback tier when name-matching finds nothing: `.json` files count only if they actually parse as an array of objects with a `name` field (the real expected shape, not just "valid JSON"); `.md` files count only with ≥2 real checklist-syntax lines. `.txt` deliberately excluded from this fallback — no reliable content signal without a name hint, and guessing there would break the tool's own core rule. Proven end-to-end: a plan named `sprint3.json` with zero name hints got found and used automatically, `README.md` correctly ignored.
- **2026-09-16 (hero-act automation attempt — rejected, correctly)** — Proposed sourcing hero-act's bug list automatically from commit messages (fix-keyword scan). Founder correctly rejected it: trusting a commit message's own "this is a fix" label as evidence is circular — the exact self-report this check exists to not trust, just relocated from chat into git, and gameable by simply not writing "fix" in the message. Routed through `/software` → `why-programs-fail`'s omniscient-debugging concept to find a genuinely self-report-free alternative: confirmed that fully automatic detection needs *some* record of the intermediate broken state, and without either a self-report or actual test execution, there's no way to distinguish "the agent fixed a real bug" from "the agent iterated normally while writing new code" from pure snapshots alone — an information-theoretic limit, not a design gap. Scoped the real path forward (background snapshotting + a known bad-pattern catalog now, test-suite pass/fail signal later) but did not build it this session — documented as deferred in `CLAUDE.md` section 4, hero-act stays on manual `--bugs-reported`.
- **2026-09-16 (confidence check automated)** — Same request applied to the confidence check, but this time it holds up: unlike hero-act, confidence-check doesn't need the commit message to be honestly self-labeled as anything — it only captures whatever confident language the agent actually wrote, then checks that claim against a verdict computed entirely independently (wiring). An agent writing modest commit messages just produces more `NOT TESTED`, not an evasion. Built `gitutil.CommitMessagesSince` and rewired `confidence.Run` to read from it automatically when `--claimed-summary` isn't given (requires a session to have been started). Proven end-to-end with real git commits and zero flags: a commit message claiming "Cancel order is done and working perfectly, ready to ship" was read automatically and correctly flagged as a CONFIDENCE MISMATCH against the real (failing) wiring result.
