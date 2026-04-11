# Council

Local-first multi-agent orchestration engine for developers and power users.

## Summary

Council is a standalone local CLI tool that runs the same task across a user-defined team of LLM agents, lets them deliberate over multiple rounds, and returns one synthesized answer by default.
The product is not limited to PR review. It should work for any task where multiple perspectives are useful, including code review, plan critique, decision support, architecture review, document analysis, and open-ended reasoning.
The primary UX is synthesis-first:

- user runs one command
- a team of agents evaluates the task
- the engine iterates through critique/revision rounds
- the user receives one final synthesized answer
- raw transcripts and per-agent responses are optional

## Core Product Principles

- Local-first only
- Use the developer's existing authenticated tools and subscriptions when possible
- CLI-first engine, with thin wrappers for OpenCode, Claude Code, and similar tools
- User-defined teams, not hard-coded model bundles
- Same model can appear multiple times with different roles
- Iterative deliberation, not just one-pass fanout
- Final synthesized answer is the default output
- Transcript is optional inspection/debug output
- General-purpose, not PR-specific

## Problem

Current multi-model workflows are manual and slow:

- user opens multiple model sessions
- user pastes the same prompt into each
- user manually pastes outputs back and forth
- user reconciles disagreements by hand
- repeated iterations are tedious and error-prone
  Council should remove that friction by orchestrating the whole process locally.

## Goals

- Let users define reusable teams of agents
- Support duplicate models with different roles/prompts
- Run tasks across all team members in parallel per round
- Support iterative critique/revision loops with configurable caps
- Produce a clean synthesized final answer
- Optionally expose transcript, raw outputs, and round-by-round evolution
- Be easy to invoke from CLI and from agent skills/wrappers

## Non-Goals For V1

- Hosted SaaS
- Browser automation for chat-only products
- Rich GUI as the primary interface
- Marketplace/plugin ecosystem
- Real-time collaborative debate UI
- Full workflow automation for every provider on day one

## Core Concepts

### Provider

Mechanism for invoking a model locally using an existing CLI or API credential.
Examples:

- OpenAI CLI/API
- Codex CLI/API
- Claude CLI/API
- local model runners

### Agent

A named callable persona defined by:

- provider
- model
- role
- system prompt
- generation settings
  Two agents may use the same model but different roles.
  Examples:
- `gpt54-general`
- `gpt54-contrarian`
- `opus-pragmatic`

### Team

A named list of agents.
Examples:

- `a-team`
- `cheap`
- `paranoid-review`

### Protocol

The deliberation workflow:

- initial pass
- critique
- revise
- repeat until stop conditions
- synthesize

### Task

The user’s request plus optional context and artifacts.

### Run

One execution of:

- task
- team
- protocol

### Synthesizer

The model/agent responsible for the final answer.

## Example Team Config

