# Council

Local-first multi-agent orchestration for developer workflows.

Council runs the same task across a user-defined team of models, lets them critique and revise across rounds, and returns one synthesized answer by default. It is designed to work well for review, plan pressure-testing, architecture analysis, and open-ended reasoning.

## Host Integration

This repo keeps one shared source of truth for the host prompt layer:
- `.shared/commands/council.md`
- `.shared/commands/council-config.md`

Those files are symlinked into:
- `.claude/commands`
- `.opencode/commands`
- `.agents/skills`

## Claude Code And OpenCode

After cloning the repo, the shared prompt files are already wired in for:
- `/council`
- `/council-config`

The wrappers used by those hosts live at:
- `wrappers/council`
- `wrappers/claude/council`
- `wrappers/opencode/council`

## Codex Setup

Codex on this machine discovers custom skills from `~/.codex/skills`. If you want Council to appear in Codex's slash picker after cloning this repo, install symlinks there:

```bash
mkdir -p ~/.codex/skills/council ~/.codex/skills/council-config
ln -s "$(pwd)/.shared/commands/council.md" ~/.codex/skills/council/SKILL.md
ln -s "$(pwd)/.shared/commands/council-config.md" ~/.codex/skills/council-config/SKILL.md
```

After restarting Codex or refreshing its skill list, Council should appear in the slash picker as:
- `/council`
- `/council-config`

If you already have existing `council` or `council-config` skills in `~/.codex/skills`, remove or rename those first.

## Notes

- Council is the deliberation engine, not the host tool.
- The host should gather context first, then hand Council a clean text brief.
- Screenshots and images should be converted to text by the host before being passed to Council.
- Use `/council-config` when you want to change persistent team definitions in `council.yaml`.

For wrapper behavior and environment variables, see `wrappers/README.md`.
