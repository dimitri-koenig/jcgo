package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGitLogFixtures(t *testing.T) {
	p := GitLog{}

	// Automatically discover all .out fixtures
	matches, err := filepath.Glob("../fixtures/**/git-log*.out")
	require.NoError(t, err)

	for _, outPath := range matches {
		t.Run(outPath, func(t *testing.T) {
			input, err := os.ReadFile(outPath)
			require.NoError(t, err)

			got, err := p.Parse(input)
			require.NoError(t, err)

			expectedJSON, err := os.ReadFile(outPath[:len(outPath)-4] + ".json")
			require.NoError(t, err)

			expected, err := p.UnmarshalExpected(expectedJSON)
			require.NoError(t, err)

			// Pretty-print both for nice diff on failure
			gotBytes, _ := json.MarshalIndent(got, "", "  ")
			expBytes, _ := json.MarshalIndent(expected, "", "  ")

			require.JSONEq(t, string(expBytes), string(gotBytes))
		})
	}
}
