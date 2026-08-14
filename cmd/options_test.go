package cmd

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/databus23/helm-diff/v3/diff"
)

func processedOptions(t *testing.T, args ...string) diff.Options {
	t.Helper()

	var o diff.Options
	f := pflag.NewFlagSet("test", pflag.ContinueOnError)
	AddDiffOptions(f, &o)
	require.NoError(t, f.Parse(args))
	ProcessDiffOptions(f, &o)

	return o
}

func TestProcessDiffOptionsSuppressSecrets(t *testing.T) {
	o := processedOptions(t, "--suppress-secrets")
	require.Contains(t, o.SuppressedKinds, "Secret")
}

func TestAddDiffOptionsHasNoExternalOutputFormat(t *testing.T) {
	var o diff.Options
	f := pflag.NewFlagSet("test", pflag.ContinueOnError)
	AddDiffOptions(f, &o)

	require.NotContains(t, f.Lookup("output").Usage, "external",
		"--diff-tool is the only way to select an external tool")
}

func TestProcessDiffOptionsDiffTool(t *testing.T) {
	t.Run("no external diff by default", func(t *testing.T) {
		t.Setenv(diff.DiffToolEnvVar, "")
		o := processedOptions(t)
		require.Equal(t, "diff", o.OutputFormat)
		require.Empty(t, o.DiffToolCommand)
		require.False(t, o.DiffTool())
	})

	t.Run("--diff-tool enables the external tool", func(t *testing.T) {
		t.Setenv(diff.DiffToolEnvVar, "")
		o := processedOptions(t, "--diff-tool", "difft")
		require.Equal(t, "difft", o.DiffToolCommand)
		require.True(t, o.DiffTool())
	})

	t.Run("HELM_DIFF_TOOL enables the external tool", func(t *testing.T) {
		t.Setenv(diff.DiffToolEnvVar, "colordiff -u")
		o := processedOptions(t)
		require.True(t, o.DiffTool())
	})

	t.Run("the external tool overrides an explicit --output", func(t *testing.T) {
		t.Setenv(diff.DiffToolEnvVar, "")
		o := processedOptions(t, "--diff-tool", "difft", "--output", "json")
		require.Equal(t, "json", o.OutputFormat, "--output keeps its value")
		require.True(t, o.DiffTool(), "but the external tool takes precedence")
	})

	t.Run("an empty --diff-tool keeps the built-in output", func(t *testing.T) {
		t.Setenv(diff.DiffToolEnvVar, "")
		o := processedOptions(t, "--diff-tool", "", "--output", "simple")
		require.False(t, o.DiffTool())
	})
}
