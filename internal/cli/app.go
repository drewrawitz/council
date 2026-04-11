package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"council/internal/config"
	"council/internal/model"
	"council/internal/run"
	"council/internal/storage"
)

func Run(args []string) int {
	if len(args) == 0 {
		printRootUsage(os.Stderr)
		return 1
	}

	switch args[0] {
	case "config":
		return runConfig(args[1:])
	case "ask":
		return runAsk(args[1:])
	case "plan":
		return runPlan(args[1:])
	case "show":
		return runShow(args[1:])
	case "help", "--help", "-h":
		printRootUsage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		printRootUsage(os.Stderr)
		return 1
	}
}

func runAsk(args []string) int {
	parsed, err := parseAskArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		printAskUsage(os.Stderr)
		return 1
	}

	loaded, err := config.Load(parsed.configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	runsDir, err := storage.DefaultRunsDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	repo := storage.NewRepository(runsDir)
	prompt, err := loadPrompt(parsed)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	artifacts, err := loadArtifacts(parsed)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	retention := run.RetentionOptions{
		RetainAgentOutputs:    parsed.retainAgentOutputs,
		RetainRawProviderIO:   parsed.retainRawProviderIO,
		RetainArtifactContent: parsed.retainArtifactContent,
	}

	executionContext := context.Background()
	cancel := func() {}
	if parsed.maxTime > 0 {
		executionContext, cancel = context.WithTimeout(executionContext, parsed.maxTime)
	}
	defer cancel()

	record, err := run.Execute(executionContext, repo, loaded.Config, parsed.teamName, prompt, artifacts, parsed.maxRounds, retention, printRunEvent)
	if err != nil {
		if parsed.maxTime > 0 && errors.Is(err, context.DeadlineExceeded) {
			err = fmt.Errorf("run exceeded max time %s: %w", parsed.maxTime, err)
		}

		if record != nil {
			fmt.Fprintf(os.Stderr, "run failed: %s\nrun id: %s\n", err, record.ID)
		} else {
			fmt.Fprintf(os.Stderr, "run failed: %s\n", err)
		}
		return 1
	}

	if parsed.jsonOutput {
		return writeJSON(record)
	}

	if record.FinalAnswer != "" {
		fmt.Println(record.FinalAnswer)
	}
	return 0
}

func runConfig(args []string) int {
	if len(args) == 0 {
		printConfigUsage(os.Stderr)
		return 1
	}

	switch args[0] {
	case "validate":
		flags := flag.NewFlagSet("config validate", flag.ContinueOnError)
		flags.SetOutput(os.Stderr)
		configPath := flags.String("config", "", "path to council config file")

		if err := flags.Parse(args[1:]); err != nil {
			return 1
		}

		loaded, err := config.Load(*configPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}

		fmt.Printf("config valid: %s\n", loaded.Path)
		fmt.Printf("providers=%d agents=%d teams=%d protocols=%d\n",
			len(loaded.Config.Providers),
			len(loaded.Config.Agents),
			len(loaded.Config.Teams),
			len(loaded.Config.Protocols),
		)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown config subcommand %q\n\n", args[0])
		printConfigUsage(os.Stderr)
		return 1
	}
}

