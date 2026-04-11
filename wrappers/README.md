# Council Wrappers

These wrappers are intentionally thin adapters around `council ask`.

Recommended mental model:
- use Claude Code or OpenCode as the interactive UI and tool host
- use Council as the behind-the-scenes deliberation engine
- let the host gather context, use skills, inspect code, and translate screenshots into text
- hand Council a clean task brief plus any local text artifacts

Available entrypoints:
- `wrappers/claude/council`
- `wrappers/opencode/council`

Both wrappers:
- read the prompt from `stdin`
- call `council ask --stdin`
- pass any CLI flags through unchanged
- optionally fill defaults from environment variables
- automatically use `<repo>/council.yaml` when `COUNCIL_CONFIG` is not set

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
printf 'Review this plan' | wrappers/claude/council
```

```bash
printf 'Review this plan' | wrappers/opencode/council --team a-team --json
```

For screenshots and rich host context:
- paste the screenshot into Claude Code or OpenCode
- let the host model describe or OCR it into text
- pass that text directly in the prompt, or save it to a local `.md` file and attach it with `--file`

Today Council itself only ingests local text artifacts. Images and screenshots should be converted to text by the host first.

These wrappers do not implement any orchestration logic themselves. They only translate wrapper invocation into the existing Council CLI.
