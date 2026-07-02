package yaml

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	KindApplication = "Application"
	KindEnvironment = "Environment"
	APIVersionV1    = "ryobi/v1"
)

// ParseFile reads and parses a YAML file, returning one or more documents.
// Supports multi-document YAML files (separated by ---).
func ParseFile(path string) ([]Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	return Parse(data)
}

// Parse parses YAML bytes into one or more Documents.
func Parse(data []byte) ([]Document, error) {
	var docs []Document

	// Split multi-document YAML
	parts := splitYAMLDocuments(data)

	for i, part := range parts {
		if len(strings.TrimSpace(string(part))) == 0 {
			continue
		}

		var doc Document
		if err := yaml.Unmarshal(part, &doc); err != nil {
			return nil, fmt.Errorf("failed to parse document %d: %w", i+1, err)
		}
		docs = append(docs, doc)
	}

	if len(docs) == 0 {
		return nil, fmt.Errorf("no documents found in YAML")
	}

	return docs, nil
}

// Validate checks that a document has valid structure.
func Validate(doc *Document) error {
	if doc.APIVersion != APIVersionV1 {
		return fmt.Errorf("unsupported apiVersion %q, expected %q", doc.APIVersion, APIVersionV1)
	}

	switch doc.Kind {
	case KindApplication:
		return validateApplication(doc)
	case KindEnvironment:
		return validateEnvironment(doc)
	default:
		return fmt.Errorf("unsupported kind %q, expected %q or %q", doc.Kind, KindApplication, KindEnvironment)
	}
}

func validateApplication(doc *Document) error {
	if doc.Metadata.Name == "" {
		return fmt.Errorf("application metadata.name is required")
	}
	if doc.Metadata.Environment == "" {
		return fmt.Errorf("application metadata.environment is required")
	}
	for i, res := range doc.Resources {
		if res.Name == "" {
			return fmt.Errorf("resources[%d].name is required", i)
		}
		if res.Type == "" {
			return fmt.Errorf("resources[%d].type is required", i)
		}
		if res.Recipe == "" {
			return fmt.Errorf("resources[%d].recipe is required", i)
		}
	}
	return nil
}

func validateEnvironment(doc *Document) error {
	if doc.Metadata.Name == "" {
		return fmt.Errorf("environment metadata.name is required")
	}
	return nil
}

// splitYAMLDocuments splits a multi-document YAML byte slice.
func splitYAMLDocuments(data []byte) [][]byte {
	lines := strings.Split(string(data), "\n")
	var docs [][]byte
	var current []string

	for _, line := range lines {
		if strings.TrimSpace(line) == "---" {
			if len(current) > 0 {
				docs = append(docs, []byte(strings.Join(current, "\n")))
				current = nil
			}
			continue
		}
		current = append(current, line)
	}

	if len(current) > 0 {
		docs = append(docs, []byte(strings.Join(current, "\n")))
	}

	return docs
}