func runPlan(args []string) int {
	flags := flag.NewFlagSet("plan", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", "", "path to council config file")
	teamName := flags.String("team", "", "team to plan")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*teamName) == "" {
		fmt.Fprintln(os.Stderr, "plan requires --team")
		return 1
	}

	loaded, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	plan, err := run.BuildPlan(loaded.Config, *teamName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Printf("team: %s\n", plan.TeamName)
	fmt.Printf("protocol: %s (%s)\n", plan.ProtocolName, plan.ProtocolKind)
	fmt.Printf("synthesizer: %s\n", plan.Synthesizer)
	fmt.Printf("members: %d (distinct models: %d)\n", len(plan.Members), plan.DistinctModel)
	for index, member := range plan.Members {
		fmt.Printf("%d. %s provider=%s(%s) model=%s role=%s\n",
			index+1,
			member.Name,
			member.ProviderName,
			member.ProviderType,
			member.Model,
			member.Role,
		)
	}

	return 0
}

func runShow(args []string) int {
	parsed, err := parseShowArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		printShowUsage(os.Stderr)
		return 1
	}

	runsDir, err := storage.DefaultRunsDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	repo := storage.NewRepository(runsDir)
	record, err := repo.Load(parsed.runID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if parsed.jsonOutput {
		return writeJSON(record)
	}

	fmt.Printf("run: %s\n", record.ID)
	fmt.Printf("status: %s\n", record.Status)
	fmt.Printf("team: %s\n", record.Team)
	fmt.Printf("protocol: %s\n", record.Protocol)
	if len(record.Artifacts) > 0 {
		fmt.Printf("artifacts: %d\n", len(record.Artifacts))
		if artifactsContentOmitted(record.Artifacts) {
			fmt.Println("artifact contents: not retained")
		}
	}
	fmt.Printf("rounds: %d/%d\n", record.CompletedRounds, record.MaxRounds)
	if len(record.AgentOutputs) == 0 && record.Status != "running" {
		fmt.Println("agent outputs: not retained")
	}
	if record.StopReason != "" {
		fmt.Printf("stop reason: %s\n", record.StopReason)
	}
	fmt.Println()
	if record.FinalAnswer != "" {
		fmt.Println(record.FinalAnswer)
		return 0
	}

	if record.Error != "" {
		fmt.Println(record.Error)
	}

	return 0
}

func printRootUsage(stream *os.File) {
	fmt.Fprintln(stream, "Council is a local CLI-first multi-agent deliberation engine.")
	fmt.Fprintln(stream)
	fmt.Fprintln(stream, "Usage:")
	fmt.Fprintln(stream, "  council ask \"prompt\" --team <name> [--file path]... [--config path] [--max-time duration] [--max-rounds n] [--retain-agent-outputs] [--retain-raw-provider-io] [--retain-artifact-content] [--json]")
	fmt.Fprintln(stream, "  council ask --prompt-file <path> --team <name> [--file path]... [--config path] [--max-time duration] [--max-rounds n] [--retain-agent-outputs] [--retain-raw-provider-io] [--retain-artifact-content] [--json]")
	fmt.Fprintln(stream, "  council ask --stdin --team <name> [--file path]... [--config path] [--max-time duration] [--max-rounds n] [--retain-agent-outputs] [--retain-raw-provider-io] [--retain-artifact-content] [--json]")
	fmt.Fprintln(stream, "  council config validate [--config path]")
	fmt.Fprintln(stream, "  council plan --team <name> [--config path]")
	fmt.Fprintln(stream, "  council show <run-id> [--json]")
}

func printConfigUsage(stream *os.File) {
	fmt.Fprintln(stream, "Usage:")
	fmt.Fprintln(stream, "  council config validate [--config path]")
}

func printAskUsage(stream *os.File) {
	fmt.Fprintln(stream, "Usage:")
	fmt.Fprintln(stream, "  council ask \"prompt\" --team <name> [--file path]... [--config path] [--max-time duration] [--max-rounds n] [--retain-agent-outputs] [--retain-raw-provider-io] [--retain-artifact-content] [--json]")
	fmt.Fprintln(stream, "  council ask --prompt-file <path> --team <name> [--file path]... [--config path] [--max-time duration] [--max-rounds n] [--retain-agent-outputs] [--retain-raw-provider-io] [--retain-artifact-content] [--json]")
	fmt.Fprintln(stream, "  council ask --stdin --team <name> [--file path]... [--config path] [--max-time duration] [--max-rounds n] [--retain-agent-outputs] [--retain-raw-provider-io] [--retain-artifact-content] [--json]")
}

func printShowUsage(stream *os.File) {
	fmt.Fprintln(stream, "Usage:")
	fmt.Fprintln(stream, "  council show <run-id> [--json]")
}

type askArgs struct {
	configPath            string
	maxTime               time.Duration
	maxRounds             int
	teamName              string
	filePaths             []string
	retainAgentOutputs    bool
	retainRawProviderIO   bool
	retainArtifactContent bool
	prompt                string
	promptFile            string
	readStdin             bool
	jsonOutput            bool
}

func parseAskArgs(args []string) (*askArgs, error) {
	parsed := &askArgs{maxRounds: 1}
	promptParts := make([]string, 0)

	for index := 0; index < len(args); index++ {
		arg := args[index]

		switch {
		case arg == "--json":
			parsed.jsonOutput = true
		case arg == "--retain-agent-outputs":
			parsed.retainAgentOutputs = true
		case arg == "--retain-raw-provider-io":
			parsed.retainRawProviderIO = true
		case arg == "--retain-artifact-content":
			parsed.retainArtifactContent = true
		case arg == "--config":
			index++
			if index >= len(args) {
				return nil, fmt.Errorf("--config requires a value")
			}
			parsed.configPath = args[index]
		case strings.HasPrefix(arg, "--config="):
			parsed.configPath = strings.TrimPrefix(arg, "--config=")
		case arg == "--max-time":
			index++
			if index >= len(args) {
				return nil, fmt.Errorf("--max-time requires a value")
			}

			maxTime, err := parseMaxTime(args[index])
			if err != nil {
				return nil, err
			}
			parsed.maxTime = maxTime
		case strings.HasPrefix(arg, "--max-time="):
			maxTime, err := parseMaxTime(strings.TrimPrefix(arg, "--max-time="))
			if err != nil {
				return nil, err
			}
			parsed.maxTime = maxTime
		case arg == "--max-rounds":
			index++
			if index >= len(args) {
				return nil, fmt.Errorf("--max-rounds requires a value")
			}

			maxRounds, err := parseMaxRounds(args[index])
			if err != nil {
				return nil, err
			}
			parsed.maxRounds = maxRounds
		case strings.HasPrefix(arg, "--max-rounds="):
			maxRounds, err := parseMaxRounds(strings.TrimPrefix(arg, "--max-rounds="))
			if err != nil {
				return nil, err
			}
			parsed.maxRounds = maxRounds
		case arg == "--file":
			index++
			if index >= len(args) {
				return nil, fmt.Errorf("--file requires a value")
			}
			parsed.filePaths = append(parsed.filePaths, args[index])
		case strings.HasPrefix(arg, "--file="):
			parsed.filePaths = append(parsed.filePaths, strings.TrimPrefix(arg, "--file="))
		case arg == "--prompt-file":
			index++
			if index >= len(args) {
				return nil, fmt.Errorf("--prompt-file requires a value")
			}
			parsed.promptFile = args[index]
		case strings.HasPrefix(arg, "--prompt-file="):
			parsed.promptFile = strings.TrimPrefix(arg, "--prompt-file=")
		case arg == "--stdin":
			parsed.readStdin = true
		case arg == "--team":
			index++
			if index >= len(args) {
				return nil, fmt.Errorf("--team requires a value")
			}
			parsed.teamName = args[index]
		case strings.HasPrefix(arg, "--team="):
			parsed.teamName = strings.TrimPrefix(arg, "--team=")
		case strings.HasPrefix(arg, "-"):
			return nil, fmt.Errorf("unknown flag %q", arg)
		default:
			promptParts = append(promptParts, arg)
		}
	}

	parsed.prompt = strings.TrimSpace(strings.Join(promptParts, " "))

	if parsed.teamName == "" {
		return nil, fmt.Errorf("ask requires --team")
	}

	promptSourceCount := 0
	if parsed.prompt != "" {
		promptSourceCount++
	}
	if parsed.promptFile != "" {
		promptSourceCount++
	}
	if parsed.readStdin {
		promptSourceCount++
	}

	if promptSourceCount == 0 {
		return nil, fmt.Errorf("ask requires a prompt, --prompt-file, or --stdin")
	}

	if promptSourceCount > 1 {
		return nil, fmt.Errorf("ask accepts only one prompt source at a time")
	}

	if parsed.retainRawProviderIO {
		parsed.retainAgentOutputs = true
	}

	return parsed, nil
}

type showArgs struct {
	runID      string
	jsonOutput bool
}

func parseShowArgs(args []string) (*showArgs, error) {
	parsed := &showArgs{}
	positionals := make([]string, 0, 1)

	for _, arg := range args {
		switch {
		case arg == "--json":
			parsed.jsonOutput = true
		case strings.HasPrefix(arg, "-"):
			return nil, fmt.Errorf("unknown flag %q", arg)
		default:
			positionals = append(positionals, arg)
		}
	}

	if len(positionals) != 1 {
		return nil, fmt.Errorf("show requires exactly one run id")
	}

	parsed.runID = positionals[0]
	return parsed, nil
}

func writeJSON(value any) int {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Println(string(data))
	return 0
}

func parseMaxTime(value string) (time.Duration, error) {
	maxTime, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid --max-time %q: %w", value, err)
	}

	if maxTime <= 0 {
		return 0, fmt.Errorf("--max-time must be greater than 0")
	}

	return maxTime, nil
}

