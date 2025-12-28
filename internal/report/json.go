package report

import (
	"encoding/json"
	"os"

	"github.com/bayneri/margin/internal/analyze"
)

func WriteJSON(path string, payload interface{}) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func WriteSummaryJSON(path string, result analyze.Result) error {
	return WriteJSON(path, result)
}

func WriteSourcesJSON(path string, sources analyze.Sources) error {
	return WriteJSON(path, sources)
}
