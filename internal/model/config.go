package model

const (
	ConfigVersion           = 1
	ProviderTypeMock        = "mock"
	ProviderTypeSubprocess  = "subprocess"
	ProtocolKindSingleRound = "single_round"
	SubprocessStdinCombined = "combined_prompt"
	SubprocessStdinPrompt   = "prompt"
	SubprocessStdinNone     = "none"
)

type Config struct {
	Version   int                       `yaml:"version" json:"version"`
	Providers map[string]ProviderConfig `yaml:"providers" json:"providers"`
	Agents    map[string]AgentConfig    `yaml:"agents" json:"agents"`
	Teams     map[string]TeamConfig     `yaml:"teams" json:"teams"`
	Protocols map[string]ProtocolConfig `yaml:"protocols" json:"protocols"`
}

type ProviderConfig struct {
	Type    string   `yaml:"type" json:"type"`
	Command string   `yaml:"command,omitempty" json:"command,omitempty"`
	Args    []string `yaml:"args,omitempty" json:"args,omitempty"`
	Stdin   string   `yaml:"stdin,omitempty" json:"stdin,omitempty"`
}

type AgentConfig struct {
	Provider     string             `yaml:"provider" json:"provider"`
	Model        string             `yaml:"model" json:"model"`
	Role         string             `yaml:"role" json:"role"`
	SystemPrompt string             `yaml:"system_prompt" json:"system_prompt"`
	Settings     GenerationSettings `yaml:"settings,omitempty" json:"settings,omitempty"`
}

type GenerationSettings struct {
	Temperature     *float32 `yaml:"temperature,omitempty" json:"temperature,omitempty"`
	MaxOutputTokens *int     `yaml:"max_output_tokens,omitempty" json:"max_output_tokens,omitempty"`
}

type TeamConfig struct {
	Members     []string `yaml:"members" json:"members"`
	Synthesizer string   `yaml:"synthesizer" json:"synthesizer"`
	Protocol    string   `yaml:"protocol" json:"protocol"`
}

type ProtocolConfig struct {
	Kind string `yaml:"kind" json:"kind"`
}
