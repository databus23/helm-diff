package manifest

import (
	rspb "helm.sh/helm/pkg/release"
	rel "k8s.io/helm/pkg/proto/hapi/release"
)

// ReleaseResponse represents a deployed release revision
type ReleaseResponse struct {
	Release   *rel.Release
	ReleaseV3 *rspb.Release
}

// ChartName returns the chart name
func (release ReleaseResponse) ChartName() string {
	if release.Release == nil {
		return release.ReleaseV3.Chart.Metadata.Name
	}
	return release.Release.Chart.Metadata.Name
}
