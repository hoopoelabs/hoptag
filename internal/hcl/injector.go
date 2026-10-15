package hcl

import (
	"fmt"
	"sort"
	"strings"

	hclbase "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hoopoelabs/hoptag/internal/models"
	"github.com/hoopoelabs/hoptag/internal/providers"
	"github.com/zclconf/go-cty/cty"
)

// InjectTags processes an HCL file's AST, tagging all eligible resource blocks.
func InjectTags(
	file *hclwrite.File,
	filePath string,
	opts models.ApplyOptions,
	registry *providers.Registry,
) (*models.FileApplyResult, error) {
	result := &models.FileApplyResult{
		FilePath: filePath,
		Stats:    make([]models.ResourceTagStats, 0),
	}

	body := file.Body()
	blocks := body.Blocks()

	for _, block := range blocks {
		if block.Type() != "resource" {
			continue
		}

		labels := block.Labels()
		if len(labels) < 2 {
			continue
		}
		resourceType := labels[0]
		resourceName := labels[1]

		// Check filter regex
		if opts.FilterRegex != nil && !opts.FilterRegex.MatchString(resourceType) {
			continue
		}

		// Check skip regex
		if opts.SkipRegex != nil && opts.SkipRegex.MatchString(resourceType) {
			continue
		}

		// Detect provider
		provider, found := registry.Detect(resourceType)
		if !found || !provider.CanTag(resourceType) {
			continue
		}

		attrName := provider.TagAttributeName(resourceType)
		resBody := block.Body()
		attr := resBody.GetAttribute(attrName)

		stats := models.ResourceTagStats{
			ResourceType:  resourceType,
			ResourceName:  resourceName,
			AttributeName: attrName,
		}

		if attr == nil {
			// Case 1: Attribute does not exist. Create new map attribute.
			ctyMap := make(map[string]cty.Value, len(opts.Tags))
			for k, v := range opts.Tags {
				ctyMap[k] = cty.StringVal(v)
			}
			resBody.SetAttributeValue(attrName, cty.ObjectVal(ctyMap))

			stats.ExistingTags = 0
			stats.InjectedTags = len(opts.Tags)
			stats.UpdatedTags = 0
			result.ResourcesTagged++
			result.Modified = true
		} else {
			// Case 2: Attribute already exists.
			exprTokens := attr.Expr().BuildTokens(nil)
			exprBytes := exprTokens.Bytes()

			parsedExpr, diags := hclsyntax.ParseExpression(exprBytes, "expr.tf", hclbase.Pos{Line: 1, Column: 1})
			if diags.HasErrors() {
				// If unparseable by hclsyntax, fallback to wrapping with merge()
				newTokens := wrapWithMerge(exprTokens, opts.Tags, opts.PreserveExisting)
				resBody.SetAttributeRaw(attrName, newTokens)
				stats.DynamicWrapped = true
				result.ResourcesTagged++
				result.Modified = true
			} else if _, isObj := parsedExpr.(*hclsyntax.ObjectConsExpr); isObj {
				// Subcase 2A: Map / Object literal { ... }
				mergedTokens, existingCount, injectedCount, updatedCount, err := mergeObjectLiteralTokens(
					exprTokens,
					exprBytes,
					opts.Tags,
					opts.PreserveExisting,
				)
				if err != nil {
					// Fallback to merge() wrapper if token merging hits syntax edge case
					newTokens := wrapWithMerge(exprTokens, opts.Tags, opts.PreserveExisting)
					resBody.SetAttributeRaw(attrName, newTokens)
					stats.DynamicWrapped = true
				} else {
					resBody.SetAttributeRaw(attrName, mergedTokens)
					stats.ExistingTags = existingCount
					stats.InjectedTags = injectedCount
					stats.UpdatedTags = updatedCount
				}
				result.ResourcesTagged++
				result.Modified = true
			} else {
				// Subcase 2B/2C: Dynamic expression (variable, function call, traversal)
				newTokens := wrapWithMerge(exprTokens, opts.Tags, opts.PreserveExisting)
				resBody.SetAttributeRaw(attrName, newTokens)
				stats.DynamicWrapped = true
				result.ResourcesTagged++
				result.Modified = true
			}
		}

		result.Stats = append(result.Stats, stats)
	}

	return result, nil
}

