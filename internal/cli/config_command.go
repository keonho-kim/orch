package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
	sqlitestore "github.com/keonho-kim/orch/internal/store/sqlite"
)

const configUsage = "usage: orch config --list [--workspace <path>] [--env-file <path>] | orch config [--workspace <path>] [--env-file <path>] --provider=<provider> [--model=<name>] [--reasoning=<true|false|low|medium|high|xhigh>] [--endpoint=<url>] [--api-key=<secret>] [--approval-policy=<policy>] [--self-driving-mode=<true|false>] [--react-ralph-iter=<n>] [--plan-ralph-iter=<n>] [--compact-threshold-k=<n>]"

type configCommandState struct {
	list  bool
	patch config.DocumentPatch
}

func parseConfigCommand(args []string) (command, error) {
	repoRoot, configFile, rest, err := parseGlobalFlags(args)
	if err != nil {
		return command{}, err
	}
	state, err := parseConfigFlags(rest)
	if err != nil {
		return command{}, err
	}
	return buildConfigCommand(repoRoot, configFile, state)
}

type parsedConfigFlags struct {
	visited           map[string]bool
	list              bool
	provider          string
	model             string
	reasoning         string
	endpoint          string
	apiKey            string
	approvalPolicy    string
	selfDrivingMode   bool
	reactRalphIter    int
	planRalphIter     int
	compactThresholdK int
}

