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

	// MaxTags returns the maximum allowed tag/label count for the resource (e.g. 10 for aws_s3_object).
	MaxTags(resourceType string) int
}

// Registry maintains the collection of registered cloud providers.
type Registry struct {
	mu             sync.RWMutex
	providers      []Provider
	customLimits   map[string]int
	customExcludes map[string]struct{}
}

// NewRegistry creates a new empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers:      make([]Provider, 0),
		customLimits:   make(map[string]int),
		customExcludes: make(map[string]struct{}),
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, p)
}

// SetLimit overrides or defines a custom tag count limit for a specific resource type.
func (r *Registry) SetLimit(resourceType string, limit int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.customLimits[resourceType] = limit
}

// SetExclude marks a resource type as untaggable.
func (r *Registry) SetExclude(resourceType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.customExcludes[resourceType] = struct{}{}
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

// CanTagResource evaluates whether a resource is eligible for tagging.
func (r *Registry) CanTagResource(p Provider, resourceType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, blocked := r.customExcludes[resourceType]; blocked {
		return false
	}
	return p.CanTag(resourceType)
}

// GetMaxTags returns the maximum allowed tags for a resource, honoring overrides.
func (r *Registry) GetMaxTags(p Provider, resourceType string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if custom, ok := r.customLimits[resourceType]; ok && custom > 0 {
		return custom
	}
	return p.MaxTags(resourceType)
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

// GenericProvider is a configurable provider created from .hoptag.yaml.
type GenericProvider struct {
	PName      string
	PPrefix    string
	PAttribute string
	PMaxTags   int
	PExclude   map[string]struct{}
}

func (g *GenericProvider) Name() string                     { return g.PName }
func (g *GenericProvider) Prefix() string                   { return g.PPrefix }
func (g *GenericProvider) TagAttributeName(r string) string { return g.PAttribute }
func (g *GenericProvider) MaxTags(r string) int {
	if g.PMaxTags > 0 {
		return g.PMaxTags
	}
	return 50
}
func (g *GenericProvider) CanTag(resourceType string) bool {
	if !strings.HasPrefix(resourceType, g.PPrefix) {
		return false
	}
	_, blocked := g.PExclude[resourceType]
	return !blocked
}