```yaml
agents:
  gpt54-general:
    provider: openai-cli
    model: gpt-5.4
    role: general-analyst
    system_prompt: |
      You are a rigorous general-purpose analyst. Be concise, specific, and evidence-driven.
  gpt54-contrarian:
    provider: openai-cli
    model: gpt-5.4
    role: contrarian
    system_prompt: |
      You are skeptical. Attack weak assumptions, overconfidence, and unsupported claims.
  codex-xhigh-reviewer:
    provider: codex-cli
    model: gpt-5.3-codex-xhigh
    role: implementation-reviewer
    system_prompt: |
      Focus on correctness, regressions, edge cases, and hidden assumptions.
  opus-pragmatic:
    provider: claude-cli
    model: opus-4.6
    role: pragmatic-architect
    system_prompt: |
      Focus on practical tradeoffs, failure modes, and second-order effects.
teams:
  a-team:
    members:
      - gpt54-general
      - codex-xhigh-reviewer
      - opus-pragmatic
    synthesizer: opus-pragmatic
  paranoid-review:
    members:
      - gpt54-general
      - gpt54-contrarian
      - opus-pragmatic
    synthesizer: opus-pragmatic
Deliberation Model
Council should use barrier-based rounds:
- all active team members complete a round
- the engine normalizes responses
- the next round begins only after the round state is assembled
Default round flow:
1. Initial
2. Normalize
3. Critique
4. Revise
5. Normalize
6. Repeat until convergence or cap
7. Synthesize
Important:
- do not just forward raw full transcripts between agents every round
- extract structured items first
- feed back only relevant disagreements, claims, risks, or questions
Internal Structured Items
The engine should normalize responses into items such as:
- claim
- finding
- risk
- recommendation
- question
Each item should track:
- id
- type
- content
- evidence
- supporters
- opposers
- confidence
- status
Statuses:
- open
- supported
- contested
- conceded
- dismissed
This item graph is the backbone for iterative deliberation.
Output
Default output:
- one synthesized final answer in Markdown
Optional output:
- consensus points
- remaining disagreements
- uncertainty / confidence
- missing information
- next best question
- full transcript
- per-agent raw outputs
- JSON output
- HTML report
Transcript should be optional, not primary.
CLI Direction
Examples:
council ask "Review this plan" --team a-team
council run --prompt-file prompt.md --team paranoid-review
council run --stdin --team cheap --mode decide
council run gh://owner/repo/pull/123 --team a-team --mode review
council show <run-id>
council transcript <run-id>
Recommended flags:
- --team
- --mode
- --protocol
- --artifact
- --stdin
- --show-transcript
- --output json|md|html
- --max-rounds
- --max-cost
- --max-time
Skill Integration
Council should be usable from OpenCode / Claude Code as a thin wrapper around the CLI.
Skill wrapper responsibilities:
- collect current task context
- invoke council with machine-readable output
- display the final synthesis
- optionally expose transcript and raw round details on demand
The CLI remains the core product. Skills are adapters, not the main implementation.
Recommended Architecture
Main modules:
- core/config
- core/run-engine
- core/protocols
- core/items
- providers/*
- artifacts/*
- storage/*
- cli/*
Suggested internals:
- config loader and validation
- provider adapter interface
- task packet builder
- round orchestrator
- normalization/item extraction layer
- convergence/stop-condition evaluator
- synthesis layer
- local run storage
V1 Scope
V1 should include:
- local CLI
- config for providers, agents, teams, protocols
- same-model-different-role support
- one iterative default protocol
- strict barrier-based rounds
- local run storage
- Markdown + JSON output
- optional transcript output
- provider adapters for locally callable tools
- basic skill-wrapper friendliness
V1 should exclude:
- browser automation
- rich UI
- advanced protocol DSL
- hosted sync
- plugin marketplace
Phase Plan
Phase 0: Product Skeleton
Goal:
- define repo, language, packaging, and initial architecture
Deliverables:
- repo scaffold
- config format decision
- run model definitions
- clear provider adapter interface
- no real provider calls required yet
Exit criteria:
- project compiles
- config can be parsed
- agents/teams/protocols can be loaded and validated
Phase 1: Single-Round Execution
Goal:
- run one prompt across a team and collect responses
Deliverables:
- provider abstraction
- one or two real providers or stubs
- task packet builder
- team fanout execution
- run storage
- basic synthesized summary without full iterative deliberation
Exit criteria:
- one command can run N agents and collect outputs
- results are stored locally
- simple synthesis works
Phase 2: Structured Normalization
Goal:
- convert raw outputs into a normalized item graph
Deliverables:
- response schema
- extraction pipeline
- dedupe/clustering
- support/opposition tracking
Exit criteria:
- engine can identify overlapping items and disagreements
- outputs are no longer just raw text blobs
Phase 3: Iterative Deliberation
Goal:
- implement critique/revise loops with caps and convergence logic
Deliverables:
- critique round prompts
- revise round prompts
- barrier-based round manager
- convergence heuristics
- stop conditions
Exit criteria:
- multi-round runs work reliably
- engine stops on heuristics or hard caps
Phase 4: Synthesis-First Reporting
Goal:
- produce a polished final answer plus optional deeper inspection
Deliverables:
- final synthesis prompt(s)
- Markdown report
- JSON result format
- transcript export
- round summaries
Exit criteria:
- final answer is useful without reading raw transcripts
- transcript remains available when needed
Phase 5: Artifact Adapters
Goal:
- support richer task inputs
Deliverables:
- local file adapter
- URL adapter
- stdin support
- GitHub adapter via gh
Exit criteria:
- engine can handle common developer workflows beyond plain prompts
Phase 6: Skill Integration
Goal:
- make Council easy to invoke from OpenCode / Claude Code
Deliverables:
- stable machine-readable CLI mode
- thin wrapper prompt/skill definitions
- examples and docs
Exit criteria:
- users can invoke Council from agent environments without bespoke glue code each time
Key Open Questions
- Which implementation language best fits local CLI + provider integration?
- Which providers should V1 support first?
- How strict should the structured output contract be?
- How much normalization should be rule-based vs model-assisted?
- Should synthesis default to a team member or a dedicated synthesizer agent?
- How should partial failure be handled in later versions?
Success Criteria
The project is successful if:
- a user can define their own teams easily
- the same model can be reused with different roles
- a team can deliberate over multiple rounds
- the default output is a strong synthesized answer
- transcript inspection is available but optional
- the tool is usable locally and fits naturally into developer workflows
Short Product Statement
Council is a local CLI-first multi-agent deliberation engine that lets users compose teams of LLM agents, run iterative rounds of critique and revision, and receive one synthesized answer without manual copy/paste reconciliation.
```
