package providers

import (
	"strings"
)

// AWSProvider handles Amazon Web Services resource tagging.
type AWSProvider struct {
	untaggableTypes map[string]struct{}
}

// NewAWSProvider creates an instance of AWSProvider.
func NewAWSProvider() *AWSProvider {
	untaggable := map[string]struct{}{
		"aws_route":                                   {},
		"aws_route_table_association":                 {},
		"aws_main_route_table_association":            {},
		"aws_security_group_rule":                     {},
		"aws_vpc_endpoint_route_table_association":    {},
		"aws_vpc_endpoint_subnet_association":         {},
		"aws_iam_policy_attachment":                   {},
		"aws_iam_user_policy_attachment":              {},
		"aws_iam_role_policy_attachment":              {},
		"aws_iam_group_policy_attachment":             {},
		"aws_alb_target_group_attachment":             {},
		"aws_lb_target_group_attachment":              {},
		"aws_cloudwatch_log_subscription_filter":      {},
		"aws_lambda_permission":                       {},
	}
	return &AWSProvider{
		untaggableTypes: untaggable,
	}
}

func (p *AWSProvider) Name() string {
	return "AWS"
}

func (p *AWSProvider) Prefix() string {
	return "aws_"
}

func (p *AWSProvider) CanTag(resourceType string) bool {
	if !strings.HasPrefix(resourceType, p.Prefix()) {
		return false
	}
	_, blocked := p.untaggableTypes[resourceType]
	return !blocked
}

func (p *AWSProvider) TagAttributeName(resourceType string) string {
	return "tags"
}
