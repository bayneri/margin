package report

import (
	"fmt"
	"os"
	"strings"
)

func WriteErrorsMarkdown(path string, errors []string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Analysis errors\n\n")
	for _, err := range errors {
		fmt.Fprintf(&b, "- %s\n", err)
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}
