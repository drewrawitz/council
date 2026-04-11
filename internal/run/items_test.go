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
		nil,
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

func TestBuildRoundPromptUsesNormalizedItemsWithoutOtherRawOutputs(t *testing.T) {
	t.Parallel()

	prompt := buildRoundPrompt(
		"Review this plan",
		nil,
		1,
		"analyst",
		&model.AgentOutput{AgentName: "analyst", Content: "My previous answer"},
		[]model.Item{
			{
				ID:           "item-001",
				Type:         model.ItemTypeQuestion,
				Content:      "what should happen on partial failure?",
				SourceAgents: []string{"skeptic"},
				Status:       model.ItemStatusOpen,
			},
		},
		2,
	)

	if !strings.Contains(prompt, "Critique/revise round 2") {
		t.Fatalf("prompt %q did not include round header", prompt)
	}

	if !strings.Contains(prompt, "Shared normalized items from the prior round:") {
		t.Fatalf("prompt %q did not include normalized items section", prompt)
	}

	if !strings.Contains(prompt, "Focus items for critique:") {
		t.Fatalf("prompt %q did not include critique focus section", prompt)
	}

	if !strings.Contains(prompt, "Your previous answer:\nMy previous answer") {
		t.Fatalf("prompt %q did not include own previous answer", prompt)
	}

	if strings.Contains(prompt, "Raw agent outputs") {
		t.Fatalf("prompt %q should not include raw agent outputs", prompt)
	}
}

func TestBuildTaskContextIncludesArtifactMetadataAndContent(t *testing.T) {
	t.Parallel()

	context := buildTaskContext("Review this plan", []model.Artifact{{
		Path:        "/tmp/brief.md",
		SHA256:      "abc123",
		Size:        17,
		ContentType: "text/markdown; charset=utf-8",
		Content:     "Artifact contents",
		Truncated:   true,
	}})

	if !strings.Contains(context, "Attached local artifacts:") {
		t.Fatalf("context %q did not include artifact section", context)
	}

	if !strings.Contains(context, "/tmp/brief.md") {
		t.Fatalf("context %q did not include artifact path", context)
	}

	if !strings.Contains(context, "content truncated: true") {
		t.Fatalf("context %q did not include truncation marker", context)
	}

	if !strings.Contains(context, "Artifact contents") {
		t.Fatalf("context %q did not include artifact content", context)
	}
}

func TestExtractItemsSkipsRoundProtocolScaffolding(t *testing.T) {
	t.Parallel()

	items := ExtractItems([]model.AgentOutput{{
		AgentName: "analyst",
		Content: strings.Join([]string{
			"Critique/revise round 2.",
			"Your previous answer:",
			"Review this plan.",
			"Instructions:",
			"1. Critique weak assumptions, gaps, and edge cases relevant to your role.",
			"2. Revise your answer using the normalized items above.",
			"3. Return one revised Markdown answer, not a transcript of the protocol.",
		}, "\n"),
	}})

	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}

	if items[0].Content != "Review this plan" {
		t.Fatalf("item content = %q, want only the actual answer content", items[0].Content)
	}
}

func TestExtractItemsStripsSourceAnnotationsFromFormattedItems(t *testing.T) {
	t.Parallel()

	items := ExtractItems([]model.AgentOutput{{
		AgentName: "analyst",
		Content:   "- Review this plan [sources: analyst, skeptic]",
	}})

	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}

	if items[0].Content != "Review this plan" {
		t.Fatalf("item content = %q, want source suffix removed", items[0].Content)
	}
}
