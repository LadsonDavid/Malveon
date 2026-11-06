# malveon

**Your AI says "done." malveon reads the code and tells you what is actually true.**

An AI coding agent reports back confidently. The button it says it built has no click handler. The form posts to a route that does not exist. It quietly added a database table you never asked for. malveon reads the real code, never the agent's word, and answers with the file and line that prove it. If it cannot prove something, it says so instead of guessing.

## Try it in 30 seconds

No plan, no setup. In your project's folder:

```bash
npx malveon
```

It lists buttons that do nothing, calls to addresses nothing serves, imports that cannot work and routes that only answer "not implemented". It changes nothing in your project. Faults in generated, test, template or example code are counted, not listed, so the top of the list is your own app.

Just had an AI work on the project?

```bash
malveon --since HEAD~1
```

It checks the commits the AI made, using their messages as what you asked for, and says what it added that nothing asked for, what it broke and what it left unfinished. (Only one commit yet? It scans instead.)

## What a result looks like

By default you get a short view: one line saying what was read, then at most three findings, each with where it is, why, and what to do next.

```
malveon checked 3 files, 1 plan item. Code read only; your app was not run.

Worth a look (2)

  1. Button does nothing (likely)   web/Orders.jsx:5
     <button> "Refund" has no click handler, or an empty one.
     Next: Add an onClick that does the job, or remove the button.

  2. Route unfinished (likely)   server.js:6
     POST /api/refunds only answers an error or "not implemented".
     Next: Write the handler.
```

The headline is one of `Found problems` (something malveon trusts is wrong), `Worth a look`, or `Nothing found in the code I read`. It never says "verified": reading code is not running your app. `malveon --all` shows every finding and the details, and `malveon --json -` prints them as JSON for a script. **malveon never blocks anything and exits 0 whenever it checked** (1 only if it could not run); a script that wants to fail a build reads the JSON and decides itself.

## The commands

```
malveon          check now (add --plan FILE to check a plan, --since REF for recent commits)
malveon on       turn on automatic checks for your AI agent (shows the change, asks once)
malveon off      turn them off (--for 2h to pause)
malveon status   what is on, and what malveon saw last
malveon settle   say a finding is wrong, or confirm what your AI was asked
malveon update   get the latest version
```

Beyond the dead buttons and broken imports above, a check with a session also lists work nothing reaches, existing code the AI changed that other files still rely on (JavaScript, TypeScript, Go and Python), things it added that your words don't name, and whether its "all tests pass" matches its own log. `malveon --layout` looks at your pages in a real browser. Wording stays "possibly" until these checks are measured on enough real projects, and `malveon help languages` says exactly which stacks are read.

## Check against a plan

Give it a checklist of what the AI should build (a plain `PLAN.md` with `- [ ]` lines works) and run:

```bash
malveon --plan PLAN.md
```

Each item comes back built and connected, broken, missing, half done, or skipped when there is nothing in the code to look up. For automatic prompt capture and warnings, run `malveon on` (it shows exactly what it will change and asks once). Older notes in the reference may still use the retired command names; each prints its replacement. Configuration is not execution proof; check `malveon status` after a real prompt. Details: [the plan file](docs/REFERENCE.md#the-plan-file), [failing a build yourself](docs/REFERENCE.md#failing-a-build-yourself), [using it from your AI agent](docs/REFERENCE.md#use-it-from-your-ai-agent-mcp).

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

## What it does not do

It reads code. It never checks a deployed site, and it starts your app only when you give it the command yourself and approve it once (`--layout --start`, optional). It opens a browser only for the optional layout check described in [the reference](docs/REFERENCE.md#every-check-in-detail) (your own Chrome or Edge, headless, on pages from your files or an app on your machine), so it cannot prove a real click works or that your business logic is right. It lists a short set of faults and says what it did not judge, so a quiet result is not a clean bill of health. Full list: [what it can't do](docs/REFERENCE.md#what-it-cant-do-yet).

## Privacy

malveon sends a few anonymous usage counts: your OS and CPU type, malveon's version, which command ran, and numbers such as how many problems a check found. It never sends your code, file names, plan, prompts or commit messages. The program has no setting to turn this off; the VS Code and Cursor extension follows your editor's own telemetry setting. The exact list is in [Usage tracking](docs/REFERENCE.md#usage-tracking). `MALVEON_NO_UPDATE_CHECK=1` (or `DO_NOT_TRACK=1`) stops the daily update check.

## Open source and license

The code map, plan reading and plan-items check are open source (Apache-2.0, in [`pkg/`](pkg)), so you can audit how a plan line becomes a verdict. The downloadable `malveon` program also contains closed code and is free to use during the beta under [BETA-LICENSE.txt](BETA-LICENSE.txt). Details: [Open source](docs/REFERENCE.md#open-source-what-you-can-read-and-what-you-cant).

## Questions or feedback

Found a bug, or a stack it handles badly? [Open a discussion](https://github.com/LadsonDavid/Malveon/discussions): that is the one place it is watched.
