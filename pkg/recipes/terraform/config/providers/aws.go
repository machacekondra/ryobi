package providers

import (
	"context"
	"strings"

	"github.com/ryobi-project/ryobi/pkg/recipes"
)

const ProviderNameAWS = "aws"

// AWSProvider generates Terraform AWS provider configuration.
type AWSProvider struct{}

// NewAWSProvider creates a new AWSProvider.
func NewAWSProvider() *AWSProvider {
	return &AWSProvider{}
}

func (p *AWSProvider) BuildConfig(ctx context.Context, envConfig *recipes.Configuration) (map[string]any, error) {
	config := map[string]any{}

	// Extract region from AWS scope
	awsProvider, ok := envConfig.Providers["aws"]
	if ok && awsProvider.Scope != "" {
		region := extractAWSRegion(awsProvider.Scope)
		if region != "" {
			config["region"] = region
		}
	}

	// Merge any additional provider config from recipe config
	if envConfig.RecipeConfig.Terraform.Providers != nil {
		for _, providerConfig := range envConfig.RecipeConfig.Terraform.Providers[ProviderNameAWS] {
			for k, v := range providerConfig {
				config[k] = v
			}
		}
	}

	return config, nil
}

// extractAWSRegion extracts the region from an AWS scope.
// Expected format: /planes/aws/aws/accounts/{accountId}/regions/{region}
func extractAWSRegion(scope string) string {
	parts := strings.Split(scope, "/")
	for i, part := range parts {
		if strings.EqualFold(part, "regions") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
