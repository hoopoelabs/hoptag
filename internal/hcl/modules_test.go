package hcl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverModuleFiles(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Without manifest, should return nil, nil, nil
	files, names, err := DiscoverModuleFiles(tempDir)
	assert.NoError(t, err)
	assert.Nil(t, files)
	assert.Nil(t, names)

	// 2. Setup mock .terraform/modules/modules.json and dummy module folders
	modulesDir := filepath.Join(tempDir, ".terraform", "modules")
	require.NoError(t, os.MkdirAll(modulesDir, 0755))

	vpcDir := filepath.Join(modulesDir, "vpc")
	require.NoError(t, os.MkdirAll(vpcDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(vpcDir, "main.tf"), []byte(`resource "aws_vpc" "mod" {}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(vpcDir, "variables.tf"), []byte(`variable "cidr" {}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(vpcDir, "README.md"), []byte(`# VPC module`), 0644))

	manifestContent := `{
		"Modules": [
			{
				"Key": "",
				"Source": "",
				"Dir": "."
			},
			{
				"Key": "vpc",
				"Source": "terraform-aws-modules/vpc/aws",
				"Version": "5.0.0",
				"Dir": ".terraform/modules/vpc"
			}
		]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(modulesDir, "modules.json"), []byte(manifestContent), 0644))

	// 3. Discover should find vpc .tf files and module name "vpc"
	files, names, err = DiscoverModuleFiles(tempDir)
	assert.NoError(t, err)
	assert.Equal(t, []string{"vpc"}, names)
	assert.Len(t, files, 2)

	fileNames := []string{filepath.Base(files[0]), filepath.Base(files[1])}
	assert.Contains(t, fileNames, "main.tf")
	assert.Contains(t, fileNames, "variables.tf")
}
