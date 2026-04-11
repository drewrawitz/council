package run

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"council/internal/model"
	"council/internal/storage"
)

func TestExecutePersistsCompletedRunWithMockProvider(t *testing.T) {
	t.Parallel()

	repo := storage.NewRepository(filepath.Join(t.TempDir(), "runs"))
	record, err := Execute(context.Background(), repo, validConfig(), "default", "Review this plan", nil)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if record.Status != "completed" {
		t.Fatalf("Status = %q, want completed", record.Status)
	}

	if record.CompletedAt == nil {
		t.Fatal("CompletedAt is nil, want completion timestamp")
	}

	if len(record.AgentOutputs) != 2 {
		t.Fatalf("len(AgentOutputs) = %d, want 2", len(record.AgentOutputs))
	}

	if record.AgentOutputs[0].Status != "completed" || record.AgentOutputs[1].Status != "completed" {
		t.Fatalf("agent statuses = %q and %q, want completed", record.AgentOutputs[0].Status, record.AgentOutputs[1].Status)
	}

	if record.Synthesis == nil {
		t.Fatal("Synthesis is nil, want synthesis output")
	}

	if record.FinalAnswer == "" {
		t.Fatal("FinalAnswer is empty")
	}

	if !strings.Contains(record.FinalAnswer, "Original task:") {
		t.Fatalf("FinalAnswer %q did not include synthesized prompt content", record.FinalAnswer)
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

	if loaded.AgentOutputs[0].Round != 0 || loaded.Synthesis.Round != 1 {
		t.Fatalf("rounds = %d and %d, want 0 and 1", loaded.AgentOutputs[0].Round, loaded.Synthesis.Round)
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
	record, err := Execute(context.Background(), repo, cfg, "default", "Review this plan", nil)
	if err == nil {
		t.Fatal("Execute returned nil error for broken subprocess provider")
	}

	if record == nil {
		t.Fatal("Execute returned nil record for failed run")
	}

	if record.Status != "failed" {
		t.Fatalf("Status = %q, want failed", record.Status)
	}

	if record.CompletedAt == nil {
		t.Fatal("CompletedAt is nil, want failure timestamp")
	}

	if record.Synthesis != nil {
		t.Fatalf("Synthesis = %#v, want nil when member round fails", record.Synthesis)
	}

	if len(record.AgentOutputs) != 1 {
		t.Fatalf("len(AgentOutputs) = %d, want 1", len(record.AgentOutputs))
	}

	if record.AgentOutputs[0].Error == "" {
		t.Fatal("AgentOutputs[0].Error is empty")
	}

	if record.AgentOutputs[0].Status != "failed" {
		t.Fatalf("AgentOutputs[0].Status = %q, want failed", record.AgentOutputs[0].Status)
	}

	if !strings.Contains(record.Error, "agent round failed") {
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
