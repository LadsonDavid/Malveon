# malveon check

Checks whether an AI coding agent's claimed-done work actually matches your plan — not by asking the agent, but by reading the real code.

Static analysis only. No live server, no browser, nothing runs. If it can't prove something from the code, it says so instead of guessing.

## The problem this solves

AI coding agents say "done" confidently, whether or not it's true. A button gets built with no backend behind it. A backend gets built with nothing calling it. Two handlers silently claim the same route. The agent audits its own work in the same session it just wrote — and usually gives itself a passing grade, even when it shouldn't.

`malveon check` reads the actual code and git history and reports eight things:

- **Wiring** — does a frontend action you planned actually reach a real backend route?
- **Contract** — does the call agree with the route on HTTP method (shape-level, not a claim the logic is correct)?
- **Overlap** — do two or more route registrations silently claim the same method+path?
- **Frontend overlap risk** — is a Tailwind `absolute`/`fixed` element sitting with no positioning context anywhere in its file (a structural risk signal, not a claim two things visually collide — confirming that needs a real render, which this deliberately doesn't do)?
- **Not-in-plan** — did the agent build something this session that your plan never asked for?
- **Hero-act (manual)** — is that "bug I fixed" a real pre-existing bug, or one the agent created and fixed in the same breath? Needs `--bugs-reported`.
- **Hero-act (automatic)** — the same question, answered with zero self-report: if `malveon watch` was running, did a known bug pattern (a real, named catalog — see below) show up in an earlier captured snapshot of a file and vanish from the current version? That's proof from a "camera," not a claim.
- **Confidence** — does the agent's own "it works" claim actually match what got verified? Reads it straight from commit messages, nothing to ask or paste.

Every result is `PASS` / `FAIL` / or `NOT TESTED` (or the check-specific equivalent) — never a guess dressed up as an answer.

## Install

Download the binary for your OS from the [Releases](../../releases) page. No other install required — it's a single static binary, nothing else to set up.

## Use

```bash
# once, right before you hand the agent a task
malveon session start
malveon watch &   # background — watches the code as the agent works, Ctrl-C when done

# ... agent does its work, committing along the way like normal ...

malveon check
```

That's it. `malveon check` with no flags: looks for a plan file in the current folder automatically (asks if it's ambiguous, never guesses), reads the agent's confidence claims straight from its commit messages, and reads known-bug-pattern findings from whatever `malveon watch` captured while it ran — nothing to ask the agent, nothing to paste, for either one.

`malveon watch` is optional but worth running — without it, hero-act falls back to the fully manual path below. With it, hero-act catches a real, named catalog of bug patterns (an accidental `if (x = 5)`, a silently swallowed Go error, a bare Python `except:`) automatically, purely from what it captured while the agent worked — no self-report involved at all.

Want to override any of the automatic behavior, or use the fully manual hero-act path?

```bash
malveon check --features features.json --bugs-reported bugs.txt --claimed-summary summary.txt
```

- `--features <path>` — skips the plan auto-detect/prompt, use this exact file.
- `--bugs-reported <path>` — ask the agent "what bugs did you introduce and fix this session?", save the answer here, for the manual hero-act check. This one stays manual on purpose: sourcing it from commit messages the way confidence does would mean trusting the commit message's own claim of being a "fix" — the exact kind of unverified self-report this whole tool exists to not trust, just moved from chat into git. `malveon watch` is the real automatic alternative instead.
- `--claimed-summary <path>` — override the automatic commit-message reading with something else, for the confidence check.

All optional — leave any out and that section of the report shows itself skipped, with a plain reason, instead of silently doing nothing.

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
- Hero-act (manual) is file-level, not line-level — if a file was touched at all this session, a reported bug pointing at it reads as self-introduced.
- Hero-act (automatic) only catches a small, named catalog of known bug patterns — not a general "was this a real bug" judgment, which isn't resolvable from static snapshots alone. And it only works if `malveon watch` was actually running; if it crashed or was never started, that section reports itself unavailable rather than guessing from a possibly-incomplete recording.
- Overlap doesn't check reachability — a route registered in dead code still counts as a registration.
- Frontend overlap risk covers Tailwind utility classes only, not plain CSS files, and is file-scoped rather than tracing the real JSX ancestor chain.
- v1 language coverage: Python, TypeScript, JavaScript, Go.

See `CLAUDE.md` for the full spec and the reasoning behind every scope decision.
