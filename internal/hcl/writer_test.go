package hcl

import (
	"os"
	"path/filepath"
	"testing"

	hclbase "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hoopoelabs/hoptag/internal/models"
	"github.com/hoopoelabs/hoptag/internal/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriterInPlace(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "main.tf")
	err := os.WriteFile(filePath, []byte("resource \"aws_instance\" \"a\" {}\n"), 0644)
	require.NoError(t, err)

	file, diags := hclwrite.ParseConfig([]byte("resource \"aws_instance\" \"a\" {\n  tags = {\n    Env = \"prod\"\n  }\n}\n"), "main.tf", hclbase.Pos{Line: 1, Column: 1})
	require.False(t, diags.HasErrors())

	w := NewWriter(models.ModeInPlace)
	err = w.WriteFile(filePath, file)
	require.NoError(t, err)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "Env = \"prod\"")
}

func TestWriterRename(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "main.tf")
	err := os.WriteFile(filePath, []byte("resource \"aws_instance\" \"a\" {}\n"), 0644)
	require.NoError(t, err)

	file, diags := hclwrite.ParseConfig([]byte("resource \"aws_instance\" \"a\" {\n  tags = {\n    Env = \"prod\"\n  }\n}\n"), "main.tf", hclbase.Pos{Line: 1, Column: 1})
	require.False(t, diags.HasErrors())

	w := NewWriter(models.ModeRename)
	err = w.WriteFile(filePath, file)
	require.NoError(t, err)

	// Original should be backed up as main.tf.bak
	bakContent, err := os.ReadFile(filePath + ".bak")
	require.NoError(t, err)
	assert.Equal(t, "resource \"aws_instance\" \"a\" {}\n", string(bakContent))

	// main.hoptag.tf should have modified content
	hoptagContent, err := os.ReadFile(filepath.Join(tmpDir, "main.hoptag.tf"))
	require.NoError(t, err)
	assert.Contains(t, string(hoptagContent), "Env = \"prod\"")
}

func TestWriteOverrideFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := `
resource "aws_instance" "web" {
  ami = "ami-123"
}
resource "azurerm_resource_group" "rg" {
  name = "rg1"
}
`
	file, diags := hclwrite.ParseConfig([]byte(src), "main.tf", hclbase.Pos{Line: 1, Column: 1})
	require.False(t, diags.HasErrors())

	opts := models.ApplyOptions{
		Tags: models.TagMap{
			"Environment": "production",
			"ManagedBy":   "hoptag",
		},
	}
	reg := providers.DefaultRegistry()

	outPath, count, err := WriteOverrideFile(tmpDir, []*hclwrite.File{file}, opts, reg)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, filepath.Join(tmpDir, "_hoptag_override.tf"), outPath)

	content, err := os.ReadFile(outPath)
	require.NoError(t, err)
	outStr := string(content)

	assert.Contains(t, outStr, "Generated automatically by hoptag")
	assert.Contains(t, outStr, `resource "aws_instance" "web"`)
	assert.Contains(t, outStr, `resource "azurerm_resource_group" "rg"`)
	assert.Contains(t, outStr, `Environment = "production"`)
	assert.Contains(t, outStr, `ManagedBy   = "hoptag"`)
}
