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

	// For jc tests involving local epoch timestamps, we need to fix the timezone
	// jc Docs: The naive fields (epoch timestamps) are based on the local time of the system the parser is run on.
	// jc Tests are based on America/Los_Angeles timezone. I set it here to ensure consistent test results
	// with GitHub actions and other environments, like my own in Europe/Zurich
	os.Setenv("TZ", "America/Los_Angeles")

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
