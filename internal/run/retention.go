package run

import (
	"council/internal/model"
	"council/internal/storage"
)

type RetentionOptions struct {
	RetainAgentOutputs    bool
	RetainRawProviderIO   bool
	RetainArtifactContent bool
}

func sanitizeRunRecord(record *model.RunRecord, options RetentionOptions) *model.RunRecord {
	if record == nil {
		return nil
	}

	clone := *record
	clone.Artifacts = sanitizeArtifacts(record.Artifacts, options)
	clone.Items = append([]model.Item(nil), record.Items...)
	clone.RoundSummaries = append([]model.RoundSummary(nil), record.RoundSummaries...)

	if options.RetainAgentOutputs {
		clone.AgentOutputs = sanitizeAgentOutputs(record.AgentOutputs, options)
		if record.Synthesis != nil {
			synthesis := sanitizeAgentOutput(*record.Synthesis, options)
			clone.Synthesis = &synthesis
		}
	} else {
		clone.AgentOutputs = nil
		clone.Synthesis = nil
	}

	return &clone
}

func sanitizeArtifacts(artifacts []model.Artifact, options RetentionOptions) []model.Artifact {
	if len(artifacts) == 0 {
		return nil
	}

	sanitized := make([]model.Artifact, len(artifacts))
	for index, artifact := range artifacts {
		sanitized[index] = artifact
		if !options.RetainArtifactContent {
			sanitized[index].Content = ""
			sanitized[index].ContentOmitted = true
		}
	}

	return sanitized
}

func sanitizeAgentOutputs(outputs []model.AgentOutput, options RetentionOptions) []model.AgentOutput {
	if len(outputs) == 0 {
		return nil
	}

	sanitized := make([]model.AgentOutput, len(outputs))
	for index, output := range outputs {
		sanitized[index] = sanitizeAgentOutput(output, options)
	}

	return sanitized
}

func sanitizeAgentOutput(output model.AgentOutput, options RetentionOptions) model.AgentOutput {
	sanitized := output
	if !options.RetainRawProviderIO {
		sanitized.RawStdout = ""
		sanitized.RawStderr = ""
	}

	return sanitized
}

func persistRun(repo *storage.Repository, record *model.RunRecord, options RetentionOptions) error {
	return repo.Save(sanitizeRunRecord(record, options))
}
