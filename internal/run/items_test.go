package run

import (
	"strings"
	"testing"

	"council/internal/model"
)

func TestExtractItemsClassifiesAndDeduplicatesAcrossAgents(t *testing.T) {
	t.Parallel()

	outputs := []model.AgentOutput{
		{
			AgentName: "analyst",
			Content: strings.Join([]string{
				"- Risk: local CLI auth can expire during long runs.",
				"- Recommendation: add --max-time.",
				"- Question: what should happen on partial failure?",
				"- Claim: Council should remain local-first.",
			}, "\n"),
		},
		{
			AgentName: "skeptic",
			Content: strings.Join([]string{
				"- risk: local CLI auth can expire during long runs.",
				"- recommendation: add --max-time.",
				"- what should happen on partial failure?",
				"- council should remain local-first.",
			}, "\n"),
		},
	}

	items := ExtractItems(outputs)
	if len(items) != 4 {
		t.Fatalf("len(items) = %d, want 4", len(items))
	}

	for index, item := range items {
		if item.ID == "" {
			t.Fatalf("item %d has empty ID", index)
		}

		if item.Status != model.ItemStatusOpen {
			t.Fatalf("item %d status = %q, want open", index, item.Status)
		}

		if len(item.SourceAgents) != 2 {
			t.Fatalf("item %d source agents = %#v, want analyst and skeptic", index, item.SourceAgents)
		}
	}

	if items[0].Type != model.ItemTypeRisk || items[0].Content != "local CLI auth can expire during long runs" {
		t.Fatalf("first item = %#v, want risk about CLI auth", items[0])
	}

	if items[1].Type != model.ItemTypeRecommendation || items[1].Content != "add --max-time" {
		t.Fatalf("second item = %#v, want recommendation about max-time", items[1])
	}

	if items[2].Type != model.ItemTypeQuestion || items[2].Content != "what should happen on partial failure?" {
		t.Fatalf("third item = %#v, want partial failure question", items[2])
	}

	if items[3].Type != model.ItemTypeClaim || items[3].Content != "Council should remain local-first" {
		t.Fatalf("fourth item = %#v, want local-first claim", items[3])
	}
}

func TestBuildSynthesisPromptPrefersNormalizedItemsWithRawFallback(t *testing.T) {
	t.Parallel()

	prompt := buildSynthesisPrompt(
		"Review this plan",
		[]model.AgentOutput{{AgentName: "analyst", Content: "Raw output"}},
		[]model.Item{{
			ID:           "item-001",
			Type:         model.ItemTypeRisk,
			Content:      "local CLI auth can expire during long runs",
			SourceAgents: []string{"analyst"},
			Status:       model.ItemStatusOpen,
		}},
	)

	if !strings.Contains(prompt, "Normalized items:") {
		t.Fatalf("prompt %q did not include normalized items section", prompt)
	}

	if !strings.Contains(prompt, "## Risks") {
		t.Fatalf("prompt %q did not include risk group", prompt)
	}

	if !strings.Contains(prompt, "Use the normalized items first") {
		t.Fatalf("prompt %q did not instruct synthesis to prefer items", prompt)
	}

	if !strings.Contains(prompt, "Raw agent outputs (fallback context):") {
		t.Fatalf("prompt %q did not include raw fallback section", prompt)
	}
}
