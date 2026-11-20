# malveon

**Your AI says "done." malveon reads the code and tells you what is actually true.**

An AI coding agent reports back confidently. The button it says it built has no click handler. The form posts to a route that does not exist. It quietly added a database table you never asked for. malveon reads the real code, never the agent's word, and answers with the file and line that prove it. If it cannot prove something, it says so instead of guessing.

v0.5.8 reports when a frontend reads a key missing from a supported Next.js JSON response, suppresses several guess-based findings, and makes desktop notifications opt-in. See the [reference](docs/REFERENCE.md) for supported cases.

## Try it in 30 seconds

No plan, no setup. In your project's folder:

```bash
npx malveon
```

It can flag calls to addresses no supported route serves, imports it cannot resolve and routes that only answer "not implemented". A source check leaves your project files as they are. Findings in generated, test, template or example code can be counted separately.

Just had an AI work on the project?

```bash
malveon --since HEAD~1
```

It checks the commits the AI made, using their messages as what you asked for, and says what it added that nothing asked for, what it broke and what it left unfinished. (Only one commit yet? It scans instead.)

## What a result looks like

By default you get a short view: one line saying what was read, then at most three findings, each with where it is, why, and what to do next.

```
malveon checked 1 file. Code read only; your app was not run.

Found problems (1)

  1. Import cannot work   index.js:1
     The import import from './refund.js' in index.js cannot work: the file is missing or no longer exports that name.
     Next: Fix the import path or restore what it imported.
```

The headline is one of `Found problems` (something malveon trusts is wrong), `Worth a look`, or `Nothing found in the code I read`. It never says "verified": reading code is not running your app. `malveon --all` shows every finding and the details, and `malveon --json -` prints them as JSON for a script. **malveon never blocks anything and exits 0 whenever it checked** (1 only if it could not run); a script that wants to fail a build reads the JSON and decides itself.

## The commands

```
malveon          check now (add --plan FILE to check a plan, --since REF for recent commits)
malveon on       set up automatic checks across your agent projects
malveon off      turn them off (--for 2h to pause)
malveon status   what is on, and what malveon saw last
malveon settle   say a finding is wrong, or confirm what your AI was asked
malveon update   get the latest version
```

A session check can also flag changes to existing code that other files rely on, additions your request does not name, and claims about test results that disagree with the recorded output. `malveon help languages` shows which stacks it reads.

## Check against a plan

Give it a checklist of what the AI should build (a plain `PLAN.md` with `- [ ]` lines works) and run:

```bash
malveon --plan PLAN.md
```

Each item comes back built and connected, broken, missing, half done, or skipped when there is nothing in the code to look up. For automatic prompt capture and warnings, run `malveon on` once and press Enter to confirm. Open a new agent session to start working. Folders without Git get source checks, and checks that compare changes need Git history. Older notes in the reference may still use the retired command names; each prints its replacement. Configuration is not execution proof; check `malveon status` after a real prompt. Details: [the plan file](docs/REFERENCE.md#the-plan-file), [failing a build yourself](docs/REFERENCE.md#failing-a-build-yourself), [using it from your AI agent](docs/REFERENCE.md#use-it-from-your-ai-agent-mcp).

## Install

Mac or Linux:

```bash
curl -fsSL https://get.malveon.com/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://get.malveon.com/install.ps1 | iex
```

Or `npm install -g malveon`, Homebrew, Scoop, or the VS Code / Cursor extension (search "malveon"). winget is submitted and waiting for review. Open a new terminal and type `malveon`. One file, nothing else to install. Your first 150 runs of a check you start yourself are counted in a footer line; nothing stops at the limit in this release. More ways and the checksum and signature details: [Install](docs/REFERENCE.md#install).

## Plugins for Claude Code, Codex and Cursor

A plugin adds Malveon's tools (`malveon_check`, `malveon_scan` and the rest) to your agent and a skill that tells it to run Malveon before it says a task is done. It needs Node.js; it runs `npx malveon@0.5.8 mcp`, which downloads the signed program on first use.

```
/plugin marketplace add LadsonDavid/Malveon        # Claude Code
/plugin install malveon@malveon

codex plugin marketplace add LadsonDavid/Malveon   # Codex
codex plugin add malveon@malveon
```

If you already installed the plugin, refresh its marketplace and update the plugin, then open a new session. Plugin bundle v0.5.9 runs Malveon CLI v0.5.8.

For a standalone CLI install, run `malveon on` to set up prompt capture and automatic checking.

For plugins installed from GitHub, Claude Code and Cursor bundle prompt and completion hooks. Codex also bundles both hooks and requires you to review and trust them in `/hooks` before running them. Use one setup per agent to avoid running the plugin hooks alongside existing Malveon hooks from `malveon on`.

## What it does not do

The default check reads source code. A report can establish a supported source connection and show what remains unchecked. Real clicks, deployed behavior and business logic still need runtime verification. See the [reference](docs/REFERENCE.md#what-it-cant-do-yet) for the limits of each check.

## Privacy

malveon sends a few anonymous usage counts: your OS and CPU type, malveon's version, which command ran, and numbers such as how many problems a check found. It never sends your code, file names, plan, prompts or commit messages. The program has no setting to turn this off; the VS Code and Cursor extension follows your editor's own telemetry setting. The exact list is in [Usage tracking](docs/REFERENCE.md#usage-tracking). `MALVEON_NO_UPDATE_CHECK=1` (or `DO_NOT_TRACK=1`) stops the daily update check.

## Open source and license

The code map, plan reading and plan-items check are open source (Apache-2.0, in [`pkg/`](pkg)), so you can audit how a plan line becomes a verdict. The downloadable `malveon` program also contains closed code and is free to use during the beta under [BETA-LICENSE.txt](BETA-LICENSE.txt). Details: [Open source](docs/REFERENCE.md#open-source-what-you-can-read-and-what-you-cant).

## Questions or feedback

Found a bug, or a stack it handles badly? [Open a discussion](https://github.com/LadsonDavid/Malveon/discussions): that is the one place it is watched.
