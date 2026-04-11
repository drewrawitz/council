package run

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"council/internal/model"
	"council/internal/storage"
)

func TestExecutePersistsCompletedRunWithMockProvider(t *testing.T) {
	t.Parallel()

	repo := storage.NewRepository(filepath.Join(t.TempDir(), "runs"))
	record, err := Execute(context.Background(), repo, validConfig(), "default", "Review this plan", nil, 1, RetentionOptions{}, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if record.Status != "completed" {
		t.Fatalf("Status = %q, want completed", record.Status)
	}

	if record.MaxRounds != 1 || record.CompletedRounds != 1 {
		t.Fatalf("round metadata = %d/%d, want 1/1", record.CompletedRounds, record.MaxRounds)
	}

	if record.StopReason != model.StopReasonMaxRounds {
		t.Fatalf("StopReason = %q, want %q", record.StopReason, model.StopReasonMaxRounds)
	}

	if record.CompletedAt == nil {
		t.Fatal("CompletedAt is nil, want completion timestamp")
	}

	if len(record.AgentOutputs) != 0 {
		t.Fatalf("len(AgentOutputs) = %d, want 0 by default", len(record.AgentOutputs))
	}

	if len(record.Items) == 0 {
		t.Fatal("len(Items) = 0, want extracted items")
	}

	if record.Synthesis != nil {
		t.Fatalf("Synthesis = %#v, want nil by default", record.Synthesis)
	}

	if record.FinalAnswer == "" {
		t.Fatal("FinalAnswer is empty")
	}

	if !strings.Contains(record.FinalAnswer, "Normalized items:") {
		t.Fatalf("FinalAnswer %q did not include normalized items section", record.FinalAnswer)
	}

	loaded, err := repo.Load(record.ID)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if loaded.Status != "completed" {
		t.Fatalf("loaded Status = %q, want completed", loaded.Status)
	}

	if loaded.FinalAnswer != record.FinalAnswer {
		t.Fatalf("loaded FinalAnswer = %q, want %q", loaded.FinalAnswer, record.FinalAnswer)
	}

	if len(loaded.Items) != len(record.Items) {
		t.Fatalf("len(loaded.Items) = %d, want %d", len(loaded.Items), len(record.Items))
	}

	if len(loaded.RoundSummaries) != 1 {
		t.Fatalf("len(loaded.RoundSummaries) = %d, want 1", len(loaded.RoundSummaries))
	}

	if len(loaded.AgentOutputs) != 0 {
		t.Fatalf("len(loaded.AgentOutputs) = %d, want 0 by default", len(loaded.AgentOutputs))
	}

	if loaded.Synthesis != nil {
		t.Fatalf("loaded Synthesis = %#v, want nil by default", loaded.Synthesis)
	}
}

func TestExecutePersistsArtifactsAndInjectsThemIntoPrompts(t *testing.T) {
	t.Parallel()

	repo := storage.NewRepository(filepath.Join(t.TempDir(), "runs"))
	artifacts := []model.Artifact{{
		Path:        "/tmp/brief.md",
		SHA256:      "abc123",
		Size:        14,
		ContentType: "text/markdown; charset=utf-8",
		Content:     "Artifact body",
	}}

	record, err := Execute(context.Background(), repo, validConfig(), "default", "Review this plan", artifacts, 1, fullRetention(), nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if len(record.Artifacts) != 1 {
		t.Fatalf("len(Artifacts) = %d, want 1", len(record.Artifacts))
	}

	if record.Artifacts[0].Path != "/tmp/brief.md" {
		t.Fatalf("artifact path = %q, want /tmp/brief.md", record.Artifacts[0].Path)
	}

	if !strings.Contains(record.FinalAnswer, "Attached local artifacts:") {
		t.Fatalf("FinalAnswer %q did not include artifact section", record.FinalAnswer)
	}

	if !strings.Contains(record.FinalAnswer, "Artifact body") {
		t.Fatalf("FinalAnswer %q did not include artifact content", record.FinalAnswer)
	}

	loaded, loadErr := repo.Load(record.ID)
	if loadErr != nil {
		t.Fatalf("Load returned error: %v", loadErr)
	}

	if len(loaded.Artifacts) != 1 {
		t.Fatalf("len(loaded.Artifacts) = %d, want 1", len(loaded.Artifacts))
	}
}

func TestExecuteOmitsArtifactContentByDefault(t *testing.T) {
	t.Parallel()

	repo := storage.NewRepository(filepath.Join(t.TempDir(), "runs"))
	artifacts := []model.Artifact{{
		Path:        "/tmp/brief.md",
		SHA256:      "abc123",
		Size:        14,
		ContentType: "text/markdown; charset=utf-8",
		Content:     "Artifact body",
	}}

	record, err := Execute(context.Background(), repo, validConfig(), "default", "Review this plan", artifacts, 1, RetentionOptions{}, nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if len(record.Artifacts) != 1 {
		t.Fatalf("len(Artifacts) = %d, want 1", len(record.Artifacts))
	}

	if record.Artifacts[0].Content != "" {
		t.Fatalf("artifact content = %q, want omitted by default", record.Artifacts[0].Content)
	}

	if !record.Artifacts[0].ContentOmitted {
		t.Fatal("artifact ContentOmitted = false, want true")
	}
}

func TestExecuteRunsCritiqueReviseRoundWhenMaxRoundsIsTwo(t *testing.T) {
	t.Parallel()

	repo := storage.NewRepository(filepath.Join(t.TempDir(), "runs"))
	record, err := Execute(context.Background(), repo, validConfig(), "default", "Review this plan", nil, 2, fullRetention(), nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if record.Status != "completed" {
		t.Fatalf("Status = %q, want completed", record.Status)
	}

	if record.MaxRounds != 2 || record.CompletedRounds != 2 {
		t.Fatalf("round metadata = %d/%d, want 2/2", record.CompletedRounds, record.MaxRounds)
	}

	if record.StopReason != model.StopReasonConverged {
		t.Fatalf("StopReason = %q, want %q", record.StopReason, model.StopReasonConverged)
	}

	if len(record.AgentOutputs) != 4 {
		t.Fatalf("len(AgentOutputs) = %d, want 4", len(record.AgentOutputs))
	}

	if len(record.RoundSummaries) != 2 {
		t.Fatalf("len(RoundSummaries) = %d, want 2", len(record.RoundSummaries))
	}

	if record.RoundSummaries[0].ItemHash != record.RoundSummaries[1].ItemHash {
		t.Fatalf("round summary hashes = %q and %q, want convergence", record.RoundSummaries[0].ItemHash, record.RoundSummaries[1].ItemHash)
	}

	if record.AgentOutputs[2].Round != 1 || record.AgentOutputs[3].Round != 1 {
		t.Fatalf("second-round outputs have rounds %d and %d, want 1 and 1", record.AgentOutputs[2].Round, record.AgentOutputs[3].Round)
	}

	if !strings.Contains(record.AgentOutputs[2].Content, "Critique/revise round 2") {
		t.Fatalf("round 2 content %q did not include critique/revise prompt", record.AgentOutputs[2].Content)
	}

	if record.Synthesis == nil || record.Synthesis.Round != 2 {
		t.Fatalf("synthesis = %#v, want round 2 synthesis", record.Synthesis)
	}
}

func TestExecuteStopsEarlyWhenItemsConvergeBeforeHardCap(t *testing.T) {
	t.Parallel()

	repo := storage.NewRepository(filepath.Join(t.TempDir(), "runs"))
	record, err := Execute(context.Background(), repo, validConfig(), "default", "Review this plan", nil, 3, fullRetention(), nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if record.Status != "completed" {
		t.Fatalf("Status = %q, want completed", record.Status)
	}

	if record.CompletedRounds != 2 {
		t.Fatalf("CompletedRounds = %d, want 2", record.CompletedRounds)
	}

	if record.StopReason != model.StopReasonConverged {
		t.Fatalf("StopReason = %q, want %q", record.StopReason, model.StopReasonConverged)
	}

	if len(record.AgentOutputs) != 4 {
		t.Fatalf("len(AgentOutputs) = %d, want 4", len(record.AgentOutputs))
	}

	if len(record.RoundSummaries) != 2 {
		t.Fatalf("len(RoundSummaries) = %d, want 2", len(record.RoundSummaries))
	}

	if record.Synthesis == nil || record.Synthesis.Round != 2 {
		t.Fatalf("synthesis = %#v, want synthesis after round 2", record.Synthesis)
	}

	loaded, loadErr := repo.Load(record.ID)
	if loadErr != nil {
		t.Fatalf("Load returned error: %v", loadErr)
	}

	if loaded.StopReason != model.StopReasonConverged {
		t.Fatalf("loaded StopReason = %q, want %q", loaded.StopReason, model.StopReasonConverged)
	}
}

func TestExecutePersistsFailedRunWhenAgentInvocationFails(t *testing.T) {
	t.Parallel()

	cfg := &model.Config{
		Version: model.ConfigVersion,
		Providers: map[string]model.ProviderConfig{
			"broken": {
				Type:    model.ProviderTypeSubprocess,
				Command: "/definitely/not/a/real/command",
			},
		},
		Agents: map[string]model.AgentConfig{
			"broken-agent": {
				Provider:     "broken",
				Model:        "fake-model",
				Role:         "analyst",
				SystemPrompt: "Try to answer the task.",
			},
		},
		Teams: map[string]model.TeamConfig{
			"default": {
				Members:     []string{"broken-agent"},
				Synthesizer: "broken-agent",
				Protocol:    "single-round",
			},
		},
		Protocols: map[string]model.ProtocolConfig{
			"single-round": {Kind: model.ProtocolKindSingleRound},
		},
	}

	repo := storage.NewRepository(filepath.Join(t.TempDir(), "runs"))
	record, err := Execute(context.Background(), repo, cfg, "default", "Review this plan", nil, 1, RetentionOptions{}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error for broken subprocess provider")
	}

	if record == nil {
		t.Fatal("Execute returned nil record for failed run")
	}

	if record.Status != "failed" {
		t.Fatalf("Status = %q, want failed", record.Status)
	}

	if record.StopReason != model.StopReasonFailed {
		t.Fatalf("StopReason = %q, want %q", record.StopReason, model.StopReasonFailed)
	}

	if record.CompletedAt == nil {
		t.Fatal("CompletedAt is nil, want failure timestamp")
	}

	if record.Synthesis != nil {
		t.Fatalf("Synthesis = %#v, want nil when member round fails", record.Synthesis)
	}

	if len(record.AgentOutputs) != 0 {
		t.Fatalf("len(AgentOutputs) = %d, want 0 by default", len(record.AgentOutputs))
	}

	if !strings.Contains(record.Error, "agent round 1 failed") {
		t.Fatalf("run error %q did not mention failed agent round", record.Error)
	}

	loaded, loadErr := repo.Load(record.ID)
	if loadErr != nil {
		t.Fatalf("Load returned error: %v", loadErr)
	}

	if loaded.Status != "failed" {
		t.Fatalf("loaded Status = %q, want failed", loaded.Status)
	}

	if loaded.Error != record.Error {
		t.Fatalf("loaded Error = %q, want %q", loaded.Error, record.Error)
	}
}

func TestExecutePersistsFailedRunWhenContextTimesOut(t *testing.T) {
	t.Parallel()

	cfg := &model.Config{
		Version: model.ConfigVersion,
		Providers: map[string]model.ProviderConfig{
			"slow": {
				Type:    model.ProviderTypeSubprocess,
				Command: "/bin/sh",
				Args:    []string{"-c", "sleep 2; printf delayed"},
			},
		},
		Agents: map[string]model.AgentConfig{
			"slow-agent": {
				Provider:     "slow",
				Model:        "fake-model",
				Role:         "analyst",
				SystemPrompt: "Try to answer the task.",
			},
		},
		Teams: map[string]model.TeamConfig{
			"default": {
				Members:     []string{"slow-agent"},
				Synthesizer: "slow-agent",
				Protocol:    "single-round",
			},
		},
		Protocols: map[string]model.ProtocolConfig{
			"single-round": {Kind: model.ProtocolKindSingleRound},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	repo := storage.NewRepository(filepath.Join(t.TempDir(), "runs"))
	record, err := Execute(ctx, repo, cfg, "default", "Review this plan", nil, 1, RetentionOptions{}, nil)
	if err == nil {
		t.Fatal("Execute returned nil error for timed out run")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline exceeded", err)
	}

	if record == nil {
		t.Fatal("Execute returned nil record for timed out run")
	}

	if record.Status != "failed" {
		t.Fatalf("Status = %q, want failed", record.Status)
	}

	if record.StopReason != model.StopReasonTimedOut {
		t.Fatalf("StopReason = %q, want %q", record.StopReason, model.StopReasonTimedOut)
	}

	if record.CompletedAt == nil {
		t.Fatal("CompletedAt is nil, want failure timestamp")
	}

	if !strings.Contains(record.Error, "run timed out") {
		t.Fatalf("run error %q did not mention timeout", record.Error)
	}

	if len(record.AgentOutputs) != 0 {
		t.Fatalf("len(AgentOutputs) = %d, want 0 by default", len(record.AgentOutputs))
	}

	loaded, loadErr := repo.Load(record.ID)
	if loadErr != nil {
		t.Fatalf("Load returned error: %v", loadErr)
	}

	if loaded.Status != "failed" {
		t.Fatalf("loaded Status = %q, want failed", loaded.Status)
	}

	if !strings.Contains(loaded.Error, "run timed out") {
		t.Fatalf("loaded Error = %q, want timeout error", loaded.Error)
	}
}

func TestExecuteRetainsAgentOutputsWithoutRawProviderIOWhenRequested(t *testing.T) {
	t.Parallel()

	repo := storage.NewRepository(filepath.Join(t.TempDir(), "runs"))
	record, err := Execute(
		context.Background(),
		repo,
		validConfig(),
		"default",
		"Review this plan",
		nil,
		1,
		RetentionOptions{RetainAgentOutputs: true},
		nil,
	)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if len(record.AgentOutputs) != 2 {
		t.Fatalf("len(AgentOutputs) = %d, want 2", len(record.AgentOutputs))
	}

	if record.AgentOutputs[0].Content == "" {
		t.Fatal("agent output content is empty")
	}

	if record.AgentOutputs[0].RawStdout != "" || record.AgentOutputs[0].RawStderr != "" {
		t.Fatalf("raw provider IO = %q / %q, want omitted by default", record.AgentOutputs[0].RawStdout, record.AgentOutputs[0].RawStderr)
	}

	if record.Synthesis == nil {
		t.Fatal("Synthesis is nil, want retained synthesis metadata")
	}
}
