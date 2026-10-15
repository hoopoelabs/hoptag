package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the optional .hoptag.yaml or .hoptag.json configuration file.
type Config struct {
	// Limits defines maximum allowed tag counts per resource type (e.g. aws_s3_object: 10)
	Limits map[string]int `json:"limits" yaml:"limits"`

	// Exclude lists resource types that should never be tagged
	Exclude []string `json:"exclude" yaml:"exclude"`

	// Providers defines custom third-party providers
	Providers []CustomProviderConfig `json:"providers" yaml:"providers"`
}

// CustomProviderConfig defines user-configured providers in .hoptag.yaml
type CustomProviderConfig struct {
	Name      string   `json:"name" yaml:"name"`
	Prefix    string   `json:"prefix" yaml:"prefix"`
	Attribute string   `json:"attribute" yaml:"attribute"`
	MaxTags   int      `json:"max_tags" yaml:"max_tags"`
	Exclude   []string `json:"exclude" yaml:"exclude"`
}

// FindAndLoadConfig looks for .hoptag.yaml, .hoptag.yml, or .hoptag.json in the specified directory.
func FindAndLoadConfig(dir string) (*Config, string, error) {
	candidates := []string{
		filepath.Join(dir, ".hoptag.yaml"),
		filepath.Join(dir, ".hoptag.yml"),
		filepath.Join(dir, ".hoptag.json"),
		filepath.Join(dir, "hoptag.yaml"),
		filepath.Join(dir, "hoptag.json"),
	}

	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			cfg, err := LoadConfigFile(path)
			return cfg, path, err
		}
	}

	return nil, "", nil
}

// LoadConfigFile loads and parses a specific configuration file.
func LoadConfigFile(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: %w", filePath, err)
	}

	cfg := &Config{
		Limits:  make(map[string]int),
		Exclude: make([]string, 0),
	}

	ext := filepath.Ext(filePath)
	if ext == ".json" {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("invalid JSON config in %q: %w", filePath, err)
		}
	} else {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("invalid YAML config in %q: %w", filePath, err)
		}
	}

	return cfg, nil
}
