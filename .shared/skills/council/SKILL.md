---
name: council
description: Use Council as the backend deliberation engine for multi-agent review, comparison, and pressure-testing tasks.
argument-hint: [natural language request]
---

Use Council as the backend deliberation engine for the current task.

Invocation notes:
- In Claude Code and OpenCode, invoke this as `/council ...`.
- In Codex, install or symlink the whole `council` skill directory into `~/.codex/skills/council`. Once installed, Codex should surface it in the slash picker as `/council`.

Treat the user's current request, or `$ARGUMENTS` when the host provides it, as a natural-language request, not as preformatted CLI flags.

Examples of valid usage:
- `/council use the gh CLI to review pr 471. use the A-team.`
- `/council use the installed review-pr command for PR 554.`
- `/council compare @PROJECT_BRIEF.md with @council.yaml and tell me what is missing.`
- `/council use the default team and pressure-test this implementation plan.`
- `/council use the A-team, run 2 rounds, stop after 90 seconds, and keep agent outputs.`

Workflow:

1. Parse the user's current request, or `$ARGUMENTS` when the host provides it, as instructions for the host tool.
2. If the current request starts with another slash command, or clearly asks you to use an installed slash command such as `review-pr`, treat that command as source instructions, not as a second slash invocation in the UI.
3. Resolve that source command by reading its installed command file from the host environment, for example `~/.claude/commands/<name>.md`, repo-local `.shared/commands/<name>.md`, or the host's equivalent command directory.
4. Extract from that command file:
   - the task instructions
   - any required host-side workflow such as `gh` commands or file gathering
   - any explicit final-answer, `Output Format`, section-order, heading, or stopping-rule requirements
5. Execute the required host-side workflow yourself, gather the relevant context, and preserve the explicit output requirements from the source command as the requested output portion of Council's final answer.
6. If the request asks you to use host tools like `gh`, do that first and gather the relevant context before calling Council.
7. Build a concise task brief from:
   - the user's latest request
   - any relevant conversation context
   - any host-tool output you collected
   - the relevant source-command instructions, quoted or summarized precisely enough that Council receives the same review rubric
8. Tell Council to always start the final answer with a short `## Brief` section that summarizes the main findings, broad agreements, disagreements, and remaining uncertainty. If the task or source slash command specifies a final answer format, include that format verbatim in the task brief and tell Council to place it under `## Requested Output`. If there is no explicit requested output format, tell Council to omit `## Requested Output` and return only the brief in concise Markdown.
9. If the user supplied screenshots or other visual context, first convert the relevant details into text. Council currently accepts text artifacts only.
10. Treat explicit `@file` references in the current request as host-side context. Read or expand them and include the relevant file text directly in the task brief you send to Council.
11. Only pass `--file <path>` to Council when you intentionally want Council itself to persist artifact metadata/content for that file. This should be optional, not the default path for slash-command usage.
12. Detect explicit team requests in natural language:
    - "A-team" -> `--team a-team`
    - "default team" -> `--team default`
    - otherwise let the wrapper use its default A-team
13. Detect explicit runtime parameter requests in natural language and convert them into Council flags only when the user clearly asked for them. Examples:
    - `run 2 rounds` -> `--max-rounds 2`
    - `stop after 90 seconds` -> `--max-time 90s`
    - `keep agent outputs` or `keep transcript` -> `--retain-agent-outputs`
    - `keep raw provider output` -> `--retain-raw-provider-io`
    - `retain artifact content` -> `--retain-artifact-content`
14. Run Council through the repo-local wrapper from the project root using a Bash heredoc so the prompt text is preserved cleanly. Only pass actual Council flags you intentionally derived from the request. Capture Council's stdout as the answer body you will send back to the user.

```bash
/Users/drewrawitz/www/council/wrappers/council <derived flags> <<'EOF'
<task brief>
EOF
```

15. Return the synthesized Council answer in a normal assistant message, with the full Council output text in the message body. Do not say the answer is "already posted above", do not tell the user to inspect tool output, and do not replace the answer with a summary. Mention the run id only when it is useful for follow-up inspection.

Important UI note:
- In hosts that tokenize slash commands inside the input box, do not invoke the nested slash command as a separate command chip while handling `/council`. Read the nested command's file and inline its instructions into the Council brief instead.

If the user is asking to change Council's persistent configuration itself, such as defining or editing teams in `council.yaml`, do not treat that as a run request. Instead, tell them to use `/council-config`, or handle the config edit directly in the host session.

Keep the host tool responsible for:
- gathering context
- using skills
- using `gh` or other local CLIs when requested
- inspecting code or screenshots
- translating rich context and `@file` references into plain text for Council

Keep Council responsible for:
- multi-agent deliberation
- critique/revise rounds
- synthesis into one final answer
