# Main Azure resource group
resource "azurerm_resource_group" "main" {
  name     = "rg-production"
  location = "eastus2"
}

# Azure virtual network
resource "azurerm_virtual_network" "vnet" {
  name                = "vnet-prod"
  resource_group_name = "rg-production"
  address_space       = ["10.0.0.0/16"]
  location            = "eastus2"
}
