package model

import "time"

type RunRecord struct {
	ID              string        `json:"id"`
	Team            string        `json:"team"`
	Protocol        string        `json:"protocol"`
	Status          string        `json:"status"`
	MaxRounds       int           `json:"max_rounds"`
	CompletedRounds int           `json:"completed_rounds"`
	Prompt          string        `json:"prompt"`
	StartedAt       time.Time     `json:"started_at"`
	CompletedAt     *time.Time    `json:"completed_at,omitempty"`
	AgentOutputs    []AgentOutput `json:"agent_outputs,omitempty"`
	Items           []Item        `json:"items,omitempty"`
	Synthesis       *AgentOutput  `json:"synthesis,omitempty"`
	FinalAnswer     string        `json:"final_answer,omitempty"`
	Error           string        `json:"error,omitempty"`
}

const (
	ItemTypeClaim          = "claim"
	ItemTypeRisk           = "risk"
	ItemTypeRecommendation = "recommendation"
	ItemTypeQuestion       = "question"
	ItemStatusOpen         = "open"
)

type Item struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	Content      string   `json:"content"`
	SourceAgents []string `json:"source_agents,omitempty"`
	Status       string   `json:"status"`
}

type AgentOutput struct {
	AgentName     string `json:"agent_name"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	Role          string `json:"role"`
	Status        string `json:"status,omitempty"`
	Content       string `json:"content"`
	RawStdout     string `json:"raw_stdout,omitempty"`
	RawStderr     string `json:"raw_stderr,omitempty"`
	FinishReason  string `json:"finish_reason,omitempty"`
	PromptTokens  int    `json:"prompt_tokens,omitempty"`
	OutputTokens  int    `json:"output_tokens,omitempty"`
	DurationMs    int64  `json:"duration_ms,omitempty"`
	Error         string `json:"error,omitempty"`
	Round         int    `json:"round"`
	SequenceIndex int    `json:"sequence_index"`
}
