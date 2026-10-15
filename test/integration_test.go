package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hoopoelabs/hoptag/internal/hcl"
	"github.com/hoopoelabs/hoptag/internal/models"
	"github.com/hoopoelabs/hoptag/internal/providers"
	"github.com/hoopoelabs/hoptag/internal/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, entry.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dst, entry.Name()), data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func TestEndToEndApplyAndCheck(t *testing.T) {
	// 1. Prepare temporary directory with fixture files
	tmpDir := t.TempDir()
	fixturesDir := filepath.Join(".", "fixtures")
	err := copyDir(fixturesDir, tmpDir)
	require.NoError(t, err)

	reg := providers.DefaultRegistry()

	// 2. Initial Governance Check: should fail on untagged fixtures
	checkRules, err := models.ParseRules("Environment=^(dev|staging|production)$,CostCenter=^CC-[0-9]{4}$")
	require.NoError(t, err)

	checkOpts := models.CheckOptions{
		Dir:       tmpDir,
		Rules:     checkRules,
		Recursive: true,
	}

	files, err := hcl.DiscoverFiles(tmpDir, true)
	require.NoError(t, err)
	require.NotEmpty(t, files)

	var initialViolations []models.Violation
	for _, f := range files {
		fileAST, _, err := hcl.LoadFile(f)
		require.NoError(t, err)
		v, _, _, err := validator.ValidateFile(fileAST, f, checkOpts, reg)
		require.NoError(t, err)
		initialViolations = append(initialViolations, v...)
	}
	assert.NotEmpty(t, initialViolations, "Untagged fixtures must fail initial check")

	// 3. Run Apply to inject tags
	tags, err := models.ParseTags("Environment=production,CostCenter=CC-9999,ManagedBy=hoptag")
	require.NoError(t, err)

	applyOpts := models.ApplyOptions{
		Dir:              tmpDir,
		Tags:             tags,
		Mode:             models.ModeInPlace,
		PreserveExisting: false,
		Recursive:        true,
	}

	writer := hcl.NewWriter(models.ModeInPlace)
	for _, f := range files {
		fileAST, _, err := hcl.LoadFile(f)
		require.NoError(t, err)
		res, err := hcl.InjectTags(fileAST, f, applyOpts, reg)
		require.NoError(t, err)
		if res.Modified {
			err = writer.WriteFile(f, fileAST)
			require.NoError(t, err)
		}
	}

	// 4. Re-run Governance Check: should pass now!
	var postViolations []models.Violation
	postFiles, err := hcl.DiscoverFiles(tmpDir, true)
	require.NoError(t, err)

	for _, f := range postFiles {
		fileAST, _, err := hcl.LoadFile(f)
		require.NoError(t, err)
		v, _, _, err := validator.ValidateFile(fileAST, f, checkOpts, reg)
		require.NoError(t, err)
		for _, violation := range v {
			if violation.Type != models.ViolationDynamicTag {
				postViolations = append(postViolations, violation)
			}
		}
	}
	assert.Empty(t, postViolations, "All resources should be compliant after hoptag apply")

	// 5. Verify GCP uses labels
	gcpPath := filepath.Join(tmpDir, "gcp_untagged.tf")
	gcpContent, err := os.ReadFile(gcpPath)
	require.NoError(t, err)
	assert.Contains(t, string(gcpContent), "labels = {")
	assert.Contains(t, string(gcpContent), `CostCenter  = "CC-9999"`)

	// 6. Verify AWS comments are preserved
	existingPath := filepath.Join(tmpDir, "existing_tags_with_comments.tf")
	existingContent, err := os.ReadFile(existingPath)
	require.NoError(t, err)
	assert.Contains(t, string(existingContent), "# Keep this custom tag!")
	assert.Contains(t, string(existingContent), `BackupPlan  = "Daily"`)
}
