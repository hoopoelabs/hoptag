package models

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Standard Apply Modes
const (
	ModeInPlace  = "in-place"
	ModeOverride = "override"
	ModeRename   = "rename"
)

// TagMap represents key-value tags or labels.
type TagMap map[string]string

// TagRule represents a validation rule for a specific tag key.
type TagRule struct {
	Key     string
	Pattern *regexp.Regexp
	Raw     string
}

// ApplyOptions configures the tagging process.
type ApplyOptions struct {
	Dir              string
	Tags             TagMap
	Mode             string
	FilterRegex      *regexp.Regexp
	SkipRegex        *regexp.Regexp
	PreserveExisting bool
	Recursive        bool
}

// CheckOptions configures the tag validation / governance process.
type CheckOptions struct {
	Dir         string
	Rules       []TagRule
	FilterRegex *regexp.Regexp
	SkipRegex   *regexp.Regexp
	Recursive   bool
}

// Violation represents a tag compliance issue detected in a resource.
type Violation struct {
	FilePath     string
	ResourceType string
	ResourceName string
	Type         ViolationType
	TagKey       string
	CurrentValue string
	ExpectedRule string
	Message      string
}

type ViolationType string

const (
	ViolationMissingTag    ViolationType = "MISSING_TAG"
	ViolationInvalidValue  ViolationType = "INVALID_VALUE"
	ViolationDynamicTag    ViolationType = "DYNAMIC_TAG_UNEVALUATED"
)

// ResourceTagStats tracks tag injection details for a single resource.
type ResourceTagStats struct {
	ResourceType   string
	ResourceName   string
	AttributeName  string
	ExistingTags   int
	InjectedTags   int
	UpdatedTags    int
	DynamicWrapped bool
}

// FileApplyResult summarizes tagging actions in a single file.
type FileApplyResult struct {
	FilePath         string
	ResourcesTagged  int
	Modified         bool
	Stats            []ResourceTagStats
	OverrideFilePath string
}

// ApplySummary summarizes the full tagging run across all files.
type ApplySummary struct {
	TotalFilesScanned int
	FilesModified     int
	TotalResources    int
	ResourcesTagged   int
	Results           []FileApplyResult
}

// CheckSummary summarizes the full governance validation run.
type CheckSummary struct {
	TotalFilesScanned int
	TotalResources    int
	CompliantCount    int
	Violations        []Violation
}

// ParseTags parses a tag string which can be either a JSON object or comma-separated key=value pairs.
// Examples:
// - `{"Environment":"production","CostCenter":"FIN-104"}`
// - `Environment=production,CostCenter=FIN-104,ManagedBy="hoptag"`
func ParseTags(input string) (TagMap, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("tags input cannot be empty")
	}

	res := make(TagMap)

	// Check if input is JSON
	if strings.HasPrefix(input, "{") && strings.HasSuffix(input, "}") {
		var rawMap map[string]interface{}
		if err := json.Unmarshal([]byte(input), &rawMap); err != nil {
			return nil, fmt.Errorf("invalid JSON tag format: %w", err)
		}
		for k, v := range rawMap {
			cleanKey := strings.TrimSpace(k)
			if cleanKey == "" {
				continue
			}
			res[cleanKey] = fmt.Sprintf("%v", v)
		}
		if len(res) == 0 {
			return nil, fmt.Errorf("no valid tags found in JSON input")
		}
		return res, nil
	}

	// Shorthand key-value format: Key=Val,Key2=Val2
	// Handle potential escaped commas or commas inside quotes
	pairs := splitKeyValuePairs(input)
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		eqIdx := strings.Index(pair, "=")
		if eqIdx == -1 {
			return nil, fmt.Errorf("invalid tag pair %q: missing '=' delimiter", pair)
		}
		k := strings.TrimSpace(pair[:eqIdx])
		v := strings.TrimSpace(pair[eqIdx+1:])

		// Strip surrounding single or double quotes
		k = stripQuotes(k)
		v = stripQuotes(v)

		if k == "" {
			return nil, fmt.Errorf("tag key cannot be empty in %q", pair)
		}
		res[k] = v
	}

	if len(res) == 0 {
		return nil, fmt.Errorf("no valid tags found in input: %q", input)
	}

	return res, nil
}

// ParseRules parses required tag definitions and regex validation rules.
// Example: `Environment=^(dev|staging|prod)$,CostCenter=^CC-[0-9]{4}$,ManagedBy`
func ParseRules(input string) ([]TagRule, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("rules input cannot be empty")
	}

	var rules []TagRule
	items := splitKeyValuePairs(input)
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		eqIdx := strings.Index(item, "=")
		if eqIdx == -1 {
			// Key only (must be present)
			key := stripQuotes(strings.TrimSpace(item))
			if key == "" {
				continue
			}
			rules = append(rules, TagRule{
				Key: key,
				Raw: item,
			})
		} else {
			key := stripQuotes(strings.TrimSpace(item[:eqIdx]))
			patternStr := stripQuotes(strings.TrimSpace(item[eqIdx+1:]))
			if key == "" {
				return nil, fmt.Errorf("rule key cannot be empty in %q", item)
			}
			re, err := regexp.Compile(patternStr)
			if err != nil {
				return nil, fmt.Errorf("invalid regex %q for key %q: %w", patternStr, key, err)
			}
			rules = append(rules, TagRule{
				Key:     key,
				Pattern: re,
				Raw:     item,
			})
		}
	}

	if len(rules) == 0 {
		return nil, fmt.Errorf("no valid rules found in input: %q", input)
	}

	return rules, nil
}

// Helper to split on commas while respecting quotes
func splitKeyValuePairs(input string) []string {
	var pairs []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, ch := range input {
		switch {
		case (ch == '"' || ch == '\'') && !inQuote:
			inQuote = true
			quoteChar = ch
			current.WriteRune(ch)
		case inQuote && ch == quoteChar:
			inQuote = false
			quoteChar = 0
			current.WriteRune(ch)
		case ch == ',' && !inQuote:
			pairs = append(pairs, current.String())
			current.Reset()
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		pairs = append(pairs, current.String())
	}
	return pairs
}

func stripQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
