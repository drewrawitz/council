package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"council/internal/model"
	"council/internal/providers"
)

var defaultPaths = []string{
	"council.yaml",
	"council.yml",
	".council.yaml",
	".council.yml",
}

type LoadedConfig struct {
	Path   string
	Config *model.Config
}

func Load(path string) (*LoadedConfig, error) {
	resolvedPath, err := resolvePath(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", resolvedPath, err)
	}

	var cfg model.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", resolvedPath, err)
	}

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &LoadedConfig{Path: resolvedPath, Config: &cfg}, nil
}

func Validate(cfg *model.Config) error {
	if cfg == nil {
		return errors.New("config is required")
	}

	var problems []string

	if cfg.Version != model.ConfigVersion {
		problems = append(problems, fmt.Sprintf("version must be %d", model.ConfigVersion))
	}

	if len(cfg.Providers) == 0 {
		problems = append(problems, "providers must define at least one provider")
	}

	if len(cfg.Agents) == 0 {
		problems = append(problems, "agents must define at least one agent")
	}

	if len(cfg.Teams) == 0 {
		problems = append(problems, "teams must define at least one team")
	}

	if len(cfg.Protocols) == 0 {
		problems = append(problems, "protocols must define at least one protocol")
	}

	for name, provider := range cfg.Providers {
		providerPath := fmt.Sprintf("providers.%s", name)

		if provider.Type == "" {
			problems = append(problems, providerPath+".type is required")
			continue
		}

		if !providers.IsSupportedType(provider.Type) {
			problems = append(problems, fmt.Sprintf("%s.type %q is unsupported", providerPath, provider.Type))
			continue
		}

		if provider.Type == model.ProviderTypeSubprocess && strings.TrimSpace(provider.Command) == "" {
			problems = append(problems, providerPath+".command is required for subprocess providers")
		}

		if provider.Type == model.ProviderTypeSubprocess {
			stdinMode := provider.Stdin
			if strings.TrimSpace(stdinMode) == "" {
				stdinMode = model.SubprocessStdinCombined
			}

			switch stdinMode {
			case model.SubprocessStdinCombined, model.SubprocessStdinPrompt, model.SubprocessStdinNone:
			default:
				problems = append(problems, fmt.Sprintf("%s.stdin %q is unsupported", providerPath, provider.Stdin))
			}

			hasPromptPlaceholder := hasAnyPlaceholder(provider.Args, "{prompt}", "{combined_prompt}")
			hasSystemPlaceholder := hasAnyPlaceholder(provider.Args, "{system_prompt}", "{combined_prompt}")

			if stdinMode == model.SubprocessStdinNone && !hasPromptPlaceholder {
				problems = append(problems, providerPath+" must deliver the user prompt via stdin or args placeholders")
			}

			if stdinMode != model.SubprocessStdinCombined && !hasSystemPlaceholder {
				problems = append(problems, providerPath+" must deliver the system prompt via combined stdin or args placeholders")
			}
		}
	}

	for name, agent := range cfg.Agents {
		agentPath := fmt.Sprintf("agents.%s", name)

		if strings.TrimSpace(agent.Provider) == "" {
			problems = append(problems, agentPath+".provider is required")
		} else if _, ok := cfg.Providers[agent.Provider]; !ok {
			problems = append(problems, fmt.Sprintf("%s.provider %q does not exist", agentPath, agent.Provider))
		}

		if strings.TrimSpace(agent.Model) == "" {
			problems = append(problems, agentPath+".model is required")
		}

		if strings.TrimSpace(agent.Role) == "" {
			problems = append(problems, agentPath+".role is required")
		}

		if strings.TrimSpace(agent.SystemPrompt) == "" {
			problems = append(problems, agentPath+".system_prompt is required")
		}

		if agent.Settings.Temperature != nil {
			temperature := *agent.Settings.Temperature
			if temperature < 0 || temperature > 2 {
				problems = append(problems, fmt.Sprintf("%s.settings.temperature must be between 0 and 2", agentPath))
			}
		}

		if agent.Settings.MaxOutputTokens != nil && *agent.Settings.MaxOutputTokens <= 0 {
			problems = append(problems, fmt.Sprintf("%s.settings.max_output_tokens must be greater than 0", agentPath))
		}
	}

	for name, protocol := range cfg.Protocols {
		protocolPath := fmt.Sprintf("protocols.%s", name)

		if strings.TrimSpace(protocol.Kind) == "" {
			problems = append(problems, protocolPath+".kind is required")
			continue
		}

		if protocol.Kind != model.ProtocolKindSingleRound {
			problems = append(problems, fmt.Sprintf("%s.kind %q is unsupported", protocolPath, protocol.Kind))
		}
	}

	for name, team := range cfg.Teams {
		teamPath := fmt.Sprintf("teams.%s", name)

		if len(team.Members) == 0 {
			problems = append(problems, teamPath+".members must include at least one agent")
		}

		seenMembers := map[string]struct{}{}
		for _, member := range team.Members {
			if _, ok := cfg.Agents[member]; !ok {
				problems = append(problems, fmt.Sprintf("%s.members includes unknown agent %q", teamPath, member))
			}

			if _, seen := seenMembers[member]; seen {
				problems = append(problems, fmt.Sprintf("%s.members includes duplicate agent %q", teamPath, member))
				continue
			}

			seenMembers[member] = struct{}{}
		}

		if strings.TrimSpace(team.Synthesizer) == "" {
			problems = append(problems, teamPath+".synthesizer is required")
		} else {
			if _, ok := cfg.Agents[team.Synthesizer]; !ok {
				problems = append(problems, fmt.Sprintf("%s.synthesizer %q does not exist", teamPath, team.Synthesizer))
			}
		}

		if strings.TrimSpace(team.Protocol) == "" {
			problems = append(problems, teamPath+".protocol is required")
		} else if _, ok := cfg.Protocols[team.Protocol]; !ok {
			problems = append(problems, fmt.Sprintf("%s.protocol %q does not exist", teamPath, team.Protocol))
		}

		if team.Run.MaxRounds != nil && *team.Run.MaxRounds <= 0 {
			problems = append(problems, fmt.Sprintf("%s.run.max_rounds must be greater than 0", teamPath))
		}

		if strings.TrimSpace(team.Run.MaxTime) != "" {
			if _, err := time.ParseDuration(team.Run.MaxTime); err != nil {
				problems = append(problems, fmt.Sprintf("%s.run.max_time %q is invalid", teamPath, team.Run.MaxTime))
			}
		}

		if team.Run.RetainRawProviderIO != nil && *team.Run.RetainRawProviderIO {
			if team.Run.RetainAgentOutputs != nil && !*team.Run.RetainAgentOutputs {
				problems = append(problems, fmt.Sprintf("%s.run.retain_raw_provider_io requires retain_agent_outputs", teamPath))
			}
		}
	}

	if len(problems) > 0 {
		return errors.New("invalid config:\n- " + strings.Join(problems, "\n- "))
	}

	return nil
}

