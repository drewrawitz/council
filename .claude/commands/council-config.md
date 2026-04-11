---
description: Create or update Council teams and defaults in council.yaml
argument-hint: [natural language config request]
allowed-tools: Read, Write, Edit, Grep
---

Treat `$ARGUMENTS` as a natural-language request to modify Council's persistent configuration in `council.yaml`.

Examples:
- `/council-config make the A-team use GPT-5.4 xhigh, Opus 4.6, and Sonnet 4.6 as synthesizer.`
- `/council-config add a fast team that only uses Sonnet with one round and a 60 second timeout.`
- `/council-config make default retention minimal and keep raw outputs off.`

Workflow:

1. Read `council.yaml`.
2. Interpret `$ARGUMENTS` as desired config changes.
3. Make the smallest correct edit to `council.yaml`.
4. Keep names stable unless the user explicitly asks to rename them.
5. If the request is ambiguous, ask one short clarification question.
6. After editing, validate with:

```bash
go run ./cmd/council config validate --config council.yaml
```

7. Summarize the resulting team or config change briefly.
