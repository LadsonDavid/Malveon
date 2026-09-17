# malveon check

Checks whether an AI coding agent's claimed-done work actually matches your plan — not by asking the agent, but by reading the real code.

Mostly static analysis — no server, no browser, no deployed app, ever. The one exception: it also runs your project's own real build/lint/typecheck/test commands and reports what they actually said, not a guess. If it can't prove something, it says so instead of guessing.

## The problem this solves

AI coding agents say "done" confidently, whether or not it's true. A button gets built with no backend behind it. A backend gets built with nothing calling it. Two handlers silently claim the same route. The agent audits its own work in the same session it just wrote — and usually gives itself a passing grade, even when it shouldn't.

`malveon check` reads the actual code and git history — and, for one check, actually runs your project's own commands — and reports ten things:

- **Wiring** — does a frontend action you planned actually reach a real backend route? A miss says exactly which side is missing — backend built with no frontend, frontend built with no backend, or neither found.
- **Contract** — does the call agree with the route on HTTP method, and (JS/TS, literal request bodies only) does it actually send every field the handler reads off `req.body`? Shape-level only, never a claim the logic is correct.
- **Overlap** — do two or more route registrations silently claim the same method+path? Route-precedence aware: a static/dynamic split across two Next.js route files, or a static route registered before a dynamic one in the same file, is never actually ambiguous and isn't flagged — everything else, including the frameworks where registration order genuinely does create a real dead-route bug (Express, Flask, FastAPI, gorilla/mux), stays flagged. Also flags when one of the colliding registrations sits inside a function that's never referenced anywhere else in the codebase — a real signal it might be dead code, not just a guess.
- **Frontend overlap risk** — is an `absolute` element (Tailwind class or plain CSS `position:` declaration) sitting with no positioning context anywhere in its file (a structural risk signal, not a claim two things visually collide — confirming that needs a real render, which this deliberately doesn't do)? `fixed` is never flagged — it always positions against the viewport — and counts as valid context for a nested `absolute` element, matching real CSS semantics.
- **Not-in-plan** — did the agent build something this session that your plan never asked for?
- **Hero-act** — did this session's own code introduce a known bug pattern — whether it's still sitting there right now, or got fixed along the way? Two zero-self-report signals: a git-baseline comparison (always on once a session started — catches a bug that's still live) and, if `malveon watch` was running, a captured-snapshot comparison (catches one that appeared and disappeared entirely within the session, which a single before/after diff can't see).
- **Incompleteness** — does the code itself admit it's unfinished (`TODO`/`FIXME`/`HACK`/`XXX`/"not implemented" left in a file changed this session)? A marker's presence is real, code-only proof; its absence proves nothing, so this can never substitute for the confidence check below.
- **Confidence** — does the agent's own "it works" claim actually match what got verified? Reads it straight from commit messages, nothing to ask or paste.
- **Command check** — do your project's own build/lint/typecheck/test commands actually pass right now? The one check that runs real subprocesses instead of reading the code graph — a Makefile target, a `package.json` script, Go's own toolchain, or whatever Python tooling is on your `PATH`, whichever the project already defines. Nothing invented, nothing guessed, no server or browser ever started.
- **Focused-test check** *(opt-in, `--focused-tests`)* — do the tests actually related to what changed this session pass, without waiting for the whole suite? Uses Jest/Vitest's own built-in `--changed` support, `pytest-picked` if you have it installed, or a coarser package-level scope for Go (clearly labeled as such). Additive evidence only — it never replaces the full test result above.

Every result is `PASS` / `FAIL` / or `NO PROOF` (or the check-specific equivalent) — never a guess dressed up as an answer. By default, `malveon check` also exits non-zero if anything came back `FAIL`, `NO PROOF`, or a check couldn't run at all — see [Blocking a commit](#blocking-a-commit) below.

## Install

Pick your system, copy the one line below, paste it into a terminal, and press Enter.

**On a Mac or Linux:** open the Terminal app, then run:
```bash
curl -fsSL https://raw.githubusercontent.com/LadsonDavid/Malveon/main/install.sh | sh
```

**On Windows:** open PowerShell, then run:
```powershell
irm https://raw.githubusercontent.com/LadsonDavid/Malveon/main/install.ps1 | iex
```

That downloads the right file for your computer and sets it up so you can just type `malveon` from any folder, from now on. On a Mac, it also clears the security warning new downloads normally get, so your first run isn't blocked.

Now close your terminal window and open a brand new one. This step actually matters: your current terminal doesn't know about the change yet, only a new one will. Then type:
```bash
malveon
```
If you see a list of commands instead of an error, it worked.

Nothing else to install first. No Go, no Node, no extra toolchain. It's one file.

**Prefer to do it by hand?**

On Mac or Linux: download the file for your OS from the [Releases](../../releases) page, run `chmod +x` on it (this just tells your computer the file is allowed to run), then move it into a folder already on your PATH, like `/usr/local/bin`.

On Windows: downloading the `.exe` by itself is not enough to make `malveon` work as a bare command. The file also needs to be renamed to `malveon.exe` and placed somewhere on your PATH, and there's no single folder every Windows setup already has ready for that the way `/usr/local/bin` works on Mac or Linux. Run the install script above instead, it does exactly this for you and has been tested to actually work. If you'd rather not run any script at all, download the `.exe` and just run it directly from wherever you saved it, for example `.\malveon-windows-amd64.exe check`, no install needed, just typed from that exact folder every time.

## How to use it

Three commands. Run the first one before the agent starts, the second one while it works, and the third one after it says it's done.

**1. Save a snapshot of your code, right before the agent starts:**
```bash
malveon session start
```
Run this from inside your project's own folder. It just remembers what your code looked like before the agent touched anything, so later checks have something real to compare against.

**2. (Optional, but worth doing) Watch your code while the agent works:**
```bash
malveon watch
```
Leave this running in its own terminal window. Press Ctrl+C to stop it once the agent is finished. Skipping this step is fine, one check (catching a bug the agent introduces and then quietly fixes) just works a little less thoroughly without it. Everything else still works.

**3. After the agent says it's done, check what actually got built:**
```bash
malveon check
```
This is the real command. It reads your plan, reads your actual code and git history, and tells you what's genuinely built versus what the agent only claims. You don't have to point it at your plan file by hand, it looks for one and asks you to confirm before using it.

By default, this also runs your project's own build, lint, type check, and test commands for real, and it stops with an error if anything looks broken. That's the whole point, so a bad commit never has to be caught by eye. See [Blocking a commit](#blocking-a-commit) below to actually wire that up.

### Changing the defaults

Everything above works with no extra input. If you want more control, you can add extra options (called flags) after the command:

```bash
malveon check --features features.json --claimed-summary summary.txt --exec-timeout 5m
```

- `--features <path>`: skip the plan file question, use this exact file.
- `--claimed-summary <path>`: use this file instead of git commit messages, for the confidence check.
- `--skip-exec`: don't run your build/lint/test commands at all.
- `--exec-timeout <duration>`: change how long each command gets before malveon gives up on it (default 3 minutes). Example: `5m`, `90s`.
- `--no-gate`: still show the full report, but always finish successfully, even if something's wrong.
- `--focused-tests`: also run just the tests related to what changed this session, on top of the full test run.

Skip any flag you don't need. Nothing breaks, that part of the report just explains it was skipped and why.

## Blocking a commit

By default, `malveon check` fails (technically: exits with a non-zero status) if it finds something broken, something it can't prove either way, or a check that couldn't run at all. Every report ends with a plain `GATE: clean` or `GATE: blocked` line, naming exactly why.

A few things never block on their own: code flagged as "not in your plan," unfinished-code markers, and frontend layout risks. Those are meant for a human to glance at, not proof of a real bug.

To actually stop a bad commit, wire malveon into a pre-commit hook. Malveon never sets this up for you, here's how to do it yourself:

```bash
#!/bin/sh
# save as .git/hooks/pre-commit, then run: chmod +x .git/hooks/pre-commit
malveon check --features path/to/your/plan.json
```

Always include `--features` in a hook like this. A hook can't ask you questions interactively, so without it, the check fails right away instead of hanging while it waits for an answer. If your build and test commands already run somewhere else, like CI, add `--skip-exec` here so this hook only checks the code itself.

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


## What it can't do (yet)

- Doesn't prove business logic is *correct* — only that the wiring and HTTP method agree.
- No server, no browser, no deployed app, ever — dynamic URLs, wrapped API clients, and templated paths report `NO PROOF`, never a guessed pass. The command check runs your project's own already-defined build/lint/typecheck/test commands (nothing invented), but that's still not a live/deployed run.
- A single fixed timeout applies to every command in the main command check rather than one tuned per category.
- Focused-test mode (`--focused-tests`) only has real, built-in "changed" support for Jest and Vitest. Python needs `pytest-picked` already installed (pytest itself has no built-in equivalent), and its `--mode=branch` selection only sees a new test file once it's at least `git add`-ed — a truly untracked file won't be picked up yet. Go has no built-in mechanism at all, so it falls back to a coarser package-level scope (test the package containing a changed file, not the real transitive dependency graph).
- Hero-act only catches a small, named catalog of known bug patterns — not a general "was this a real bug" judgment, which isn't resolvable from static snapshots alone. And it only works if `malveon watch` was actually running; if it crashed or was never started, that section reports itself unavailable rather than guessing from a possibly-incomplete recording.
- Incompleteness is one-directional — a marker's presence is real proof, but its absence proves nothing (most finished code has none either). Never a substitute for the confidence check.
- Overlap's reachability note is a best-effort heuristic (checks whether an enclosing function's name is ever mentioned elsewhere in the codebase), not real call-graph analysis — an anonymous handler or dead code it can't attribute to a named function still just counts as a plain registration.
- Contract's field-agreement check is JS/TS only, and only fires when both sides are a literal object (no spread, no variable) — Python/Go request bodies and response-shape comparison aren't covered yet.
- Frontend overlap risk (both Tailwind and plain CSS) is file-scoped rather than tracing the real JSX/selector ancestor chain — no CSS specificity/cascade resolution.
- v1 language coverage: Python, TypeScript, JavaScript, Go. Recognizes both Express-style route registrations and Next.js App Router route handlers (`route.ts` files); the older Pages Router (`pages/api/*.ts`) isn't covered yet.
