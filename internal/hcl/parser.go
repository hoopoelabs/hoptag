package hcl

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	hclbase "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

var ignoredDirNames = map[string]struct{}{
	".terraform":   {},
	".git":         {},
	".hoptag":      {},
	"node_modules": {},
}

// DiscoverFiles finds all applicable Terraform configuration files in a target directory.
func DiscoverFiles(rootDir string, recursive bool) ([]string, error) {
	info, err := os.Stat(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to access directory %q: %w", rootDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path %q is not a directory", rootDir)
	}

	var files []string

	if !recursive {
		entries, err := os.ReadDir(rootDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory %q: %w", rootDir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if isCandidateFile(name) {
				files = append(files, filepath.Join(rootDir, name))
			}
		}
	} else {
		err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				name := d.Name()
				if _, ignored := ignoredDirNames[name]; ignored {
					return filepath.SkipDir
				}
				// Skip hidden dirs except root "."
				if strings.HasPrefix(name, ".") && path != rootDir && name != "." {
					return filepath.SkipDir
				}
				return nil
			}

			if isCandidateFile(d.Name()) {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("error scanning directory %q: %w", rootDir, err)
		}
	}

	sort.Strings(files)
	return files, nil
}

// isCandidateFile returns true if the file is a standard .tf file and not an override/backup file.
func isCandidateFile(name string) bool {
	if !strings.HasSuffix(name, ".tf") {
		return false
	}
	// Avoid hoptag artifacts, backup, and override files
	if strings.HasSuffix(name, "_override.tf") ||
		strings.HasSuffix(name, ".hoptag.tf") ||
		strings.HasSuffix(name, ".bak") ||
		strings.HasPrefix(name, "_hoptag_") {
		return false
	}
	return true
}

// LoadFile reads and parses an HCL file into an AST using hclwrite.
func LoadFile(filePath string) (*hclwrite.File, []byte, error) {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read %q: %w", filePath, err)
	}

	file, diags := hclwrite.ParseConfig(src, filePath, hclbase.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, nil, fmt.Errorf("failed to parse HCL in %q: %s", filePath, diags.Error())
	}

	return file, src, nil
}
