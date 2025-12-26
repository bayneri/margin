package spec

import (
	"fmt"
	"strings"
)

func ParseLabels(input string) (map[string]string, error) {
	labels := map[string]string{}
	if strings.TrimSpace(input) == "" {
		return labels, nil
	}
	pairs := strings.Split(input, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid label %q", pair)
		}
		labels[parts[0]] = parts[1]
	}
	return labels, nil
}
