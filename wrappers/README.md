# Council Wrappers

These wrappers are intentionally thin adapters around `council ask`.

Recommended mental model:
- use Claude Code, OpenCode, or Codex as the interactive UI and tool host
- use Council as the behind-the-scenes deliberation engine
- let the host gather context, use skills, inspect code, and translate screenshots into text
- hand Council a clean task brief plus any local text artifacts

Recommended usage in the host tool:
- speak to `/council` in natural language
- mention host actions directly, for example: `use the gh CLI to review pr 471`
- mention teams naturally, for example: `use the A-team`
- mention run parameters naturally, for example: `run 2 rounds` or `stop after 90 seconds`
- reference files naturally with `@path/to/file`

For Codex specifically:
- use the repo-scoped `$council` and `$council-config` skills in `.agents/skills`
- Codex currently exposes built-in slash commands only, so repo-local Council integration uses skills rather than custom `/council` commands

Use `/council-config` when you want to change persistent team definitions or run defaults in `council.yaml`.

The slash-command layer should:
- interpret that natural-language request
- use host tools first when requested
- treat `@file` references as host-side text context by default
- only use Council `--file` when you intentionally want persisted artifact metadata/content
- translate team mentions like `A-team` into `--team a-team`
- translate explicit runtime requests like `run 2 rounds` into the matching Council flags
- default to `a-team` when no team is specified

Available entrypoints:
- `wrappers/council`
- `wrappers/claude/council`
- `wrappers/opencode/council`

Shared command definitions live in `.shared/commands` and are symlinked into `.claude/commands`, `.opencode/commands`, and `.agents/skills`.

All wrapper entrypoints:
- read the prompt from `stdin`
- call `council ask --stdin`
- pass any CLI flags through unchanged
- optionally fill defaults from environment variables
- automatically use `<repo>/council.yaml` when `COUNCIL_CONFIG` is not set
- default to `a-team` when no `--team` or `COUNCIL_TEAM` is provided

Supported environment variables:
- `COUNCIL_BIN`: explicit `council` binary path
- `COUNCIL_CONFIG`: config file path
- `COUNCIL_TEAM`: default team name when `--team` is not passed
- `COUNCIL_MAX_TIME`: default `--max-time` value
- `COUNCIL_MAX_ROUNDS`: default `--max-rounds` value
- `COUNCIL_RETAIN_AGENT_OUTPUTS=1`
- `COUNCIL_RETAIN_RAW_PROVIDER_IO=1`
- `COUNCIL_RETAIN_ARTIFACT_CONTENT=1`

Examples:

```bash
printf 'Review this plan' | wrappers/council
```

```bash
printf 'Review this plan' | wrappers/council --team a-team --json
```

For screenshots and rich host context:
- paste the screenshot into Claude Code or OpenCode
- let the host model describe or OCR it into text
- pass that text directly in the prompt
- save it to a local `.md` file and use `--file` only if you explicitly want Council to retain it as an artifact

Today Council itself only ingests local text artifacts. Images and screenshots should be converted to text by the host first.

These wrappers do not implement any orchestration logic themselves. They only translate wrapper invocation into the existing Council CLI.
