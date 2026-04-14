---
description: Run the current task through the Council deliberation engine
---

Use Council as the backend deliberation engine for the current task.

Treat `$ARGUMENTS` as a natural-language request, not as preformatted CLI flags.

Examples of valid usage:
- `/council use the gh CLI to review pr 471. use the A-team.`
- `/council /review-pr 554`
- `/council compare @PROJECT_BRIEF.md with @council.yaml and tell me what is missing.`
- `/council use the default team and pressure-test this implementation plan.`
- `/council use the A-team, run 2 rounds, stop after 90 seconds, and keep agent outputs.`

Workflow:

1. Parse `$ARGUMENTS` as instructions for the host tool.
2. If `$ARGUMENTS` starts with another slash command, treat that slash command as the underlying task. Follow its host-side workflow as needed, gather the relevant context, and preserve any explicit final-answer or `Output Format` requirements from that command as the requested output portion of Council's final answer.
3. If the request asks you to use host tools like `gh`, do that first and gather the relevant context before calling Council.
4. Build a concise task brief from:
   - the user's latest request
   - any relevant conversation context
   - any host-tool output you collected
5. Tell Council to always start the final answer with a short `## Brief` section that summarizes the main findings, broad agreements, disagreements, and remaining uncertainty. If the task or source slash command specifies a final answer format, include that format verbatim in the task brief and tell Council to place it under `## Requested Output`. If there is no explicit requested output format, tell Council to omit `## Requested Output` and return only the brief in concise Markdown.
6. If the user supplied screenshots or other visual context, first convert the relevant details into text. Council currently accepts text artifacts only.
7. Treat explicit `@file` references in `$ARGUMENTS` as host-side context. Read or expand them and include the relevant file text directly in the task brief you send to Council.
8. Only pass `--file <path>` to Council when you intentionally want Council itself to persist artifact metadata/content for that file. This should be optional, not the default path for slash-command usage.
9. Detect explicit team requests in natural language:
   - "A-team" -> `--team a-team`
   - "default team" -> `--team default`
   - otherwise let the wrapper use its default A-team
10. Detect explicit runtime parameter requests in natural language and convert them into Council flags only when the user clearly asked for them. Examples:
   - `run 2 rounds` -> `--max-rounds 2`
   - `stop after 90 seconds` -> `--max-time 90s`
   - `keep agent outputs` or `keep transcript` -> `--retain-agent-outputs`
   - `keep raw provider output` -> `--retain-raw-provider-io`
   - `retain artifact content` -> `--retain-artifact-content`
11. Run Council through the repo-local wrapper from the project root using a Bash heredoc so the prompt text is preserved cleanly. Only pass actual Council flags you intentionally derived from the request.

```bash
/Users/drewrawitz/www/council/wrappers/opencode/council <derived flags> <<'EOF'
<task brief>
EOF
```

12. Return the synthesized Council answer to the user. Mention the run id only when it is useful for follow-up inspection.

If the user is asking to change Council's persistent configuration itself, such as defining or editing teams in `council.yaml`, do not treat that as a run request. Instead, tell them to use `/council-config` or handle the config edit directly in the host session.

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
