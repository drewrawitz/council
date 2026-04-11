package providers

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"council/internal/model"
)

type SubprocessProvider struct {
	command   string
	args      []string
	stdinMode string
}

func NewSubprocessProvider(command string, args []string, stdinMode string) *SubprocessProvider {
	if strings.TrimSpace(stdinMode) == "" {
		stdinMode = model.SubprocessStdinCombined
	}

	return &SubprocessProvider{
		command:   command,
		args:      append([]string(nil), args...),
		stdinMode: stdinMode,
	}
}

func (p *SubprocessProvider) Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	templateValues := map[string]string{
		"{agent_name}":      req.AgentName,
		"{combined_prompt}": renderPrompt(req.SystemPrompt, req.UserPrompt),
		"{model}":           req.Model,
		"{prompt}":          strings.TrimSpace(req.UserPrompt),
		"{run_id}":          req.RunID,
		"{system_prompt}":   strings.TrimSpace(req.SystemPrompt),
	}

	outputFilePath, err := p.prepareOutputFilePath()
	if err != nil {
		return GenerateResult{}, err
	}
	if outputFilePath != "" {
		defer os.Remove(outputFilePath)
		templateValues["{output_file}"] = outputFilePath
	}

	commandArgs := make([]string, 0, len(p.args))
	for _, arg := range p.args {
		commandArgs = append(commandArgs, expandTemplate(arg, templateValues))
	}

	cmd := exec.CommandContext(ctx, p.command, commandArgs...)
	stdinPayload, err := p.buildStdinPayload(req)
	if err != nil {
		return GenerateResult{}, err
	}
	if stdinPayload != "" {
		cmd.Stdin = strings.NewReader(stdinPayload)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	result := GenerateResult{
		Content:      strings.TrimSpace(stdout.String()),
		RawStdout:    stdout.String(),
		RawStderr:    stderr.String(),
		FinishReason: "stop",
	}

	if err != nil {
		return result, fmt.Errorf("run %q: %w", p.command, err)
	}

	if outputFilePath != "" {
		content, readErr := os.ReadFile(outputFilePath)
		if readErr != nil {
			return result, fmt.Errorf("read output file %q: %w", outputFilePath, readErr)
		}

		result.Content = strings.TrimSpace(string(content))
	}

	if result.Content == "" {
		if outputFilePath != "" {
			return result, fmt.Errorf("run %q returned empty output file and stdout", p.command)
		}

		return result, fmt.Errorf("run %q returned empty stdout", p.command)
	}

	return result, nil
}

func (p *SubprocessProvider) buildStdinPayload(req GenerateRequest) (string, error) {
	switch p.stdinMode {
	case model.SubprocessStdinCombined:
		return renderPrompt(req.SystemPrompt, req.UserPrompt), nil
	case model.SubprocessStdinPrompt:
		return strings.TrimSpace(req.UserPrompt), nil
	case model.SubprocessStdinNone:
		return "", nil
	default:
		return "", fmt.Errorf("unsupported subprocess stdin mode %q", p.stdinMode)
	}
}

func (p *SubprocessProvider) prepareOutputFilePath() (string, error) {
	for _, arg := range p.args {
		if !strings.Contains(arg, "{output_file}") {
			continue
		}

		file, err := os.CreateTemp("", "council-provider-output-*.txt")
		if err != nil {
			return "", fmt.Errorf("create temp output file: %w", err)
		}

		path := file.Name()
		if err := file.Close(); err != nil {
			return "", fmt.Errorf("close temp output file %q: %w", path, err)
		}

		return filepath.Clean(path), nil
	}

	return "", nil
}

func expandTemplate(value string, templateValues map[string]string) string {
	replacements := make([]string, 0, len(templateValues)*2)
	for placeholder, replacement := range templateValues {
		replacements = append(replacements, placeholder, replacement)
	}

	return strings.NewReplacer(replacements...).Replace(value)
}

func renderPrompt(systemPrompt string, userPrompt string) string {
	var prompt strings.Builder

	if strings.TrimSpace(systemPrompt) != "" {
		prompt.WriteString("System instructions:\n")
		prompt.WriteString(strings.TrimSpace(systemPrompt))
		prompt.WriteString("\n\n")
	}

	prompt.WriteString("User task:\n")
	prompt.WriteString(strings.TrimSpace(userPrompt))
	prompt.WriteString("\n")

	return prompt.String()
}
