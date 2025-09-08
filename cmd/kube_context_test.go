package cmd

import (
	"testing"
)

func TestKubeContextFlag(t *testing.T) {
	tests := []struct {
		name        string
		kubeContext string
		expected    bool
	}{
		{
			name:        "with kube-context",
			kubeContext: "test-context",
			expected:    true,
		},
		{
			name:        "without kube-context",
			kubeContext: "",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test diffCmd struct
			d := &diffCmd{
				kubeContext: tt.kubeContext,
				namespace:   "test-namespace",
			}

			// Verify the field is set correctly
			if (d.kubeContext != "") != tt.expected {
				t.Errorf("kubeContext field: expected %v, got %v", tt.expected, d.kubeContext != "")
			}

			// Test release struct
			r := &release{
				kubeContext: tt.kubeContext,
			}

			if (r.kubeContext != "") != tt.expected {
				t.Errorf("release kubeContext field: expected %v, got %v", tt.expected, r.kubeContext != "")
			}

			// Test revision struct
			rev := &revision{
				kubeContext: tt.kubeContext,
			}

			if (rev.kubeContext != "") != tt.expected {
				t.Errorf("revision kubeContext field: expected %v, got %v", tt.expected, rev.kubeContext != "")
			}

			// Test rollback struct
			rb := &rollback{
				kubeContext: tt.kubeContext,
			}

			if (rb.kubeContext != "") != tt.expected {
				t.Errorf("rollback kubeContext field: expected %v, got %v", tt.expected, rb.kubeContext != "")
			}
		})
	}
}

func TestKubeContextInDiffCmd(t *testing.T) {
	// Test that the diffCmd has the kubeContext field in the struct
	d := diffCmd{
		kubeContext: "test-context",
		namespace:   "test-namespace",
		release:     "test-release",
		chart:       "test-chart",
	}

	if d.kubeContext != "test-context" {
		t.Errorf("Expected kubeContext to be 'test-context', got '%s'", d.kubeContext)
	}

	// Test that the field can be set to empty
	d.kubeContext = ""
	if d.kubeContext != "" {
		t.Errorf("Expected kubeContext to be empty, got '%s'", d.kubeContext)
	}
}
