# Council Wrappers

These wrappers are intentionally thin adapters around `council ask`.

Available entrypoints:
- `wrappers/claude/council`
- `wrappers/opencode/council`

Both wrappers:
- read the prompt from `stdin`
- call `council ask --stdin`
- pass any CLI flags through unchanged
- optionally fill defaults from environment variables

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
printf 'Review this plan' | wrappers/claude/council --team local-duo --config examples/local-clis.yaml
```

```bash
COUNCIL_CONFIG=examples/mock-defaults.yaml COUNCIL_TEAM=default \
  printf 'Review this plan' | wrappers/opencode/council --json
```

These wrappers do not implement any orchestration logic themselves. They only translate wrapper invocation into the existing Council CLI.
