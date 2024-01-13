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

	os.Args = []string{"helm-diff", "upgrade", "-f", "test/testdata/test-values.yaml", "test-release", "test/testdata/test-chart"}
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
}

func runFakeHelm() int {
	var stub *fakeHelmSubcmd

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "fake helm does not support invocations without subcommands")
		return 1
	}

	cmdAndArgs := os.Args[1:]
	for i := range helmSubcmdStubs {
		s := helmSubcmdStubs[i]
		if reflect.DeepEqual(s.cmd, cmdAndArgs[:len(s.cmd)]) {
			stub = &s
			break
		}
	}

	if stub == nil {
		fmt.Fprintf(os.Stderr, "no stub for %s\n", cmdAndArgs)
		return 1
	}

	want := stub.args
	if want == nil {
		want = []string{}
	}
	got := cmdAndArgs[len(stub.cmd):]
	if !reflect.DeepEqual(want, got) {
		fmt.Fprintf(os.Stderr, "want: %v\n", want)
		fmt.Fprintf(os.Stderr, "got : %v\n", got)
		fmt.Fprintf(os.Stderr, "args : %v\n", os.Args)
		fmt.Fprintf(os.Stderr, "env : %v\n", os.Environ())
		return 1
	}
	fmt.Fprintf(os.Stdout, "%s", stub.stdout)
	fmt.Fprintf(os.Stderr, "%s", stub.stderr)
	return stub.exitCode
}
