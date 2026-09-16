// Package features loads whatever plan/checklist file the user already
// has, in the one simple shape this tool expects: a flat list of
// {id, name}. No fixed universal format is forced beyond that.
package features

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Feature struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Words returns the feature's name split into lowercase tokens for loose
// matching against graph node words.
func (f Feature) Words() []string {
	return strings.Fields(strings.ToLower(strings.NewReplacer("-", " ", "_", " ").Replace(f.Name)))
}

func Load(path string) ([]Feature, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading features file: %w", err)
	}
	var fs []Feature
	if err := json.Unmarshal(raw, &fs); err != nil {
		return nil, fmt.Errorf("parsing features file: %w", err)
	}
	return fs, nil
}
