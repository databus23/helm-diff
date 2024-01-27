package cmd

import "testing"

func TestIsRemoteAccessAllowed(t *testing.T) {
	cases := []struct {
		name     string
		cmd      diffCmd
		expected bool
	}{
		{
			name: "no flags",
			cmd: diffCmd{
				dryRunMode: "none",
			},
			expected: true,
		},
		{
			name: "legacy explicit dry-run=true flag",
			cmd: diffCmd{
				dryRunMode: "true",
			},
			expected: false,
		},
		{
			name: "legacy explicit dry-run=false flag",
			cmd: diffCmd{
				dryRunMode: "false",
			},
			expected: true,
		},
		{
			name: "legacy empty dry-run flag",
			cmd: diffCmd{
				dryRunMode: dryRunNoOptDefVal,
			},
			expected: false,
		},
		{
			name: "server-side dry-run flag",
			cmd: diffCmd{
				dryRunMode: "server",
			},
			expected: true,
		},
		{
			name: "client-side dry-run flag",
			cmd: diffCmd{
				dryRunMode: "client",
			},
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.cmd.clusterAccessAllowed()
			if actual != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, actual)
			}
		})
	}
}
