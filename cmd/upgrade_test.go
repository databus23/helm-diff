package cmd

import "testing"

func TestIsRemoteAccessAllowed(t *testing.T) {
	cases := []struct {
		name     string
		cmd      diffCmd
		expected bool
	}{
		{
			name:     "no flags",
			cmd:      diffCmd{},
			expected: true,
		},
		{
			name: "legacy explicit dry-run flag",
			cmd: diffCmd{
				dryRunModeSpecified: true,
				dryRunMode:          "true",
			},
			expected: false,
		},
		{
			name: "legacy empty dry-run flag",
			cmd: diffCmd{
				dryRunModeSpecified: true,
				dryRunMode:          "",
			},
			expected: false,
		},
		{
			name: "server-side dry-run flag",
			cmd: diffCmd{
				dryRunModeSpecified: true,
				dryRunMode:          "server",
			},
			expected: true,
		},
		{
			name: "client-side dry-run flag",
			cmd: diffCmd{
				dryRunModeSpecified: true,
				dryRunMode:          "client",
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