func parseMaxRounds(value string) (int, error) {
	maxRounds, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid --max-rounds %q: %w", value, err)
	}

	if maxRounds <= 0 {
		return 0, fmt.Errorf("--max-rounds must be greater than 0")
	}

	return maxRounds, nil
}

func printRunEvent(event run.Event) {
	switch event.Type {
	case run.EventRunStarted:
		fmt.Fprintf(os.Stderr, "run id: %s\n", event.RunID)
	case run.EventRunStopped:
		fmt.Fprintf(os.Stderr, "stopping after round %d: %s\n", event.Round+1, event.StopReason)
	case run.EventAgentStarted:
		fmt.Fprintf(os.Stderr, "starting round %d %s via %s (%s)\n", event.Round+1, event.AgentName, event.Provider, event.Model)
	case run.EventAgentCompleted:
		fmt.Fprintf(os.Stderr, "completed round %d %s in %s\n", event.Round+1, event.AgentName, formatDuration(event.Duration))
	case run.EventAgentFailed:
		fmt.Fprintf(os.Stderr, "failed round %d %s after %s: %v\n", event.Round+1, event.AgentName, formatDuration(event.Duration), event.Err)
	case run.EventSynthesisStarted:
		fmt.Fprintf(os.Stderr, "starting synthesis via %s (%s)\n", event.AgentName, event.Model)
	case run.EventSynthesisComplete:
		fmt.Fprintf(os.Stderr, "completed synthesis in %s\n", formatDuration(event.Duration))
	}
}

