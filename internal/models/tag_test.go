package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTags(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  TagMap
		expectErr bool
	}{
		{
			name:  "valid JSON format",
			input: `{"Environment":"production","CostCenter":"FIN-104","ManagedBy":"hoptag"}`,
			expected: TagMap{
				"Environment": "production",
				"CostCenter":  "FIN-104",
				"ManagedBy":   "hoptag",
			},
			expectErr: false,
		},
		{
			name:  "valid key-value shorthand",
			input: "Environment=production,CostCenter=FIN-104,ManagedBy=hoptag",
			expected: TagMap{
				"Environment": "production",
				"CostCenter":  "FIN-104",
				"ManagedBy":   "hoptag",
			},
			expectErr: false,
		},
		{
			name:  "quoted keys and values with spaces",
			input: `"Project Name"="Project Phoenix",'Data Class'='Confidential'`,
			expected: TagMap{
				"Project Name": "Project Phoenix",
				"Data Class":   "Confidential",
			},
			expectErr: false,
		},
		{
			name:      "empty input",
			input:     "   ",
			expectErr: true,
		},
		{
			name:      "invalid JSON",
			input:     `{"Environment": "incomplete"`,
			expectErr: true,
		},
		{
			name:      "invalid key-value missing equals",
			input:     "Environment=production,CostCenter",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ParseTags(tc.input)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, res)
			}
		})
	}
}

func TestParseRules(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		validate  func(t *testing.T, rules []TagRule)
		expectErr bool
	}{
		{
			name:  "mixed presence and regex rules",
			input: "Environment=^(dev|staging|prod)$,CostCenter=^CC-[0-9]{4}$,ManagedBy",
			validate: func(t *testing.T, rules []TagRule) {
				require.Len(t, rules, 3)
				assert.Equal(t, "Environment", rules[0].Key)
				assert.NotNil(t, rules[0].Pattern)
				assert.True(t, rules[0].Pattern.MatchString("staging"))
				assert.False(t, rules[0].Pattern.MatchString("qa"))

				assert.Equal(t, "CostCenter", rules[1].Key)
				assert.NotNil(t, rules[1].Pattern)
				assert.True(t, rules[1].Pattern.MatchString("CC-1234"))
				assert.False(t, rules[1].Pattern.MatchString("1234"))

				assert.Equal(t, "ManagedBy", rules[2].Key)
				assert.Nil(t, rules[2].Pattern)
			},
			expectErr: false,
		},
		{
			name:      "invalid regex",
			input:     "Environment=[a-z(",
			expectErr: true,
		},
		{
			name:      "empty input",
			input:     "",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rules, err := ParseRules(tc.input)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tc.validate(t, rules)
			}
		})
	}
}
