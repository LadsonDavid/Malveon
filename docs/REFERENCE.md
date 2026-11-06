# malveon reference

The full details behind the [README](../README.md): every check, the plan file, `malveon.json`, agent hooks, MCP, storage, telemetry and licensing. Start with the README; come here when you need the exact rule.

## The six commands, and `malveon account` (local source, unreleased; replaces the older command list below)

| Command | What it does |
|---|---|
| `malveon` | Check this project now. With a saved plan or captured prompts it checks the work against them; otherwise it just looks. `--since <ref>` checks what changed after a commit (the commit messages stand in for the request), `--plan <file>` checks a plan (repeatable), `--run` also runs your own build, lint, type and test scripts, `--all` shows everything, `--json <path\|->` writes findings as JSON, `--exclude <dir>` skips a folder, `--layout` opens pages in your own Chrome or Edge. Changes nothing in your project. |
| `malveon on` | One yes/no question after showing exactly what it will change: opens a session, sets up each agent folder it finds (prompt capture and warnings), copies the program to a steady place so the hooks survive an update, and starts a background watcher. Warn-only. |
| `malveon off [--for 2h]` | Pauses the warnings and stops the watcher. Needs a real terminal. |
| `malveon status [--json] [--rules]` | Is it on, the last look and its age, whether the watcher is alive, which agents are set up, which languages it reads here (and how much of each), and where its files are. |
| `malveon settle` | Answers only you can give, from a real terminal: `looked <n>` marks a finding "Not a problem" (hidden until the code under it changes), `<n> right\|wrong`, `promote <RULE>`, `confirm <n>`, `waive <n> <words>`, `drop <n>`, `plan`. An agent's shell is refused. |
| `malveon update` | Installs the newest version. |
| `malveon account` | Your plan (a seventh command, outside the six): `status` (default), `activate <key>`, `refresh`, `logout`, `billing`. Always works, at any state. |

`malveon help languages | files | privacy | exit-codes` print the facts the short help leaves out; `help languages` is generated from the program's own capability table (cells inferred rather than read from the notes show `[guess]`).

**The 150-Run Free Trial and the paid plan (dark launch: nothing is stopped in this build).** You get 150 runs, once, for life. A run is a check *you* start: plain `malveon`, an AI agent's `malveon_check` or `malveon_scan`. It counts once it has finished; a run that fails before finishing does not. Hooks, the watcher, the stop and after-edit warnings and your editor's automatic checks never count and are never stopped. After each counted run malveon prints how many runs are left (on standard error). The paid plan is **$19/month + tax**, unlimited runs; it renews monthly, you can cancel any time (`malveon account billing`) and keep it until the end of the paid period, and there is a 14-day refund by email. Once a later release turns enforcement on, a check you start after your 150th run prints the price and how to activate and exits 1; everything else keeps working. The counter is a local file (with a second copy in your home folder): it is honest-user friction, not copy protection. A license key activates up to 3 machines; the program stores a signed token (valid 30 days, checked offline with a public key built into the program) and renews it in the background at most once a day, with a 2-second limit, and only for people who activated a key. A slow or failing license server never stops a run; with `MALVEON_LOCAL_ONLY=1` malveon never contacts it (`malveon account refresh` does, when you ask). The license service stores your email, Paddle ids, subscription status and the random install id of each activated machine: no code, file names or project data.

**Exit codes.** 0 = it checked, even if it found problems. 1 = it could not run, or you typed a removed command. malveon never fails a build by itself; read `malveon --json -` and decide.

**Removed names** print one line and exit 1: `scan` (use plain `malveon`), `review` (`malveon --since <ref>`), `check` (`malveon --plan <file>`), `session`/`hooks`/`trust` (`malveon on`), `gate` (`malveon off` / `malveon on`), `prompt` and `plan` (`malveon settle`), `updates` (`malveon status`), `register` (removed). `--exec` is now `--run`. `hook` and `mcp` are called by agents and keep their names; `session`, `paths`, `version` and `identity` still exist for the VS Code extension.

**Settings** are no longer a `malveon.json` in your project. They are saved in malveon's own folder for the project (`malveon status` prints it). A `malveon.json` left by an older version is copied there once (checked by SHA-256) and then ignored; if it differs from the saved settings, the saved ones win and `status` says so. A fresh clone, another machine or a throwaway cloud environment therefore starts without your plan files and answers.

**The watcher** (`malveon on` starts it, `malveon off` stops it) polls your files once a second, waits until they stop changing for three seconds (longer after a slow look), looks, and tells you only what is new. Each finding is said once; only the ones malveon trusts make a notice, which the watcher writes to `<data folder>/watch/notices.log` (it has no terminal) and `malveon status` prints the newest three; the rest wait in a list with their age (`<data folder>/watch/live.json`). Nothing interrupts your terminal or your agent: run `malveon status` to see them. It always re-reads the whole project (no incremental scan yet), so on a large project results arrive late. A dead watcher shows as "watcher stopped" in `status`.

## v0.4.6 integrity changes

**Upgrade once:** update any manually configured agent command that still points at `~/.malveon/editor` or an old extension folder, using the editor's **Copy setup for another agent** action or `malveon paths --json`, then reconnect that agent. Restarting alone still launches the old engine if its configured executable path is unchanged. Existing pre-upgrade processes cannot join the new coordination protocol retroactively.

State transactions hold OS locks until completion; an old lock timestamp never permits taking ownership from a live process. Failed file replacement preserves accepted bytes and returns an error instead of writing in place. Critical evidence has retained creation markers: a missing known session, prompt history or plan index stops verification rather than resetting history. Retained original snapshots also prevent a missing index from adopting weaker current text. Recover from verified backups or deliberately reconcile the history; deleting all user data cannot reconstruct original intent. These are local process-crash safeguards, not a universal power-loss or network-filesystem guarantee. Reconnect pre-upgrade MCP processes once before migration.

Reviews carry a request-local baseline and cannot restore old session bytes over a newer task. Checks cannot close a newer session generation. Reports retain every original plan span separately from model classifications and shortened claims; this preserves traceability, not proof of complete semantic interpretation. Missing evidence calls for interpretation or relevant execution evidence, without requiring legitimate plans to be rewritten. Failed editor checks replace current success indications with an error and mark previous results historical. Editor MCP connections reject legacy engines missing required tools and forward `MALVEON_DATA_DIR` consistently.

Five small snippet assessments were observed in the existing Antigravity model across four conversations: the denial defect was identified, the correct callback counterpart was not accused, and an irrelevant citation was rejected. These are targeted observations, not an accuracy estimate. The host then exposed legacy tools, so they do not establish the updated native prepare/submit workflow. Semantic completeness, unsupported syntax and real application behavior remain unverified unless supported by appropriate independent evidence.

**Your AI says "done." malveon reads the code and tells you what is actually true.**

You give an AI coding agent a to-do list. It writes code and reports back, confidently. The button it says it built has no click handler. The form posts to a route that doesn't exist. It quietly added a database table you never asked for. You find out when a user does, or when you read every changed file yourself, which was the job you gave the AI.

malveon reads the real code, never the agent's word, and answers each item on your list: built and connected, broken, missing, or half done. Every answer comes with the file and line that proves it. If it can't prove something, it says so instead of guessing.

Every report opens with one word. This is the real top of a report on a small test app:

```
VERDICT: FAILED
==============================================================================
  3 problems are proven wrong in the code (details below):
  - plan items: 1 BROKEN, 0 NOT BUILT, 1 HALF BUILT
  - new bugs this session: 1 confirmed structural regression(s) or broken import(s)
  - agent's claims vs reality: 1 claim(s) contradicted by what was actually verified
  Also yours to settle:
  - the agent added things your words do not name: 1 route, 1 call, 1 table, 4 packages (start at src/app/api/receipts/route.ts:4)
  - the agent changed 2 files nothing you wrote points at: src/app/api/receipts/route.ts, src/components/ReceiptButton.tsx
  Not established (looked at, could not settle):
  - 1 plan line names nothing malveon can look up, so nothing was checked for it
  malveon reads the code; it never ran your app, so a real click or page load is still yours to try.

FIX THESE FIRST
==============================================================================
  1. BROKEN — `CancelButton` cancels an order with `DELETE /api/orders/[id]`
     it calls DELETE /api/order/:param (src\components\CancelButton.tsx:5), and no server route handles that
```

The three possible words are `FAILED` (something is proven wrong), `NEEDS YOU` (nothing is proven wrong, but something is yours to settle) and `VERIFIED BY CODE`. "Verified" never means the app was run.

## Local user-data storage (v0.4.6)

Generated state now lives outside the checkout. The default data root is `%LOCALAPPDATA%\Malveon` on Windows, `~/Library/Application Support/Malveon` on macOS, and `$XDG_DATA_HOME/malveon` (or `~/.local/share/malveon`) on Linux. `malveon paths --root <project> --json` lists the actual locations without creating project state. An absolute `MALVEON_DATA_DIR` overrides the root for portable use, CI and tests; the editor, MCP host, hooks and scheduled helper must use the same override.

- `projects/<SHA-256 of canonical checkout/analysis-root path>/` contains the session, captured prompts, prompt/gate state, original-plan snapshots and model assessments. Clones, worktrees and separate analysis roots have separate histories. A moved checkout gets a new identity; history is not silently reassociated. Original snapshots are durable evidence, not disposable cache.
- `settings/` contains local command approvals, prompt-confirmation seals, gate preferences/audit log and the anonymous telemetry ID. Approval bindings retain their original project/config identities. `cache/` contains the update-check cache. `engine/` contains immutable engine generations; `editor/` contains the stable extension executable. The existing Windows installer bootstrap stays at its stable `%LOCALAPPDATA%\malveon\malveon.exe` path. Package-manager-owned binaries and publisher signing keys are not relocated.
- First use copies recognized legacy project state and plan snapshots, verifies their SHA-256 contents, and publishes the new directory only when the copy is complete. Concurrent migrations use an OS-backed lock. The old `.malveon/` files are retained for recovery; arbitrary development binaries, validation caches and unrelated files are not imported. Interrupted copies can be retried. Conflicting/incomplete destination history or later changes from an older writer cause an explicit storage error, not a baseline reset or a clean verification claim. This detects ordinary old-writer changes, not edit-and-revert races; reconnect pre-upgrade processes once before migration.
- Existing engine stores migrate only while their participating work leases are idle. Migrated installations retain the old user-home engine coordination lock/lease location to coordinate with older processes; new installations keep coordination in the new engine directory. The signed generation bytes and previous selection are retained, with generation paths rebased. A pre-upgrade idle MCP process still needs the one-time reconnect described above.
- The CLI, MCP and hooks use the same Go resolver. The extension asks the executable for paths and watches that project's user-data prompt files; Antigravity reads the session time from the same location. An engine that cannot report valid paths produces an editor error. The edit gate protects user-data evidence as well as retained legacy files, subject to its existing shell-tool limitation.

