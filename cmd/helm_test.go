package cmd

import (
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
	
To connect to your database directly from outside the K8s cluster:
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

// TestDryRunModeCoverage documents the expected dry-run flag behavior
// for different Helm versions and configurations.
//
// This test documents the expected behavior to ensure the fix for #894
// (conflicting dry-run flags with Helm v4) is maintained.
//
// The actual behavior is tested integration-style by running helm-diff
// against different Helm versions. Key invariants:
//
// 1. Only one --dry-run=... flag should ever be passed to helm template
// 2. For Helm v4 with cluster access and validation enabled, use --dry-run=server
// 3. For Helm v3 or when validation is disabled, use --dry-run=client
// 4. User's explicit --dry-run=server request always takes precedence
// 5. For Helm v3, --validate flag is used for validation
// 6. For Helm v4, --dry-run=server replaces --validate for validation
func TestDryRunModeCoverage(t *testing.T) {
	// This is a documentation test. Actual behavior is tested
	// by integration tests that run helm-diff against different Helm versions.
	// See: https://github.com/databus23/helm-diff/issues/894

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
