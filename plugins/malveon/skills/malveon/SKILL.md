---
name: malveon
description: Use before telling the user a coding task is done, or when asked to verify work. Runs Malveon, which reads the real code (never the agent's word) and reports what is built, broken, unused or not asked for.
---

# Malveon

Malveon checks the project's code against what the user asked for. It reads code only; it does not start the app.

1. When you finish a task, and before you say it is done, call `malveon_check` (or `malveon_scan` when there is no plan or prompt to check against).
2. Show the user the verdict and findings exactly as written. Never call a run clean when the verdict says otherwise, and never leave a finding out.
3. If the report says prompts are waiting, call `malveon_prompt_claims` and say in one line per prompt what it asked for, naming a URL, file or code name in backticks. Then call `malveon_check` again.
4. `malveon_unused_walk` and `malveon_changed_walk` are only for when the user asks to be walked through unused or changed code. Show the steps as written; only the user can mark an item fine.
5. A quiet result is not proof the work is right. Say what was checked and that the app was not run.
