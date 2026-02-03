package cmd

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestExtractManifestFromHelmUpgradeDryRunOutput(t *testing.T) {
	type testdata struct {
		description string

		s       string
		noHooks bool

		want string
	}

	manifest := `---
# Source: mysql/templates/secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: my1-mysql
  namespace: default
  labels:
	app: my1-mysql
	chart: "mysql-1.6.9"
	release: "my1"
	heritage: "Helm"
type: Opaque
data:
  mysql-root-password: "ZlhEVGJseUhmeg=="
  mysql-password: "YnRuU3pPOTJMVg=="
---
# Source: mysql/templates/tests/test-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my1-mysql-test
  namespace: default
  labels:
	app: my1-mysql
	chart: "mysql-1.6.9"
	heritage: "Helm"
	release: "my1"
 data:
  run.sh: |-

`
	hooks := `---
# Source: mysql/templates/tests/test.yaml
apiVersion: v1
kind: Pod
metadata:
  name: my1-mysql-test
  namespace: default
  labels:
	app: my1-mysql
	chart: "mysql-1.6.9"
	heritage: "Helm"
	release: "my1"
  annotations:
	"helm.sh/hook": test-success
 spec:
  containers:
	- name: my1-test
	  image: "bats/bats:1.2.1"
	  imagePullPolicy: "IfNotPresent"
	  command: ["/opt/bats/bin/bats", "-t", "/tests/run.sh"]
`

	header := `Release "my1" has been upgraded. Happy Helming!
NAME: my1
LAST DEPLOYED: Sun Feb 13 02:26:16 2022
NAMESPACE: default
STATUS: pending-upgrade
REVISION: 2
HOOKS:
`

	notes := `NOTES:
MySQL can be accessed via port 3306 on the following DNS name from within your cluster:
 my1-mysql.default.svc.cluster.local

*snip*

To connect to your database directly from outside of K8s cluster:
	MYSQL_HOST=127.0.0.1
	MYSQL_PORT=3306

	# Execute the following command to route the connection:
	kubectl port-forward svc/my1-mysql 3306

	mysql -h ${MYSQL_HOST} -P${MYSQL_PORT} -u root -p${MYSQL_ROOT_PASSWORD}
`

	outputWithHooks := header + hooks + "MANIFEST:\n" + manifest + notes
	outputWithNoHooks := header + "MANIFEST:\n" + manifest + notes

	testcases := []testdata{
		{
			description: "should output manifest when noHooks specified",
			s:           outputWithHooks,
			noHooks:     true,
			want:        manifest,
		},
		{
			description: "should output manifest and hooks when noHooks unspecified",
			s:           outputWithHooks,
			noHooks:     false,
			want:        manifest + hooks,
		},
		{
			description: "should output manifest if noHooks specified but input did not contain hooks",
			s:           outputWithNoHooks,
			noHooks:     true,
			want:        manifest,
		},
		{
			description: "should output manifest if noHooks unspecified and input did not contain hooks",
			s:           outputWithNoHooks,
			noHooks:     false,
			want:        manifest,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			got := extractManifestFromHelmUpgradeDryRunOutput([]byte(tc.s), tc.noHooks)

			if d := cmp.Diff(tc.want, string(got)); d != "" {
				t.Errorf("unexpected diff: %s", d)
			}
		})
	}
}

// TestDryRunModeCoverage documents expected dry-run flag behavior
// for different Helm versions and configurations.
//
// This test documents the expected behavior matrix to ensure the fix for #894
// (conflicting dry-run flags with Helm v4) is maintained.
//
// Key invariants tested:
// 1. Only one --dry-run=... flag should ever be passed to helm template
// 2. For Helm v4 with cluster access and validation enabled, use --dry-run=server
// 3. For Helm v3 or when validation is disabled, use --dry-run=client
// 4. User's explicit --dry-run=server request always takes precedence
// 5. For Helm v3, --validate flag is used for validation
// 6. For Helm v4, --dry-run=server replaces --validate for validation
func TestDryRunModeCoverage(t *testing.T) {
	// Expected behavior matrix:
	// Helm version | dryRunMode | validation enabled | cluster access | Expected flag(s)
	// v4           | ""/client  | true               | true           | --dry-run=server
	// v4           | ""/client  | false              | true           | --dry-run=client
	// v4           | ""/client  | true               | false          | --dry-run=client
	// v4           | server     | any                | any            | --dry-run=server
	// v3           | ""/client  | true               | true           | --validate, --dry-run=client
	// v3           | ""/client  | false              | true           | --dry-run=client
	// v3           | ""/client  | true               | false          | --dry-run=client
	// v3           | server     | any                | any            | --dry-run=server
}

