package run

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"strings"
	"sync"
	"time"

	"council/internal/model"
	"council/internal/providers"
	"council/internal/storage"
)

type EventType string

const (
	EventRunStarted        EventType = "run_started"
	EventAgentStarted      EventType = "agent_started"
	EventAgentCompleted    EventType = "agent_completed"
	EventAgentFailed       EventType = "agent_failed"
	EventSynthesisStarted  EventType = "synthesis_started"
	EventSynthesisComplete EventType = "synthesis_completed"
)

type Event struct {
	Type      EventType
	RunID     string
	AgentName string
	Provider  string
	Model     string
	Duration  time.Duration
	Err       error
}

type Observer func(Event)

func Execute(ctx context.Context, repo *storage.Repository, cfg *model.Config, teamName string, prompt string, observer Observer) (*model.RunRecord, error) {
	plan, err := BuildPlan(cfg, teamName)
	if err != nil {
		return nil, err
	}

	runRecord := &model.RunRecord{
		ID:        newRunID(),
		Team:      teamName,
		Protocol:  plan.ProtocolName,
		Status:    "running",
		Prompt:    prompt,
		StartedAt: time.Now().UTC(),
	}

	notify(observer, Event{Type: EventRunStarted, RunID: runRecord.ID})

	team := cfg.Teams[teamName]
	runRecord.AgentOutputs = make([]model.AgentOutput, len(team.Members))
	for index, memberName := range team.Members {
		agent := cfg.Agents[memberName]
		runRecord.AgentOutputs[index] = model.AgentOutput{
			AgentName:     memberName,
			Provider:      agent.Provider,
			Model:         agent.Model,
			Role:          agent.Role,
			Status:        "pending",
			Round:         0,
			SequenceIndex: index,
		}
	}

	if err := repo.Save(runRecord); err != nil {
		return nil, err
	}

	providerSet, err := instantiateProviders(cfg)
	if err != nil {
		return failRun(repo, runRecord, err)
	}

	outputs := runRecord.AgentOutputs
	var mu sync.Mutex

	var wg sync.WaitGroup
	for index, memberName := range team.Members {
		wg.Add(1)

		go func(index int, memberName string) {
			defer wg.Done()

			agent := cfg.Agents[memberName]
			notify(observer, Event{
				Type:      EventAgentStarted,
				RunID:     runRecord.ID,
				AgentName: memberName,
				Provider:  agent.Provider,
				Model:     agent.Model,
			})

			mu.Lock()
			outputs[index].Status = "running"
			runRecord.AgentOutputs = outputs
			_ = repo.Save(runRecord)
			mu.Unlock()

			start := time.Now()
			result, err := providerSet[agent.Provider].Generate(ctx, providers.GenerateRequest{
				RunID:        runRecord.ID,
				AgentName:    memberName,
				Model:        agent.Model,
				SystemPrompt: agent.SystemPrompt,
				UserPrompt:   prompt,
				Settings:     agent.Settings,
			})
			err = normalizeProviderError(ctx, err)

			outputs[index] = model.AgentOutput{
				AgentName:     memberName,
				Provider:      agent.Provider,
				Model:         agent.Model,
				Role:          agent.Role,
				Status:        "completed",
				Content:       result.Content,
				RawStdout:     result.RawStdout,
				RawStderr:     result.RawStderr,
				FinishReason:  result.FinishReason,
				PromptTokens:  result.PromptTokens,
				OutputTokens:  result.OutputTokens,
				DurationMs:    time.Since(start).Milliseconds(),
				Round:         0,
				SequenceIndex: index,
			}

			if err != nil {
				outputs[index].Error = err.Error()
				outputs[index].Status = "failed"
			}

			mu.Lock()
			runRecord.AgentOutputs = outputs
			_ = repo.Save(runRecord)
			mu.Unlock()

			if err != nil {
				notify(observer, Event{
					Type:      EventAgentFailed,
					RunID:     runRecord.ID,
					AgentName: memberName,
					Provider:  agent.Provider,
					Model:     agent.Model,
					Duration:  time.Since(start),
					Err:       err,
				})
				return
			}

			notify(observer, Event{
				Type:      EventAgentCompleted,
				RunID:     runRecord.ID,
				AgentName: memberName,
				Provider:  agent.Provider,
				Model:     agent.Model,
				Duration:  time.Since(start),
			})
		}(index, memberName)
	}

	wg.Wait()
	runRecord.AgentOutputs = outputs

	if err := repo.Save(runRecord); err != nil {
		return nil, err
	}

	if err := executionContextError(ctx); err != nil {
		return failRun(repo, runRecord, err)
	}

	memberErrors := collectOutputErrors(outputs)
	if len(memberErrors) > 0 {
		return failRun(repo, runRecord, fmt.Errorf("agent round failed:\n- %s", strings.Join(memberErrors, "\n- ")))
	}

	synthesizer := cfg.Agents[plan.Synthesizer]
	notify(observer, Event{
		Type:      EventSynthesisStarted,
		RunID:     runRecord.ID,
		AgentName: plan.Synthesizer,
		Provider:  synthesizer.Provider,
		Model:     synthesizer.Model,
	})
	runRecord.Synthesis = &model.AgentOutput{
		AgentName:     plan.Synthesizer,
		Provider:      synthesizer.Provider,
		Model:         synthesizer.Model,
		Role:          synthesizer.Role,
		Status:        "running",
		Round:         1,
		SequenceIndex: len(outputs),
	}
	if err := repo.Save(runRecord); err != nil {
		return nil, err
	}

	runRecord.Items = ExtractItems(outputs)
	if err := repo.Save(runRecord); err != nil {
		return nil, err
	}

	synthesisStart := time.Now()
	synthesisResult, err := providerSet[synthesizer.Provider].Generate(ctx, providers.GenerateRequest{
		RunID:        runRecord.ID,
		AgentName:    plan.Synthesizer,
		Model:        synthesizer.Model,
		SystemPrompt: synthesizer.SystemPrompt,
		UserPrompt:   buildSynthesisPrompt(prompt, outputs, runRecord.Items),
		Settings:     synthesizer.Settings,
	})
	err = normalizeProviderError(ctx, err)

	runRecord.Synthesis = &model.AgentOutput{
		AgentName:     plan.Synthesizer,
		Provider:      synthesizer.Provider,
		Model:         synthesizer.Model,
		Role:          synthesizer.Role,
		Status:        "completed",
		Content:       synthesisResult.Content,
		RawStdout:     synthesisResult.RawStdout,
		RawStderr:     synthesisResult.RawStderr,
		FinishReason:  synthesisResult.FinishReason,
		PromptTokens:  synthesisResult.PromptTokens,
		OutputTokens:  synthesisResult.OutputTokens,
		DurationMs:    time.Since(synthesisStart).Milliseconds(),
		Round:         1,
		SequenceIndex: len(outputs),
	}

	if err != nil {
		runRecord.Synthesis.Status = "failed"
		runRecord.Synthesis.Error = err.Error()
		if ctxErr := executionContextError(ctx); ctxErr != nil {
			return failRun(repo, runRecord, ctxErr)
		}

		return failRun(repo, runRecord, fmt.Errorf("synthesis failed: %w", err))
	}

	notify(observer, Event{
		Type:      EventSynthesisComplete,
		RunID:     runRecord.ID,
		AgentName: plan.Synthesizer,
		Provider:  synthesizer.Provider,
		Model:     synthesizer.Model,
		Duration:  time.Since(synthesisStart),
	})

	runRecord.FinalAnswer = synthesisResult.Content
	completedAt := time.Now().UTC()
	runRecord.CompletedAt = &completedAt
	runRecord.Status = "completed"

	if err := repo.Save(runRecord); err != nil {
		return nil, err
	}

	return runRecord, nil
}

