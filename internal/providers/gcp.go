package providers

import (
	"strings"
)

// GCPProvider handles Google Cloud Platform resource labeling.
type GCPProvider struct {
	untaggableTypes map[string]struct{}
}

// NewGCPProvider creates an instance of GCPProvider.
func NewGCPProvider() *GCPProvider {
	untaggable := map[string]struct{}{
		"google_project_service":               {},
		"google_project_iam_member":            {},
		"google_project_iam_binding":           {},
		"google_project_iam_policy":            {},
		"google_service_account_iam_member":    {},
		"google_service_account_iam_binding":   {},
		"google_compute_subnetwork_iam_member": {},
		"google_compute_firewall":              {},
	}
	return &GCPProvider{
		untaggableTypes: untaggable,
	}
}

func (p *GCPProvider) Name() string {
	return "GCP"
}

func (p *GCPProvider) Prefix() string {
	return "google_"
}

func (p *GCPProvider) CanTag(resourceType string) bool {
	if !strings.HasPrefix(resourceType, p.Prefix()) {
		return false
	}
	_, blocked := p.untaggableTypes[resourceType]
	return !blocked
}

func (p *GCPProvider) TagAttributeName(resourceType string) string {
	return "labels"
}

func (p *GCPProvider) MaxTags(resourceType string) int {
	return 64 // Standard GCP label limit
}