// BuildObjectTokens constructs an hclwrite.Tokens slice representing a formatted map literal.
func BuildObjectTokens(tags models.TagMap) hclwrite.Tokens {
	f := hclwrite.NewEmptyFile()
	ctyMap := make(map[string]cty.Value, len(tags))
	for k, v := range tags {
		ctyMap[k] = cty.StringVal(v)
	}
	f.Body().SetAttributeValue("dummy", cty.ObjectVal(ctyMap))
	dummyAttr := f.Body().GetAttribute("dummy")
	if dummyAttr == nil {
		return nil
	}
	return dummyAttr.Expr().BuildTokens(nil)
}

// wrapWithMerge wraps an expression with merge(..., { ... }) adhering to preserveExisting.
func wrapWithMerge(existingTokens hclwrite.Tokens, tags models.TagMap, preserveExisting bool) hclwrite.Tokens {
	injectedTokens := BuildObjectTokens(tags)
	cleanExisting := trimTokenWhitespace(existingTokens)

	if preserveExisting {
		// Existing tags override injected: merge(injected, existing)
		return hclwrite.TokensForFunctionCall("merge", injectedTokens, cleanExisting)
	}
	// Injected tags override existing: merge(existing, injected)
	return hclwrite.TokensForFunctionCall("merge", cleanExisting, injectedTokens)
}

