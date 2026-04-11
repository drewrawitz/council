package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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