Keep `malveon.json`, your original plan documents and the selected agent/editor integration settings in the project. They are shared configuration and user content. Normal operation no longer creates a project-local `.malveon/` directory. Retained legacy backups are not automatically deleted, and this source change does not upgrade an already installed binary.

## Verification contract (v0.4.6)

Release builds use the matching published public core v0.4.6 without local replacements. Local development can use an ignored Go workspace; a workspace build is not a release artifact.

- `CONNECTED` establishes the specific extracted connection described by its evidence. A file or symbol existing, changing, or being imported is insufficient to prove its promised behavior. Such requirements stay `SKIPPED` with `Unchecked: true` and a coverage explanation. They must not be described as implemented or broken. Ordinary prose with no source mapping is unresolved, not an instruction to rewrite the plan.
- Markdown keeps heading context and indented continuation conditions. External-reference sections, declared Source columns in Markdown tables, and explicitly attributed parenthetical citations are distinguished from local implementation anchors. Source-column text remains in the original requirement and report citation fields; external targets and the truth of their attached claims stay unverified. This is a bounded parser, not general semantic understanding; a mistaken classification remains possible.
- TypeScript type-star re-exports no longer fabricate missing named-export failures. Type-only imports, test imports and fixture imports do not prove runtime production integration. A type-only port relationship also cannot be accused of a missing runtime connection merely because effect tracing excludes it. `testdata`, `__fixtures__` and `__mocks__` are fixture provenance, not production application code; selecting a fixture itself as the scan root still checks it normally. Open re-export sets can leave missing-export defects undetected: the compiler remains the stronger oracle.
- Regex pattern candidates are advisory. Comments and string examples are masked for executable patterns; putting a comment inside an empty catch does not remove the candidate. A secret-shaped test fixture is not reported as a live application credential. Proven broken imports and route regressions remain separate blocking findings.
- The first observed original plan is pinned under the project's user-data `plans/` directory, with original/current SHA-256 hashes. Subsequent edits do not replace its requirements. `malveon plan accept` adopts an amendment from a human terminal; retained snapshots preserve the earlier text. First observation does not recover requirements already weakened before this version saw them. Recover those from Git deliberately.
- The new MCP tool `malveon_plan_review` prepares original spans and records interpretation by the coding platform's existing model. It does not call a separate model service or assume MCP sampling support. It returns the selected original plan to the coding platform through MCP; plan content is excluded from usage telemetry. Every span needs a disposition, rationale and source lookups or explicitly unresolved conditions. Stored model mappings get fresh source lookups on later checks; those results remain separate from whether the model preserved the original meaning. Each interpretation is limited to 256 lookups. Missing/stale/repeated spans are rejected, and retries are bounded to three interpretations per original document. Structural coverage does not prove that a model preserved every condition semantically. Model assessments never become independent proof. If the model is unavailable or no interpretation is submitted, the report keeps interpretation pending and semantics unresolved; static checks still run. No accuracy percentage is claimed. Five snippet assessments were observed in four actual Antigravity conversations; the host exposed legacy tools, so those observations do not establish the updated native prepare/submit workflow.
- Reports separate source facts, captured command results, model assessments and unresolved requirements. JSON schema version 2 includes plan review and provenance: executable path/hash, build revision and dirty flag, core dependency/replacement, source fingerprints, HEAD and the recorded session baseline. Fingerprints detect ordinary concurrent edits, not edit-and-revert races or an atomic filesystem snapshot. A changed plan or changing source blocks as incomplete verification, not as a proven implementation defect. A blocked gate cannot render `VERIFIED BY CODE`. The editor shows unresolved verification as needs review, including when the commit gate itself is clean. The baseline is shown and never silently reset to fit a task.
- For direct Windows installations, `malveon update` verifies the publisher signature and checksum, stages a content-addressed executable, probes its actual version, and atomically selects it only while no participating checks/tools hold an OS-backed workload lease. Interrupted downloads, failed probes and corrupt candidates cannot replace the selection. The signed installed bootstrap is preserved before the first activation; the previous generation is retained; `malveon updates recover` explicitly verifies and restores it while idle. New commands delegate through the stable bootstrap. Idle MCP processes reject new work after selection changes; the extension serializes calls, reconnects and retries once. Existing pre-upgrade processes cannot participate retroactively: reconnect them once when migrating.
- The Windows installer enables a per-user scheduled task every six hours, with a hidden bounded helper; it can update while the editor/application is closed, while the user session is available. `malveon updates enable|disable|status` manages it. `MALVEON_NO_UPDATE_CHECK` and `DO_NOT_TRACK` stop automatic network checks. Disabling does not remove existing engine versions. Scheduler availability and a successful task registration are required. Automatic background installation is Windows-only; macOS/Linux keep explicit updates. Homebrew, Scoop, WinGet and npm remain owned by their package manager and are never overwritten by this updater. The editor does not downgrade a newer installed release to an older bundle. Extension packaging uses its manifest version for the engine by default and rejects mismatched release labels; a `dev` engine must be requested explicitly.

These changes reduce the demonstrated false accusations and false passes. They do not make verification universal across languages/frameworks, prove all business behavior, or guarantee zero false positives. Unsupported syntax/effects and semantic judgments remain explicit limits.

## What it catches

| What goes wrong | What malveon does |
|---|---|
| The AI says the Save button is done, but nothing is hooked up | Follows each item from the page to the server route to what gets saved. A button with no click handler, or one that does nothing, comes back `HALF BUILT` |
| A page sends one thing and the server expects another | Compares the HTTP method and the fields the call sends with what the route reads |
| Something that worked now doesn't | Compares your code with how it was when the task started: a call that now goes nowhere, an import that no longer works |
| The AI added things you never asked for | Lists new requests, pages, database tables, buttons that change data and packages, plus changed files nothing you wrote points at |
| The AI claims something in its notes, commits or final message | Looks each claim up in the code. One the code contradicts blocks |
| Half-finished work | Lists `TODO`s, stub functions, sample data, leftover debug logs, commented-out code, skipped or deleted tests |
| Requests you typed in chat, not in a plan | Checks your prompts too (optional one-time setup). A vague one like "make it better" gets a "how will I know it's done" line from the agent, which malveon keeps and lists as yours to confirm |
| The AI marking its own homework | Static facts use deterministic checks; optional platform-model interpretations are separately labeled assessments |

