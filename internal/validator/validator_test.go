package validator

import (
	"regexp"
	"testing"

	hclbase "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hoopoelabs/hoptag/internal/models"
	"github.com/hoopoelabs/hoptag/internal/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateFile(t *testing.T) {
	src := `
resource "aws_instance" "compliant" {
  ami = "ami-123"
  tags = {
    Environment = "production"
    CostCenter  = "CC-1234"
  }
}

resource "aws_instance" "invalid_value" {
  ami = "ami-456"
  tags = {
    Environment = "invalid_env"
    CostCenter  = "CC-1234"
  }
}

resource "aws_instance" "missing_tag" {
  ami = "ami-789"
  tags = {
    Environment = "production"
  }
}

resource "aws_instance" "no_tags_at_all" {
  ami = "ami-000"
}
`
	file, diags := hclwrite.ParseConfig([]byte(src), "test.tf", hclbase.Pos{Line: 1, Column: 1})
	require.False(t, diags.HasErrors())

	opts := models.CheckOptions{
		Rules: []models.TagRule{
			{
				Key:     "Environment",
				Pattern: regexp.MustCompile("^(dev|staging|production)$"),
				Raw:     "Environment=^(dev|staging|production)$",
			},
			{
				Key:     "CostCenter",
				Pattern: regexp.MustCompile("^CC-[0-9]{4}$"),
				Raw:     "CostCenter=^CC-[0-9]{4}$",
			},
		},
	}
	reg := providers.DefaultRegistry()

	violations, total, compliant, err := ValidateFile(file, "test.tf", opts, reg)
	require.NoError(t, err)

	assert.Equal(t, 4, total)
	assert.Equal(t, 1, compliant) // only the first one is fully compliant
	require.Len(t, violations, 4)

	// Violation 1: invalid_value Environment
	assert.Equal(t, "invalid_value", violations[0].ResourceName)
	assert.Equal(t, models.ViolationInvalidValue, violations[0].Type)
	assert.Equal(t, "Environment", violations[0].TagKey)

	// Violation 2: missing_tag CostCenter
	assert.Equal(t, "missing_tag", violations[1].ResourceName)
	assert.Equal(t, models.ViolationMissingTag, violations[1].Type)
	assert.Equal(t, "CostCenter", violations[1].TagKey)

	// Violations 3 & 4: no_tags_at_all Environment & CostCenter
	assert.Equal(t, "no_tags_at_all", violations[2].ResourceName)
	assert.Equal(t, models.ViolationMissingTag, violations[2].Type)
	assert.Equal(t, "no_tags_at_all", violations[3].ResourceName)
	assert.Equal(t, models.ViolationMissingTag, violations[3].Type)
}