func formatDuration(duration time.Duration) string {
	if duration < time.Second {
		return duration.Round(time.Millisecond).String()
	}

	return duration.Round(100 * time.Millisecond).String()
}

func loadPrompt(args *askArgs) (string, error) {
	switch {
	case args == nil:
		return "", fmt.Errorf("ask arguments are required")
	case args.prompt != "":
		return args.prompt, nil
	case args.promptFile != "":
		data, err := os.ReadFile(args.promptFile)
		if err != nil {
			return "", fmt.Errorf("read prompt file %q: %w", args.promptFile, err)
		}

		prompt := strings.TrimSpace(string(data))
		if prompt == "" {
			return "", fmt.Errorf("prompt file %q is empty", args.promptFile)
		}

		return prompt, nil
	case args.readStdin:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read prompt from stdin: %w", err)
		}

		prompt := strings.TrimSpace(string(data))
		if prompt == "" {
			return "", fmt.Errorf("stdin prompt is empty")
		}

		return prompt, nil
	default:
		return "", fmt.Errorf("ask requires a prompt source")
	}
}

func artifactsContentOmitted(artifacts []model.Artifact) bool {
	if len(artifacts) == 0 {
		return false
	}

	for _, artifact := range artifacts {
		if artifact.ContentOmitted {
			return true
		}
	}

	return false
}
