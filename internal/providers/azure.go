package providers

import (
	"strings"
)

// AzureProvider handles Microsoft Azure resource tagging.
type AzureProvider struct {
	untaggableTypes map[string]struct{}
}

// NewAzureProvider creates an instance of AzureProvider.
func NewAzureProvider() *AzureProvider {
	untaggable := map[string]struct{}{
		"azurerm_resource_group_policy_assignment":                                       {},
		"azurerm_subnet_network_security_group_association":                              {},
		"azurerm_subnet_route_table_association":                                         {},
		"azurerm_role_assignment":                                                        {},
		"azurerm_role_definition":                                                        {},
		"azurerm_network_interface_security_group_association":                           {},
		"azurerm_network_interface_application_gateway_backend_address_pool_association": {},
	}
	return &AzureProvider{
		untaggableTypes: untaggable,
	}
}

func (p *AzureProvider) Name() string {
	return "Azure"
}

func (p *AzureProvider) Prefix() string {
	return "azurerm_"
}

func (p *AzureProvider) CanTag(resourceType string) bool {
	if !strings.HasPrefix(resourceType, p.Prefix()) {
		return false
	}
	_, blocked := p.untaggableTypes[resourceType]
	return !blocked
}

func (p *AzureProvider) TagAttributeName(resourceType string) string {
	return "tags"
}

func (p *AzureProvider) MaxTags(resourceType string) int {
	return 50 // Standard Azure resource limit
}
