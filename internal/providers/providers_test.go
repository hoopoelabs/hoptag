package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultRegistry(t *testing.T) {
	reg := DefaultRegistry()
	require.NotNil(t, reg)

	// Test AWS detection
	awsProv, found := reg.Detect("aws_instance")
	assert.True(t, found)
	assert.Equal(t, "AWS", awsProv.Name())
	assert.Equal(t, "tags", awsProv.TagAttributeName("aws_instance"))
	assert.True(t, awsProv.CanTag("aws_instance"))
	assert.True(t, awsProv.CanTag("aws_s3_bucket"))
	assert.False(t, awsProv.CanTag("aws_route"))

	// Test Azure detection
	azureProv, found := reg.Detect("azurerm_resource_group")
	assert.True(t, found)
	assert.Equal(t, "Azure", azureProv.Name())
	assert.Equal(t, "tags", azureProv.TagAttributeName("azurerm_resource_group"))
	assert.True(t, azureProv.CanTag("azurerm_resource_group"))
	assert.False(t, azureProv.CanTag("azurerm_role_assignment"))

	// Test GCP detection
	gcpProv, found := reg.Detect("google_compute_instance")
	assert.True(t, found)
	assert.Equal(t, "GCP", gcpProv.Name())
	assert.Equal(t, "labels", gcpProv.TagAttributeName("google_compute_instance"))
	assert.True(t, gcpProv.CanTag("google_compute_instance"))
	assert.False(t, gcpProv.CanTag("google_project_iam_member"))

	// Test unknown resource
	_, found = reg.Detect("random_pet")
	assert.False(t, found)
}
