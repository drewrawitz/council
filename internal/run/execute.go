package run

import (
	"context"
	crand "crypto/rand"
	"errors"
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
	EventRunStopped        EventType = "run_stopped"
	EventAgentStarted      EventType = "agent_started"
	EventAgentCompleted    EventType = "agent_completed"
	EventAgentFailed       EventType = "agent_failed"
	EventSynthesisStarted  EventType = "synthesis_started"
	EventSynthesisComplete EventType = "synthesis_completed"
)

type Event struct {
	Type       EventType
	RunID      string
	Round      int
	StopReason string
	AgentName  string
	Provider   string
	Model      string
	Duration   time.Duration
	Err        error
}

type Observer func(Event)

func Execute(ctx context.Context, repo *storage.Repository, cfg *model.Config, teamName string, prompt string, maxRounds int, observer Observer) (*model.RunRecord, error) {
	if maxRounds <= 0 {
		maxRounds = 1
	}

	plan, err := BuildPlan(cfg, teamName)
	if err != nil {
		return nil, err
	}

	runRecord := &model.RunRecord{
		ID:        newRunID(),
		Team:      teamName,
		Protocol:  plan.ProtocolName,
		Status:    "running",
		MaxRounds: maxRounds,
		Prompt:    prompt,
		StartedAt: time.Now().UTC(),
	}

	notify(observer, Event{Type: EventRunStarted, RunID: runRecord.ID})

	if err := repo.Save(runRecord); err != nil {
		return nil, err
	}

	providerSet, err := instantiateProviders(cfg)
	if err != nil {
		return failRun(repo, runRecord, err)
	}

	var latestOutputs []model.AgentOutput
	for round := 0; round < maxRounds; round++ {
		latestOutputs, err = executeRound(ctx, repo, runRecord, cfg, teamName, prompt, round, latestOutputs, runRecord.Items, providerSet, observer)
		if err != nil {
			return failRun(repo, runRecord, err)
		}

		runRecord.CompletedRounds = round + 1
		runRecord.Items = ExtractItems(latestOutputs)
		roundSummary := buildRoundSummary(round, runRecord.Items)
		runRecord.RoundSummaries = append(runRecord.RoundSummaries, roundSummary)
		if err := repo.Save(runRecord); err != nil {
			return nil, err
		}

		if round > 0 && roundSummary.ItemHash == runRecord.RoundSummaries[len(runRecord.RoundSummaries)-2].ItemHash {
			runRecord.StopReason = model.StopReasonConverged
			if err := repo.Save(runRecord); err != nil {
				return nil, err
			}

			notify(observer, Event{
				Type:       EventRunStopped,
				RunID:      runRecord.ID,
				Round:      round,
				StopReason: runRecord.StopReason,
			})
			break
		}
	}

	if runRecord.StopReason == "" {
		runRecord.StopReason = model.StopReasonMaxRounds
	}

	synthesizer := cfg.Agents[plan.Synthesizer]
	notify(observer, Event{
		Type:      EventSynthesisStarted,
		RunID:     runRecord.ID,
		Round:     runRecord.CompletedRounds,
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
		Round:         runRecord.CompletedRounds,
		SequenceIndex: len(latestOutputs),
	}
	if err := repo.Save(runRecord); err != nil {
		return nil, err
	}

	synthesisStart := time.Now()
	synthesisResult, err := providerSet[synthesizer.Provider].Generate(ctx, providers.GenerateRequest{
		RunID:        runRecord.ID,
		AgentName:    plan.Synthesizer,
		Model:        synthesizer.Model,
		SystemPrompt: synthesizer.SystemPrompt,
		UserPrompt:   buildSynthesisPrompt(prompt, latestOutputs, runRecord.Items),
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
		Round:         runRecord.CompletedRounds,
		SequenceIndex: len(latestOutputs),
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
		Round:     runRecord.CompletedRounds,
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

func executeRound(
	ctx context.Context,
	repo *storage.Repository,
	runRecord *model.RunRecord,
	cfg *model.Config,
	teamName string,
	originalPrompt string,
	round int,
	previousOutputs []model.AgentOutput,
	items []model.Item,
	providerSet map[string]providers.Provider,
	observer Observer,
) ([]model.AgentOutput, error) {
	team := cfg.Teams[teamName]
	startIndex := len(runRecord.AgentOutputs)
	roundOutputs := make([]model.AgentOutput, len(team.Members))
	for index, memberName := range team.Members {
		agent := cfg.Agents[memberName]
		roundOutputs[index] = model.AgentOutput{
			AgentName:     memberName,
			Provider:      agent.Provider,
			Model:         agent.Model,
			Role:          agent.Role,
			Status:        "pending",
			Round:         round,
			SequenceIndex: index,
		}
	}

	runRecord.AgentOutputs = append(runRecord.AgentOutputs, roundOutputs...)
	if err := repo.Save(runRecord); err != nil {
		return nil, err
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for index, memberName := range team.Members {
		wg.Add(1)

		go func(index int, memberName string) {
			defer wg.Done()

			agent := cfg.Agents[memberName]
			outputIndex := startIndex + index
			userPrompt := buildRoundPrompt(originalPrompt, round, memberName, findAgentOutput(previousOutputs, memberName), items, len(team.Members))

			notify(observer, Event{
				Type:      EventAgentStarted,
				RunID:     runRecord.ID,
				Round:     round,
				AgentName: memberName,
				Provider:  agent.Provider,
				Model:     agent.Model,
			})

			mu.Lock()
			runRecord.AgentOutputs[outputIndex].Status = "running"
			_ = repo.Save(runRecord)
			mu.Unlock()

			start := time.Now()
			result, err := providerSet[agent.Provider].Generate(ctx, providers.GenerateRequest{
				RunID:        runRecord.ID,
				AgentName:    memberName,
				Model:        agent.Model,
				SystemPrompt: agent.SystemPrompt,
				UserPrompt:   userPrompt,
				Settings:     agent.Settings,
			})
			err = normalizeProviderError(ctx, err)

			output := model.AgentOutput{
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
				Round:         round,
				SequenceIndex: index,
			}

			if err != nil {
				output.Error = err.Error()
				output.Status = "failed"
			}

			mu.Lock()
			runRecord.AgentOutputs[outputIndex] = output
			_ = repo.Save(runRecord)
			mu.Unlock()

			if err != nil {
				notify(observer, Event{
					Type:      EventAgentFailed,
					RunID:     runRecord.ID,
					Round:     round,
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
				Round:     round,
				AgentName: memberName,
				Provider:  agent.Provider,
				Model:     agent.Model,
				Duration:  time.Since(start),
			})
		}(index, memberName)
	}

	wg.Wait()

	if err := executionContextError(ctx); err != nil {
		return nil, err
	}

	completedRoundOutputs := append([]model.AgentOutput(nil), runRecord.AgentOutputs[startIndex:startIndex+len(team.Members)]...)
	if memberErrors := collectOutputErrors(completedRoundOutputs); len(memberErrors) > 0 {
		return nil, fmt.Errorf("agent round %d failed:\n- %s", round+1, strings.Join(memberErrors, "\n- "))
	}

	return completedRoundOutputs, nil
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
	record.StopReason = stopReasonForError(err)
	completedAt := time.Now().UTC()
	record.CompletedAt = &completedAt

	if saveErr := repo.Save(record); saveErr != nil {
		return nil, fmt.Errorf("%v; additionally failed to persist run: %w", err, saveErr)
	}

	return record, err
}

func stopReasonForError(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return model.StopReasonTimedOut
	case errors.Is(err, context.Canceled):
		return model.StopReasonCanceled
	default:
		return model.StopReasonFailed
	}
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
