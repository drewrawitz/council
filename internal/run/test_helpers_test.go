package run

import "council/internal/model"

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

func fullRetention() RetentionOptions {
	return RetentionOptions{
		RetainAgentOutputs:    true,
		RetainRawProviderIO:   true,
		RetainArtifactContent: true,
	}
}