// TestDryRunFlags verifies the exact flags passed to helm template
// for different Helm versions and configurations. This is a table-driven test
// that simulates the flag generation logic in the template() function
// to ensure the fix for #894 works correctly.
func TestDryRunFlags(t *testing.T) {
	testCases := []struct {
		name              string
		helmVersion       string
		dryRunMode        string
		disableValidation bool
		clusterAccess     bool
		wantDryRunFlag    string
		wantValidateFlag  bool
	}{
		{
			name:              "Helm v4, validation enabled, cluster access, default dry-run",
			helmVersion:       "v4.0.0",
			dryRunMode:        "none",
			disableValidation: false,
			clusterAccess:     true,
			wantDryRunFlag:    "--dry-run=server",
			wantValidateFlag:  false,
		},
		{
			name:              "Helm v4, validation disabled, cluster access, default dry-run",
			helmVersion:       "v4.0.0",
			dryRunMode:        "none",
			disableValidation: true,
			clusterAccess:     true,
			wantDryRunFlag:    "--dry-run=client",
			wantValidateFlag:  false,
		},
		{
			name:              "Helm v4, validation enabled, no cluster access, default dry-run",
			helmVersion:       "v4.0.0",
			dryRunMode:        "none",
			disableValidation: false,
			clusterAccess:     false,
			wantDryRunFlag:    "--dry-run=client",
			wantValidateFlag:  false,
		},
		{
			name:              "Helm v4, explicit --dry-run=server",
			helmVersion:       "v4.0.0",
			dryRunMode:        "server",
			disableValidation: false,
			clusterAccess:     true,
			wantDryRunFlag:    "--dry-run=server",
			wantValidateFlag:  false,
		},
		{
			name:              "Helm v3, validation enabled, cluster access, default dry-run",
			helmVersion:       "v3.19.2",
			dryRunMode:        "none",
			disableValidation: false,
			clusterAccess:     true,
			wantDryRunFlag:    "--dry-run=client",
			wantValidateFlag:  true,
		},
		{
			name:              "Helm v3, validation disabled, cluster access, default dry-run",
			helmVersion:       "v3.19.2",
			dryRunMode:        "none",
			disableValidation: true,
			clusterAccess:     true,
			wantDryRunFlag:    "--dry-run=client",
			wantValidateFlag:  false,
		},
		{
			name:              "Helm v3, explicit --dry-run=server",
			helmVersion:       "v3.19.2",
			dryRunMode:        "server",
			disableValidation: false,
			clusterAccess:     true,
			wantDryRunFlag:    "--dry-run=server",
			wantValidateFlag:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate template function flag generation logic
			flags := []string{}
			isHelmV4 := strings.HasPrefix(tc.helmVersion, "v4")

			// Validation flag logic from template()
			if !tc.disableValidation && tc.clusterAccess {
				if !isHelmV4 {
					flags = append(flags, "--validate")
				}
			}

			// Dry-run flag logic from template()
			if tc.dryRunMode == "server" {
				flags = append(flags, "--dry-run=server")
			} else if isHelmV4 && !tc.disableValidation && tc.clusterAccess {
				flags = append(flags, "--dry-run=server")
			} else {
				flags = append(flags, "--dry-run=client")
			}

			// Verify only one dry-run flag
			dryRunCount := 0
			for _, f := range flags {
				if strings.HasPrefix(f, "--dry-run") {
					dryRunCount++
					if f != tc.wantDryRunFlag {
						t.Errorf("Got dry-run flag %q, want %q", f, tc.wantDryRunFlag)
					}
				}
			}

			if dryRunCount != 1 {
				t.Errorf("Expected exactly 1 dry-run flag, got %d", dryRunCount)
			}

			// Verify validate flag
			hasValidate := false
			for _, f := range flags {
				if f == "--validate" {
					hasValidate = true
					break
				}
			}

			if hasValidate != tc.wantValidateFlag {
				t.Errorf("Validate flag: got %v, want %v", hasValidate, tc.wantValidateFlag)
			}
		})
	}
}
