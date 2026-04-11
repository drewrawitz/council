package providers

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type SubprocessProvider struct {
	command string
	args    []string
}

func NewSubprocessProvider(command string, args []string) *SubprocessProvider {
	return &SubprocessProvider{
		command: command,
		args:    append([]string(nil), args...),
	}
}

func (p *SubprocessProvider) Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	commandArgs := make([]string, 0, len(p.args))
	for _, arg := range p.args {
		commandArgs = append(commandArgs, strings.ReplaceAll(arg, "{model}", req.Model))
	}

	cmd := exec.CommandContext(ctx, p.command, commandArgs...)
	cmd.Stdin = strings.NewReader(renderPrompt(req.SystemPrompt, req.UserPrompt))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := GenerateResult{
		Content:      strings.TrimSpace(stdout.String()),
		RawStdout:    stdout.String(),
		RawStderr:    stderr.String(),
		FinishReason: "stop",
	}

	if err != nil {
		return result, fmt.Errorf("run %q: %w", p.command, err)
	}

	if result.Content == "" {
		return result, fmt.Errorf("run %q returned empty stdout", p.command)
	}

	return result, nil
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
