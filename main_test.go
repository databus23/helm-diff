package main

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/databus23/helm-diff/v3/cmd"
)

func TestMain(m *testing.M) {
	if os.Getenv(env) == envValue {
		os.Exit(runFakeHelm())
	}

	os.Exit(m.Run())
}

func TestHelmDiff(t *testing.T) {
	os.Setenv(env, envValue)
	defer os.Unsetenv(env)

	helmBin, helmBinSet := os.LookupEnv("HELM_BIN")
	os.Setenv("HELM_BIN", os.Args[0])
	defer func() {
		if helmBinSet {
			os.Setenv("HELM_BIN", helmBin)
		} else {
			os.Unsetenv("HELM_BIN")
		}
	}()

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"helm-diff", "upgrade", "-f", "test/testdata/test-values.yaml", "test-release", "test/testdata/test-chart"}
	require.NoError(t, cmd.New().Execute())
}

func TestHelmDiffWithKubeContext(t *testing.T) {
	os.Setenv(env, envValue)
	defer os.Unsetenv(env)

	helmBin, helmBinSet := os.LookupEnv("HELM_BIN")
	os.Setenv("HELM_BIN", os.Args[0])
	defer func() {
		if helmBinSet {
			os.Setenv("HELM_BIN", helmBin)
		} else {
			os.Unsetenv("HELM_BIN")
		}
	}()

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"helm-diff", "upgrade", "-f", "test/testdata/test-values.yaml", "--kube-context", "test-context", "test-release", "test/testdata/test-chart"}
	require.NoError(t, cmd.New().Execute())
}

func TestHelmDiffWithKubeContextReuseValues(t *testing.T) {
	os.Setenv(env, envValue)
	defer os.Unsetenv(env)

	helmBin, helmBinSet := os.LookupEnv("HELM_BIN")
	os.Setenv("HELM_BIN", os.Args[0])
	defer func() {
		if helmBinSet {
			os.Setenv("HELM_BIN", helmBin)
		} else {
			os.Unsetenv("HELM_BIN")
		}
	}()

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"helm-diff", "upgrade", "--reuse-values", "--kube-context", "test-context", "-f", "test/testdata/test-values.yaml", "test-release", "test/testdata/test-chart"}
	require.NoError(t, cmd.New().Execute())
}

func TestHelmDiffRevisionWithKubeContext(t *testing.T) {
	os.Setenv(env, envValue)
	defer os.Unsetenv(env)

	helmBin, helmBinSet := os.LookupEnv("HELM_BIN")
	os.Setenv("HELM_BIN", os.Args[0])
	defer func() {
		if helmBinSet {
			os.Setenv("HELM_BIN", helmBin)
		} else {
			os.Unsetenv("HELM_BIN")
		}
	}()

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"helm-diff", "revision", "--kube-context", "test-context", "test-release", "2"}
	require.NoError(t, cmd.New().Execute())
}

func TestHelmDiffRollbackWithKubeContext(t *testing.T) {
	os.Setenv(env, envValue)
	defer os.Unsetenv(env)

	helmBin, helmBinSet := os.LookupEnv("HELM_BIN")
	os.Setenv("HELM_BIN", os.Args[0])
	defer func() {
		if helmBinSet {
			os.Setenv("HELM_BIN", helmBin)
		} else {
			os.Unsetenv("HELM_BIN")
		}
	}()

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"helm-diff", "rollback", "--kube-context", "test-context", "test-release", "2"}
	require.NoError(t, cmd.New().Execute())
}

func TestHelmDiffReleaseWithKubeContext(t *testing.T) {
	os.Setenv(env, envValue)
	defer os.Unsetenv(env)

	helmBin, helmBinSet := os.LookupEnv("HELM_BIN")
	os.Setenv("HELM_BIN", os.Args[0])
	defer func() {
		if helmBinSet {
			os.Setenv("HELM_BIN", helmBin)
		} else {
			os.Unsetenv("HELM_BIN")
		}
	}()

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"helm-diff", "release", "--kube-context", "test-context", "test-release1", "test-release2"}
	require.NoError(t, cmd.New().Execute())
}

const (
	env      = "BECOME_FAKE_HELM"
	envValue = "1"
)

