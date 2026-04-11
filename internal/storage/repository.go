package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"council/internal/model"
)

type Repository struct {
	runsDir string
}

func DefaultRunsDir() (string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}

	return filepath.Join(workingDir, ".council", "runs"), nil
}

func NewRepository(runsDir string) *Repository {
	return &Repository{runsDir: runsDir}
}

func (r *Repository) Save(record *model.RunRecord) error {
	if record == nil {
		return fmt.Errorf("run record is required")
	}

	runDir := filepath.Join(r.runsDir, record.ID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return fmt.Errorf("create run directory %q: %w", runDir, err)
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run record %q: %w", record.ID, err)
	}

	runPath := filepath.Join(runDir, "run.json")
	if err := os.WriteFile(runPath, data, 0o644); err != nil {
		return fmt.Errorf("write run file %q: %w", runPath, err)
	}

	return nil
}

func (r *Repository) Load(runID string) (*model.RunRecord, error) {
	runPath := filepath.Join(r.runsDir, runID, "run.json")
	data, err := os.ReadFile(runPath)
	if err != nil {
		return nil, fmt.Errorf("read run file %q: %w", runPath, err)
	}

	var record model.RunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("parse run file %q: %w", runPath, err)
	}

	return &record, nil
}
