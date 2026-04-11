package run

import (
	"fmt"

	"council/internal/model"
)

type PlannedAgent struct {
	Name         string `json:"name"`
	ProviderName string `json:"provider_name"`
	ProviderType string `json:"provider_type"`
	Model        string `json:"model"`
	Role         string `json:"role"`
}

type Plan struct {
	TeamName      string         `json:"team_name"`
	ProtocolName  string         `json:"protocol_name"`
	ProtocolKind  string         `json:"protocol_kind"`
	Synthesizer   string         `json:"synthesizer"`
	Members       []PlannedAgent `json:"members"`
	DistinctModel int            `json:"distinct_model_count"`
}

func BuildPlan(cfg *model.Config, teamName string) (*Plan, error) {
	team, ok := cfg.Teams[teamName]
	if !ok {
		return nil, fmt.Errorf("team %q does not exist", teamName)
	}

	protocol := cfg.Protocols[team.Protocol]
	plan := &Plan{
		TeamName:     teamName,
		ProtocolName: team.Protocol,
		ProtocolKind: protocol.Kind,
		Synthesizer:  team.Synthesizer,
		Members:      make([]PlannedAgent, 0, len(team.Members)),
	}

	models := map[string]struct{}{}
	for _, memberName := range team.Members {
		agent := cfg.Agents[memberName]
		provider := cfg.Providers[agent.Provider]

		plan.Members = append(plan.Members, PlannedAgent{
			Name:         memberName,
			ProviderName: agent.Provider,
			ProviderType: provider.Type,
			Model:        agent.Model,
			Role:         agent.Role,
		})

		models[agent.Model] = struct{}{}
	}

	plan.DistinctModel = len(models)

	return plan, nil
}
