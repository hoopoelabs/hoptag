package hcl

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hoopoelabs/hoptag/internal/models"
	"github.com/hoopoelabs/hoptag/internal/providers"
	"github.com/zclconf/go-cty/cty"
)

const (
	OverrideFileName = "_hoptag_override.tf"
	HeaderComment    = "// Generated automatically by hoptag (Hoopoe Labs). DO NOT EDIT MANUALLY.\n\n"
)

// Writer manages writing modified AST files based on the chosen mode.
type Writer struct {
	Mode string
}

// NewWriter creates a new Writer with the designated mode.
func NewWriter(mode string) *Writer {
	if mode == "" {
		mode = models.ModeInPlace
	}
	return &Writer{
		Mode: mode,
	}
}

// WriteFile writes the modified file or generates override/backup files.
func (w *Writer) WriteFile(filePath string, file *hclwrite.File) error {
	formattedBytes := hclwrite.Format(file.Bytes())

	switch w.Mode {
	case models.ModeInPlace:
		return writeAtomic(filePath, formattedBytes)

	case models.ModeRename:
		// Backup original to .bak
		bakPath := filePath + ".bak"
		if err := copyFile(filePath, bakPath); err != nil {
			return fmt.Errorf("failed to backup original file to %q: %w", bakPath, err)
		}
		// Write modified to <name>.hoptag.tf
		dir := filepath.Dir(filePath)
		base := filepath.Base(filePath)
		ext := filepath.Ext(base)
		nameOnly := base[:len(base)-len(ext)]
		hoptagPath := filepath.Join(dir, nameOnly+".hoptag.tf")
		return writeAtomic(hoptagPath, formattedBytes)

	case models.ModeOverride:
		// Override mode handled per directory by WriteOverrideFile
		return nil

	default:
		return fmt.Errorf("unsupported write mode %q", w.Mode)
	}
}

// WriteOverrideFile generates a Terraform-native _hoptag_override.tf file for resources in a directory.
func WriteOverrideFile(
	dir string,
	files []*hclwrite.File,
	opts models.ApplyOptions,
	registry *providers.Registry,
) (string, int, error) {
	overrideFile := hclwrite.NewEmptyFile()
	overrideBody := overrideFile.Body()
	taggedCount := 0

	ctyMap := make(map[string]cty.Value, len(opts.Tags))
	for k, v := range opts.Tags {
		ctyMap[k] = cty.StringVal(v)
	}
	tagVal := cty.ObjectVal(ctyMap)

	for _, file := range files {
		for _, block := range file.Body().Blocks() {
			if block.Type() != "resource" {
				continue
			}
			labels := block.Labels()
			if len(labels) < 2 {
				continue
			}
			resourceType := labels[0]
			resourceName := labels[1]

			if opts.FilterRegex != nil && !opts.FilterRegex.MatchString(resourceType) {
				continue
			}
			if opts.SkipRegex != nil && opts.SkipRegex.MatchString(resourceType) {
				continue
			}

			provider, found := registry.Detect(resourceType)
			if !found || !provider.CanTag(resourceType) {
				continue
			}

			attrName := provider.TagAttributeName(resourceType)

			// Create a partial resource block in the override file
			resBlock := overrideBody.AppendNewBlock("resource", []string{resourceType, resourceName})
			resBlock.Body().SetAttributeValue(attrName, tagVal)
			taggedCount++
		}
	}

	if taggedCount == 0 {
		return "", 0, nil
	}

	outputPath := filepath.Join(dir, OverrideFileName)
	content := []byte(HeaderComment + string(hclwrite.Format(overrideFile.Bytes())))
	if err := writeAtomic(outputPath, content); err != nil {
		return "", 0, fmt.Errorf("failed to write override file %q: %w", outputPath, err)
	}

	return outputPath, taggedCount, nil
}

func writeAtomic(filePath string, data []byte) error {
	dir := filepath.Dir(filePath)
	tmpFile, err := os.CreateTemp(dir, ".hoptag-tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write data: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Rename temp file to target path
	if err := os.Rename(tmpName, filePath); err != nil {
		return fmt.Errorf("failed to atomically write file %q: %w", filePath, err)
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
