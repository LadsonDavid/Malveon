# Malveon check

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
  - the agent added things nobody asked for: 1 route, 1 call, 1 table, 4 packages (start at src/app/api/receipts/route.ts:4)
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

Want it to hold the AI back while something is proven wrong? There is an optional setting for Claude Code, Codex, Cursor and Antigravity. See [Holding your agent](#holding-your-agent-optional-claude-code-codex-cursor-antigravity).

## What it doesn't do

It reads code. It never opens a browser, starts your app or checks a deployed site, so `CONNECTED` establishes only the specific extracted source connection, not every promised behavior or a real click. It doesn't prove your business logic is right. Two things go beyond reading: `--exec` (opt-in) runs your project's own build, lint, type-check and test commands and reports what they really said, and malveon sends small anonymous usage signals (see [Usage tracking](#usage-tracking)). Full list: [What it can't do (yet)](#what-it-cant-do-yet).

## Start in two minutes

You don't need a plan, a session or any setup for the first look.

1. In your project's folder, run `npx malveon scan` (no install; [or install it](#install) and run `malveon scan`). It lists buttons that do nothing, calls to addresses nothing serves and imports that can't work, in seconds, and changes nothing.
2. Just had an AI work on the project? Run `malveon review`. It checks the commits it made, using their messages as what you asked for, and says what it added that nothing asked for, what it broke and what it left unfinished.
3. To have every task checked from now on, run `malveon hooks install` once. Or give malveon a plan: `malveon check` finds your plan file and asks you to confirm it ([what a plan looks like](#the-plan-file); a plain checklist works).

Using VS Code or Cursor? Search "malveon" in Extensions. The command "Malveon: Scan this project" does step 1, and the same report as `malveon check` shows up beside your code.

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

On Mac or Linux: download the file for your OS from the [Releases](../../releases) page, run `chmod +x` on it (this just tells your computer the file is allowed to run), then move it into a folder already on your PATH, like `/usr/local/bin`.

On Windows: downloading the `.exe` by itself is not enough to make `malveon` work as a bare command. The file also needs to be renamed to `malveon.exe` and placed somewhere on your PATH, and there's no single folder every Windows setup already has ready for that the way `/usr/local/bin` works on Mac or Linux. Run the install script above instead, it does exactly this for you and has been tested to actually work. If you'd rather not run any script at all, download the `.exe` and just run it directly from wherever you saved it, for example `.\malveon-windows-amd64.exe check`, no install needed, just typed from that exact folder every time.

## How to use it

**No plan yet? Start here.** Two commands need nothing set up:

```bash
malveon scan      # what's wrong in the project right now
malveon review    # what your AI just did, from the commits it made
```

`malveon scan` reads the whole project and lists the faults that need no plan or session to see: a button with no click handler (or an empty one), a call to an address none of your routes serves, an import of a file or name that doesn't exist, a route that only answers "not implemented". It shows the first three (`--all` for every one) and one next step. It writes nothing into your project and always exits 0: it's a look, not a gate. Each kind is judged only where malveon can be sure (see [Every check, in detail](#every-check-in-detail)), so a short list is not a clean bill of health: the report says what it did not judge.

`malveon review` takes a git ref (default `HEAD~1`, the commit before the last one) and checks everything committed since it, plus uncommitted changes, the way a session would have: what was added that nothing asked for, what broke, tests removed, work left unfinished, and what the commit messages claim that the code doesn't back up. There was no plan or prompt, so your commit messages stand in for what you asked for: a weak stand-in, and the report says so. `malveon review HEAD~5` looks further back, `--full` prints the whole report. It borrows the session file while it runs and puts yours back exactly as it was.

**With a plan or prompts, the full loop.** Two commands. Run the first one before the agent starts and the second one after it says it's done.

**1. Save a snapshot of your code, right before the agent starts:**
```bash
malveon session start
```
Run this from inside your project's own folder. It just remembers what your code looked like before the agent touched anything, so later checks have something real to compare against. If you've set up [prompt checking](#check-your-prompts-too-not-just-plan-docs), skip this step: your first prompt does it for you. Forgot? `malveon check` starts one for you and says so; it then covers changes from that moment on (including ones not committed yet), but not work the agent already committed before it.

If your folder already has uncommitted changes when you run `session start`, the snapshot includes them: what was already there is your starting point and is not blamed on the agent, so only what changes after that counts. (If more than 2,000 files are uncommitted — usually a build folder nobody ignored — it falls back to your last commit instead.)

**2. After the agent says it's done, check what actually got built:**
```bash
malveon check
```
This is the real command. It reads your plan, reads your actual code and git history, and tells you what's genuinely built versus what the agent only claims. You don't have to point it at your plan file by hand, it looks for one and asks you to confirm before using it.

It always stops with an error if it finds something actually broken. Want it to also actually run your project's own build, lint, type check, and test commands for real (not required, but the most thorough option)? Add `--exec`:
```bash
malveon check --exec
```
This is opt-in on purpose, since it runs real commands and can take a few minutes. See [Blocking a commit](#blocking-a-commit) below for wiring any of this into an actual commit hook.

### Changing the defaults

Everything above works with no extra input. If you want more control, you can add extra options (called flags) after the command:

```bash
malveon check --features features.json --exec --exec-timeout 5m
```

- `--features <path>`: skip the plan file question, use this exact file.
- `--exec`: also run your project's own build/lint/typecheck/test commands for real (off by default).
- `--exec-timeout <duration>`: change how long each command gets before malveon gives up on it, only relevant with `--exec` (default 3 minutes). Example: `5m`, `90s`.
- `--no-gate`: still show the full report, but always finish successfully, even if something's wrong.

Skip any flag you don't need. Nothing breaks, that part of the report just explains it was skipped and why.

## Blocking a commit

By default, `malveon check` fails (technically: exits with a non-zero status) if it finds something broken, something it can't prove either way, or a check that couldn't run at all. Every report ends with a plain `GATE: clean` or `GATE: blocked` line, naming exactly why.

A few things never block on their own: code flagged as "not in your plan," unfinished-code markers, frontend layout risks, and plan lines malveon couldn't look up (they say nothing about your code). Those are meant for a human to glance at, not proof of a real bug.

Plan work you haven't ticked yet doesn't block either. A `- [ ]` line that's NOT BUILT or HALF BUILT is just not done yet: it's still checked and listed, marked "not ticked in your plan yet, so it doesn't block". A `- [x]` line (the plan says it's done) or a plain `- item` line blocks when the code disagrees, and BROKEN always blocks, ticked or not. Likewise with `--exec`, a command your project doesn't define (no `test` script, say) is reported but doesn't block, as long as something else ran; and a prompt you typed only blocks once your AI agent has been asked to say what it meant and hasn't.

To actually stop a bad commit, wire malveon into a pre-commit hook. Malveon never sets this up for you, here's how to do it yourself:

```bash
#!/bin/sh
# save as .git/hooks/pre-commit, then run: chmod +x .git/hooks/pre-commit
malveon check
```

This needs a committed [`malveon.json`](#keep-the-checks-with-your-repo-malveonjson) naming your plan file. A hook can't answer questions, so without one the check stops right away instead of hanging while it waits for an answer (or pass `--features path/to/plan.md` in the hook instead). With `"exec": true` in `malveon.json`, the hook also runs your project's own build/lint/typecheck/test commands and blocks on a failure. Run `malveon trust` once in the repo first (see below): a hook can't ask you, so until you've approved the commands it skips them and blocks, saying why.

## Usage tracking

malveon sends small anonymous signals so the person building it can see whether anyone actually uses it (a download doesn't say that). Each one carries your OS, CPU type and malveon's version, plus:

| Signal | When | Also includes |
|---|---|---|
| first run | the first time malveon runs on a machine | nothing else |
| check | each `malveon check`, from the terminal, your editor or your AI agent | clean or blocked, whether your build/tests ran, how many of your prompts were captured, and the name your agent or editor gives itself |
| scan / review | each `malveon scan` or `malveon review`, from the terminal, your editor or your AI agent | how many problems it found (a number, never what they were), how many commits a review looked at, how long it took, and whether `malveon check` fell back to a scan because you had no plan |
| command | `update`, `session`, `register`, `hooks`, `prompt`, `trust` | the command's name |
| agent connected / tool used | an AI agent or the editor connects to `malveon mcp` | the agent's or editor's own name, the tool's name |
| update | an update is offered, installed or fails | the two version numbers, and a one-word reason if it failed |
| error | a check, scan or review can't run | a one-word kind, never the message |

It never sends your code, file names, plan, prompts, commit messages, error messages, or anything else about your project, and never looks up your location (turned off on purpose). Typing a prompt to your agent never sends anything. Downloads made through `get.malveon.com` (the install scripts, `malveon update`, npm) are counted there too: which file and where it was downloaded from, nothing about you.

The only thing it remembers between runs is a random ID stored on your own machine, so two runs from you count as one person, not two. That ID isn't tied to your name, your email, or anything that could identify you.

## Use it from your AI agent (MCP)

`malveon mcp` runs malveon as an MCP server, so Claude Code, Cursor, Antigravity, or any other MCP client can call the `malveon_check` tool itself and read the same report you'd see in the terminal, GATE line included. No copying output into the chat. The `malveon_scan` tool does the plan-free scan, so an agent can look at a project nobody has set up yet.

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

Most of what you ask an agent for never makes it into a plan doc. malveon can check your prompts as well. You don't have to find a command for it: the first `malveon check` in a project that has an agent's folder (`.claude`, `.cursor`, ...) asks once, in the terminal or as a dialog in your editor, shows exactly what it would add, and sets it up on a yes. A no is remembered and never asked again. To set it up yourself later:

```bash
malveon hooks install
```

Run it once, in a terminal, inside your project. It finds which agents this project uses (Claude Code, Codex, Cursor, Antigravity), shows you exactly what it will add to each one's settings, and only writes after you say yes. From then on:

- Every prompt you type is saved word for word to `prompts.jsonl` in this project's Malveon user-data directory (see `malveon paths`). It comes from your editor's own hook, never from the agent. The state is outside your checkout, and nothing about your prompts is ever sent anywhere.
- Your first prompt starts a session by itself, so you don't need `malveon session start` any more. A check that comes back clean ends the session, and your next prompt starts a new one.
- Through the MCP tool, the agent says what each prompt asked for, one checkable line at a time: "`GET /api/search` is served and called from `SearchBar.tsx`". malveon checks every line against your code, exactly like a plan line. What the agent says is only a list of things to check, never proof. Each line must name a URL, file, or code name and be about what you actually asked, and once recorded it can't be edited or removed.
- The report gets a new section, "Your prompts vs the code," with one answer per prompt: CONNECTED, BROKEN, NOT BUILT, HALF BUILT, NOT TRANSLATED (no checkable interpretation was supplied, or its source lookups remain unresolved), NOT A CODE TASK (the agent's word, listed for you to review), or SKIPPED (a short reply like "yes", or a follow-up like "make it faster" that carries on the work before it). A short prompt that asks for something ("fix the login", "add dark mode") is a task like any other and is checked; only replies and follow-ups are skipped. Any prompt with an answer from the code also lists your words that none of the agent's lines mention, so you can spot a request that got quietly dropped; those words are also named in the verdict as yours to settle.
- **A vague request** ("make the checkout faster": it names no file, URL or code name) can't be settled by reading code, so malveon doesn't pretend to. Instead, in Claude Code, the prompt hook asks the agent to say, before it starts, how you will know it is done (`done_when`): a number ("loads in under 1 second") or a file, URL or test it will point at. "It works better" is refused, because it can't be wrong. malveon keeps the line word for word, only ever adds to it, and when the agent says it is done, shows you: what you asked, what the agent said would show it done, and what malveon could do with that. A number is a measurement malveon never makes; something named is looked up in the code like any claim, and the fact is shown. Neither blocks, and neither settles the prompt: only you can. If the agent never gave a done line, the report says so. Other agents don't get the ask from the hook (their hook output is unverified); their agent is told through the MCP tool instead.
- If you cancel something ("actually, drop the search box"), the agent can only propose withdrawing the earlier prompt. You confirm it, either when malveon asks you in the agent's chat or with `malveon prompt drop <number>` in a terminal.
- If your prompt names a URL, file or code name but the agent's lines are about something you didn't type, malveon asks you, in the agent's chat, whether those lines are what you meant. A prompt that names nothing ("make the header sticky") isn't asked about: the agent's lines are then only its guess at what you meant, so malveon lists the prompt as needing you at the end (`NEEDS YOU` in the report) until you look yourself or run `malveon prompt confirm <number>`. Nothing the agent sends can answer for you. If your agent can't show the question, or you dismiss it, approve in a terminal with `malveon prompt confirm <number>`; it shows you the prompt and every line first. Words of your prompt that no line mentions and no code can show ("faster", "nicer") can be marked as yours to check with `malveon prompt waive <number> <words>`. The approval is recorded, shown to the agent and in the report; it only stops the agent from editing code if you turn on the edit gate (below).

Codex asks you to approve the new hook once, with `/hooks` inside Codex.

### Holding your agent (optional: Claude Code, Codex, Cursor, Antigravity)

```bash
malveon hooks install --gate shadow
```

This adds two more hooks for each agent your project uses, with the same show-then-ask-first rules: one runs before the agent edits a code file, the other when it says it is done. They go in `.claude/settings.local.json` (Claude Code), `.codex/hooks.json` (Codex), `.cursor/hooks.json` (Cursor) and `.agents/hooks.json` (Antigravity). **Only Claude Code's has been run against the real agent.** The other three were written from each agent's documentation and tested against sample payloads in the shape that documentation gives, never against the real agent (Codex's command-line install is broken on the build machine; Cursor and Antigravity weren't available). Try them in `shadow` mode first. Antigravity's documentation doesn't say what happens when a hook fails and one source says a failed hook blocks the tool, so the command warns you and tells you how to remove the entry. Codex asks you to approve the new hook once with `/hooks`.

- `--gate shadow` holds nothing. It only writes down what malveon would have held, in this project's user-data `shadow.jsonl`, so you can see what it would do before you let it.
- `--gate enforce` holds a code edit only when your prompt names code (a URL, a file or a code name) and what the agent says it will check doesn't match it: no claims yet, claims that only say things exist, or claims about something you didn't write (then you are asked, or you run `malveon prompt confirm N`). It tells Claude what to do first. A prompt that names nothing ("make the header sticky") never holds an edit. When Claude says it is done, it runs malveon's checks and sends Claude back to fix any problem malveon can prove. It asks twice at most for the same problems, then lets go and tells you.
- **Switching it off (you, not the agent).** In a terminal: `malveon gate off --for 2h` lets the agent through without checking for two hours (`malveon gate shadow --for 2h` only logs, `malveon gate enforce` turns it back on, `malveon gate reset` forgets the setting, `malveon gate status` says what is in force and why). Changing it needs a real terminal and a typed `y`, so an agent that only has a shell tool can't do it; the setting is kept in your home folder (`settings/gate-mode.json` under the data root), not in the project, with a log of every change in `settings/gate-log.jsonl` under the data root. While it is off, each time the agent says it is done you are told that nothing was checked. A damaged or expired setting never means "off". `MALVEON_GATE=off` (or `shadow`, `enforce`) in the environment the agent was started from also works, and wins over a saved setting. While the gate is on, the agent is also refused when it tries to edit one of the hook settings files above (or `.claude/settings.json`) that runs malveon, with a message saying the switch is yours. That is a speed bump, not a lock: an agent that can write any file you can could still edit the setting or the hook files through its shell. Run the install command again with the other `--gate` value to change shadow and enforce.
- When Claude says it is done, malveon also lists your prompts that name no file, URL or code name, because it could only check the agent's own guess at what you meant: look yourself, or run `malveon prompt confirm N` once you have. It lists what Claude added that nothing you wrote asked for (a page, table, data-changing button, request or package) the same way: for you to look at, never a reason to hold Claude. The report shows the same as `NEEDS YOU`. Then it holds Claude only for what this session could have caused: a new bug, a prompt that isn't built, a claim the code contradicts, and — for route conflicts, front/back-end mismatches and plan lines — only those that involve a file changed since the session started (or a plan document Claude edited). A problem your project already had is not Claude's to fix, so it doesn't hold Claude; `malveon check` still reports it. The text Claude is given names each problem and where it is.
- Speed: the first "done" in a session on a large project takes about 10 seconds (malveon reads the code as it was at the start), later ones about 2 seconds, and a "done" with nothing new since the last clean check is instant.
- If malveon itself fails, runs out of time (the stop check waits at most 20 seconds), or isn't found, the hooks let Claude carry on.
- The hooks call the exact program you ran `hooks install` with, and the command shows its path before you say yes. If you have an older malveon installed elsewhere, run the new one by its full path.
- Hooks and MCP servers are read when a Claude Code session starts, so open a new session after installing.

What it does not do: it only sees each agent's file-editing tools (Claude's Edit, Write and NotebookEdit; Codex's `apply_patch`; Cursor's Write, StrReplace and similar; Antigravity's `write_to_file`, `replace_file_content` and similar). An agent can still change a file through its shell tool, so this is a speed bump, not a lock. It never runs your build or tests (use `--exec` for that). The Claude Code hooks have been checked against Claude Code 2.1.237 on Windows only, using a scripted stand-in for the model, so they show how Claude Code reacts to being held, not how a real model does. The Codex, Cursor and Antigravity hooks have not been run against those agents at all (see above).

## Updates

The interactive update notice checks GitHub at most once a day. The unreleased Windows idle updater described above uses a separate six-hour scheduled helper (and the shared cache). `malveon check` asks GitHub's public releases page whether a newer malveon exists. This request doesn't include anything about you or your project. If a newer version exists, it tells you, and in a real terminal it asks `Update now? [y/N]`. You can also run `malveon update` any time to get the latest release, and `malveon version` shows which version you have. Set `MALVEON_NO_UPDATE_CHECK=1` (or `DO_NOT_TRACK=1`) to turn the daily check off.

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

First run `malveon updates disable` on a Windows installation with idle updates enabled. Use `malveon paths --json` before removing the executable to locate its data. If you also want to remove saved history and settings, back up any original-plan snapshots or prompt history you need, then remove that data directory. Legacy project `.malveon/` directories and old user-home state may remain as recovery copies; remove those only when you no longer need them. Package-manager files and publisher signing material are separate. If you ran `malveon hooks install`, remove the malveon entry from `.claude/settings.local.json`, `.codex/hooks.json`, `.cursor/hooks.json` or `.agents/hooks.json`.

## The plan file

Whatever format you already keep your plan in — no fixed shape forced on you, and no particular filename or location required (it searches the whole project tree, skipping `node_modules`/`.git`/build output and tool caches or downloaded editor builds like `.vscode-test`). Auto-detection tries two ways:

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

Tick a box (`- [x]`) when the plan says that item is done: then malveon blocks if the code disagrees. Unticked items (`- [ ]`) are checked too, but they don't block until you tick them (unless what's there is broken).

The more precise a line, the more precise the answer. "Refund button" gets NOT BUILT if nothing in the code mentions refunds, and SKIPPED (not checked, with the closest routes and pages in your code listed) if some code does but malveon can't tell which. Naming the URL, file or component (`` `POST /api/refunds` ``, `` `RefundButton` ``) lets it check exactly that. Any common URL placeholder style works: `:id`, `[id]`, `<id>`, `{id}`.

**Plain text** (`.txt`, or anything else): one feature name per line.

## Keep the checks with your repo (`malveon.json`)

Commit a `malveon.json` at your project root and every run uses the same gate: you in a terminal, your agent over MCP, a fresh session, or a commit hook. The first time you pick a plan file, malveon offers to write it for you.

```json
{
  "plans": ["docs/PLAN.md"],
  "exec": true
}
```

- `plans`: your plan file(s), relative to the project root.
- `exec`: also run your project's own build, lint, typecheck and test commands, and block on a failure (`true` in a new file).
- `exclude` (optional): folder names every check skips, anywhere in the project, the way `node_modules` already is: `"exclude": ["generated", "old-docs"]`. On the command line: `--exclude generated` (repeatable) for `malveon scan` and `malveon check`. Folder names only; a path like `src/old` is refused with a message, not guessed at. A single file can't be excluded yet.

A flag you pass on the command line (`--features`, `--exec=false`) always wins over the file.

**malveon only runs a repo's commands after you approve them.** `malveon.json` comes with the repo, so a repo you just cloned could ask malveon to run anything its `package.json` or `Makefile` says. The first time a repo's `malveon.json` turns commands on, malveon shows you exactly which commands would run and asks; your answer is remembered for that repo on this machine until its `malveon.json` changes. `malveon trust` approves them up front (needed for a commit hook), and your AI agent's checks ask you through your editor instead of running them unasked. Passing `--exec` yourself needs no approval: you typed it.

## What the output looks like

A real run on a small example project: a plan with four lines, a frontend in `frontend/`, a server in `backend/`. Two lines are built and connected; two are broken in ways an agent could easily call "done".

```text
$ malveon check --features features.json
FIX THESE FIRST
==============================================================================
  1. BROKEN — cancel order
     the frontend calls GET /cancel-order (frontend/app.js:8), but no server route handles it
  2. BROKEN — update settings
     the frontend sends PUT /settings (frontend/settings.js:2), but the server only accepts POST there (backend/settings.js:3)

OVERVIEW
==============================================================================
  read from the code ✔ · your build/tests: not run (turn on "exec" in malveon.json, or pass --exec) · your real app: not verified (malveon never runs it)

  Plan items built & connected       2 CONNECTED · 2 BROKEN · 0 NOT BUILT · 0 HALF BUILT · 0 SKIPPED
  Your prompts vs the code           not set up (malveon hooks install)
  Frontend & backend agree on data   2 MATCH · 1 MISMATCH (request fields compared on 0)
  Backend route conflicts            clean
  UI elements may overlap            clean
  Unplanned code                     clean
  New bugs this session              clean
  Unfinished code                    clean
  Agent's claims vs reality          no commit messages since the session start
  Agent's words vs the code          clean
  Build, lint & tests actually ran   not run (pass --exec to include)

Plan items — built and connected? (wiring check)
------------------------------------------------------------------------------
BROKEN (2) — built, but it won't work as written: fix these first
  cancel order
    the frontend calls GET /cancel-order (frontend/app.js:8), but no server route handles it
    evidence: frontend/app.js:8
  update settings
    the frontend sends PUT /settings (frontend/settings.js:2), but the server only accepts POST there (backend/settings.js:3)
    evidence: frontend/settings.js:2
    evidence: backend/settings.js:3

CONNECTED (2) — built and connected in the code
  refund button
    path: page not traced → POST /refund (frontend/app.js:3) → route POST /refund (backend/server.js:3) → no data change malveon can see
  ...
  note: "page not traced": malveon finds the pages of Next.js, Nuxt, SvelteKit, Remix / React Router and Astro only
  note: "no data change malveon can see": malveon reads Supabase, Prisma and Drizzle database calls only

GATE: blocked — a commit gated on this run should not proceed
  - plan items: 2 BROKEN, 0 NOT BUILT, 0 HALF BUILT
  - frontend & backend agree on data: 1 MISMATCH
```

Some sections are shortened here (`...`). Notice what is *not* printed: a check that found nothing is just its line in the overview, and plan lines that aren't code tasks are counted, not listed. The first line keeps three states apart: what was read from the code, whether your own build and tests actually ran and passed, and your real app, which malveon never runs. `CONNECTED` is only ever the first of those. The run exits with code 1 because the gate is blocked, which is what stops a commit hook.

## Every check, in detail

The summary near the top is the short version. This is every check, with its exact scope and limits.

`malveon check` reads the actual code and git history, and for one opt-in check actually runs your project's own commands. It reports these things:

- **Scan** *(`malveon scan`, needs no plan, session or setup)* — four kinds of fault, each judged only where malveon can be sure, so a short list is expected:
  - *Dead button*: a `<button>` or `<Button>` in JSX/TSX with visible text and no click handler, or one that is an empty function. Not judged: an icon-only button, a file with spread props or a framework attribute (`hx-post`, `@click`), a button just under a link, a label, a dialog or menu trigger or `asChild` (the element around it may handle the click), and stories and tests.
  - *Broken import*: a relative import of a file that doesn't exist, or of a plain name the file doesn't export. Not judged: a package or alias (`@acme/core`, `@/x`), an asset, a path through a build-output folder, a file that re-exports names malveon can't list (`export *`, `export type *`), and anything not JavaScript or TypeScript.
  - *Call to nowhere*: a plain `fetch`/`axios`/SWR call with the address written out in quotes, to an address no route in the same app serves with that method. Judged only in an app (the nearest folder with a manifest) that has other routes under the same first path segment: `/api/missing` beside `/api/items` is a typo or a route nobody wrote, while `/v1/chat/completions` beside no `/v1` routes is another service. A call through a helper (`ghGet(token, '/repos/...')`) isn't judged.
  - *Unfinished route*: a JavaScript, TypeScript, Python or Go handler that only answers with an error or "not implemented" (sometimes on purpose: the report says what it saw, not that it's wrong).
  It lists the three most certain first (dead buttons, then imports, then calls, then routes). Run on four real projects it found 2 problems in a 320-file app (both dead buttons, checked by hand), 5 in a 1,071-file monorepo (all stale imports of files that no longer exist), 3 in a 2,504-file monorepo (two stale imports, one deliberate 501) and none in an 84-file app. Its first version listed 51 problems on the 1,071-file monorepo, nearly all wrong; every restriction above is one of those.
- **Plan items (wiring)** — every line of your plan gets one direct answer. malveon looks up what the line names — a URL (`/api/jobs`, `POST /api/jobs`, `/admin/digest`), a file (`PostJobModal.tsx`), or a code name in backticks (`InvitesTab`, `generate_referral_code`) — and checks it against the real code, including calls straight to Supabase (`.from()`, `.rpc()`, `.auth.*`), Prisma (`prisma.post.create()`, `$queryRaw`) and Drizzle (`db.insert(posts)`):
  - `CONNECTED` — built, and the screen actually reaches its server route (or database)
  - `BROKEN` — built, but it won't work as written: the screen calls a URL nothing serves, or with a method the server doesn't accept
  - `NOT BUILT` — the plan names it, the code doesn't have it
  - `HALF BUILT` — one part exists, the other doesn't (a server route nothing in your app calls, even if a test does; a component that never saves anything the line says it should; a line about a button or form whose request is made by a function nothing in your app ever calls, so clicking it does nothing)
  - `SKIPPED` — not checked, and never a problem in your code. Either it isn't a code task (a heading, a design note, a manual step like "open the page and check", or a build/test step — that's what `--exec` is for), or it names nothing malveon can look up ("make search better", a field list, a sub-bullet). The report counts the first kind and names a few of the second, with the closest routes and pages in your code, so you can add the URL, file, or component the line is about and have it checked. A skipped line never blocks a commit; only a plan where *nothing* could be checked does

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
- **The verdict** — the report opens with one answer that comes only from what the code and the checks said, never from what the agent said about its own work: `FAILED` (something is proven wrong in the code), `NEEDS YOU` (nothing is proven wrong, but something is yours to settle: words of your prompt that no check covered, such as "faster" or "nicer", a task nothing was checked for, work the agent left unfinished, tests it deleted or thinned out, a page, table, data-changing button, request or package it added that nothing you wrote asked for, a check that couldn't run), or `VERIFIED BY CODE` (nothing is proven wrong, and everything malveon could check is built and connected). "Verified" never means the app was run; the report says so under every verdict. Under any verdict it also lists what was **not established**: plan lines that name nothing malveon can look up, and connections where the fields your screen sends were not compared with what the server reads. When the stop hook is installed, the person also sees malveon's own short account of the session next to the agent's "done": what changed, what is proven wrong, what is built and connected, what is not established, and that the app was never run. The agent is not shown it.
- **Command check** *(opt-in, `--exec`)* — do your project's own build/lint/typecheck/test commands actually pass right now? The one check that runs real subprocesses instead of reading the code graph — a Makefile target, a `package.json` script, Go's own toolchain, or whatever Python tooling is on your `PATH`, whichever the project already defines. Nothing invented, nothing guessed, no server or browser ever started. Off by default since it can take a few minutes. When a command fails and the session has a start, malveon also runs the same commands on the project as it was at the session start (in a temporary copy; your folder is never written to) and says whether it **passed then** (this session broke it) or **also failed on that copy** (so it may not be new; the copy holds only the tracked files, so a missing generated file or secret can cause that). It checks whole commands, not single tests.

Every answer states the fact it's based on, with file:line — never a guess dressed up as an answer. By default, `malveon check` also exits non-zero if a plan line came back `BROKEN`, `NOT BUILT` or `HALF BUILT`, nothing in the plan could be checked at all, something else failed, or a check couldn't run at all — see [Blocking a commit](#blocking-a-commit) below.

## What it can't do (yet)

- Doesn't prove business logic is *correct* — only that the wiring and HTTP method agree.
- No server, no browser, no deployed app, ever — `CONNECTED` means built and connected in the code, not clicked through in a running app. A URL built from a variable (`fetch(url)`) can't be matched to a route, so it never counts as evidence either way. The command check runs your project's own already-defined build/lint/typecheck/test commands (nothing invented), but that's still not a live/deployed run.
- A single fixed timeout applies to every command in the main command check rather than one tuned per category.
- The new-bugs check only knows a small, named catalog of bug patterns plus one structural comparison (calls that stopped reaching a route) — not a general "was this a real bug" judgment, which can't be settled from static code. It needs a session start and a git repository to compare against.
- Incompleteness is one-directional — a marker's presence is real proof, but its absence proves nothing (most finished code has none either). Never a substitute for the confidence check.
- Overlap's reachability note is a best-effort heuristic (checks whether an enclosing function's name is ever mentioned elsewhere in the codebase), not real call-graph analysis — an anonymous handler or dead code it can't attribute to a named function still just counts as a plain registration.
- Contract's field-agreement check is JS/TS only, and only fires when both sides are a literal object (no spread, no variable) — Python/Go request bodies and response-shape comparison aren't covered yet.
- Frontend overlap risk (both Tailwind and plain CSS) is file-scoped rather than tracing the real JSX/selector ancestor chain — no CSS specificity/cascade resolution.
- Prompt checking only sees prompts typed after `malveon hooks install`, and only covers Claude Code, Codex, Cursor, and Antigravity. Antigravity doesn't hand its hook the prompt text, so malveon reads it from Antigravity's conversation log, whose format isn't documented and could change; if it can't be read, the report says so. A line the agent writes can be real and pass while still asking for less than you did; the "not covered" note points at that, but you make the call.
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
| Code map: reads your routes, requests, pages, imports and database calls (`extractor`, `graph`, `skipdirs`) | [`pkg/`](pkg) | Open, Apache-2.0 |
| Plan reading (`features`) | [`pkg/features`](pkg/features) | Open, Apache-2.0 |
| The plan-items check: CONNECTED, BROKEN, NOT BUILT, HALF BUILT, SKIPPED (`wiring`) | [`pkg/wiring`](pkg/wiring) | Open, Apache-2.0 |
| Everything else in the `malveon` program: new bugs, agent's words, weakened tests, unplanned code, prompts, original-plan snapshots and platform interpretation, the gate and hooks, the report, MCP server, engine coordination, updates, telemetry | not published | Closed. Free to use during the beta under [BETA-LICENSE.txt](BETA-LICENSE.txt) |

What this means: the downloadable `malveon` program is built from the open packages plus the closed ones. It is not built from this repository alone, and the release binaries are built and signed outside it. So you can audit how a plan line becomes a verdict, but you cannot rebuild the whole program from this source. The closed checks keep their limits documented above.

To run the open tests:

```bash
go test ./...
```

`go test -short ./...` skips one timing test that is slow on loaded machines.

## License

**Two licenses, one repo.**

- **Source code** in `pkg/` and `testdata/`: Apache License 2.0. Use it, change it, redistribute it. See [LICENSE](LICENSE).
- **The `malveon` program** you download: free to use during the beta, including for commercial work. It contains the open code above plus closed code. See [BETA-LICENSE.txt](BETA-LICENSE.txt).

Later versions may come with different terms. The version you already have keeps the terms it shipped with.

## Questions or feedback

Using this on a stack it doesn't handle well, found a bug, or just want to say something's off? [Open a discussion](https://github.com/LadsonDavid/Malveon/discussions) — that's the one place I'm actually watching.

Want a custom check rule built for your specific stack?
```bash
malveon register
```
Prints (and tries to open) a pre-filled link to a new GitHub Discussion in the Q&A category — describe your framework and what you need, and you'll get a reply there, in public. No email, no sign-up: the discussion only exists if you write it and submit it yourself. (Like every command, it sends one anonymous usage event naming the command; see [Usage tracking](#usage-tracking).)
