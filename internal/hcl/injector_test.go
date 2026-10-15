package hcl

import (
	"regexp"
	"strings"
	"testing"

	hclbase "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hoopoelabs/hoptag/internal/models"
	"github.com/hoopoelabs/hoptag/internal/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInjectTagsUntaggedResources(t *testing.T) {
	src := `
resource "aws_instance" "web" {
  ami           = "ami-123456"
  instance_type = "t3.micro"
}

resource "azurerm_resource_group" "rg" {
  name     = "rg-test"
  location = "eastus"
}

resource "google_compute_instance" "vm" {
  name         = "gcp-vm"
  machine_type = "e2-medium"
}
`
	file, diags := hclwrite.ParseConfig([]byte(src), "test.tf", hclbase.Pos{Line: 1, Column: 1})
	require.False(t, diags.HasErrors())

	opts := models.ApplyOptions{
		Tags: models.TagMap{
			"Environment": "production",
			"CostCenter":  "FIN-104",
		},
	}
	reg := providers.DefaultRegistry()

	res, err := InjectTags(file, "test.tf", opts, reg)
	require.NoError(t, err)
	assert.Equal(t, 3, res.ResourcesTagged)
	assert.True(t, res.Modified)

	out := string(hclwrite.Format(file.Bytes()))

	// AWS should have tags attribute
	assert.Contains(t, out, `resource "aws_instance" "web"`)
	assert.Contains(t, out, "tags = {")
	assert.Contains(t, out, `Environment = "production"`)
	assert.Contains(t, out, `CostCenter  = "FIN-104"`)

	// Azure should have tags attribute
	assert.Contains(t, out, `resource "azurerm_resource_group" "rg"`)

	// GCP should have labels attribute
	assert.Contains(t, out, `resource "google_compute_instance" "vm"`)
	assert.Contains(t, out, "labels = {")
}

func TestInjectTagsExistingMapAndComments(t *testing.T) {
	src := `
resource "aws_instance" "db" {
  ami           = "ami-99999"
  instance_type = "m5.large"

  # Important database tags
  tags = {
    # Keep this custom tag!
    BackupPlan  = "Daily"
    Environment = "dev" # inline comment
  }
}
`
	file, diags := hclwrite.ParseConfig([]byte(src), "test.tf", hclbase.Pos{Line: 1, Column: 1})
	require.False(t, diags.HasErrors())

	opts := models.ApplyOptions{
		Tags: models.TagMap{
			"Environment": "production",
			"CostCenter":  "FIN-104",
		},
		PreserveExisting: false,
	}
	reg := providers.DefaultRegistry()

	res, err := InjectTags(file, "test.tf", opts, reg)
	require.NoError(t, err)
	assert.Equal(t, 1, res.ResourcesTagged)

	out := string(hclwrite.Format(file.Bytes()))

	// Check that custom tags and comments are preserved
	assert.Contains(t, out, "# Important database tags")
	assert.Contains(t, out, "# Keep this custom tag!")
	assert.Regexp(t, regexp.MustCompile(`BackupPlan\s*=\s*"Daily"`), out)
	assert.Contains(t, out, "# inline comment")

	// Check that Environment was updated to production
	assert.Regexp(t, regexp.MustCompile(`Environment\s*=\s*"production"`), out)
	assert.NotContains(t, out, `"dev"`)

	// Check that CostCenter was added
	assert.Regexp(t, regexp.MustCompile(`CostCenter\s*=\s*"FIN-104"`), out)
}

func TestInjectTagsPreserveExisting(t *testing.T) {
	src := `
resource "aws_instance" "db" {
  ami = "ami-99999"
  tags = {
    Environment = "dev"
  }
}
`
	file, diags := hclwrite.ParseConfig([]byte(src), "test.tf", hclbase.Pos{Line: 1, Column: 1})
	require.False(t, diags.HasErrors())

	opts := models.ApplyOptions{
		Tags: models.TagMap{
			"Environment": "production",
			"CostCenter":  "FIN-104",
		},
		PreserveExisting: true,
	}
	reg := providers.DefaultRegistry()

	res, err := InjectTags(file, "test.tf", opts, reg)
	require.NoError(t, err)
	assert.Equal(t, 1, res.ResourcesTagged)

	out := string(hclwrite.Format(file.Bytes()))

	// With preserveExisting=true, Environment should remain "dev"
	assert.Contains(t, out, `Environment = "dev"`)
	assert.NotContains(t, out, `Environment = "production"`)
	// New tag should still be added
	assert.Contains(t, out, `CostCenter  = "FIN-104"`)
}

func TestInjectTagsDynamicExpression(t *testing.T) {
	src := `
resource "aws_instance" "app" {
  ami  = "ami-123"
  tags = var.common_tags
}
`
	file, diags := hclwrite.ParseConfig([]byte(src), "test.tf", hclbase.Pos{Line: 1, Column: 1})
	require.False(t, diags.HasErrors())

	opts := models.ApplyOptions{
		Tags: models.TagMap{
			"Environment": "production",
		},
	}
	reg := providers.DefaultRegistry()

	res, err := InjectTags(file, "test.tf", opts, reg)
	require.NoError(t, err)
	assert.Equal(t, 1, res.ResourcesTagged)

	out := string(hclwrite.Format(file.Bytes()))
	assert.Contains(t, out, "tags = merge(var.common_tags, {")
	assert.Contains(t, out, `Environment = "production"`)
}

func TestInjectTagsFilters(t *testing.T) {
	src := `
resource "aws_instance" "web" {
  ami = "ami-123"
}

resource "aws_s3_bucket" "b" {
  bucket = "test"
}
`
	file, diags := hclwrite.ParseConfig([]byte(src), "test.tf", hclbase.Pos{Line: 1, Column: 1})
	require.False(t, diags.HasErrors())

	opts := models.ApplyOptions{
		Tags: models.TagMap{
			"Environment": "staging",
		},
		FilterRegex: regexp.MustCompile("^aws_s3_bucket$"),
	}
	reg := providers.DefaultRegistry()

	res, err := InjectTags(file, "test.tf", opts, reg)
	require.NoError(t, err)
	assert.Equal(t, 1, res.ResourcesTagged)

	out := string(hclwrite.Format(file.Bytes()))
	// s3 bucket tagged
	assert.True(t, strings.Contains(out, `bucket = "test"`))
	// aws_instance not tagged
	webBlock := file.Body().Blocks()[0]
	assert.Nil(t, webBlock.Body().GetAttribute("tags"))
}
