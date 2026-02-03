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

// TestHelmDiffWithHelmV4 tests that helm-diff uses --dry-run=server
// for Helm v4 when validation is enabled and cluster access is allowed
func TestHelmDiffWithHelmV4(t *testing.T) {
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

	os.Setenv(testStubsEnv, "v4")
	defer os.Unsetenv(testStubsEnv)

	os.Args = []string{"helm-diff", "upgrade", "-f", "test/testdata/test-values.yaml", "test-release", "test/testdata/test-chart"}
	require.NoError(t, cmd.New().Execute())
}

// TestHelmDiffWithHelmV4DisabledValidation tests that helm-diff uses --dry-run=client
// for Helm v4 when validation is disabled
func TestHelmDiffWithHelmV4DisabledValidation(t *testing.T) {
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

	os.Setenv(testStubsEnv, "v4DisabledValidation")
	defer os.Unsetenv(testStubsEnv)

	os.Args = []string{"helm-diff", "upgrade", "--disable-validation", "-f", "test/testdata/test-values.yaml", "test-release", "test/testdata/test-chart"}
	require.NoError(t, cmd.New().Execute())
}

// TestHelmDiffWithHelmV4DryRunServer tests that helm-diff uses --dry-run=server
// when explicitly requested, regardless of Helm version
func TestHelmDiffWithHelmV4DryRunServer(t *testing.T) {
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

	os.Setenv(testStubsEnv, "v4DryRunServer")
	defer os.Unsetenv(testStubsEnv)

	os.Args = []string{"helm-diff", "upgrade", "--dry-run=server", "-f", "test/testdata/test-values.yaml", "test-release", "test/testdata/test-chart"}
	require.NoError(t, cmd.New().Execute())
}

// TestHelmDiffWithHelmV3 tests that helm-diff uses --validate and --dry-run=client
// for Helm v3 when validation is enabled
func TestHelmDiffWithHelmV3(t *testing.T) {
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

	os.Setenv(testStubsEnv, "v3")
	defer os.Unsetenv(testStubsEnv)

	os.Args = []string{"helm-diff", "upgrade", "-f", "test/testdata/test-values.yaml", "test-release", "test/testdata/test-chart"}
	require.NoError(t, cmd.New().Execute())
}

// TestHelmDiffWithHelmV3DisabledValidation tests that helm-diff uses --dry-run=client
// for Helm v3 when validation is disabled
func TestHelmDiffWithHelmV3DisabledValidation(t *testing.T) {
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

	os.Setenv(testStubsEnv, "v3DisabledValidation")
	defer os.Unsetenv(testStubsEnv)

	os.Args = []string{"helm-diff", "upgrade", "--disable-validation", "-f", "test/testdata/test-values.yaml", "test-release", "test/testdata/test-chart"}
	require.NoError(t, cmd.New().Execute())
}

const (
	env          = "BECOME_FAKE_HELM"
	envValue     = "1"
	testStubsEnv = "TEST_STUBS"
)

type fakeHelmSubcmd struct {
	cmd      []string
	args     []string
	stdout   string
	stderr   string
	exitCode int
}

// getTestStubs returns appropriate stubs based on test environment variable
func getTestStubs() []fakeHelmSubcmd {
	switch os.Getenv(testStubsEnv) {
	case "v4":
		return []fakeHelmSubcmd{
			{
				cmd:    []string{"version"},
				stdout: `version.BuildInfo{Version:"v4.0.0", GitCommit:"12345", GitTreeState:"clean", GoVersion:"go1.21.0"}`,
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
				args: []string{"test-release", "test/testdata/test-chart", "--values", "test/testdata/test-values.yaml", "--dry-run=server", "--is-upgrade"},
			},
			{
				cmd:  []string{"get", "hooks"},
				args: []string{"test-release"},
			},
		}
	case "v4DisabledValidation":
		return []fakeHelmSubcmd{
			{
				cmd:    []string{"version"},
				stdout: `version.BuildInfo{Version:"v4.0.0", GitCommit:"12345", GitTreeState:"clean", GoVersion:"go1.21.0"}`,
			},
			{
				cmd:  []string{"template"},
				args: []string{"test-release", "test/testdata/test-chart", "--values", "test/testdata/test-values.yaml", "--dry-run=client", "--is-upgrade"},
			},
		}
	case "v4DryRunServer":
		return []fakeHelmSubcmd{
			{
				cmd:    []string{"version"},
				stdout: `version.BuildInfo{Version:"v4.0.0", GitCommit:"12345", GitTreeState:"clean", GoVersion:"go1.21.0"}`,
			},
			{
				cmd:  []string{"template"},
				args: []string{"test-release", "test/testdata/test-chart", "--values", "test/testdata/test-values.yaml", "--dry-run=server", "--is-upgrade"},
			},
		}
	case "v3":
		return []fakeHelmSubcmd{
			{
				cmd:    []string{"version"},
				stdout: `version.BuildInfo{Version:"v3.19.2", GitCommit:"12345", GitTreeState:"clean", GoVersion:"go1.20.12"}`,
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
				args: []string{"test-release", "test/testdata/test-chart", "--values", "test/testdata/test-values.yaml", "--validate", "--dry-run=client", "--is-upgrade"},
			},
			{
				cmd:  []string{"get", "hooks"},
				args: []string{"test-release"},
			},
		}
	case "v3DisabledValidation":
		return []fakeHelmSubcmd{
			{
				cmd:    []string{"version"},
				stdout: `version.BuildInfo{Version:"v3.19.2", GitCommit:"12345", GitTreeState:"clean", GoVersion:"go1.20.12"}`,
			},
			{
				cmd:  []string{"template"},
				args: []string{"test-release", "test/testdata/test-chart", "--values", "test/testdata/test-values.yaml", "--dry-run=client", "--is-upgrade"},
			},
		}
	default:
		return []fakeHelmSubcmd{
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
	}
}

func runFakeHelm() int {
	var stub *fakeHelmSubcmd

	if len(os.Args) < 2 {
		_, _ = fmt.Fprintln(os.Stderr, "fake helm does not support invocations without subcommands")
		return 1
	}

	cmdAndArgs := os.Args[1:]
	stubs := getTestStubs()

	for i := range stubs {
		s := stubs[i]
		if reflect.DeepEqual(s.cmd, cmdAndArgs[:len(s.cmd)]) {
			stub = &s
			break
		}
	}

	if stub == nil {
		_, _ = fmt.Fprintf(os.Stderr, "no stub for %s\n", cmdAndArgs)
		return 1
	}

	want := stub.args
	if want == nil {
		want = []string{}
	}
	got := cmdAndArgs[len(stub.cmd):]
	if !reflect.DeepEqual(want, got) {
		_, _ = fmt.Fprintf(os.Stderr, "want: %v\n", want)
		_, _ = fmt.Fprintf(os.Stderr, "got : %v\n", got)
		_, _ = fmt.Fprintf(os.Stderr, "args : %v\n", os.Args)
		_, _ = fmt.Fprintf(os.Stderr, "env : %v\n", os.Environ())
		return 1
	}
	_, _ = fmt.Fprintf(os.Stdout, "%s", stub.stdout)
	_, _ = fmt.Fprintf(os.Stderr, "%s", stub.stderr)
	return stub.exitCode
}
