# malveon check

Checks whether an AI coding agent's claimed-done work actually matches your plan — not by asking the agent, but by reading the real code.

Static analysis only. No live server, no browser, nothing runs. If it can't prove something from the code, it says so instead of guessing.

## The problem this solves

AI coding agents say "done" confidently, whether or not it's true. A button gets built with no backend behind it. A backend gets built with nothing calling it. The agent audits its own work in the same session it just wrote — and usually gives itself a passing grade, even when it shouldn't.

`malveon check` reads the actual code and git history and reports four things:

- **Wiring** — does a frontend action you planned actually reach a real backend route?
- **Contract** — does the call agree with the route on HTTP method (shape-level, not a claim the logic is correct)?
- **Not-in-plan** — did the agent build something this session that your plan never asked for?
- **Hero-act** — is that "bug I fixed" a real pre-existing bug, or one the agent created and fixed in the same breath?

Every result is `PASS` / `FAIL` / or `NOT TESTED` — never a guess dressed up as an answer.

## Install

Download the binary for your OS from the [Releases](../../releases) page. No other install required — it's a single static binary, nothing else to set up.

## Use

```bash
# once, right before you hand the agent a task
malveon session start

# ... agent does its work ...
# ask it: "what bugs did you introduce and fix this session?" -> save the answer to bugs.txt

malveon check --features features.json --bugs-reported bugs.txt
```

`features.json` is whatever plan you already have, in this shape:

```json
[
  { "id": "refund-button", "name": "refund button" }
]
```

## Example

Try it against the included example repo first, so you know what a working run looks like:

```bash
malveon check --root testdata/fixture --features testdata/fixture/features.json
```

## What it can't do (yet)

- Doesn't prove business logic is *correct* — only that the wiring and HTTP method agree.
- Doesn't run anything live — dynamic URLs, wrapped API clients, and templated paths report `NOT TESTED`, never a guessed pass.
- Hero-act is file-level, not line-level — if a file was touched at all this session, a reported bug pointing at it reads as self-introduced.
- v1 language coverage: Python, TypeScript, JavaScript, Go.

See `CLAUDE.md` for the full spec and the reasoning behind every scope decision.
