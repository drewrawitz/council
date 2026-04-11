package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"council/internal/model"
)

func TestValidateAllowsSameModelWithDifferentRoles(t *testing.T) {
	t.Parallel()

	cfg := validConfig()

	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate returned error for valid config: %v", err)
	}
}

func TestValidateRejectsSynthesizerOutsideMembers(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Agents["orphan"] = model.AgentConfig{
		Provider:     "mock",
		Model:        "mock-v1",
		Role:         "synthesizer",
		SystemPrompt: "Synthesize the answer.",
	}
	cfg.Teams["default"] = model.TeamConfig{
		Members:     []string{"analyst", "skeptic"},
		Synthesizer: "orphan",
		Protocol:    "single-round",
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate returned nil error for invalid synthesizer")
	}

	if !strings.Contains(err.Error(), "must be included in members") {
		t.Fatalf("Validate error %q did not mention missing synthesizer membership", err)
	}
}

func TestLoadReadsExplicitPath(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "council.yaml")
	configYAML := strings.TrimSpace(`
version: 1

providers:
  mock:
    type: mock

agents:
  analyst:
    provider: mock
    model: mock-v1
    role: analyst
    system_prompt: |
      You are a concise analyst.

teams:
  default:
    members: [analyst]
    synthesizer: analyst
    protocol: single-round

protocols:
  single-round:
    kind: single_round
`) + "\n"

	if err := os.WriteFile(configPath, []byte(configYAML), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if loaded.Path != configPath {
		t.Fatalf("Load path = %q, want %q", loaded.Path, configPath)
	}

	if loaded.Config.Agents["analyst"].Model != "mock-v1" {
		t.Fatalf("Load model = %q, want mock-v1", loaded.Config.Agents["analyst"].Model)
	}

	if loaded.Config.Teams["default"].Synthesizer != "analyst" {
		t.Fatalf("Load synthesizer = %q, want analyst", loaded.Config.Teams["default"].Synthesizer)
	}
}

func validConfig() *model.Config {
	return &model.Config{
		Version: model.ConfigVersion,
		Providers: map[string]model.ProviderConfig{
			"mock": {Type: model.ProviderTypeMock},
		},
		Agents: map[string]model.AgentConfig{
			"analyst": {
				Provider:     "mock",
				Model:        "mock-v1",
				Role:         "analyst",
				SystemPrompt: "You are a concise analyst.",
			},
			"skeptic": {
				Provider:     "mock",
				Model:        "mock-v1",
				Role:         "skeptic",
				SystemPrompt: "Challenge assumptions and surface risks.",
			},
		},
		Teams: map[string]model.TeamConfig{
			"default": {
				Members:     []string{"analyst", "skeptic"},
				Synthesizer: "analyst",
				Protocol:    "single-round",
			},
		},
		Protocols: map[string]model.ProtocolConfig{
			"single-round": {Kind: model.ProtocolKindSingleRound},
		},
	}
}
