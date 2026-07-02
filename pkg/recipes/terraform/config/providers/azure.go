package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/ryobi-project/ryobi/pkg/recipes"
)

const ProviderNameAzureRM = "azurerm"

// AzureProvider generates Terraform azurerm provider configuration.
type AzureProvider struct{}

// NewAzureProvider creates a new AzureProvider.
func NewAzureProvider() *AzureProvider {
	return &AzureProvider{}
}

func (p *AzureProvider) BuildConfig(ctx context.Context, envConfig *recipes.Configuration) (map[string]any, error) {
	config := map[string]any{
		"features": map[string]any{},
	}

	// Extract subscription ID from Azure scope
	azureProvider, ok := envConfig.Providers["azure"]
	if ok && azureProvider.Scope != "" {
		subscriptionID := extractSubscriptionID(azureProvider.Scope)
		if subscriptionID != "" {
			config["subscription_id"] = subscriptionID
		}
	}

	// Merge any additional provider config from recipe config
	if envConfig.RecipeConfig.Terraform.Providers != nil {
		for _, providerConfig := range envConfig.RecipeConfig.Terraform.Providers[ProviderNameAzureRM] {
			for k, v := range providerConfig {
				config[k] = v
			}
		}
	}

	return config, nil
}

// extractSubscriptionID extracts the subscription ID from an Azure scope.
// Expected format: /subscriptions/{subscriptionId}/resourceGroups/{rgName}
func extractSubscriptionID(scope string) string {
	parts := strings.Split(scope, "/")
	for i, part := range parts {
		if strings.EqualFold(part, "subscriptions") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	_ = fmt.Sprintf("could not extract subscription ID from scope: %s", scope)
	return ""
}
