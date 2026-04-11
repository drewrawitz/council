package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"council/internal/model"
)

func TestValidateAllowsSameModelWithDifferentRoles(t *testing.T) {
	t.Parallel()

	cfg := validConfig()

	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate returned error for valid config: %v", err)
	}
}

func TestValidateAllowsDedicatedSynthesizerOutsideMembers(t *testing.T) {
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

	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate returned error for dedicated synthesizer: %v", err)
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

func TestValidateRejectsSubprocessProviderWithoutPromptDelivery(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Providers["mock"] = model.ProviderConfig{
		Type:    model.ProviderTypeSubprocess,
		Command: "/bin/sh",
		Args:    []string{"-c", "cat"},
		Stdin:   model.SubprocessStdinNone,
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate returned nil error for subprocess provider without prompt delivery")
	}

	if !strings.Contains(err.Error(), "must deliver the user prompt") {
		t.Fatalf("Validate error %q did not mention user prompt delivery", err)
	}

	if !strings.Contains(err.Error(), "must deliver the system prompt") {
		t.Fatalf("Validate error %q did not mention system prompt delivery", err)
	}
}

func TestValidateAcceptsSubprocessProviderWithArgumentPlaceholders(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Providers["mock"] = model.ProviderConfig{
		Type:    model.ProviderTypeSubprocess,
		Command: "claude",
		Args: []string{
			"--print",
			"--model",
			"{model}",
			"--system-prompt",
			"{system_prompt}",
			"{prompt}",
		},
		Stdin: model.SubprocessStdinNone,
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate returned error for args-delivered subprocess prompt: %v", err)
	}
}

func TestValidateRejectsInvalidTeamRunDefaults(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	zero := 0
	falseValue := false
	cfg.Teams["default"] = model.TeamConfig{
		Members:     []string{"analyst", "skeptic"},
		Synthesizer: "analyst",
		Protocol:    "single-round",
		Run: model.RunConfig{
			MaxRounds:           &zero,
			MaxTime:             "soon",
			RetainAgentOutputs:  &falseValue,
			RetainRawProviderIO: boolPtr(true),
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate returned nil error for invalid team run defaults")
	}

	if !strings.Contains(err.Error(), "run.max_rounds") {
		t.Fatalf("Validate error %q did not mention invalid max_rounds", err)
	}

	if !strings.Contains(err.Error(), "run.max_time") {
		t.Fatalf("Validate error %q did not mention invalid max_time", err)
	}

	if !strings.Contains(err.Error(), "retain_raw_provider_io requires retain_agent_outputs") {
		t.Fatalf("Validate error %q did not mention retention dependency", err)
	}
}

func TestResolveTeamRunConfigAppliesTeamDefaults(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	three := 3
	cfg.Teams["default"] = model.TeamConfig{
		Members:     []string{"analyst", "skeptic"},
		Synthesizer: "analyst",
		Protocol:    "single-round",
		Run: model.RunConfig{
			MaxRounds:             &three,
			MaxTime:               "2m30s",
			RetainRawProviderIO:   boolPtr(true),
			RetainArtifactContent: boolPtr(true),
		},
	}

	resolved, err := ResolveTeamRunConfig(cfg, "default")
	if err != nil {
		t.Fatalf("ResolveTeamRunConfig returned error: %v", err)
	}

	if resolved.MaxRounds != 3 {
		t.Fatalf("MaxRounds = %d, want 3", resolved.MaxRounds)
	}

	if resolved.MaxTime != 150*time.Second {
		t.Fatalf("MaxTime = %s, want 2m30s", resolved.MaxTime)
	}

	if !resolved.RetainAgentOutputs || !resolved.RetainRawProviderIO || !resolved.RetainArtifactContent {
		t.Fatalf("resolved retention = %#v, want all true", resolved)
	}
}

func TestResolveTeamRunConfigDefaultsToSingleRoundMinimalRetention(t *testing.T) {
	t.Parallel()

	resolved, err := ResolveTeamRunConfig(validConfig(), "default")
	if err != nil {
		t.Fatalf("ResolveTeamRunConfig returned error: %v", err)
	}

	if resolved.MaxRounds != 1 {
		t.Fatalf("MaxRounds = %d, want 1", resolved.MaxRounds)
	}

	if resolved.MaxTime != 0 {
		t.Fatalf("MaxTime = %s, want 0", resolved.MaxTime)
	}

	if resolved.RetainAgentOutputs || resolved.RetainRawProviderIO || resolved.RetainArtifactContent {
		t.Fatalf("resolved retention = %#v, want all false", resolved)
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

func boolPtr(value bool) *bool {
	return &value
}
