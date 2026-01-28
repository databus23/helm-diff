package diff

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aryann/difflib"
	"github.com/stretchr/testify/require"
)

func TestLoadFromKey(t *testing.T) {
	keyToReportTemplateSpec := map[string]ReportTemplateSpec{
		"default, nginx, Deployment (apps)": {
			Namespace: "default",
			Name:      "nginx",
			Kind:      "Deployment",
			API:       "apps",
		},
		"default, probes.monitoring.coreos.com, CustomResourceDefinition (apiextensions.k8s.io)": {
			Namespace: "default",
			Name:      "probes.monitoring.coreos.com",
			Kind:      "CustomResourceDefinition",
			API:       "apiextensions.k8s.io",
		},
		"default, my-cert, Certificate (cert-manager.io/v1)": {
			Namespace: "default",
			Name:      "my-cert",
			Kind:      "Certificate",
			API:       "cert-manager.io/v1",
		},
	}

	for key, expectedTemplateSpec := range keyToReportTemplateSpec {
		templateSpec := &ReportTemplateSpec{}
		require.NoError(t, templateSpec.loadFromKey(key))
		require.Equal(t, expectedTemplateSpec, *templateSpec)
	}
}

func TestPrintAIReport(t *testing.T) {
	tests := []struct {
		name     string
		report   *Report
		expected string
	}{
		{
			name: "single entry with modify diff",
			report: &Report{
				Entries: []ReportEntry{
					{
						Key:        "default, nginx, Deployment (apps)",
						ChangeType: "MODIFY",
						Diffs: []difflib.DiffRecord{
							{Delta: difflib.Common, Payload: "spec:"},
							{Delta: difflib.RightOnly, Payload: "  replicas: 3"},
							{Delta: difflib.LeftOnly, Payload: "  replicas: 2"},
						},
					},
				},
			},
			expected: `[
  {
    "change": "MODIFY",
    "summary": "Modified: +1, -1",
    "metadata": {
      "namespace": "default",
      "name": "nginx",
      "kind": "Deployment",
      "api": "apps"
    },
    "content": {
      "added": [
        "  replicas: 3"
      ],
      "removed": [
        "  replicas: 2"
      ],
      "modified": [
        "spec:"
      ]
    }
  }
]`,
		},
		{
			name: "multiple entries",
			report: &Report{
				Entries: []ReportEntry{
					{
						Key:        "default, nginx, Deployment (apps)",
						ChangeType: "ADD",
						Diffs:      []difflib.DiffRecord{{Delta: difflib.RightOnly, Payload: "spec:"}},
					},
					{
						Key:        "default, redis, Service (v1)",
						ChangeType: "REMOVE",
						Diffs:      []difflib.DiffRecord{{Delta: difflib.LeftOnly, Payload: "spec:"}},
					},
				},
			},
			expected: `[
  {
    "change": "ADD",
    "summary": "Added 1 lines",
    "metadata": {
      "namespace": "default",
      "name": "nginx",
      "kind": "Deployment",
      "api": "apps"
    },
    "content": {
      "added": [
        "spec:"
      ]
    }
  },
  {
    "change": "REMOVE",
    "summary": "Removed 1 lines",
    "metadata": {
      "namespace": "default",
      "name": "redis",
      "kind": "Service",
      "api": "v1"
    },
    "content": {
      "removed": [
        "spec:"
      ]
    }
  }
]`,
		},
		{
			name:     "empty report",
			report:   &Report{Entries: []ReportEntry{}},
			expected: "[]\n",
		},
		{
			name: "entry with special characters in content",
			report: &Report{
				Entries: []ReportEntry{
					{
						Key:        "default, test-config, ConfigMap (v1)",
						ChangeType: "MODIFY",
						Diffs: []difflib.DiffRecord{
							{Delta: difflib.RightOnly, Payload: `key: "value with \"quotes\""`},
							{Delta: difflib.RightOnly, Payload: "another: new\nline"},
						},
					},
				},
			},
			expected: `[
  {
    "change": "MODIFY",
    "summary": "Modified: +2, -0",
    "metadata": {
      "namespace": "default",
      "name": "test-config",
      "kind": "ConfigMap",
      "api": "v1"
    },
    "content": {
      "added": [
        "key: \"value with \\\"quotes\\\"\"",
        "another: new\nline"
      ]
    }
  }
]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printAIReport(tt.report, &buf)

			actual := strings.TrimSpace(buf.String())
			expected := strings.TrimSpace(tt.expected)

			var actualJSON, expectedJSON interface{}
			require.NoError(t, json.Unmarshal([]byte(actual), &actualJSON))
			require.NoError(t, json.Unmarshal([]byte(expected), &expectedJSON))

			require.Equal(t, expectedJSON, actualJSON)
		})
	}
}
