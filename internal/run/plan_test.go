package run

import (
	"strings"
	"testing"
)

func TestBuildPlanIncludesSameModelAgentsSeparately(t *testing.T) {
	t.Parallel()

	plan, err := BuildPlan(validConfig(), "default")
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}

	if plan.TeamName != "default" {
		t.Fatalf("TeamName = %q, want default", plan.TeamName)
	}

	if plan.ProtocolKind != "single_round" {
		t.Fatalf("ProtocolKind = %q, want single_round", plan.ProtocolKind)
	}

	if plan.Synthesizer != "analyst" {
		t.Fatalf("Synthesizer = %q, want analyst", plan.Synthesizer)
	}

	if len(plan.Members) != 2 {
		t.Fatalf("len(Members) = %d, want 2", len(plan.Members))
	}

	if plan.DistinctModel != 1 {
		t.Fatalf("DistinctModel = %d, want 1", plan.DistinctModel)
	}

	if plan.Members[0].Name != "analyst" || plan.Members[1].Name != "skeptic" {
		t.Fatalf("member ordering = %#v, want analyst then skeptic", plan.Members)
	}

	if plan.Members[0].Role == plan.Members[1].Role {
		t.Fatalf("member roles should differ, got %q and %q", plan.Members[0].Role, plan.Members[1].Role)
	}
}

func TestBuildPlanErrorsForUnknownTeam(t *testing.T) {
	t.Parallel()

	_, err := BuildPlan(validConfig(), "missing")
	if err == nil {
		t.Fatal("BuildPlan returned nil error for unknown team")
	}

	if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("BuildPlan error %q did not mention missing team", err)
	}
}
