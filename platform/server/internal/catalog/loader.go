package catalog

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func Load(path string) (Catalog, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, fmt.Errorf("read service catalog: %w", err)
	}

	var catalog Catalog
	if err := yaml.Unmarshal(content, &catalog); err != nil {
		return Catalog{}, fmt.Errorf("parse service catalog: %w", err)
	}

	if err := Validate(catalog); err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}