func ResolveTeamRunConfig(cfg *model.Config, teamName string) (model.ResolvedRunConfig, error) {
	resolved := model.ResolvedRunConfig{MaxRounds: 1}
	if cfg == nil {
		return resolved, errors.New("config is required")
	}

	team, ok := cfg.Teams[teamName]
	if !ok {
		return resolved, fmt.Errorf("team %q does not exist", teamName)
	}

	if team.Run.MaxRounds != nil {
		resolved.MaxRounds = *team.Run.MaxRounds
	}

	if strings.TrimSpace(team.Run.MaxTime) != "" {
		maxTime, err := time.ParseDuration(team.Run.MaxTime)
		if err != nil {
			return resolved, fmt.Errorf("team %q run.max_time %q is invalid: %w", teamName, team.Run.MaxTime, err)
		}

		resolved.MaxTime = maxTime
	}

	if team.Run.RetainAgentOutputs != nil {
		resolved.RetainAgentOutputs = *team.Run.RetainAgentOutputs
	}

	if team.Run.RetainRawProviderIO != nil {
		resolved.RetainRawProviderIO = *team.Run.RetainRawProviderIO
		if resolved.RetainRawProviderIO {
			resolved.RetainAgentOutputs = true
		}
	}

	if team.Run.RetainArtifactContent != nil {
		resolved.RetainArtifactContent = *team.Run.RetainArtifactContent
	}

	return resolved, nil
}

func resolvePath(path string) (string, error) {
	if strings.TrimSpace(path) != "" {
		resolvedPath, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("resolve config path %q: %w", path, err)
		}

		return resolvedPath, nil
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}

	for _, candidate := range defaultPaths {
		resolvedPath := filepath.Join(workingDir, candidate)
		if _, err := os.Stat(resolvedPath); err == nil {
			return resolvedPath, nil
		}
	}

	return "", fmt.Errorf("no config file found; looked for %s", strings.Join(defaultPaths, ", "))
}

func hasAnyPlaceholder(values []string, placeholders ...string) bool {
	for _, value := range values {
		for _, placeholder := range placeholders {
			if strings.Contains(value, placeholder) {
				return true
			}
		}
	}

	return false
}