func instantiateProviders(cfg *model.Config) (map[string]providers.Provider, error) {
	providerSet := make(map[string]providers.Provider, len(cfg.Providers))
	for name, providerConfig := range cfg.Providers {
		provider, err := providers.New(providerConfig)
		if err != nil {
			return nil, fmt.Errorf("initialize provider %q: %w", name, err)
		}

		providerSet[name] = provider
	}

	return providerSet, nil
}

func collectOutputErrors(outputs []model.AgentOutput) []string {
	problems := make([]string, 0)
	for _, output := range outputs {
		if output.Error == "" {
			continue
		}

		problems = append(problems, fmt.Sprintf("%s: %s", output.AgentName, output.Error))
	}

	return problems
}

func failRun(repo *storage.Repository, record *model.RunRecord, err error) (*model.RunRecord, error) {
	record.Status = "failed"
	record.Error = err.Error()
	completedAt := time.Now().UTC()
	record.CompletedAt = &completedAt

	if saveErr := repo.Save(record); saveErr != nil {
		return nil, fmt.Errorf("%v; additionally failed to persist run: %w", err, saveErr)
	}

	return record, err
}

func newRunID() string {
	randomBytes := make([]byte, 4)
	if _, err := crand.Read(randomBytes); err != nil {
		return time.Now().UTC().Format("20060102T150405Z")
	}

	return fmt.Sprintf("%s-%x", time.Now().UTC().Format("20060102T150405Z"), randomBytes)
}

func notify(observer Observer, event Event) {
	if observer == nil {
		return
	}

	observer(event)
}

func normalizeProviderError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	if ctxErr := executionContextError(ctx); ctxErr != nil {
		return ctxErr
	}

	return err
}

func executionContextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}

	switch ctx.Err() {
	case nil:
		return nil
	case context.DeadlineExceeded:
		return fmt.Errorf("run timed out: %w", ctx.Err())
	case context.Canceled:
		return fmt.Errorf("run canceled: %w", ctx.Err())
	default:
		return ctx.Err()
	}
}