type fakeHelmSubcmd struct {
	cmd      []string
	args     []string
	stdout   string
	stderr   string
	exitCode int
}

var helmSubcmdStubs = []fakeHelmSubcmd{
	{
		cmd:    []string{"version"},
		stdout: `version.BuildInfo{Version:"v3.1.0-rc.1", GitCommit:"12345", GitTreeState:"clean", GoVersion:"go1.20.12"}`,
	},
	{
		cmd:  []string{"get", "manifest"},
		args: []string{"test-release"},
		stdout: `---
# Source: test-chart/templates/cm.yaml
`,
	},
	{
		cmd:  []string{"template"},
		args: []string{"test-release", "test/testdata/test-chart", "--values", "test/testdata/test-values.yaml", "--validate", "--is-upgrade"},
	},
	{
		cmd:  []string{"get", "hooks"},
		args: []string{"test-release"},
	},
	{
		cmd:  []string{"get", "manifest"},
		args: []string{"test-release", "--kube-context", "test-context"},
		stdout: `---
# Source: test-chart/templates/cm.yaml
`,
	},
	{
		cmd:  []string{"template"},
		args: []string{"test-release", "test/testdata/test-chart", "--kube-context", "test-context", "--values", "test/testdata/test-values.yaml", "--validate", "--is-upgrade"},
	},
	{
		cmd:  []string{"get", "hooks"},
		args: []string{"test-release", "--kube-context", "test-context"},
	},
	{
		cmd:  []string{"get", "values"},
		args: []string{"test-release", "--output", "yaml", "--all", "--kube-context", "test-context"},
	},
	{
		cmd:  []string{"get", "values"},
		args: []string{"test-release", "--output", "yaml", "--all", "--kube-context", "test-context", "--namespace", "*"},
	},
	{
		cmd:  []string{"template"},
		args: []string{"test-release", "test/testdata/test-chart", "--kube-context", "test-context", "--values", "*", "--values", "test/testdata/test-values.yaml", "--validate", "--is-upgrade"},
	},
	{
		cmd:  []string{"get", "manifest"},
		args: []string{"test-release", "--revision", "2", "--kube-context", "test-context"},
		stdout: `---
# Source: test-chart/templates/cm.yaml
`,
	},
	{
		cmd:  []string{"get", "manifest"},
		args: []string{"test-release1", "--kube-context", "test-context"},
		stdout: `---
# Source: test-chart/templates/cm.yaml
`,
	},
	{
		cmd:    []string{"get", "all"},
		args:   []string{"test-release1", "--template", "*", "--kube-context", "test-context"},
		stdout: `test-chart`,
	},
	{
		cmd:  []string{"get", "manifest"},
		args: []string{"test-release2", "--kube-context", "test-context"},
		stdout: `---
# Source: test-chart/templates/cm.yaml
`,
	},
	{
		cmd:    []string{"get", "all"},
		args:   []string{"test-release2", "--template", "*", "--kube-context", "test-context"},
		stdout: `test-chart`,
	},
}

func runFakeHelm() int {
	var stub *fakeHelmSubcmd

	if len(os.Args) < 2 {
		_, _ = fmt.Fprintln(os.Stderr, "fake helm does not support invocations without subcommands")
		return 1
	}

	cmdAndArgs := os.Args[1:]
	for i := range helmSubcmdStubs {
		s := helmSubcmdStubs[i]
		if reflect.DeepEqual(s.cmd, cmdAndArgs[:len(s.cmd)]) {
			want := s.args
			if want == nil {
				want = []string{}
			}
			got := cmdAndArgs[len(s.cmd):]
			if argsMatch(want, got) {
				stub = &s
				break
			}
		}
	}

	if stub == nil {
		_, _ = fmt.Fprintf(os.Stderr, "no stub for %s\n", cmdAndArgs)
		return 1
	}

	_, _ = fmt.Fprintf(os.Stdout, "%s", stub.stdout)
	_, _ = fmt.Fprintf(os.Stderr, "%s", stub.stderr)
	return stub.exitCode
}

func argsMatch(want, got []string) bool {
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if want[i] == "*" {
			continue
		}
		if want[i] != got[i] {
			return false
		}
	}
	return true
}
