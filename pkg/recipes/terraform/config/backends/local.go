package backends

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"

	"github.com/ryobi-project/ryobi/pkg/recipes"
)

const BackendLocal = "local"

// LocalBackend implements the Backend interface using Terraform's local backend.
// Useful for development and testing.
type LocalBackend struct {
	stateDir string
}

// NewLocalBackend creates a new LocalBackend that stores state in the given directory.
func NewLocalBackend(stateDir string) *LocalBackend {
	return &LocalBackend{stateDir: stateDir}
}

// BuildBackend returns the Terraform local backend configuration.
func (b *LocalBackend) BuildBackend(resourceRecipe *recipes.ResourceMetadata) (string, map[string]any, error) {
	hash := sha256.Sum256([]byte(resourceRecipe.ResourceID))
	stateFile := fmt.Sprintf("tfstate-%x.tfstate", hash[:8])

	config := map[string]any{
		"path": filepath.Join(b.stateDir, stateFile),
	}

	return BackendLocal, config, nil
}
