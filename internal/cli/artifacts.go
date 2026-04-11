package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"unicode/utf8"

	"council/internal/model"
)

const maxArtifactContentBytes = 16 * 1024

func loadArtifacts(args *askArgs) ([]model.Artifact, error) {
	if args == nil || len(args.filePaths) == 0 {
		return nil, nil
	}

	artifacts := make([]model.Artifact, 0, len(args.filePaths))
	seenPaths := map[string]struct{}{}
	for _, filePath := range args.filePaths {
		artifact, err := loadArtifact(filePath)
		if err != nil {
			return nil, err
		}

		if _, seen := seenPaths[artifact.Path]; seen {
			continue
		}

		seenPaths[artifact.Path] = struct{}{}
		artifacts = append(artifacts, artifact)
	}

	return artifacts, nil
}

func loadArtifact(filePath string) (model.Artifact, error) {
	resolvedPath, err := filepath.Abs(filePath)
	if err != nil {
		return model.Artifact{}, fmt.Errorf("resolve artifact path %q: %w", filePath, err)
	}

	fileInfo, err := os.Stat(resolvedPath)
	if err != nil {
		return model.Artifact{}, fmt.Errorf("read artifact %q: %w", resolvedPath, err)
	}

	if fileInfo.IsDir() {
		return model.Artifact{}, fmt.Errorf("artifact %q must be a file", resolvedPath)
	}

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return model.Artifact{}, fmt.Errorf("read artifact %q: %w", resolvedPath, err)
	}

	if !isTextArtifact(data) {
		return model.Artifact{}, fmt.Errorf("artifact %q must be UTF-8 text", resolvedPath)
	}

	content := data
	truncated := false
	if len(content) > maxArtifactContentBytes {
		content = truncateUTF8(content, maxArtifactContentBytes)
		truncated = true
	}

	hash := sha256.Sum256(data)
	contentType := http.DetectContentType(data)
	if len(data) == 0 {
		contentType = "text/plain; charset=utf-8"
	}

	return model.Artifact{
		Path:        filepath.Clean(resolvedPath),
		SHA256:      hex.EncodeToString(hash[:]),
		Size:        fileInfo.Size(),
		ContentType: contentType,
		Content:     string(content),
		Truncated:   truncated,
	}, nil
}

func isTextArtifact(data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}

	for _, b := range data {
		if b == 0 {
			return false
		}
	}

	return true
}

func truncateUTF8(data []byte, limit int) []byte {
	if len(data) <= limit {
		return data
	}

	truncated := data[:limit]
	for len(truncated) > 0 && !utf8.Valid(truncated) {
		truncated = truncated[:len(truncated)-1]
	}

	return truncated
}
