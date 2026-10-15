package validator

import (
	"fmt"
	"strings"

	hclbase "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hoopoelabs/hoptag/internal/models"
	"github.com/hoopoelabs/hoptag/internal/providers"
)

// ValidateFile inspects a parsed HCL file against governance rules.
func ValidateFile(
	file *hclwrite.File,
	filePath string,
	opts models.CheckOptions,
	registry *providers.Registry,
) ([]models.Violation, int, int, error) {
	var violations []models.Violation
	totalResources := 0
	compliantResources := 0

	body := file.Body()
	for _, block := range body.Blocks() {
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

		totalResources++
		attrName := provider.TagAttributeName(resourceType)
		attr := block.Body().GetAttribute(attrName)

		if attr == nil {
			// Entire attribute is missing
			for _, rule := range opts.Rules {
				violations = append(violations, models.Violation{
					FilePath:     filePath,
					ResourceType: resourceType,
					ResourceName: resourceName,
					Type:         models.ViolationMissingTag,
					TagKey:       rule.Key,
					ExpectedRule: rule.Raw,
					Message:      fmt.Sprintf("Resource is missing required %s %q", attrName, rule.Key),
				})
			}
			continue
		}

		// Attribute exists, parse keys and values
		exprTokens := attr.Expr().BuildTokens(nil)
		rawBytes := exprTokens.Bytes()

		tagMap, isDynamic := extractTagsFromExpr(rawBytes)

		resourceViolations := 0
		for _, rule := range opts.Rules {
			val, exists := tagMap[rule.Key]
			if !exists {
				if isDynamic {
					// Tag map is dynamically computed; check if key might be in it or unevaluated
					violations = append(violations, models.Violation{
						FilePath:     filePath,
						ResourceType: resourceType,
						ResourceName: resourceName,
						Type:         models.ViolationDynamicTag,
						TagKey:       rule.Key,
						ExpectedRule: rule.Raw,
						Message:      fmt.Sprintf("Tag attribute uses dynamic expression; key %q cannot be statically verified", rule.Key),
					})
					resourceViolations++
				} else {
					violations = append(violations, models.Violation{
						FilePath:     filePath,
						ResourceType: resourceType,
						ResourceName: resourceName,
						Type:         models.ViolationMissingTag,
						TagKey:       rule.Key,
						ExpectedRule: rule.Raw,
						Message:      fmt.Sprintf("Missing required tag %q", rule.Key),
					})
					resourceViolations++
				}
				continue
			}

			// Key exists, validate regex pattern if defined
			if rule.Pattern != nil && !rule.Pattern.MatchString(val) {
				violations = append(violations, models.Violation{
					FilePath:     filePath,
					ResourceType: resourceType,
					ResourceName: resourceName,
					Type:         models.ViolationInvalidValue,
					TagKey:       rule.Key,
					CurrentValue: val,
					ExpectedRule: rule.Pattern.String(),
					Message:      fmt.Sprintf("Tag %q value %q does not match pattern %q", rule.Key, val, rule.Pattern.String()),
				})
				resourceViolations++
			}
		}

		if resourceViolations == 0 {
			compliantResources++
		}
	}

	return violations, totalResources, compliantResources, nil
}

// extractTagsFromExpr extracts statically known key-values from an attribute expression.
func extractTagsFromExpr(rawBytes []byte) (map[string]string, bool) {
	tagMap := make(map[string]string)
	parsedExpr, diags := hclsyntax.ParseExpression(rawBytes, "expr.tf", hclbase.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return tagMap, true
	}

	switch expr := parsedExpr.(type) {
	case *hclsyntax.ObjectConsExpr:
		for _, item := range expr.Items {
			keyBytes := rawBytes[item.KeyExpr.Range().Start.Byte:item.KeyExpr.Range().End.Byte]
			cleanKey := stripQuotes(string(keyBytes))

			// Check if value is a string template literal
			valBytes := rawBytes[item.ValueExpr.Range().Start.Byte:item.ValueExpr.Range().End.Byte]
			cleanVal := stripQuotes(string(valBytes))

			tagMap[cleanKey] = cleanVal
		}
		return tagMap, false

	case *hclsyntax.FunctionCallExpr:
		// e.g. merge(..., { ... })
		// We can extract statically defined items from any ObjectConsExpr args in the merge call
		for _, arg := range expr.Args {
			if objArg, ok := arg.(*hclsyntax.ObjectConsExpr); ok {
				for _, item := range objArg.Items {
					keyBytes := rawBytes[item.KeyExpr.Range().Start.Byte:item.KeyExpr.Range().End.Byte]
					cleanKey := stripQuotes(string(keyBytes))
					valBytes := rawBytes[item.ValueExpr.Range().Start.Byte:item.ValueExpr.Range().End.Byte]
					tagMap[cleanKey] = stripQuotes(string(valBytes))
				}
			}
		}
		return tagMap, true

	default:
		return tagMap, true
	}
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
