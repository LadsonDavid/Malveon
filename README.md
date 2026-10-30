# malveon

**Your AI says "done." malveon reads the code and tells you what is actually true.**

An AI coding agent reports back confidently. The button it says it built has no click handler. The form posts to a route that does not exist. It quietly added a database table you never asked for. malveon reads the real code, never the agent's word, and answers with the file and line that prove it. If it cannot prove something, it says so instead of guessing.

## Try it in 30 seconds

No plan, no setup. In your project's folder:

```bash
npx malveon scan
```

It lists buttons that do nothing, calls to addresses nothing serves, imports that cannot work and routes that only answer "not implemented". It changes nothing in your project. Faults in generated, test, template or example code are counted, not listed, so the top of the list is your own app.

Just had an AI work on the project?

```bash
malveon review
```

It checks the commits the AI made, using their messages as what you asked for, and says what it added that nothing asked for, what it broke and what it left unfinished. (Only one commit yet? It scans instead.)

## What a report looks like

Every report opens with one word: `FAILED` (something is proven wrong), `NEEDS YOU` (nothing is proven wrong, but something is yours to settle) or `VERIFIED BY CODE`. "Verified" never means the app was run.

```
VERDICT: FAILED
  3 problems are proven wrong in the code (details below):
  - plan items: 1 BROKEN, 0 NOT BUILT, 1 HALF BUILT
  - new bugs this session: 1 confirmed structural regression(s) or broken import(s)
  - agent's claims vs reality: 1 claim(s) contradicted by what was actually verified
  Also yours to settle:
  - the agent added things nobody asked for: 1 route, 1 call, 1 table, 4 packages

FIX THESE FIRST
  1. BROKEN: `CancelButton` cancels an order with `DELETE /api/orders/[id]`
     it calls DELETE /api/order/:param (src\components\CancelButton.tsx:5), and no server route handles that
```

## Check against a plan

Give it a checklist of what the AI should build (a plain `PLAN.md` with `- [ ]` lines works) and run:

```bash
malveon check
```

Each item comes back built and connected, broken, missing, half done, or skipped when there is nothing in the code to look up. To have every task checked from now on, run `malveon hooks install` once. Details: [the plan file](docs/REFERENCE.md#the-plan-file), [blocking a commit](docs/REFERENCE.md#blocking-a-commit), [using it from your AI agent](docs/REFERENCE.md#use-it-from-your-ai-agent-mcp).

## Install

Mac or Linux:

```bash
curl -fsSL https://get.malveon.com/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://get.malveon.com/install.ps1 | iex
```

Or `npm install -g malveon`, Homebrew, Scoop, or the VS Code / Cursor extension (search "malveon"). Open a new terminal and type `malveon`. One file, nothing else to install. More ways and the checksum and signature details: [Install](docs/REFERENCE.md#install).

## What it does not do

It reads code. It never opens a browser, starts your app or checks a deployed site, so it cannot prove a real click works or that your business logic is right. It lists a short set of faults and says what it did not judge, so a quiet result is not a clean bill of health. Full list: [what it can't do](docs/REFERENCE.md#what-it-cant-do-yet).

## Privacy

malveon sends small anonymous usage signals (your OS, CPU type, malveon's version, which command ran, and counts such as how many problems a scan found). It never sends your code, file names, plan, prompts or commit messages. The exact list is in [Usage tracking](docs/REFERENCE.md#usage-tracking). `DO_NOT_TRACK=1` stops the update check.

## Open source and license

The code map, plan reading and plan-items check are open source (Apache-2.0, in [`pkg/`](pkg)), so you can audit how a plan line becomes a verdict. The downloadable `malveon` program also contains closed code and is free to use during the beta under [BETA-LICENSE.txt](BETA-LICENSE.txt). Details: [Open source](docs/REFERENCE.md#open-source-what-you-can-read-and-what-you-cant).

## Questions or feedback

Found a bug, or a stack it handles badly? [Open a discussion](https://github.com/LadsonDavid/Malveon/discussions): that is the one place it is watched.
