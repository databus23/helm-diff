package cmd

import (
	"reflect"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type dryRunFlagsConfig struct {
	isHelmV4             bool
	supportsDryRunLookup bool
	clusterAccessAllowed bool
	disableValidation    bool
	dryRunMode           string
}

func getTemplateDryRunFlags(cfg dryRunFlagsConfig) []string {
	var flags []string

	if !cfg.disableValidation && cfg.clusterAccessAllowed {
		if cfg.isHelmV4 {
			if !slices.Contains([]string{"client", "true", "false"}, cfg.dryRunMode) {
				flags = append(flags, "--dry-run=server")
			}
		} else {
			flags = append(flags, "--validate")
		}
	}

	if cfg.supportsDryRunLookup {
		if cfg.dryRunMode == "false" {
			// "false" means no dry-run, skip adding any dry-run flag
		} else if !(cfg.isHelmV4 && !slices.Contains([]string{"client", "true"}, cfg.dryRunMode)) {
			if cfg.dryRunMode == "server" {
				flags = append(flags, "--dry-run=server")
			} else {
				flags = append(flags, "--dry-run=client")
			}
		}
	}

	return flags
}

func TestGetTemplateDryRunFlags(t *testing.T) {
	cases := []struct {
		name     string
		config   dryRunFlagsConfig
		expected []string
	}{
		{
			name: "Helm v4 with no explicit dry-run flag uses server mode",
			config: dryRunFlagsConfig{
				isHelmV4:             true,
				supportsDryRunLookup: true,
				clusterAccessAllowed: true,
				disableValidation:    false,
				dryRunMode:           "none",
			},
			expected: []string{"--dry-run=server"},
		},
		{
			name: "Helm v4 with dry-run=client uses client mode",
			config: dryRunFlagsConfig{
				isHelmV4:             true,
				supportsDryRunLookup: true,
				clusterAccessAllowed: false,
				disableValidation:    false,
				dryRunMode:           "client",
			},
			expected: []string{"--dry-run=client"},
		},
		{
			name: "Helm v4 with dry-run=server uses server mode",
			config: dryRunFlagsConfig{
				isHelmV4:             true,
				supportsDryRunLookup: true,
				clusterAccessAllowed: true,
				disableValidation:    false,
				dryRunMode:           "server",
			},
			expected: []string{"--dry-run=server"},
		},
		{
			name: "Helm v4 with validation disabled and dry-run=none skips dry-run flags",
			config: dryRunFlagsConfig{
				isHelmV4:             true,
				supportsDryRunLookup: true,
				clusterAccessAllowed: true,
				disableValidation:    true,
				dryRunMode:           "none",
			},
			expected: nil,
		},
		{
			name: "Helm v4 with validation disabled and dry-run=client uses client mode",
			config: dryRunFlagsConfig{
				isHelmV4:             true,
				supportsDryRunLookup: true,
				clusterAccessAllowed: true,
				disableValidation:    true,
				dryRunMode:           "client",
			},
			expected: []string{"--dry-run=client"},
		},
		{
			name: "Helm v3 with no explicit dry-run flag uses validate and client",
			config: dryRunFlagsConfig{
				isHelmV4:             false,
				supportsDryRunLookup: true,
				clusterAccessAllowed: true,
				disableValidation:    false,
				dryRunMode:           "none",
			},
			expected: []string{"--validate", "--dry-run=client"},
		},
		{
			name: "Helm v3 with dry-run=server uses server mode",
			config: dryRunFlagsConfig{
				isHelmV4:             false,
				supportsDryRunLookup: true,
				clusterAccessAllowed: true,
				disableValidation:    false,
				dryRunMode:           "server",
			},
			expected: []string{"--validate", "--dry-run=server"},
		},
		{
			name: "Helm v3 with dry-run=client uses client mode",
			config: dryRunFlagsConfig{
				isHelmV4:             false,
				supportsDryRunLookup: true,
				clusterAccessAllowed: false,
				disableValidation:    false,
				dryRunMode:           "client",
			},
			expected: []string{"--dry-run=client"},
		},
		{
			name: "Helm v3 without dry-run lookup support uses only validate",
			config: dryRunFlagsConfig{
				isHelmV4:             false,
				supportsDryRunLookup: false,
				clusterAccessAllowed: true,
				disableValidation:    false,
				dryRunMode:           "none",
			},
			expected: []string{"--validate"},
		},
		{
			name: "Helm v4 without dry-run lookup support uses server mode",
			config: dryRunFlagsConfig{
				isHelmV4:             true,
				supportsDryRunLookup: false,
				clusterAccessAllowed: true,
				disableValidation:    false,
				dryRunMode:           "none",
			},
			expected: []string{"--dry-run=server"},
		},
		{
			name: "Helm v4 with empty dry-run mode uses server mode",
			config: dryRunFlagsConfig{
				isHelmV4:             true,
				supportsDryRunLookup: true,
				clusterAccessAllowed: true,
				disableValidation:    false,
				dryRunMode:           "",
			},
			expected: []string{"--dry-run=server"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := getTemplateDryRunFlags(tc.config)
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, actual)
			}
		})
	}
}

func TestGetTemplateDryRunFlagsBoolModes(t *testing.T) {
	cases := []struct {
		name     string
		config   dryRunFlagsConfig
		expected []string
	}{
		{
			name: "Helm v3 dryRunMode=true behaves like client",
			config: dryRunFlagsConfig{
				isHelmV4:             false,
				supportsDryRunLookup: true,
				clusterAccessAllowed: true,
				disableValidation:    false,
				dryRunMode:           "true",
			},
			expected: []string{"--validate", "--dry-run=client"},
		},
		{
			name: "Helm v3 dryRunMode=false behaves like none",
			config: dryRunFlagsConfig{
				isHelmV4:             false,
				supportsDryRunLookup: true,
				clusterAccessAllowed: true,
				disableValidation:    false,
				dryRunMode:           "false",
			},
			expected: []string{"--validate"},
		},
		{
			name: "Helm v4 dryRunMode=true behaves like client",
			config: dryRunFlagsConfig{
				isHelmV4:             true,
				supportsDryRunLookup: true,
				clusterAccessAllowed: false,
				disableValidation:    false,
				dryRunMode:           "true",
			},
			expected: []string{"--dry-run=client"},
		},
		{
			name: "Helm v4 dryRunMode=false behaves like none",
			config: dryRunFlagsConfig{
				isHelmV4:             true,
				supportsDryRunLookup: true,
				clusterAccessAllowed: true,
				disableValidation:    false,
				dryRunMode:           "false",
			},
			expected: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := getTemplateDryRunFlags(tc.config)
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("Expected %v, got %v", tc.expected, actual)
			}
		})
	}
}

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
