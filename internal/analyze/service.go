package analyze

import (
	"fmt"
	"strings"
)

func NormalizeService(project, service string) (string, string, error) {
	if strings.HasPrefix(service, "projects/") {
		parts := strings.Split(service, "/")
		if len(parts) < 4 || parts[2] != "services" {
			return "", "", fmt.Errorf("invalid service resource name %q", service)
		}
		serviceID := parts[len(parts)-1]
		if project != "" && parts[1] != project {
			return "", "", fmt.Errorf("--project %q does not match service resource %q", project, parts[1])
		}
		return service, serviceID, nil
	}
	if project == "" {
		return "", "", fmt.Errorf("--project is required when --service is not a resource name")
	}
	return fmt.Sprintf("projects/%s/services/%s", project, service), service, nil
}
