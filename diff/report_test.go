package diff

import (
	"testing"

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
