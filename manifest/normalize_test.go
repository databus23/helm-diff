package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeContent(t *testing.T) {
	tests := []struct {
		name           string
		content        []byte
		expectedOutput []byte
		expectError    bool
	}{
		{
			name: "Valid content",
			content: []byte(`apiVersion: v1
kind: Pod
metadata:
  name: my-pod
spec:
  containers:
  - name: my-container
    image: nginx`),
			expectedOutput: []byte(`apiVersion: v1
kind: Pod
metadata:
  name: my-pod
spec:
  containers:
  - image: nginx
    name: my-container
`),
		},
		{
			// yaml.v2 marshals a nil map to "{}\n". Empty content never
			// reaches this path in practice (Parse filters it earlier),
			// but document the behavior regardless.
			name:           "Empty content",
			content:        []byte(""),
			expectedOutput: []byte("{}\n"),
		},
		{
			name:        "Sequence cannot unmarshal into map",
			content:     []byte("- a\n- b"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := normalizeContent(tt.content)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedOutput, output)
		})
	}
}