func parseConfigFlags(rest []string) (parsedConfigFlags, error) {
	if len(rest) == 0 {
		return parsedConfigFlags{}, fmt.Errorf("%s", configUsage)
	}

	flagSet := flag.NewFlagSet("config", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	state := parsedConfigFlags{}
	flagSet.BoolVar(&state.list, "list", false, "")
	flagSet.StringVar(&state.provider, "provider", "", "")
	flagSet.StringVar(&state.model, "model", "", "")
	flagSet.StringVar(&state.reasoning, "reasoning", "", "")
	flagSet.StringVar(&state.endpoint, "endpoint", "", "")
	flagSet.StringVar(&state.apiKey, "api-key", "", "")
	flagSet.StringVar(&state.approvalPolicy, "approval-policy", "", "")
	flagSet.BoolVar(&state.selfDrivingMode, "self-driving-mode", false, "")
	flagSet.IntVar(&state.reactRalphIter, "react-ralph-iter", 0, "")
	flagSet.IntVar(&state.planRalphIter, "plan-ralph-iter", 0, "")
	flagSet.IntVar(&state.compactThresholdK, "compact-threshold-k", 0, "")

	if err := flagSet.Parse(rest); err != nil {
		return parsedConfigFlags{}, err
	}
	if extra := flagSet.Args(); len(extra) > 0 {
		return parsedConfigFlags{}, fmt.Errorf("unexpected config arguments: %s", strings.Join(extra, " "))
	}
	state.visited = visitedConfigFlags(flagSet)
	return state, nil
}

func buildConfigCommand(repoRoot string, configFile string, state parsedConfigFlags) (command, error) {
	if state.list {
		if err := validateListOnlyFlags(state.visited); err != nil {
			return command{}, err
		}
		return command{
			name:       "config-list",
			repoRoot:   repoRoot,
			configFile: configFile,
			configCommand: configCommandState{
				list: true,
			},
		}, nil
	}
	patch, err := buildConfigPatch(state)
	if err != nil {
		return command{}, err
	}
	if patch.IsEmpty() {
		return command{}, fmt.Errorf("%s", configUsage)
	}
	return command{
		name:       "config-set",
		repoRoot:   repoRoot,
		configFile: configFile,
		configCommand: configCommandState{
			patch: patch,
		},
	}, nil
}

func visitedConfigFlags(flagSet *flag.FlagSet) map[string]bool {
	visited := make(map[string]bool)
	flagSet.Visit(func(item *flag.Flag) {
		visited[item.Name] = true
	})
	return visited
}

func validateListOnlyFlags(visited map[string]bool) error {
	for key := range visited {
		if key == "list" {
			continue
		}
		return fmt.Errorf("--list may not be combined with write flags")
	}
	return nil
}

func buildConfigPatch(state parsedConfigFlags) (config.DocumentPatch, error) {
	patch := config.DocumentPatch{}

	if state.visited["provider"] {
		provider, err := domain.ParseProvider(state.provider)
		if err != nil {
			return config.DocumentPatch{}, err
		}
		value := provider.String()
		patch.Provider = &value
	}
	if err := applyProviderPatchFlags(&patch, state); err != nil {
		return config.DocumentPatch{}, err
	}
	if state.visited["approval-policy"] {
		value := strings.TrimSpace(state.approvalPolicy)
		patch.ApprovalPolicy = &value
	}
	if state.visited["self-driving-mode"] {
		value := state.selfDrivingMode
		patch.SelfDrivingMode = &value
	}
	if state.visited["react-ralph-iter"] {
		value, err := nonNegativeFlagValue("--react-ralph-iter", state.reactRalphIter)
		if err != nil {
			return config.DocumentPatch{}, err
		}
		patch.ReactRalphIter = &value
	}
	if state.visited["plan-ralph-iter"] {
		value, err := nonNegativeFlagValue("--plan-ralph-iter", state.planRalphIter)
		if err != nil {
			return config.DocumentPatch{}, err
		}
		patch.PlanRalphIter = &value
	}
	if state.visited["compact-threshold-k"] {
		value, err := nonNegativeFlagValue("--compact-threshold-k", state.compactThresholdK)
		if err != nil {
			return config.DocumentPatch{}, err
		}
		patch.CompactThresholdK = &value
	}

	return patch, nil
}

func applyProviderPatchFlags(patch *config.DocumentPatch, state parsedConfigFlags) error {
	if state.visited["model"] {
		value := strings.TrimSpace(state.model)
		if err := setProviderPatchString(patch, "--model", value, func(provider domain.Provider, value string) {
			patch.SetProviderPatch(provider, config.ProviderPatch{Model: &value})
		}); err != nil {
			return err
		}
	}
	if state.visited["reasoning"] {
		value, err := domain.ParseReasoningValue(state.reasoning)
		if err != nil {
			return fmt.Errorf("--reasoning: %w", err)
		}
		if err := setProviderPatchString(patch, "--reasoning", value, func(provider domain.Provider, value string) {
			patch.SetProviderPatch(provider, config.ProviderPatch{Reasoning: &value})
		}); err != nil {
			return err
		}
	}
	if state.visited["endpoint"] {
		value := strings.TrimSpace(state.endpoint)
		if err := setProviderPatchString(patch, "--endpoint", value, func(provider domain.Provider, value string) {
			patch.SetProviderPatch(provider, config.ProviderPatch{Endpoint: &value})
		}); err != nil {
			return err
		}
	}
	if state.visited["api-key"] {
		value := strings.TrimSpace(state.apiKey)
		if err := setProviderPatchString(patch, "--api-key", value, func(provider domain.Provider, value string) {
			patch.SetProviderPatch(provider, config.ProviderPatch{APIKey: &value})
		}); err != nil {
			return err
		}
	}
	return nil
}

func setProviderPatchString(
	patch *config.DocumentPatch,
	flagName string,
	value string,
	apply func(provider domain.Provider, value string),
) error {
	if patch.Provider == nil {
		return fmt.Errorf("%s requires --provider", flagName)
	}
	provider, err := domain.ParseProvider(*patch.Provider)
	if err != nil {
		return fmt.Errorf("%s requires --provider", flagName)
	}
	apply(provider, value)
	return nil
}

func nonNegativeFlagValue(flagName string, value int) (int, error) {
	if value < 0 {
		return 0, fmt.Errorf("%s must be >= 0", flagName)
	}
	return value, nil
}

func runConfigList(repoRoot string, configFile string, stdout io.Writer) error {
	paths, err := resolveAppPaths(repoRoot, configFile)
	if err != nil {
		return err
	}
	document, err := config.LoadDocument(paths)
	if err != nil {
		return err
	}
	data, err := config.MarshalDocument(document, true)
	if err != nil {
		return err
	}
	if _, err := stdout.Write(data); err != nil {
		return fmt.Errorf("write config list: %w", err)
	}
	return nil
}

func runConfigUpdate(repoRoot string, configFile string, state configCommandState) error {
	paths, err := resolveAppPaths(repoRoot, configFile)
	if err != nil {
		return err
	}
	document, err := config.LoadDocument(paths)
	if err != nil {
		return err
	}

	if err := config.ApplyDocumentPatch(&document, state.patch); err != nil {
		return err
	}
	if err := config.SaveDocument(paths, document); err != nil {
		return err
	}

	store, err := sqlitestore.Open(paths.DBPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = store.Close()
	}()

	settings, err := config.LoadSettings(paths)
	if err != nil {
		return err
	}
	if settings.DefaultProvider != "" {
		if err := store.SaveDefaultProvider(context.Background(), settings.DefaultProvider); err != nil {
			return err
		}
	}
	return nil
}

func resolveAppPaths(repoRoot string, configFile string) (config.Paths, error) {
	absoluteRepoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return config.Paths{}, fmt.Errorf("resolve working directory: %w", err)
	}

	paths, err := config.ResolvePathsWithConfigFile(absoluteRepoRoot, configFile)
	if err != nil {
		return config.Paths{}, err
	}
	return paths, nil
}