Want malveon to speak up while the AI works? There is an optional setting for Claude Code, Codex, Cursor and Antigravity. Malveon never stops the AI: it tells you, and for rules you trust, the AI. See [Telling your agent](#holding-your-agent-optional-claude-code-codex-cursor-antigravity).

## What it doesn't do

It reads code. It never checks a deployed site and never guesses a command to start your app; it starts your app only when you pass the command yourself (`--layout --start "<command>"`, approved once at a terminal), and it opens a browser only for the optional layout check (see "Screen layout" under Every check), so `CONNECTED` establishes only the specific extracted source connection, not every promised behavior or a real click. It doesn't prove your business logic is right. Two things go beyond reading: `--run` (opt-in) runs your project's own build, lint, type-check and test commands and reports what they really said, and malveon sends small anonymous usage signals (see [Usage tracking](#usage-tracking)). Full list: [What it can't do (yet)](#what-it-cant-do-yet).

## Start in two minutes

You don't need a plan, a session or any setup for the first look.

1. In your project's folder, run `npx malveon` (no install; [or install it](#install) and run `malveon`). It lists buttons that do nothing, calls to addresses nothing serves, imports that can't work, routes that only answer "not implemented" and files nothing in the app reaches, in seconds, three at a time, and changes nothing.
2. Just had an AI work on the project? Run `malveon --since HEAD~1` (or an older commit). It checks the commits the AI made, using their messages as what you asked for, and says what it added that nothing asked for, what it broke and what it left unfinished.
3. To have every task checked from now on, run `malveon on` once. Or give malveon a plan: `malveon --plan PLAN.md` ([what a plan looks like](#the-plan-file); a plain checklist works).

Using VS Code or Cursor? Search "malveon" in Extensions. The command "Malveon: Scan this project" does step 1.

## Install

Pick your system, copy the one line below, paste it into a terminal, and press Enter.

**On a Mac or Linux:** open the Terminal app, then run:
```bash
curl -fsSL https://get.malveon.com/install.sh | sh
```

**On Windows:** open PowerShell, then run:
```powershell
irm https://get.malveon.com/install.ps1 | iex
```

That downloads the right file for your computer and sets it up so you can just type `malveon` from any folder, from now on. On a Mac, it also clears the security warning new downloads normally get, so your first run isn't blocked.

Downloads must match the release checksum list. `malveon update` always verifies its publisher signature; the Mac/Linux installer verifies the signature when OpenSSL 3 is available. The Windows bootstrap installer checks the checksum from GitHub over HTTPS, not the signature. A file that doesn't match is refused, nothing is installed.

**Other ways to install** (same signed files):

| | |
|---|---|
| npm (any OS with Node 16+) | `npm install -g malveon` |
| Homebrew (Mac, Linux) | `brew tap ladsondavid/malveon https://github.com/LadsonDavid/Malveon` then `brew install malveon` |
| Scoop (Windows) | `scoop bucket add malveon https://github.com/LadsonDavid/Malveon` then `scoop install malveon` |

Now close your terminal window and open a brand new one. This step actually matters: your current terminal doesn't know about the change yet, only a new one will. Then type:
```bash
malveon
```
If you see a list of commands instead of an error, it worked.

Nothing else to install first. No Go, no Node, no extra toolchain. It's one file.

**Prefer to do it by hand?**

On Mac or Linux: download the file for your OS from the [Releases](../../../releases) page, run `chmod +x` on it (this just tells your computer the file is allowed to run), then move it into a folder already on your PATH, like `/usr/local/bin`.

On Windows: downloading the `.exe` by itself is not enough to make `malveon` work as a bare command. The file also needs to be renamed to `malveon.exe` and placed somewhere on your PATH, and there's no single folder every Windows setup already has ready for that the way `/usr/local/bin` works on Mac or Linux. Run the install script above instead, it does exactly this for you and has been tested to actually work. If you'd rather not run any script at all, download the `.exe` and just run it directly from wherever you saved it, for example `.\malveon-windows-amd64.exe check`, no install needed, just typed from that exact folder every time.

## How to use it

**No plan yet? Start here.** One command needs nothing set up:

```bash
malveon           # what's wrong in the project right now
```

It reads the whole project and lists the faults that need no plan or session to see: a button with no click handler (or an empty one), a call to an address none of your routes serves, an import of a file or name that doesn't exist, a route that only answers "not implemented", a file nothing in the app reaches. It shows the three most certain, says how many more there are (`malveon --all` lists them), and says what it did not look at (languages it has no reader for, see `malveon help languages`).

**After an AI has worked.** When a session is open (`malveon on`, or your agent's prompt hook opened one) plain `malveon` also compares with the start: what was added that your words do not name, what existing code the change may have broken, what the agent claimed about its tests, whether tests were weakened. `malveon --since <ref>` does the same without a session, using the commits since `<ref>` and their messages as the request (a weak stand-in).

**With a plan.** `malveon --plan PLAN.md` (repeatable) checks each plan line: built and connected, broken, missing, half done, or not checkable.

```bash
malveon --plan PLAN.md --run
```

`--run` also runs your project's own build, lint, type-check and test commands. It asks once, in a terminal, before running them (the approval is remembered by a hash of the files that define the commands, so changing a script asks again); with no terminal and no earlier approval nothing runs and it says so. A command that fails is run once more; a failure that passes the second time is labelled flaky and is not reported as a plain failure.

### Options

- `--since <ref>`: check what changed after this commit.
- `--plan <file>`: a plan file (repeat for more).
- `--run`: also run the project's commands (asks once).
- `--all`: every finding and the full detail.
- `--json <path|->`: write the findings as JSON (`-` is standard output; the text then goes to standard error).
- `--exclude <dir>`: skip a folder by name, anywhere in the tree (repeat for more).
- `--layout`: open pages in your own Chrome or Edge (with `--url`, `--file`, `--widths`).
- `--layout --start "<command>"` (with `--ready-path`, `--start-timeout`; local source, unreleased): malveon runs your command on a free loopback port it picks (`{port}` in the command, and `PORT`, are filled in), waits until that port answers 2xx or 3xx while the started process is still alive, looks with `--layout`, then stops the whole process tree, also on an error, a time-out, Ctrl+C or SIGTERM. You approve the exact command once at a real terminal (a changed command asks again); with no terminal and no earlier approval nothing runs. The app runs with your environment and your rights, so it can reach your database and send what your app sends; malveon cannot see those side effects. It never reuses a server that is already running. A kill -9 of malveon itself, or a power cut, can leave the app running. Only Windows was run.

## Failing a build yourself

malveon never blocks and never fails a build: it exits 0 whenever it checked, and 1 only if it could not run (a bad option, an unreadable folder) or you typed a removed command. If you want a commit or a CI step to fail on what it finds, read the JSON and decide yourself, for example:

```bash
#!/bin/sh
# .git/hooks/pre-commit - fail when malveon found something it trusts
malveon --json - 2>/dev/null | python3 -c "import json,sys; d=json.load(sys.stdin); sys.exit(1 if d['findings_verdict']['word']=='Found problems' else 0)"
```

`findings` is the list shown to people (id, headline, where, why, next, strength Sure | Likely | Note); `findings_verdict` is `Found problems`, `Worth a look` or `Nothing found in the code I read`. The older per-check fields stay in the JSON. malveon never installs a git hook for you.

## Usage tracking

malveon sends small anonymous signals so the person building it can see whether anyone actually uses it (a download doesn't say that). Each one carries your OS, CPU type and malveon's version, plus:

| Signal | When | Also includes |
|---|---|---|
| first run | the first time malveon runs on a machine | nothing else |
| check | each `malveon`, from the terminal, your editor or your AI agent | clean or blocked, whether your build/tests ran, how many of your prompts were captured, and the name your agent or editor gives itself |
| scan / review | each plain `malveon` or `malveon --since`, from the terminal, your editor or your AI agent | how many problems it found (a number, never what they were), how many commits a review looked at, how long it took, and whether `malveon` fell back to a scan because you had no plan |
| layout | each `malveon_layout` call from an AI agent or your editor | how many things it found (a number, never what they were), whether a browser was found and how long it took |
| command | `update`, `session`, `register`, `hooks`, `prompt`, `trust` | the command's name |
| agent connected / tool used | an AI agent or the editor connects to `malveon mcp` | the agent's or editor's own name, the tool's name |
| update | an update is offered, installed or fails | the two version numbers, and a one-word reason if it failed |
| error | a check, scan or review can't run | a one-word kind, never the message |
| plan | with each check | whether you are on the free trial, at its limit, or paid, and how many runs you have used in steps (0-49, 50-99, 100-149, 150+); never your key or email |

It never sends your code, file names, plan, prompts, commit messages, error messages, or anything else about your project, and never looks up your location (turned off on purpose). Typing a prompt to your agent never sends anything. Downloads made through `get.malveon.com` (the install scripts, `malveon update`, npm) are counted there too: which file and where it was downloaded from, nothing about you.

The only thing it remembers between runs is a random ID stored on your own machine, so two runs from you count as one person, not two. That ID isn't tied to your name, your email, or anything that could identify you.

## Use it from your AI agent (MCP)

`malveon mcp` runs malveon as an MCP server, so Claude Code, Cursor, Antigravity, or any other MCP client can call the `malveon_check` tool itself and read the same report you'd see in the terminal, GATE line included. No copying output into the chat. The `malveon_scan` tool does the plan-free scan, so an agent can look at a project nobody has set up yet. The `malveon_changed_walk` tool (same rules) walks you through existing things the agent changed. The `malveon_unused_walk` tool (local source, unreleased) returns what the agent built that nothing seems to reach and a fixed-wording list of things for *you* to check by hand; no tool can record an answer, only you can, in a terminal.

Claude Code:
```bash
claude mcp add malveon -- malveon mcp
```

Cursor, Antigravity, and other clients: add this to the client's MCP config file:
```json
{
  "mcpServers": {
    "malveon": { "command": "malveon", "args": ["mcp"] }
  }
}
```

If more than one plan file could be the plan, malveon asks you to pick, right inside the agent's chat, instead of guessing.

## Check your prompts too, not just plan docs

Most of what you ask an agent for never makes it into a plan doc. malveon can check your prompts as well. `malveon on` sets it up: it shows exactly what it will add to each agent's settings and writes only after you say yes.

```bash
malveon on
```

Run it once, in a terminal, inside your project. It finds which agents this project uses (it sets up Claude Code and Codex from their folders; Cursor and Antigravity only when your editor is one of them or you name them with `--agent`, because their hooks have never been run against the real agents), shows you exactly what it will add to each one's settings, and only writes after you say yes. From then on:

- Every prompt you type is saved word for word to `prompts.jsonl` in this project's Malveon user-data directory (see `malveon paths`). It comes from your editor's own hook, never from the agent. The state is outside your checkout, and nothing about your prompts is ever sent anywhere.
- Your first prompt starts a session by itself, so you never start one by hand. A check that comes back clean ends the session, and your next prompt starts a new one.
- Through the MCP tool, the agent says what each prompt asked for, one checkable line at a time: "`GET /api/search` is served and called from `SearchBar.tsx`". malveon checks every line against your code, exactly like a plan line. What the agent says is only a list of things to check, never proof. Each line must name a URL, file, or code name and be about what you actually asked, and once recorded it can't be edited or removed.
- The report gets a new section, "Your prompts vs the code," with one answer per prompt: CONNECTED, BROKEN, NOT BUILT, HALF BUILT, NOT TRANSLATED (no checkable interpretation was supplied, or its source lookups remain unresolved), NOT A CODE TASK (the agent's word, listed for you to review), or SKIPPED (a short reply like "yes", or a follow-up like "make it faster" that carries on the work before it). A short prompt that asks for something ("fix the login", "add dark mode") is a task like any other and is checked; only replies and follow-ups are skipped. Any prompt with an answer from the code also lists your words that none of the agent's lines mention, so you can spot a request that got quietly dropped; those words are also named in the verdict as yours to settle.
- **A vague request** ("make the checkout faster": it names no file, URL or code name) can't be settled by reading code, so malveon doesn't pretend to. Instead, in Claude Code, the prompt hook asks the agent to say, before it starts, how you will know it is done (`done_when`): a number ("loads in under 1 second") or a file, URL or test it will point at. "It works better" is refused, because it can't be wrong. malveon keeps the line word for word, only ever adds to it, and when the agent says it is done, shows you: what you asked, what the agent said would show it done, and what malveon could do with that. A number is a measurement malveon never makes; something named is looked up in the code like any claim, and the fact is shown. Neither blocks, and neither settles the prompt: only you can. If the agent never gave a done line, the report says so. Other agents don't get the ask from the hook (their hook output is unverified); their agent is told through the MCP tool instead.
- If you cancel something ("actually, drop the search box"), the agent can only propose withdrawing the earlier prompt. You confirm it, either when malveon asks you in the agent's chat or with `malveon settle drop <number>` in a terminal.
- If your prompt names a URL, file or code name but the agent's lines are about something you didn't type, malveon asks you, in the agent's chat, whether those lines are what you meant. A prompt that names nothing ("make the header sticky") isn't asked about: the agent's lines are then only its guess at what you meant, so malveon lists the prompt as needing you at the end (`NEEDS YOU` in the report) until you look yourself or run `malveon prompt confirm <number>`. Nothing the agent sends can answer for you. If your agent can't show the question, or you dismiss it, approve in a terminal with `malveon prompt confirm <number>`; it shows you the prompt and every line first. Words of your prompt that no line mentions and no code can show ("faster", "nicer") can be marked as yours to check with `malveon prompt waive <number> <words>`. The approval is recorded, shown to the agent and in the report; it only stops the agent from editing code if you turn on the edit gate (below).

Codex asks you to approve the new hook once, with `/hooks` inside Codex.

### Telling your agent while it works (optional: Claude Code, Codex, Cursor, Antigravity)

**Since 2026-11-01 (local source, unreleased) malveon never holds, refuses or blocks an agent at any stage.** The hook replies are always "allow". Where the text below says "holds" or "refuses", read it as what malveon works out it *would* have held: you are told that ("malveon does not hold the agent, but it would have held this: …"), the agent is told nothing from it, and nothing is stopped. Why: feature 5 of the 2026-11-01 plans, decision 13. The old behavior is kept as an engine so its rules stay tested.

What the hooks do now, for Claude Code (the other three agents' hooks never hold either, but their edit text is not read):
- **Before an edit** malveon reads the edit as it would land and runs fixed rules on it: `SECRET` (an added live-looking key), `STUB` (an added function that only says it is not implemented), `TESTDEL` (an edit to a test file that removes tests or assertions, skips a test, or adds an assertion of a constant), `PACKAGE` (a runtime dependency nothing you wrote names), `BOUNDARY` (a file outside the folders you listed in `malveon.json` `boundary`). **After an edit lands** it checks whether the edit broke an import (`IMPORT`) or a request that reached a server route at the session start (`ROUTE`), within 3 seconds, and says nothing if it runs out of time.
- **Every rule starts in `shadow`: it only writes to a log** in your data folder (`gate/outcomes.jsonl`, a hash chain whose newest fingerprint is also kept outside the project, so a changed or removed record is detected; detected, not prevented). You judge a finding with `malveon settle <n> right|wrong` (a terminal and a typed `y`; an agent's shell is refused). `malveon rules` prints each rule's record. A rule with at least 16 right verdicts and none wrong (a 95% margin-of-error lower bound of 80%) can be promoted with `malveon settle promote <RULE>`; it demotes itself if its record stops qualifying. A promoted rule tells **you** and **the agent** when it fires, up to 10 heads-ups to the agent per session; the agent hears only the rule, file path, line and a fixed next step, never source text or your prompts. The strong word "sure" would need 35 judged cases and 95% precision; nothing has earned it yet.
- The agent's heads-up is **not** rewritten by `--local-only` (the hooks always pass it). To make it one generic sentence instead, set `MALVEON_AGENT_TEXT=generic` or put `generic` in a file named `agent-text` in the `settings` folder of your data folder.
- **The briefing** (off): `MALVEON_BRIEFING=on`, or `on` in a file named `briefing` in the same `settings` folder, adds a note when you send a prompt that names code: the files, routes and code names you wrote, the optional `boundary`, and "not requested: new packages, new tables, new pages or buttons". Off by default because it is not yet decided whether every named prompt should get one.
- When Claude says it is done, you are also told if the project has no server routes at all, so route claims could not be judged.

`malveon on` sets these hooks up and `malveon off [--for 2h]` pauses the warnings (a terminal and a typed yes; an agent's shell is refused). The old `hooks install --gate` and `gate` commands are gone: nothing is ever held.

- Speed: the first "done" in a session on a large project takes about 10 seconds (malveon reads the code as it was at the start), later ones about 2 seconds, and a "done" with nothing new since the last clean check is instant.
- If malveon itself fails, runs out of time (the stop check waits at most 20 seconds), or isn't found, the hooks let Claude carry on.
- The hooks call the exact program you ran `hooks install` with, and the command shows its path before you say yes. If you have an older malveon installed elsewhere, run the new one by its full path.
- Hooks and MCP servers are read when a Claude Code session starts, so open a new session after installing.

What it does not do: it only sees each agent's file-editing tools (Claude's Edit, Write and NotebookEdit; Codex's `apply_patch`; Cursor's Write, StrReplace and similar; Antigravity's `write_to_file`, `replace_file_content` and similar). An agent can still change a file through its shell tool, so this is a speed bump, not a lock. It never runs your build or tests (use `--run` for that). The Claude Code hooks have been checked against Claude Code 2.1.237 on Windows only, using a scripted stand-in for the model, so they show how Claude Code reacts to being held, not how a real model does. The Codex, Cursor and Antigravity hooks have not been run against those agents at all (see above).

## Updates

The interactive update notice checks GitHub at most once a day. The unreleased Windows idle updater described above uses a separate six-hour scheduled helper (and the shared cache). `malveon` asks GitHub's public releases page whether a newer malveon exists. This request doesn't include anything about you or your project. If a newer version exists, it tells you, and in a real terminal it asks `Update now? [y/N]`. You can also run `malveon update` any time to get the latest release, and `malveon version` shows which version you have. Set `MALVEON_NO_UPDATE_CHECK=1` (or `DO_NOT_TRACK=1`) to turn the daily check off.

## Uninstall

Remove malveon the same way you installed it:

| Installed with | Remove with |
|---|---|
| `curl … install.sh` | `sudo rm /usr/local/bin/malveon` (or wherever `MALVEON_INSTALL_DIR` pointed) |
| `irm … install.ps1` | delete `%LOCALAPPDATA%\malveon`, then remove that folder from your user `Path` |
| npm | `npm uninstall -g malveon` |
| Homebrew | `brew uninstall malveon`, then `brew untap ladsondavid/malveon` |
| Scoop | `scoop uninstall malveon`, then `scoop bucket rm malveon` |
| VS Code / Cursor | uninstall the malveon extension from the Extensions view |

First run `malveon updates disable` on a Windows installation with idle updates enabled. Use `malveon paths --json` before removing the executable to locate its data. If you also want to remove saved history and settings, back up any original-plan snapshots or prompt history you need, then remove that data directory. Legacy project `.malveon/` directories and old user-home state may remain as recovery copies; remove those only when you no longer need them. Package-manager files and publisher signing material are separate. If you ran `malveon on`, remove the malveon entry from `.claude/settings.local.json`, `.codex/hooks.json`, `.cursor/hooks.json` or `.agents/hooks.json`.

## The plan file

Whatever format you already keep your plan in — no fixed shape forced on you, and no particular filename or location required. Name it with `--plan PLAN.md` (repeatable); plain `malveon` uses a plan you already named once (saved in malveon's own folder) and otherwise does not guess. When you do not name one, it only looks (a scan of the project) and does not ask you questions. What counts as a plan line:

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

Tick a box (`- [x]`) when the plan says that item is done: then malveon blocks if the code disagrees. Unticked items (`- [ ]`), plain bullets and numbered lines are checked too, but they don't block until you tick them (unless what's there is broken).

The more precise a line, the more precise the answer. "Refund button" gets NOT BUILT if nothing in the code mentions refunds, and SKIPPED (not checked, with the closest routes and pages in your code listed) if some code does but malveon can't tell which. Naming the URL, file or component (`` `POST /api/refunds` ``, `` `RefundButton` ``) lets it check exactly that. Any common URL placeholder style works: `:id`, `[id]`, `<id>`, `{id}`.

**Plain text** (`.txt`, or anything else): one feature name per line.

## Your saved settings (formerly `malveon.json`)

Your plan files and a few options are saved by malveon in its own folder for the project (`malveon status` prints where), not in your project. The first time, an old `malveon.json` is copied there (checked by SHA-256) and then ignored; if it differs from the saved settings, the saved ones win and `status` says so.

- `plans`: your plan file(s), relative to the project root.
- `exclude`: folder names every check skips, anywhere in the project, the way `node_modules` already is: `"exclude": ["generated", "old-docs"]`. On the command line: `--exclude generated` (repeatable). Folder names only, not paths or patterns.
- `entry_points`: files that something outside the project starts (a worker a scheduler runs, a script you run by hand): `"entry_points": ["src/worker.ts"]`. The unused-work check treats them as used. Up to 64; slash paths inside the project.
- `called_from_outside`: routes or files that something outside the project uses: `"called_from_outside": ["POST /api/webhooks/stripe", "src/cron/nightly.ts"]`. A route is written `METHOD /path` (`ANY` allowed). A bad entry is an error, not guessed around.
- `boundary`: folders the agent was allowed to work in; a changed file outside them is listed.

A flag you pass on the command line always wins over the saved settings. A fresh clone, another machine or a throwaway cloud environment starts without them.

**malveon only runs your project's commands after you approve them.** The approval is asked once, in a terminal, the first time you use `--run`, and is remembered by a hash of the files that define the commands (`package.json` scripts, `Makefile`, test and build config): change one and it asks again. With no terminal and no earlier approval, nothing runs and it says so. The approval prompt lists the exact commands and says that they run the project's own code, including its test files.

## Declared behavior checks (local source, unreleased)

Use this optional mapping when an existing root `package.json` script checks a particular original plan obligation:

```json
{
  "plans": ["PLAN.md"],
  "exec": true,
  "behavior_checks": [
    {
      "id": "double-output",
      "plan": "PLAN.md",
      "line": 1,
      "files": ["src/math.ts"],
      "script": "test:double"
    }
  ]
}
```

For example, the original `PLAN.md` line is `- double returns twice its input`, and the existing `test:double` script asserts that a consumer of `double(3)` returns 6. Changing the helper to return 9 can fail that script even while all imports remain connected. Malveon retains the actual failure output and lists extracted import paths from the changed helper to potential consumers. The script and assertions are authored by the project; Malveon does not infer or generate them from prose.

- `plan` must be selected for this run (normally in `plans`); `line` identifies a span of its **preserved original**, not a rewritten current plan. The report includes the exact original span, span ID and original SHA-256. Missing selected spans stop the run before project execution. Existing history is not reset. To deliberately amend a plan, use the existing human-terminal acceptance workflow.
- `files` is 1–64 JS/TS paths (`.js`, `.jsx`, `.ts`, `.tsx`, `.mjs`, `.cjs`), relative to the root with `/` separators and no traversal. These anchor import-impact reporting; they do not select tests or establish the meaning of the requirement. There are at most 64 checks, with unique IDs. IDs and script names accept letters, digits, `:`, `_`, and `-`, starting with a letter or digit.
- `script` must already exist in the root `package.json`. The same npm/pnpm/yarn detection as normal commands is used. TypeScript execution/transpilation belongs to that existing script; Malveon installs nothing and invents no test command. Missing scripts or package managers produce NO PROOF, not a silent skip. Normal detected commands still run as before; a script already called by your general test script may execute again.
- Scripts run only through existing approved execution. The human approval preview lists them; changing config/recognized command sources invalidates the existing approval. `--exec=false`, automatic editor checks and Stop hooks do not execute them; the report says NOT RUN and retains the review need. MCP requests cannot approve themselves. An approval authorizes project scripts and their effects, not just static inspection; this feature is not a sandbox.
- Requested execution failures and unavailable configured scripts block through the command gate. The same per-command timeout/process cleanup and 4,000-byte trailing-output cap apply. Matching baseline outcomes are kept separate by check ID and mapping/root-package-definition hash. A passing baseline and current failure demonstrate that the command outcome changed; they do not independently diagnose a business-logic defect or establish environment parity.
- Potential impacts follow current transitive file-import edges from declared files changed since the recorded session boundary, with up to 256 paths per check and an explicit truncation flag. Type-only and test imports may appear. No paths is not proof of no impact: deleted files, dynamic discovery, unsupported imports and external consumers can be missed. Scripts run regardless of whether any path was found. Unknown session changes are reported as unknown.
- CLI/MCP text and JSON (`behavior`, an additive schema-v2 field), the editor page/sidebar and Stop review notes show these evidence bands separately. A **COMMAND PASS** means that command exited successfully; it does not establish that any tests ran, that assertions cover the prose, or that a declared mapping preserves intent. Semantic coverage remains unverified and needs review even when the command gate is clean. No wiring verdict or original obligation is replaced by this evidence.

This increment does not add assertion-count adapters, mutation testing, formal verification, runtime dependency tracing, browser rendering, remote/dev-environment proof, or team/merge orchestration. Existing project scripts may themselves start processes; Malveon adds no server/browser startup or dependency installation. These are local source changes and do not upgrade installed or published releases.

## What the output looks like

A small example project: a plan with four lines, a frontend in `frontend/`, a server in `backend/`. Two lines are built and connected; two are broken in ways an agent could easily call "done". By default you get the short view:

```text
$ malveon --plan features.json
malveon checked 4 files, 4 plan items. Code read only; your app was not run.

Found problems (2)

  1. Plan item is broken   PLAN.md:3
     "cancel order" — the frontend calls GET /cancel-order (frontend/app.js:8), but no server route handles it
     Next: Fix the connection the line describes, or change the line.

  2. Plan item is broken   PLAN.md:4
     "update settings" — the frontend sends PUT /settings (frontend/settings.js:2), but the server only accepts POST there (backend/settings.js:3)
     Next: Fix the connection the line describes, or change the line.
```

`malveon --all` adds every finding with its evidence and then the older per-check report (an OVERVIEW line per check, then a section for each check that has something to say). A check with nothing to show is just a line in the overview; plan lines that aren't code tasks are counted, not listed. `malveon --json -` prints everything as JSON.

## Every check, in detail

The summary near the top is the short version. This is every check, with its exact scope and limits.

`malveon` reads the actual code and git history, and for one opt-in check actually runs your project's own commands. It reports these things:

- **Scan** *(plain `malveon`, needs no plan, session or setup)* — four kinds of fault, each judged only where malveon can be sure, so a short list is expected:
  - *Dead button*: a `<button>` or `<Button>` in JSX/TSX with visible text and no click handler, or one that is an empty function. Not judged: an icon-only button, a file with spread props or a framework attribute (`hx-post`, `@click`), a button just under a link, a label, a dialog or menu trigger or `asChild` (the element around it may handle the click), and stories and tests.
  - *Broken import*: a relative import of a file that doesn't exist, or of a plain name the file doesn't export. Not judged: a package or alias (`@acme/core`, `@/x`), an asset, a path through a build-output folder, a file that re-exports names malveon can't list (`export *`, `export type *`), and anything not JavaScript or TypeScript.
  - *Call to nowhere*: a plain `fetch`/`axios`/SWR call with the address written out in quotes, to an address no route in the same app serves with that method. Judged only in an app (the nearest folder with a manifest) that has other routes under the same first path segment: `/api/missing` beside `/api/items` is a typo or a route nobody wrote, while `/v1/chat/completions` beside no `/v1` routes is another service. A call through a helper (`ghGet(token, '/repos/...')`) isn't judged.
  - *Unfinished route*: a JavaScript, TypeScript, Python or Go handler that only answers with an error or "not implemented" (sometimes on purpose: the report says what it saw, not that it's wrong).
  It lists the three most certain first (dead buttons, then imports, then calls, then routes). Run on four real projects it found 2 problems in a 320-file app (both dead buttons, checked by hand), 5 in a 1,071-file monorepo (all stale imports of files that no longer exist), 3 in a 2,504-file monorepo (two stale imports, one deliberate 501) and none in an 84-file app. Its first version listed 51 problems on the 1,071-file monorepo, nearly all wrong; every restriction above is one of those.
- **Work nothing reaches** *(`malveon unused`, also a section of `malveon`; local source, unreleased)* — did this session build files, functions or routes that nothing in the app reaches? From the places the outside world enters the app (pages, files a framework loads by name or folder, `package.json` `main`/`bin`/`exports` (a built `dist/` file maps to its `src/` file) and any file a script names (a runner, or a bundler entry like `esbuild src/extension.ts`), files named like `main`/`index`/`server` at the top, a page's `<script src>`, and anything in `entry_points`), malveon follows the imports and the calls to routes it can read. A new file nothing reaches is **possibly unused**; one reached only by a name written as text, only by type-only imports, or only by tests, or a route shaped like a webhook, cron, callback or health check, is listed with its reason as **can't tell**. Also: a new private function (not exported, in a file that already existed) whose name nothing in the project mentions. Items that link to each other are grouped as one stranded feature. Wording stays "possibly unused": the stronger wording is switched off until it has been measured (95% precision and a Wilson lower bound of at least 90%, on cases from at least three projects), and nothing has been measured yet. **It never blocks anything.** Limits: reachability is read for JavaScript and TypeScript only (other languages get only a "is this name written anywhere else" test, always can't tell); new pages are not judged because links to pages are not read yet; a route is not judged in a project with no screens, or when you declared it called from outside; a request whose address is built while the app runs makes routes can't tell; a framework that loads files by a convention malveon does not know can make working code look unused (declare it in `entry_points`); the answer to "is it used" is only as good as the links malveon can read. `malveon unused walk` is the on-request walk: a short fixed-wording list, nothing invented, one thing to try per item. `malveon unused answer <id> <answer>` records what *you* saw (needs a terminal, so an agent cannot answer for you); an answer is kept while the code under it is unchanged and asked again when it changes. Answers live in the project's private data folder (`unused/doubts.json`), never in the repo.
- **Existing things that changed** *(`malveon changed`, also a section of `malveon`; local source, unreleased)* — of the things that already existed when the session started, which did the agent change in a way other code may not survive? It compares the code now with the session start: (1) the inputs of every exported JavaScript, TypeScript, Go or Python function in a changed file (for Go a changed input *type* counts too, because the Go compiler would reject the old callers; a variadic input is the rest; for Python the way an input can be passed counts: by position, by name, name-only, position-only, `*args`, `**kwargs`; a Python class counts as its constructor, a static or class method as `Class.method`) — a new required input, a removed or reordered input, an optional input made required, a lost rest input; a renamed parameter, a type change or a new input that is optional or has a default is not a change a caller must follow; (2) whether each call to your own server still sends every field its route insists on. A changed function with a caller the agent did not change is **at risk**; a function whose inside changed (same name, same inputs) with an untouched caller is a weak **changed inside** note (three listed at most, never in the verdict line); a call that sent every needed field at the start and misses one now is **probably broken**. The strong wording is off until measured. It never blocks, counts what it looked at ("Checked: N requests, M imports, K changed functions") and says what it could not read (odd forms). Limits: JavaScript, TypeScript, Go and Python only (a changed file in any other language is named in "could not check"); for Go and Python only direct callers are followed (no walk back through importers of importers); for Go only plain functions are followed to their callers, a method is reported as "callers cannot be found by reading", and a generic function is "can't tell"; a Go function's return type is not compared; for Python callers are found by reading imports (`from x import f`, `import x`, `from x import *`, relative imports, `__init__.py` re-exports), resolved by the end of the module path so a `src/` layout needs no settings, and when two folders could be meant the one your `pyproject.toml`, `setup.cfg`, `pytest.ini` or `tox.ini` names as an import folder wins; a Python function whose definition is in a branch, a decorated function (a library decorator, a class, one with arguments we cannot read) or a class with inherited, generated or metaclass-built constructors is "can't tell" — except a decorator defined in your own project that only passes `(*args, **kwargs)` on, which is read as not changing the call; instance methods are not read (they are called through a value whose type reading cannot tell); a name picked while the program runs (`getattr` with a variable name, `import_module`, `__import__`) cannot be followed, and when the project has files that do this the report says so; with `--run`, if your project sets up mypy or pyright and it is installed, malveon runs it on the code now and on the session-start copy and adds only the errors that are new: one in a file that relies on a changed function is written into that item ("mypy reports a new error in a file that relies on it"), the rest are listed apart (never a verdict, never a block; the copy holds only tracked files, so an error that depends on an untracked file can look new; that part has been tested with stand-ins for the two tools, not with the real mypy or pyright); it cannot tell whether a function still returns what its callers want; route cuts and broken imports are reported by the new-bugs check, tables and pages are not compared; a request built from a variable is not compared; type-only imports are ignored. `malveon changed walk` is the on-request walk (existing screens to look at, fixed wording); `malveon changed answer <id> <answer>` records what *you* saw (terminal only; kept while the code is unchanged, asked again when it changes; stored in the project's private data folder).
- **Things your words do not name** *(part of the "Unplanned code" section of `malveon`; local source, unreleased)* — what the session added that no line of your request names, ranked: tier 1 database tables, packages, buttons that send a changing request or change data, requests to other servers; tier 2 routes, requests, pages, files changed outside a `boundary` you declared; tier 3 button labels; tier 4 things reached from a button you named and new files such a button's file imports. A route is "named" only by its URL with its method, or all the words of its path in one line (words inside a file path do not count). A request that names no URL, file or code name is only *listed*, never compared. The report shows what it looked at, says when the agent's final message did not mention an addition, and never blocks; the wording is "not named in your words" until its precision is measured. `malveon.json` `boundary` (optional, folders relative to the root, slashes, no `..`). `malveon unasked` shows what the last check recorded; `malveon unasked answer <id> wanted|not_wanted` records *your* decision (terminal only; kept while the thing is unchanged). Limits: a button is judged only when its handler is in the same file; a package swapped for a similar one reads as added; the host of a request to another server is read from one source line.
- **Claims about its own results** *(a section of `malveon`; local source, unreleased)* — when the agent says "all tests pass", "the build succeeds" or "lint is clean" (final message, commit messages), malveon sets the claim beside evidence that is not the agent's words: a run **you asked for** (`--run`), and the agent's own log of what it ran (Claude Code's transcript, read at its "done" hook). A failed requested run, or the agent's own failing last run on code it did not touch since, is **contradicted**; a passing requested run is **confirmed by a run**; a pass in the agent's log alone is "its record, not proof"; "ran, then edited files", "never ran" and "no command defined" are **no evidence**; an older passing run on older code is **stale**; a cause such as "the server is slow" is **not checkable here**. A contradiction read only from the code is worded "looks wrong". The build/lint/typecheck/test commands the project defines are listed with what the log shows, whatever the agent wrote. Malveon never runs your commands on its own; test output may hold a secret, so only the last 4,000 characters are kept locally and never shown to the agent. Never blocks. Limits: English closed phrases; Claude Code's log only; a command wrapped in an unusual way may not match; your real app is never run.
- **Rules while the agent works** *(`malveon rules`, `malveon settle`; local source, unreleased)* — see "Telling your agent while it works" above. Limits: the rules read one proposed edit (or, after it lands, the project map), so they catch only mistakes of that narrow kind; edits made through shell commands are not seen; only Claude Code's edit text is read; the rule log is local, never sent; with one user most rules stay in `shadow` for a long time, since a promotion needs about 16 judged cases.
- **Screen layout** *(`malveon layout`; also a small automatic run inside `malveon`; local source, unreleased)* — opens pages in **your own Chrome or Edge** (headless, a fresh temporary profile, nothing downloaded) and lists what a person would see by looking: text on text (`TEXT_OVERLAP`), cut-off text (`CLIPPED`), a page wider than the window (`SIDEWAYS_SCROLL`), unreadable colours (`CONTRAST`, WCAG ratio; not judged over images, gradients or transparency), a blank page or a script error (`BLANK`), an image that failed to load (`BROKEN_IMAGE`). Wording is fixed sentences filled with facts: `TEXT_OVERLAP — index.html at 375 px (light): "Title" and "NEW" look overlapped (about 691 px²).` Window widths come from your own CSS (every `min-width`/`max-width` in a media query, minus one, itself and plus one, plus 320 and 1440; `em`/`rem` count as 16 px; at most 12), so a fault that exists only between 600 and 700 px is found; `--widths 320,600` replaces them. Dark mode is measured only when your CSS has a `prefers-color-scheme: dark` rule. Each measurement runs twice; a page whose two runs differ is listed as unstable, not as a fault. Every run ends with a coverage line (pages measured of total, widths, themes, requests blocked, contrast checks not judged, unstable, browser build) because a quiet result proves nothing without it. Pages: with a session, the changed `.html` files and the `.html` files that link a changed stylesheet; with none, every static page (20 at most); `--file PATH` opens one; `--url http://localhost:3000/` attaches to an app on this machine (localhost, 127.0.0.1 and [::1] only, `http`/`https`, no user name). A changed framework or stylesheet file that no static page uses is named "needs a running app". Browser: `MALVEON_BROWSER`, or Chrome/Edge in the usual places (Linux, macOS and Windows places are from memory and only the Linux one has been run); none found means "not measured", never a download. **The page runs its own scripts in your browser, so these are blocked and counted:** any request that leaves this machine (`--allow-external` lifts it), any request that is not GET or HEAD, any file read outside the page's own folder, downloads, and a hung page is stopped at 30 s (2 minutes for the whole run). **The automatic run** inside `malveon` and the MCP `malveon_check` is small on purpose: changed static pages only, at most 5 widths, 20 seconds, never your running app; the stop gate never starts it. It informs and never blocks (exit 0, no hold). MCP tool `malveon_layout` does the same for an agent; its calls are a convenience, never evidence the work is correct. The old reading-only check is now labelled "Layout risk signals (reading only)" and still proves nothing. **Limits:** results are repeatable on one machine and one browser build, not across machines or browser updates; intended overlaps (a badge on a photo) will sometimes be listed; interaction states (menus, modals), container queries and JavaScript-driven layout are not read; class-based dark mode is not applied; no rule has measured precision yet, so every sentence says "looks", never "is"; a throwaway cloud environment usually has no browser. Adds the Go package `chromedp` (pinned v0.9.5, MIT) to the program.
- **Plan items (wiring)** — every line of your plan gets one direct answer. malveon looks up what the line names — a URL (`/api/jobs`, `POST /api/jobs`, `/admin/digest`), a file (`PostJobModal.tsx`), or a code name in backticks (`InvitesTab`, `generate_referral_code`) — and checks it against the real code, including calls straight to Supabase (`.from()`, `.rpc()`, `.auth.*`), Prisma (`prisma.post.create()`, `$queryRaw`) and Drizzle (`db.insert(posts)`):
  - `CONNECTED` — built, and the screen actually reaches its server route (or database)
  - `BROKEN` — built, but it won't work as written: the screen calls a URL nothing serves, or with a method the server doesn't accept
  - `NOT BUILT` — the plan names it, the code doesn't have it
  - `HALF BUILT` — one part exists, the other doesn't (a server route nothing in your app calls, even if a test does; a component that never saves anything the line says it should; a line about a button or form whose request is made by a function nothing in your app ever calls, so clicking it does nothing)
  - `SKIPPED` — not checked, and never a problem in your code. Either it isn't a code task (a heading, a design note, a manual step like "open the page and check", or a build/test step — that's what `--run` is for), or it names nothing malveon can look up ("make search better", a field list, a sub-bullet). The report counts the first kind and names a few of the second, with the closest routes and pages in your code, so you can add the URL, file, or component the line is about and have it checked. A skipped line never blocks a commit; only a plan where *nothing* could be checked does

  A `CONNECTED` line with an extracted HTTP flow also shows its **end-to-end path**, traced through the code (never by running it): the page it's on → the call → the server route → what that route changes. For example: `page /jobs → POST /api/jobs (PostJobModal.tsx:66) → route POST /api/jobs (route.ts:42) → inserts into table jobs (route.ts:59)`. If the line expects a change (a PUT/PATCH/DELETE call, or a POST with a word like "save" or "submit") and the route provably changes nothing, the line is `HALF BUILT`. A route that uses something malveon doesn't read (another database, an email or payment service, code it couldn't find) is never counted as "changes nothing". Source-existence/import facts have no end-to-end path and establish no behavior beyond the exact structural claim. Data changes are recognised for Supabase, Prisma and Drizzle only for now. A route that uses another database or service malveon knows by name (Mongoose, TypeORM, Knex, Sequelize, Firebase, Redis, Stripe, an email service, …) is read as "may change data", never "changes nothing"; a package malveon doesn't know by name and can't find in your repo (a database package published elsewhere) isn't followed, and a route that writes through one can read as changing nothing.

  **Does the button do anything?** When a line talks about a button, a click or a form, the path also names the button or form that runs the request, for example `onClick on <button> "Refund" (RefundButton.tsx:9) → POST /api/refund (RefundButton.tsx:2) → …`. malveon calls a button dead in two cases it can prove. (1) Nothing in your project uses the function that makes the request (no call, no handler, no import that uses it, not even under another name). (2) The button your line names (its visible text is words of the line) has no handler, or a handler that runs nothing like `onClick={() => {}}`, even if something else in the app can send the request. It does not call a button dead when the click could be handled out of sight: a wrapper or the form around it handles it, props are spread onto an element in the file, a framework attribute (`hx-post`, `@click`) is used, a listener is added in code, or another button with the same text works. Anything it can't prove is left as `CONNECTED`: a handler passed down as a prop, a function used by another function that is itself never used. It reads React/Next (JSX) buttons, forms and links only; Vue, Svelte and plain HTML handlers aren't read, but any mention of a function in them counts as "used". A button it can't see (after an apostrophe in text on the same line, or a handler expression over 16 KB) is simply missed.
- **Contract** — does the call agree with the route on HTTP method, and (JS/TS, literal request bodies only) does it actually send every field the handler reads off `req.body`? Shape-level only, never a claim the logic is correct.
- **Overlap** — do two or more route registrations silently claim the same method+path? Route-precedence aware: a static/dynamic split across two Next.js route files, or a static route registered before a dynamic one in the same file, is never actually ambiguous and isn't flagged — everything else, including the frameworks where registration order genuinely does create a real dead-route bug (Express, Flask, FastAPI, gorilla/mux), stays flagged. Also flags when one of the colliding registrations sits inside a function that's never referenced anywhere else in the codebase — a real signal it might be dead code, not just a guess.
- **Frontend overlap risk** — is an `absolute` element (Tailwind class or plain CSS `position:` declaration) sitting with no positioning context anywhere in its file (a structural risk signal, not a claim two things visually collide — confirming that needs a real render, which this deliberately doesn't do)? `fixed` is never flagged — it always positions against the viewport — and counts as valid context for a nested `absolute` element, matching real CSS semantics.
- **Not-in-plan** — did the agent build something this session that neither your plan nor your own prompts asked for? It lists what is new against the code as it was when the session started, in a file the agent changed, and that no plan line or prompt of yours names: a request your page makes or a server address the code answers, a **page**, a database **table**, a **button or form that changes data** (inserts, updates, deletes or uploads; a button that only changes what the page shows isn't listed), and a runtime **package** (`dependencies` in `package.json`, a `requirements.txt` line, a `go.mod` `require` that isn't indirect; dev tools aren't). Something counts as asked for when one plan line or prompt of yours holds every word of its name (singular and plural alike; a package also by its scope or the part after it, so "react query" covers `@tanstack/react-query`); your agent's own summary never counts as asking. Something that was already there isn't listed, even if the agent edited its file, and a button moved to another file isn't new. It only judges what it can read: a button whose handler is defined in another file, a database call that only reads, `pyproject.toml` dependencies and purely visual changes aren't. It never blocks, but a run with something on the list reads `NEEDS YOU` rather than `VERIFIED BY CODE`.
- **New bugs this session** — did the agent's own work this session break something that worked, or add a known bug pattern? Compared with the code at the session start, read from git and the code (never from the agent): a call from your screen that reached a route then and goes nowhere now, an import that worked then and cannot now (the file it names is gone, or no longer exports the name; JavaScript and TypeScript, never for a package, an asset or a file whose exports can't be listed), a new call to a URL nothing serves (a test fetching a URL to expect a 404 isn't one), or a known bug pattern added to a file (an empty `catch`, a leftover `debugger`/`.only`, a hard-coded secret, a swallowed Go error, a bare Python `except`). Established structural regressions and broken imports can block. Text-pattern matches are advisory review candidates; their presence does not prove a runtime bug or a live credential.
- **Unfinished code** — did the session leave work undone, on lines it added in JavaScript, TypeScript, Python or Go files? A `TODO`/`FIXME`/`HACK`/`XXX` in a comment; a function whose whole body throws "not implemented"; sample data left in ("Test User", "John Doe", lorem ipsum; not in tests or fixtures); a debug log; 3 or more lines of commented-out code; a `@ts-ignore` or a bare `eslint-disable`/`# noqa`/`# type: ignore` (a comment that names the rule it turns off is a decision and isn't listed); a skipped test; a new exported function nothing in the project uses. A function that throws "not implemented" **and that something calls** blocks, because the path ends in nothing; everything else is only listed for you to look at (the verdict reads `NEEDS YOU`, and the agent's "done" shows the list). Its absence proves nothing, so this can never substitute for the confidence check below.
- **Weakened tests** — did the agent make your tests easier to pass? Each test file it changed is compared with the same file at the session start (JavaScript/TypeScript, Python and Go test files): tests deleted or commented out (`TESTS REMOVED`), two or more assertions taken out of tests that are still there (`ASSERTIONS REMOVED`), and a new assertion about a constant such as `expect(true).toBe(true)`, which passes whatever the code does (`ASSERTS NOTHING`). Moving tests to another file isn't removing them: a file is named only when the project as a whole lost tests. A test deleted together with a source file of the same name (`Card.tsx` and `Card.test.tsx`) is read as a cleanup and isn't listed; a test whose code was deleted under another name still is. It only lists, for you to look at, because a removed test is sometimes a good cleanup: it never blocks, but a run with something on the list reads `NEEDS YOU`. It counts test and assertion calls in code (not in comments or strings), so a test written in a way it doesn't recognise isn't counted either way.
- **Files nobody asked about** — which changed code files does nothing you wrote point at? A file is pointed at when a prompt or plan line of yours names it, a function or component in it, or a URL it serves or calls; when it is one import away from such a file, or is its test; when its own name is words you wrote (`EventsSection.tsx` for "the events section"); or when it sits in a folder you named ("the landing page" points at `components/landing/`). Docs, manifests and configs are never judged. If any prompt of yours names no file, URL or code name, it judges nothing and says so, because that prompt could be what any file was for. It only lists, never blocks: a run with something on the list reads `NEEDS YOU`. Measured only on commit messages standing in for prompts, not yet on real prompts.
- **Confidence** — does the agent's own "it works" claim actually match what got verified? Reads it straight from commit messages, nothing to ask or paste.
- **Agent's words** — the agent also writes what it built in notes files it adds or edits (`NOTES.md`, `progress.md`, `CHANGELOG.md`), in commit messages and in its final message ("I added `POST /api/refund` and it works"). Each sentence that says something was built or works, and names a URL, a source file or a code name, is looked up in your code like a plan line. One the code contradicts (no such route, or a function nothing uses) is a falsehood the agent wrote, and it blocks. Not checked, because the lookup can't tell whether they are true: sentences that name nothing ("everything works"), hedge ("not wired yet"), plan ("should add"), report a removal or rename, are instructions or examples, are about documents, config files or paths outside the project, or are not in English. A plain name that appears nowhere (`lodash`, a tool name) is only called missing if the agent said it created it and it is nowhere in the project's text. The final message is read where the agent's hook gives it (Claude Code's Stop hook does). Your own plan files are checked as plans, not as the agent's claims.
- **The verdict** — the report opens with one answer that comes only from what the code and the checks said, never from what the agent said about its own work: `FAILED` (something is proven wrong in the code), `NEEDS YOU` (nothing is proven wrong, but something is yours to settle: words of your prompt that no check covered, such as "faster" or "nicer", a task nothing was checked for, work the agent left unfinished, tests it deleted or thinned out, a page, table, data-changing button, request or package it added that nothing you wrote asked for, a check that couldn't run), or `VERIFIED BY CODE` (nothing is proven wrong, and everything malveon could check is built and connected). "Verified" never means the app was run; the report says so under every verdict. Under any verdict it also lists what was **not established**: each plan line that names nothing malveon can look up (named, marked `unchecked; behavior unknown`), requests whose URL is built at runtime and so can't be read (counted in the overview), and connections where the fields your screen sends were not compared with what the server reads. When the stop hook is installed, the person also sees malveon's own short account of the session next to the agent's "done": what changed, what is proven wrong, what is built and connected, what is not established, and that the app was never run. The agent is not shown it.
- **Command check** *(opt-in, `--run`)* — do your project's own build/lint/typecheck/test commands actually pass right now? The one check that runs real subprocesses instead of reading the code graph — a Makefile target, a `package.json` script, Go's own toolchain, or whatever Python tooling is on your `PATH`, whichever the project already defines. Nothing invented, nothing guessed, no server or browser ever started. Off by default since it can take a few minutes. When a command fails and the session has a start, malveon also runs the same commands on the project as it was at the session start (in a temporary copy; your folder is never written to) and says whether it **passed then** (this session broke it) or **also failed on that copy** (so it may not be new; the copy holds only the tracked files, so a missing generated file or secret can cause that). It checks whole commands, not single tests.

Every answer states the fact it's based on, with file:line — never a guess dressed up as an answer. malveon never exits non-zero because of what it finds (it exits 0 whenever it checked); the verdict is the headline of the short view and the `findings_verdict` field of the JSON.

## What it can't do (yet)

- Doesn't prove business logic is *correct* — only that the wiring and HTTP method agree.
- No server and no deployed app, ever; the only browser is the optional layout check's (it loads pages, it does not click through them) — `CONNECTED` means built and connected in the code, not clicked through in a running app. A URL built from a variable (`fetch(url)`) can't be matched to a route, so it never counts as evidence either way. The command check runs your project's own already-defined build/lint/typecheck/test commands (nothing invented), but that's still not a live/deployed run.
- A single fixed timeout applies to every command in the main command check rather than one tuned per category.
- The new-bugs check only knows a small, named catalog of bug patterns plus one structural comparison (calls that stopped reaching a route) — not a general "was this a real bug" judgment, which can't be settled from static code. It needs a session start and a git repository to compare against.
- Incompleteness is one-directional — a marker's presence is real proof, but its absence proves nothing (most finished code has none either). Never a substitute for the confidence check.
- Overlap's reachability note is a best-effort heuristic (checks whether an enclosing function's name is ever mentioned elsewhere in the codebase), not real call-graph analysis — an anonymous handler or dead code it can't attribute to a named function still just counts as a plain registration.
- Contract's field-agreement check is JS/TS only, and only fires when both sides are a literal object (no spread, no variable) — Python/Go request bodies and response-shape comparison aren't covered yet.
- Frontend overlap risk (both Tailwind and plain CSS) is file-scoped rather than tracing the real JSX/selector ancestor chain — no CSS specificity/cascade resolution.
- Prompt checking only sees prompts typed after `malveon on`, and only covers Claude Code, Codex, Cursor, and Antigravity. Antigravity doesn't hand its hook the prompt text, so malveon reads it from Antigravity's conversation log, whose format isn't documented and could change; if it can't be read, the report says so. A line the agent writes can be real and pass while still asking for less than you did; the "not covered" note points at that, but you make the call.
- Frontend calls are read from JavaScript and TypeScript, and from the script of Vue, Svelte and Astro files (including `fetch`, `axios`, `useSWR`, `useQuery`, Nuxt's `$fetch`/`useFetch`, form actions, and htmx `hx-post` attributes in server-rendered templates); backend routes from Python, Go, JavaScript/TypeScript, Rust, Java, Ruby, PHP, and C#.
- Frameworks whose routes come from files or decorators, read only in a project whose `package.json` (or code) shows it uses them: **Next.js** (App Router and Pages Router), **Nuxt** (`server/api`, `server/routes`, `pages/`), **SvelteKit** (`+server.ts`, `+page.svelte`, form actions and the forms that post to them), **Remix / React Router** (`app/routes.ts`, flat file routes, `loader`/`action`, `<Form>`, `fetcher`), **Astro** (`src/pages` pages and endpoints, Astro Actions), **NestJS** (`@Controller`/`@Get`, with `setGlobalPrefix`, URI versioning and `RouterModule`), **Django** (`urls.py` with `include()`, Django REST Framework routers, ViewSets and `@action`, and `{% url %}` links in templates), and **tRPC** (each procedure is checked as `post.create`, with its calls from `api.post.create.useMutation()`, the vanilla client or a server-side caller). When a prefix or a version can't be read from the code (a global prefix set from a variable, `RouterModule`, a tRPC base path), the route's URL is marked unverified instead of guessed.
- Databases: Supabase, **Prisma** (calls are read only on a model your `.prisma` files declare) and **Drizzle** (only on a table you declare with `pgTable`/`mysqlTable`/`sqliteTable`), including raw SQL when its statement is a plain string. A model chosen at runtime or SQL built elsewhere is read as "may change data", never as a read. TypeORM, Mongoose, Sequelize, Knex, Firebase and other database libraries aren't read.
- Not read yet: Angular's `HttpClient` calls, the pages of Vue and Svelte apps that don't use Nuxt or SvelteKit, tRPC procedures defined in a package outside your repo, GraphQL and gRPC, Django's `reverse()` calls in Python code, Kotlin, Swift, Dart, Elixir and Scala. For Nuxt, SvelteKit, Remix, Astro, NestJS, tRPC and Django routes the method is checked but request fields aren't compared (the contract check says "fields not compared"). A call in a test file (`*.test.*`, `*.spec.*`, `*.cy.*`, `__tests__/`, `cypress/`, `e2e/`, `*_test.go`, `test_*.py`) doesn't count as your app calling a route: a route only your tests call is half built, and the report says "only tests call it". Tests are recognised by those names only; a folder called just `test` or `tests` could be a real page, so a test file inside one that doesn't match a pattern above still counts as a call.
- Routes are followed through the mounts malveon can see: Express `app.use('/api', router)` (also nested, and through `import`/`require`), FastAPI `APIRouter(prefix=…)`/`include_router`, and Flask blueprints. A router malveon never sees mounted is matched by the end of the URL and the report says its prefix is unverified. A mount built from a variable or a function call isn't followed.
- A handler that only returns 500/501 or raises "not implemented" is reported as half built; this is read from short handlers only. Whether a longer handler works is beyond what reading the code can show.


## Open source: what you can read, and what you can't

The part of malveon that decides most plan-line verdicts is open source under the Apache License 2.0, so you can check the claims on this page against the code.

| Part | Where | Status |
|---|---|---|
| Code map: reads your routes, requests, pages, imports and database calls (`extractor`, `graph`, `skipdirs`) | [`pkg/`](../pkg) | Open, Apache-2.0 |
| Plan reading (`features`) | [`pkg/features`](../pkg/features) | Open, Apache-2.0 |
| The plan-items check: CONNECTED, BROKEN, NOT BUILT, HALF BUILT, SKIPPED (`wiring`) | [`pkg/wiring`](../pkg/wiring) | Open, Apache-2.0 |
| Everything else in the `malveon` program: new bugs, agent's words, weakened tests, unplanned code, prompts, original-plan snapshots and platform interpretation, the gate and hooks, the report, MCP server, engine coordination, updates, telemetry | not published | Closed. Free to use during the beta under [BETA-LICENSE.txt](../BETA-LICENSE.txt) |

What this means: the downloadable `malveon` program is built from the open packages plus the closed ones. It is not built from this repository alone, and the release binaries are built and signed outside it. So you can audit how a plan line becomes a verdict, but you cannot rebuild the whole program from this source. The closed checks keep their limits documented above.

To run the open tests:

```bash
go test ./...
```

`go test -short ./...` skips one timing test that is slow on loaded machines.

## License

**Two licenses, one repo.**

- **Source code** in `pkg/` and `testdata/`: Apache License 2.0. Use it, change it, redistribute it. See [LICENSE](../LICENSE).
- **The `malveon` program** you download: free to use during the beta, including for commercial work. It contains the open code above plus closed code. See [BETA-LICENSE.txt](../BETA-LICENSE.txt).

Later versions may come with different terms. The version you already have keeps the terms it shipped with.

## Questions or feedback

Using this on a stack it doesn't handle well, found a bug, or just want to say something's off? [Open a discussion](https://github.com/LadsonDavid/Malveon/discussions) — that's the one place I'm actually watching.

Want a custom check rule built for your specific stack?
```bash
malveon register
```
Prints (and tries to open) a pre-filled link to a new GitHub Discussion in the Q&A category — describe your framework and what you need, and you'll get a reply there, in public. No email, no sign-up: the discussion only exists if you write it and submit it yourself. (Like every command, it sends one anonymous usage event naming the command; see [Usage tracking](#usage-tracking).)

## Automatic workflow (local source, unreleased)

> This section describes the hooks as built before the six-command surface. `hooks install`, `hooks status`, `gate` and `--gate` are now `malveon on`, `malveon status` and `malveon off`; hooks never hold anything. The behavior described below is otherwise current.

Preview before setup:

```powershell
malveon on --root <project> --agent claude --agent codex --gate shadow --status-line --dry-run
```

Omit `--dry-run` in a real terminal to review and approve the same settings.
There is no automatic approval switch. Explicit agents work before their folders
exist. Existing settings are merged; settings changed during review are refused,
and accepted bytes are replaced atomically under a lock. Codex separately requires
host trust of exact hook definitions. Plain installation without `--gate` captures
prompts only. MCP `malveon_setup_hooks` accepts optional `gate: shadow|enforce` and
continues to require the human's actual answer. Optional `--status-line` (MCP `status_line: true`) reviews a project-specific Claude terminal display in the same settings merge, showing any project status line it would replace. Global settings are retained.

Inspect without copying prompt text:

```powershell
malveon hooks status --root <project> --json
malveon hooks status --root <project> --agent claude --compact
```

`malveon hook status --root <project> --agent claude --compact --data-dir <absolute-local-directory> --local-only`
is suitable for a Claude Code terminal status-line command. It reads local
observations; it does not run checks or consume model tokens. Do not overwrite an
existing status-line configuration without reviewing it. No custom Codex desktop
panel is claimed. Reuse the existing editor page for details.

MCP `malveon_status` exposes the same local observations. These inspect project
hook files, not every possible global/managed host setting. They distinguish
configuration, last captured prompt and the latest automatic attempt. Completed
checks are historical; later edits need another check even if no new prompt arrived.
A new captured prompt invalidates an older displayed result. An interrupted attempt
has no observed completion; it is not a pass. Local `automation/<agent>.json`
receipts and unique `automation/reports/*.json` live under the resolved project data
directory, not the checkout. Reports retain actual source and executable provenance.
Receipts are observations, not substitutes for original intent or approval history.

Capture retains the exact prompt, pre-edit HEAD and dirty paths. These identify
pre-existing work without resetting the accepted session; they are not an atomic
snapshot or full behavioral attribution. Missing old conversations cannot be
recovered by inventing prompts. The selected original plan stays pinned. Zero
static extraction now explains an analysis gap instead of claiming the user wrote
no requirements. Legitimate prose must not be weakened to obtain source matches.

Set `MALVEON_LOCAL_ONLY=1` before startup for private dogfooding. It disables
Malveon telemetry and automatic update checks, withholds raw MCP scan/check
results from the coding model and stores them locally. Exported results contain
only verdict/count summaries and the local report location. Interpretation tools,
withdrawal and interactive setup are unavailable through that connection; use a
human terminal when needed. Hook feedback also withholds raw findings. Ordinary
release telemetry still ignores `DO_NOT_TRACK`; this explicit mode is the local
privacy control. Approved scripts can have their own network/side effects, and
Codex/Claude conversations are not made local by this flag. No general zero-network
or semantic-completeness guarantee follows.

Automatic completion hooks still use the bounded static check and existing retry
limits. They never silently approve or run project commands. Use the full check
with valid human command approval for build/test evidence. Native prompt and stop
events must be observed separately in each installed host, including after restart;
simulated payloads only test adapters.

MCP status also returns actual serving-engine identity and the preserved baseline. Codex completion hooks emit the documented `systemMessage` warning, including observation-mode results; native rendering still requires an observed host run. Local-only scans retain actual numeric scan counts while withholding source details.

### Windows Codex hook transport correction

A quoted executable followed by arguments is not directly invocable PowerShell
syntax. Windows Codex definitions now carry `commandWindows`, launching a
no-profile PowerShell script via `-EncodedCommand`. The script uses UTF-8 for
payload forwarding and native output; input is passed to Malveon as data and never
inserted into executable script. Installer previews display both the exact
encoded command and the decoded script. Existing definitions are updated rather
than duplicated and must be reviewed again in the host. The agent does not change
trusted hashes. Actual PowerShell and Git Bash regression controls cover original
Tamil/emoji input and paths with spaces/apostrophes. Native shell success does not
establish all desktop event delivery or model authentication.

`commands_approved` in local status is a read through the existing fingerprint
binding, not an approval action. Setup reuses a valid saved approval; changed
configuration still invalidates it. No API key/login credentials are changed by
this correction. A screenshot's separate model API 401 must be diagnosed as
host authentication, independently of hook execution.

### Automatic completion failure and Git batching (local, unreleased)

Automatic attempts may include `failure_code: CHECK_TIMEOUT` or `CHECK_FAILED`; raw errors remain in local receipts. Local-only hook output uses fixed messages for these codes rather than exporting detailed errors. The Stop budget remains 20 seconds; no result is fabricated after timeout. Pattern, incomplete-work, agent-note and weakened-test comparisons use request-local Git batches. Baseline blob contents use length-framed `cat-file --batch` reads; added-line evidence uses the explicit baseline tree and diff, including untracked-file absence. No cross-run evidence cache is introduced. This is not general semantic proof or a guarantee of bounded total runtime.