// mergeObjectLiteralTokens surgically modifies an object literal's tokens to preserve comments and format.
func mergeObjectLiteralTokens(
	tokens hclwrite.Tokens,
	rawBytes []byte,
	incomingTags models.TagMap,
	preserveExisting bool,
) (hclwrite.Tokens, int, int, int, error) {
	parsedExpr, diags := hclsyntax.ParseExpression(rawBytes, "expr.tf", hclbase.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, 0, 0, 0, fmt.Errorf("failed to parse object literal: %s", diags.Error())
	}

	obj, ok := parsedExpr.(*hclsyntax.ObjectConsExpr)
	if !ok {
		return nil, 0, 0, 0, fmt.Errorf("expression is not an object literal")
	}

	existingKeys := make(map[string]bool)
	for _, item := range obj.Items {
		keyBytes := rawBytes[item.KeyExpr.Range().Start.Byte:item.KeyExpr.Range().End.Byte]
		k := cleanKeyName(string(keyBytes))
		existingKeys[k] = true
	}

	existingCount := len(existingKeys)
	updatedCount := 0

	// Find which incoming tags are new vs existing
	keysToAdd := make([]string, 0)
	for k := range incomingTags {
		if !existingKeys[k] {
			keysToAdd = append(keysToAdd, k)
		}
	}
	sort.Strings(keysToAdd)
	injectedCount := len(keysToAdd)

	var newToks hclwrite.Tokens
	inObject := false

	for i := 0; i < len(tokens); i++ {
		t := tokens[i]

		if t.Type == hclsyntax.TokenOBrace {
			inObject = true
			newToks = append(newToks, t)
			continue
		}

		// Check if this token starts an existing key
		if inObject && (t.Type == hclsyntax.TokenIdent || t.Type == hclsyntax.TokenOQuote) {
			var keyName string

			if t.Type == hclsyntax.TokenIdent {
				keyName = string(t.Bytes)
			} else if t.Type == hclsyntax.TokenOQuote {
				var sb strings.Builder
				j := i + 1
				for j < len(tokens) && tokens[j].Type != hclsyntax.TokenCQuote {
					sb.Write(tokens[j].Bytes)
					j++
				}
				keyName = sb.String()
			}

			keyName = strings.TrimSpace(keyName)
			newVal, hasIncoming := incomingTags[keyName]

			if hasIncoming && !preserveExisting {
				// Copy tokens up to and including '='
				for i < len(tokens) {
					newToks = append(newToks, tokens[i])
					if tokens[i].Type == hclsyntax.TokenEqual {
						i++
						break
					}
					i++
				}

				// Skip old value tokens until newline, comma, comment, or closing brace
				depth := 0
				for i < len(tokens) {
					curr := tokens[i]
					if curr.Type == hclsyntax.TokenOBrace || curr.Type == hclsyntax.TokenOParen || curr.Type == hclsyntax.TokenOBrack {
						depth++
					} else if curr.Type == hclsyntax.TokenCBrace || curr.Type == hclsyntax.TokenCParen || curr.Type == hclsyntax.TokenCBrack {
						if depth == 0 {
							break
						}
						depth--
					} else if depth == 0 && (curr.Type == hclsyntax.TokenNewline || curr.Type == hclsyntax.TokenComment || curr.Type == hclsyntax.TokenComma) {
						break
					}
					i++
				}

				// Inject updated string value tokens
				newToks = append(newToks,
					&hclwrite.Token{Type: hclsyntax.TokenOQuote, Bytes: []byte("\""), SpacesBefore: 1},
					&hclwrite.Token{Type: hclsyntax.TokenQuotedLit, Bytes: []byte(escapeHCLString(newVal))},
					&hclwrite.Token{Type: hclsyntax.TokenCQuote, Bytes: []byte("\"")},
				)
				updatedCount++
				i-- // step back for outer loop
				continue
			}
		}

		// Check if we reached the final TokenCBrace
		if t.Type == hclsyntax.TokenCBrace && isFinalCBrace(tokens, i) {
			// Ensure previous token ends with newline if we have tags to add
			if len(keysToAdd) > 0 {
				if len(newToks) > 0 && newToks[len(newToks)-1].Type != hclsyntax.TokenNewline {
					newToks = append(newToks, &hclwrite.Token{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")})
				}
				for _, k := range keysToAdd {
					v := incomingTags[k]
					newToks = append(newToks,
						&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(formatKeyIdent(k)), SpacesBefore: 4},
						&hclwrite.Token{Type: hclsyntax.TokenEqual, Bytes: []byte("="), SpacesBefore: 1},
						&hclwrite.Token{Type: hclsyntax.TokenOQuote, Bytes: []byte("\""), SpacesBefore: 1},
						&hclwrite.Token{Type: hclsyntax.TokenQuotedLit, Bytes: []byte(escapeHCLString(v))},
						&hclwrite.Token{Type: hclsyntax.TokenCQuote, Bytes: []byte("\"")},
						&hclwrite.Token{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
					)
				}
				// Adjust closing brace
				t.SpacesBefore = 2
			}
		}

		newToks = append(newToks, t)
	}

	return newToks, existingCount, injectedCount, updatedCount, nil
}

func isFinalCBrace(tokens hclwrite.Tokens, index int) bool {
	for j := index + 1; j < len(tokens); j++ {
		if tokens[j].Type != hclsyntax.TokenNewline &&
			tokens[j].Type != hclsyntax.TokenComment {
			return false
		}
	}
	return true
}

func cleanKeyName(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 {
		if (raw[0] == '"' && raw[len(raw)-1] == '"') || (raw[0] == '\'' && raw[len(raw)-1] == '\'') {
			return raw[1 : len(raw)-1]
		}
	}
	return raw
}

func formatKeyIdent(k string) string {
	needsQuote := false
	for _, ch := range k {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
			needsQuote = true
			break
		}
	}
	if needsQuote {
		return fmt.Sprintf("%q", k)
	}
	return k
}

func escapeHCLString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

func trimTokenWhitespace(toks hclwrite.Tokens) hclwrite.Tokens {
	start := 0
	for start < len(toks) && (toks[start].Type == hclsyntax.TokenNewline) {
		start++
	}
	end := len(toks)
	for end > start && (toks[end-1].Type == hclsyntax.TokenNewline) {
		end--
	}
	return toks[start:end]
}
