package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"council/internal/config"
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
	record, err := run.Execute(context.Background(), repo, loaded.Config, parsed.teamName, parsed.prompt)
	if err != nil {
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
	fmt.Fprintf(os.Stderr, "run id: %s\n", record.ID)
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
	fmt.Printf("protocol: %s\n\n", record.Protocol)
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
	fmt.Fprintln(stream, "  council ask \"prompt\" --team <name> [--config path] [--json]")
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
	fmt.Fprintln(stream, "  council ask \"prompt\" --team <name> [--config path] [--json]")
}

func printShowUsage(stream *os.File) {
	fmt.Fprintln(stream, "Usage:")
	fmt.Fprintln(stream, "  council show <run-id> [--json]")
}

type askArgs struct {
	configPath string
	teamName   string
	prompt     string
	jsonOutput bool
}

func parseAskArgs(args []string) (*askArgs, error) {
	parsed := &askArgs{}
	promptParts := make([]string, 0)

	for index := 0; index < len(args); index++ {
		arg := args[index]

		switch {
		case arg == "--json":
			parsed.jsonOutput = true
		case arg == "--config":
			index++
			if index >= len(args) {
				return nil, fmt.Errorf("--config requires a value")
			}
			parsed.configPath = args[index]
		case strings.HasPrefix(arg, "--config="):
			parsed.configPath = strings.TrimPrefix(arg, "--config=")
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

	if parsed.prompt == "" {
		return nil, fmt.Errorf("ask requires a prompt")
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
