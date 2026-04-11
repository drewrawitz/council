package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"council/internal/model"
)

type Provider interface {
	Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error)
}

type Factory func(cfg model.ProviderConfig) (Provider, error)

type GenerateRequest struct {
	RunID        string
	AgentName    string
	Model        string
	SystemPrompt string
	UserPrompt   string
	Settings     model.GenerationSettings
}

type GenerateResult struct {
	Content      string
	Raw          json.RawMessage
	RawStdout    string
	RawStderr    string
	PromptTokens int
	OutputTokens int
	FinishReason string
}

var supportedTypes = []string{
	model.ProviderTypeMock,
	model.ProviderTypeSubprocess,
}

func IsSupportedType(providerType string) bool {
	return slices.Contains(supportedTypes, providerType)
}

func SupportedTypes() []string {
	return slices.Clone(supportedTypes)
}

func New(cfg model.ProviderConfig) (Provider, error) {
	switch cfg.Type {
	case model.ProviderTypeMock:
		return MockProvider{}, nil
	case model.ProviderTypeSubprocess:
		if strings.TrimSpace(cfg.Command) == "" {
			return nil, fmt.Errorf("subprocess provider requires command")
		}

		return NewSubprocessProvider(cfg.Command, cfg.Args, cfg.Stdin), nil
	default:
		return nil, fmt.Errorf("unsupported provider type %q", cfg.Type)
	}
}

type MockProvider struct{}

func (MockProvider) Generate(_ context.Context, req GenerateRequest) (GenerateResult, error) {
	content := fmt.Sprintf("mock response from %s (%s)\n\n%s", req.AgentName, req.Model, strings.TrimSpace(req.UserPrompt))

	return GenerateResult{
		Content:      content,
		RawStdout:    content,
		FinishReason: "stop",
	}, nil
}
