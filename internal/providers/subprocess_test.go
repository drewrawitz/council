package providers

import (
	"context"
	"strings"
	"testing"

	"council/internal/model"
)

func TestSubprocessProviderUsesCombinedPromptOnStdin(t *testing.T) {
	t.Parallel()

	provider := NewSubprocessProvider("/bin/sh", []string{"-c", "cat"}, model.SubprocessStdinCombined)
	result, err := provider.Generate(context.Background(), GenerateRequest{
		RunID:        "run-1",
		AgentName:    "analyst",
		Model:        "mock-v1",
		SystemPrompt: "Be concise.",
		UserPrompt:   "Review this plan.",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if !strings.Contains(result.Content, "System instructions:\nBe concise.") {
		t.Fatalf("Content %q did not include combined system prompt", result.Content)
	}

	if !strings.Contains(result.Content, "User task:\nReview this plan.") {
		t.Fatalf("Content %q did not include user prompt", result.Content)
	}
}

func TestSubprocessProviderExpandsArgsAndReadsOutputFile(t *testing.T) {
	t.Parallel()

	provider := NewSubprocessProvider(
		"/bin/sh",
		[]string{
			"-c",
			"printf '%s\n%s\n%s' \"$1\" \"$2\" \"$3\" > \"$4\"",
			"sh",
			"{model}",
			"{system_prompt}",
			"{prompt}",
			"{output_file}",
		},
		model.SubprocessStdinNone,
	)

	result, err := provider.Generate(context.Background(), GenerateRequest{
		RunID:        "run-1",
		AgentName:    "analyst",
		Model:        "claude-sonnet-4-6",
		SystemPrompt: "Be concise.",
		UserPrompt:   "Review this plan.",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if result.Content != "claude-sonnet-4-6\nBe concise.\nReview this plan." {
		t.Fatalf("Content = %q, want placeholders written through output file", result.Content)
	}
}
