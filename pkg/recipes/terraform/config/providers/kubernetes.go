package providers

import (
	"github.com/ryobi-project/ryobi/pkg/recipes"
)

// BuildKubernetesConfig generates Terraform kubernetes provider configuration
// from the environment's recipeConfig.terraform.providers.kubernetes settings.
func BuildKubernetesConfig(envConfig *recipes.Configuration) map[string]any {
	config := map[string]any{}

	if envConfig.RecipeConfig.Terraform.Providers != nil {
		for _, providerConfig := range envConfig.RecipeConfig.Terraform.Providers["kubernetes"] {
			for k, v := range providerConfig {
				config[k] = v
			}
		}
	}

	return config
}
