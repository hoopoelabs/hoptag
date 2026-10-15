package providers

import (
	"strings"
	"sync"
)

// Provider defines the interface for cloud IaC resource tagging rules.
type Provider interface {
	// Name returns the friendly name of the cloud provider (e.g. "AWS", "Azure", "GCP").
	Name() string

	// Prefix returns the Terraform resource type prefix (e.g. "aws_", "azurerm_", "google_").
	Prefix() string

	// CanTag reports whether the specified resource type can accept tags or labels.
	CanTag(resourceType string) bool

	// TagAttributeName returns the attribute key used for tags/labels (e.g. "tags" or "labels").
	TagAttributeName(resourceType string) string
}

// Registry maintains the collection of registered cloud providers.
type Registry struct {
	mu        sync.RWMutex
	providers []Provider
}

// NewRegistry creates a new empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make([]Provider, 0),
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, p)
}

// Detect returns the provider that matches the resource type prefix, if any.
func (r *Registry) Detect(resourceType string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.providers {
		if strings.HasPrefix(resourceType, p.Prefix()) {
			return p, true
		}
	}
	return nil, false
}

// All returns all registered providers.
func (r *Registry) All() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]Provider, len(r.providers))
	copy(res, r.providers)
	return res
}

// DefaultRegistry creates and initializes a registry with AWS, Azure, and GCP providers.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(NewAWSProvider())
	r.Register(NewAzureProvider())
	r.Register(NewGCPProvider())
	return r
}
