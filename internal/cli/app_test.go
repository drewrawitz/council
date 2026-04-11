package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"council/internal/model"
)

func TestParseAskArgsSupportsPromptFile(t *testing.T) {
	t.Parallel()

	parsed, err := parseAskArgs([]string{"--team", "default", "--prompt-file", "prompt.md"})
	if err != nil {
		t.Fatalf("parseAskArgs returned error: %v", err)
	}

	if parsed.teamName != "default" {
		t.Fatalf("teamName = %q, want default", parsed.teamName)
	}

	if parsed.promptFile != "prompt.md" {
		t.Fatalf("promptFile = %q, want prompt.md", parsed.promptFile)
	}

	if parsed.readStdin {
		t.Fatal("readStdin = true, want false")
	}
}

func TestParseAskArgsRejectsMultiplePromptSources(t *testing.T) {
	t.Parallel()

	_, err := parseAskArgs([]string{"hello", "--team", "default", "--stdin"})
	if err == nil {
		t.Fatal("parseAskArgs returned nil error for multiple prompt sources")
	}

	if err.Error() != "ask accepts only one prompt source at a time" {
		t.Fatalf("error = %q, want multiple prompt sources message", err)
	}
}

func TestLoadPromptFromFile(t *testing.T) {
	t.Parallel()

	promptPath := filepath.Join(t.TempDir(), "prompt.md")
	if err := os.WriteFile(promptPath, []byte("\nReview this plan\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	prompt, err := loadPrompt(&askArgs{promptFile: promptPath})
	if err != nil {
		t.Fatalf("loadPrompt returned error: %v", err)
	}

	if prompt != "Review this plan" {
		t.Fatalf("prompt = %q, want trimmed file contents", prompt)
	}
}

func TestParseAskArgsSupportsMaxTime(t *testing.T) {
	t.Parallel()

	parsed, err := parseAskArgs([]string{"hello", "--team", "default", "--max-time", "45s"})
	if err != nil {
		t.Fatalf("parseAskArgs returned error: %v", err)
	}

	if parsed.maxTime != 45*time.Second {
		t.Fatalf("maxTime = %s, want 45s", parsed.maxTime)
	}
}

func TestParseAskArgsRejectsInvalidMaxTime(t *testing.T) {
	t.Parallel()

	_, err := parseAskArgs([]string{"hello", "--team", "default", "--max-time", "soon"})
	if err == nil {
		t.Fatal("parseAskArgs returned nil error for invalid max time")
	}

	if err.Error() != "invalid --max-time \"soon\": time: invalid duration \"soon\"" {
		t.Fatalf("error = %q, want invalid max time message", err)
	}
}

func TestParseAskArgsSupportsMaxRounds(t *testing.T) {
	t.Parallel()

	parsed, err := parseAskArgs([]string{"hello", "--team", "default", "--max-rounds", "2"})
	if err != nil {
		t.Fatalf("parseAskArgs returned error: %v", err)
	}

	if parsed.maxRounds != 2 {
		t.Fatalf("maxRounds = %d, want 2", parsed.maxRounds)
	}
}

func TestParseAskArgsRejectsInvalidMaxRounds(t *testing.T) {
	t.Parallel()

	_, err := parseAskArgs([]string{"hello", "--team", "default", "--max-rounds", "0"})
	if err == nil {
		t.Fatal("parseAskArgs returned nil error for invalid max rounds")
	}

	if err.Error() != "--max-rounds must be greater than 0" {
		t.Fatalf("error = %q, want invalid max rounds message", err)
	}
}

func TestParseAskArgsSupportsRetentionFlags(t *testing.T) {
	t.Parallel()

	parsed, err := parseAskArgs([]string{"hello", "--team", "default", "--retain-raw-provider-io", "--retain-artifact-content"})
	if err != nil {
		t.Fatalf("parseAskArgs returned error: %v", err)
	}

	if !parsed.retainAgentOutputs || !parsed.retainRawProviderIO || !parsed.retainArtifactContent {
		t.Fatalf("retention flags = %#v, want all true", parsed)
	}
}

func TestResolveRunSettingsUsesTeamDefaults(t *testing.T) {
	t.Parallel()

	three := 3
	config := &model.Config{
		Teams: map[string]model.TeamConfig{
			"default": {
				Run: model.RunConfig{
					MaxRounds:             &three,
					MaxTime:               "2m",
					RetainAgentOutputs:    boolPtr(true),
					RetainArtifactContent: boolPtr(true),
				},
			},
		},
	}

	resolved, err := resolveRunSettings(config, &askArgs{teamName: "default"})
	if err != nil {
		t.Fatalf("resolveRunSettings returned error: %v", err)
	}

	if resolved.MaxRounds != 3 {
		t.Fatalf("MaxRounds = %d, want 3", resolved.MaxRounds)
	}

	if resolved.MaxTime != 2*time.Minute {
		t.Fatalf("MaxTime = %s, want 2m", resolved.MaxTime)
	}

	if !resolved.RetainAgentOutputs || !resolved.RetainArtifactContent {
		t.Fatalf("resolved = %#v, want configured retention defaults", resolved)
	}
}

func TestResolveRunSettingsAllowsCliOverrides(t *testing.T) {
	t.Parallel()

	five := 5
	config := &model.Config{
		Teams: map[string]model.TeamConfig{
			"default": {
				Run: model.RunConfig{
					MaxRounds:           &five,
					MaxTime:             "2m",
					RetainAgentOutputs:  boolPtr(true),
					RetainRawProviderIO: boolPtr(true),
				},
			},
		},
	}

	resolved, err := resolveRunSettings(config, &askArgs{
		teamName:                 "default",
		maxRounds:                2,
		maxRoundsSet:             true,
		maxTime:                  30 * time.Second,
		maxTimeSet:               true,
		retainArtifactContent:    true,
		retainArtifactContentSet: true,
	})
	if err != nil {
		t.Fatalf("resolveRunSettings returned error: %v", err)
	}

	if resolved.MaxRounds != 2 {
		t.Fatalf("MaxRounds = %d, want 2", resolved.MaxRounds)
	}

	if resolved.MaxTime != 30*time.Second {
		t.Fatalf("MaxTime = %s, want 30s", resolved.MaxTime)
	}

	if !resolved.RetainAgentOutputs || !resolved.RetainRawProviderIO {
		t.Fatalf("resolved = %#v, want team retention defaults preserved", resolved)
	}

	if !resolved.RetainArtifactContent {
		t.Fatalf("resolved = %#v, want CLI artifact retention override", resolved)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
