package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAskArgsSupportsRepeatedFiles(t *testing.T) {
	t.Parallel()

	parsed, err := parseAskArgs([]string{"hello", "--team", "default", "--file", "a.txt", "--file=b.txt"})
	if err != nil {
		t.Fatalf("parseAskArgs returned error: %v", err)
	}

	if len(parsed.filePaths) != 2 {
		t.Fatalf("len(filePaths) = %d, want 2", len(parsed.filePaths))
	}

	if parsed.filePaths[0] != "a.txt" || parsed.filePaths[1] != "b.txt" {
		t.Fatalf("filePaths = %#v, want a.txt and b.txt", parsed.filePaths)
	}
}

func TestLoadArtifactsLoadsTextFiles(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "brief.md")
	if err := os.WriteFile(filePath, []byte("hello artifact\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	artifacts, err := loadArtifacts(&askArgs{filePaths: []string{filePath}})
	if err != nil {
		t.Fatalf("loadArtifacts returned error: %v", err)
	}

	if len(artifacts) != 1 {
		t.Fatalf("len(artifacts) = %d, want 1", len(artifacts))
	}

	if artifacts[0].Path != filePath {
		t.Fatalf("artifact path = %q, want %q", artifacts[0].Path, filePath)
	}

	if artifacts[0].Content != "hello artifact\n" {
		t.Fatalf("artifact content = %q, want file contents", artifacts[0].Content)
	}

	if artifacts[0].SHA256 == "" {
		t.Fatal("artifact SHA256 is empty")
	}

	if artifacts[0].Truncated {
		t.Fatal("artifact should not be truncated")
	}
}

func TestLoadArtifactsRejectsMissingFile(t *testing.T) {
	t.Parallel()

	_, err := loadArtifacts(&askArgs{filePaths: []string{"/definitely/missing.txt"}})
	if err == nil {
		t.Fatal("loadArtifacts returned nil error for missing file")
	}

	if !strings.Contains(err.Error(), "read artifact") {
		t.Fatalf("error = %q, want artifact read failure", err)
	}
}

func TestLoadArtifactsTruncatesLargeFiles(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "large.txt")
	largeContent := strings.Repeat("abcdef", maxArtifactContentBytes/6+64)
	if err := os.WriteFile(filePath, []byte(largeContent), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	artifacts, err := loadArtifacts(&askArgs{filePaths: []string{filePath}})
	if err != nil {
		t.Fatalf("loadArtifacts returned error: %v", err)
	}

	if len(artifacts) != 1 {
		t.Fatalf("len(artifacts) = %d, want 1", len(artifacts))
	}

	if !artifacts[0].Truncated {
		t.Fatal("artifact should be truncated")
	}

	if len(artifacts[0].Content) > maxArtifactContentBytes {
		t.Fatalf("len(content) = %d, want <= %d", len(artifacts[0].Content), maxArtifactContentBytes)
	}
}
