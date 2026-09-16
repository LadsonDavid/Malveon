# malveon check

Checks whether an AI coding agent's claimed-done work actually matches your plan — not by asking the agent, but by reading the real code.

Static analysis only. No live server, no browser, nothing runs. If it can't prove something from the code, it says so instead of guessing.

## The problem this solves

AI coding agents say "done" confidently, whether or not it's true. A button gets built with no backend behind it. A backend gets built with nothing calling it. Two handlers silently claim the same route. The agent audits its own work in the same session it just wrote — and usually gives itself a passing grade, even when it shouldn't.

`malveon check` reads the actual code and git history and reports seven things:

- **Wiring** — does a frontend action you planned actually reach a real backend route?
- **Contract** — does the call agree with the route on HTTP method (shape-level, not a claim the logic is correct)?
- **Overlap** — do two or more route registrations silently claim the same method+path?
- **Frontend overlap risk** — is a Tailwind `absolute`/`fixed` element sitting with no positioning context anywhere in its file (a structural risk signal, not a claim two things visually collide — confirming that needs a real render, which this deliberately doesn't do)?
- **Not-in-plan** — did the agent build something this session that your plan never asked for?
- **Hero-act** — is that "bug I fixed" a real pre-existing bug, or one the agent created and fixed in the same breath?
- **Confidence** — does the agent's own "it works" claim actually match what got verified?

Every result is `PASS` / `FAIL` / or `NOT TESTED` (or the check-specific equivalent) — never a guess dressed up as an answer.

## Install

Download the binary for your OS from the [Releases](../../releases) page. No other install required — it's a single static binary, nothing else to set up.

## Use

```bash
# once, right before you hand the agent a task
malveon session start

# ... agent does its work ...

malveon check
```

That's it — `malveon check` with no flags looks for a plan file in the current folder automatically. If it finds exactly one, it uses it and says so. If it finds more than one, it asks which — never guesses. If it finds none, it asks you for the path. Nothing is assumed silently.

Want to skip the question entirely (scripts, CI, or just being explicit), or add the optional inputs?

```bash
malveon check --features features.json --bugs-reported bugs.txt --claimed-summary summary.txt
```

- `--features <path>` — skips the auto-detect/prompt, use this exact file.
- `--bugs-reported <path>` — ask the agent "what bugs did you introduce and fix this session?", save the answer here, for the hero-act check.
- `--claimed-summary <path>` — ask the agent to summarize what it built and whether it works, save that here, for the confidence check.

Both are optional — leave either out and that section of the report shows itself skipped, with a plain reason, instead of silently doing nothing.

## The plan file

Whatever format you already keep your plan in — no fixed shape forced on you, and no particular filename required. Auto-detection tries two ways:

1. **By filename first** — anything containing "plan," "feature," "checklist," or "todo," with a supported extension.
2. **By content, if nothing matched by name** — a `.json` file counts if it's actually shaped like a feature list (array of objects with a `name` field), a `.md` file counts if it has real checklist lines (`- [ ]`, `- [x]`, etc.). So `sprint3.json` or `notes.md` gets found too, not just files literally named `plan.json`.

If nothing matches either way, or more than one file looks right, it asks — never guesses.

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

## What it can't do (yet)

- Doesn't prove business logic is *correct* — only that the wiring and HTTP method agree.
- Doesn't run anything live — dynamic URLs, wrapped API clients, and templated paths report `NOT TESTED`, never a guessed pass.
- Hero-act is file-level, not line-level — if a file was touched at all this session, a reported bug pointing at it reads as self-introduced.
- Overlap doesn't check reachability — a route registered in dead code still counts as a registration.
- Frontend overlap risk covers Tailwind utility classes only, not plain CSS files, and is file-scoped rather than tracing the real JSX ancestor chain.
- v1 language coverage: Python, TypeScript, JavaScript, Go.

See `CLAUDE.md` for the full spec and the reasoning behind every scope decision.
