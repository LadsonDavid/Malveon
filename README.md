# malveon check

Checks whether an AI coding agent's claimed-done work actually matches your plan — not by asking the agent, but by reading the real code.

Static analysis only. No live server, no browser, nothing runs. If it can't prove something from the code, it says so instead of guessing.

## The problem this solves

AI coding agents say "done" confidently, whether or not it's true. A button gets built with no backend behind it. A backend gets built with nothing calling it. Two handlers silently claim the same route. The agent audits its own work in the same session it just wrote — and usually gives itself a passing grade, even when it shouldn't.

`malveon check` reads the actual code and git history and reports nine things:

- **Wiring** — does a frontend action you planned actually reach a real backend route? A miss says exactly which side is missing — backend built with no frontend, frontend built with no backend, or neither found.
- **Contract** — does the call agree with the route on HTTP method, and (JS/TS, literal request bodies only) does it actually send every field the handler reads off `req.body`? Shape-level only, never a claim the logic is correct.
- **Overlap** — do two or more route registrations silently claim the same method+path? Route-precedence aware: a static/dynamic split across two Next.js route files, or a static route registered before a dynamic one in the same file, is never actually ambiguous and isn't flagged — everything else, including the frameworks where registration order genuinely does create a real dead-route bug (Express, Flask, FastAPI, gorilla/mux), stays flagged. Also flags when one of the colliding registrations sits inside a function that's never referenced anywhere else in the codebase — a real signal it might be dead code, not just a guess.
- **Frontend overlap risk** — is an `absolute` element (Tailwind class or plain CSS `position:` declaration) sitting with no positioning context anywhere in its file (a structural risk signal, not a claim two things visually collide — confirming that needs a real render, which this deliberately doesn't do)? `fixed` is never flagged — it always positions against the viewport — and counts as valid context for a nested `absolute` element, matching real CSS semantics.
- **Not-in-plan** — did the agent build something this session that your plan never asked for?
- **Hero-act** — did this session's own code introduce a known bug pattern — whether it's still sitting there right now, or got fixed along the way? Two zero-self-report signals: a git-baseline comparison (always on once a session started — catches a bug that's still live) and, if `malveon watch` was running, a captured-snapshot comparison (catches one that appeared and disappeared entirely within the session, which a single before/after diff can't see).
- **Incompleteness** — does the code itself admit it's unfinished (`TODO`/`FIXME`/`HACK`/`XXX`/"not implemented" left in a file changed this session)? A marker's presence is real, code-only proof; its absence proves nothing, so this can never substitute for the confidence check below.
- **Confidence** — does the agent's own "it works" claim actually match what got verified? Reads it straight from commit messages, nothing to ask or paste.

Every result is `PASS` / `FAIL` / or `NO PROOF` (or the check-specific equivalent) — never a guess dressed up as an answer.

## Install

**macOS / Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/LadsonDavid/beta-test/main/install.sh | sh
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/LadsonDavid/beta-test/main/install.ps1 | iex
```

Either one downloads the right binary for your machine, puts it on your PATH, and (macOS) clears the Gatekeeper quarantine flag so the first run doesn't get blocked. Open a new terminal afterward and run `malveon` to confirm.

No Go, no toolchain, nothing else to set up — it's a single static binary either way.

**Prefer to do it by hand?** Download the binary for your OS from the [Releases](../../releases) page directly, `chmod +x` it (macOS/Linux), and put it somewhere on your PATH.

## Use

```bash
# once, right before you hand the agent a task
malveon session start
malveon watch &   # background — watches the code as the agent works, Ctrl-C when done

# ... agent does its work, committing along the way like normal ...

malveon check
```

That's it. `malveon check` with no flags: looks for a plan file across the whole project automatically and always confirms with you before using it (never assumes, even a clear single match — see below), reads the agent's confidence claims straight from its commit messages, scans files changed this session for incompleteness markers, and reads known-bug-pattern findings from whatever `malveon watch` captured while it ran — nothing to ask the agent, nothing to paste, for any of it.

`malveon watch` is optional — hero-act still works without it, using the git-baseline signal alone (catches a bad pattern that's still present right now). Running it adds a second signal: catching a bad pattern (a real, named catalog — an accidental `if (x = 5)`, a silently swallowed Go error, a bare Python `except:`) that appeared and got fixed entirely within the session, which the git comparison alone can't see. No self-report involved either way.

Want to override any of the automatic behavior?

```bash
malveon check --features features.json --claimed-summary summary.txt
```

- `--features <path>` — skips the plan auto-detect/prompt, use this exact file.
- `--claimed-summary <path>` — override the automatic commit-message reading with something else, for the confidence check.

All optional — leave any out and that section of the report shows itself skipped, with a plain reason, instead of silently doing nothing.

## The plan file

Whatever format you already keep your plan in — no fixed shape forced on you, and no particular filename or location required (it searches the whole project tree, skipping `node_modules`/`.git`/build output). Auto-detection tries two ways:

1. **By filename first** — anything containing "plan," "feature," "checklist," or "todo," with a supported extension, wherever it actually lives (`docs/PLAN.md` works fine).
2. **By content, if nothing matched by name** — a `.json` file counts if it's actually shaped like a feature list (array of objects with a `name` field), a `.md` file counts if it has real checklist lines (`- [ ]`, `- [x]`, etc.). So `sprint3.json` or `notes.md` gets found too, not just files literally named `plan.json`.

Either way, it never assumes — even one clear match gets shown to you first: `found a possible plan file (by name): docs/PLAN.md — use it? [Y/n]`. Say no and it asks for the real path instead. More than one candidate, or none at all, and it asks the same way. The only way to skip being asked is `--features <path>`.

**JSON** (`.json`):
```json
[
  { "id": "refund-button", "name": "refund button" }
]
```

**Markdown checklist** (`.md`):
```markdown
- [ ] Refund button
- [x] Cancel order
```

**Plain text** (`.txt`, or anything else): one feature name per line.

## Example

Try it against the included example repo first, so you know what a working run looks like:

```bash
malveon check --root testdata/fixture --features testdata/fixture/features.json
```

Python and Go examples (`testdata/fixture-python`, `testdata/fixture-go`) work the same way — same four verdict shapes, different language.

## What it can't do (yet)

- Doesn't prove business logic is *correct* — only that the wiring and HTTP method agree.
- Doesn't run anything live — dynamic URLs, wrapped API clients, and templated paths report `NO PROOF`, never a guessed pass.
- Hero-act only catches a small, named catalog of known bug patterns — not a general "was this a real bug" judgment, which isn't resolvable from static snapshots alone. And it only works if `malveon watch` was actually running; if it crashed or was never started, that section reports itself unavailable rather than guessing from a possibly-incomplete recording.
- Incompleteness is one-directional — a marker's presence is real proof, but its absence proves nothing (most finished code has none either). Never a substitute for the confidence check.
- Overlap's reachability note is a best-effort heuristic (checks whether an enclosing function's name is ever mentioned elsewhere in the codebase), not real call-graph analysis — an anonymous handler or dead code it can't attribute to a named function still just counts as a plain registration.
- Contract's field-agreement check is JS/TS only, and only fires when both sides are a literal object (no spread, no variable) — Python/Go request bodies and response-shape comparison aren't covered yet.
- Frontend overlap risk (both Tailwind and plain CSS) is file-scoped rather than tracing the real JSX/selector ancestor chain — no CSS specificity/cascade resolution.
- v1 language coverage: Python, TypeScript, JavaScript, Go. Recognizes both Express-style route registrations and Next.js App Router route handlers (`route.ts` files); the older Pages Router (`pages/api/*.ts`) isn't covered yet.

See `CLAUDE.md` for the full spec and the reasoning behind every scope decision.
