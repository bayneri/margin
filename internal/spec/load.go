package spec

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func Load(path string) (Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, fmt.Errorf("read spec: %w", err)
	}
	var s Spec
	if err := yaml.Unmarshal(data, &s); err != nil {
		return Spec{}, fmt.Errorf("parse spec: %w", err)
	}
	return s, nil
}
