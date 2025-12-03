package parser

import (
	"encoding/json"
)

// Parser is the common interface for all command parsers
type Parser interface {
	// Parse takes raw command output and returns parsed data
	Parse(input []byte) (any, error)

	// UnmarshalExpected is a helper for tests: loads expected JSON from jc fixtures
	UnmarshalExpected(data []byte) (any, error)
}

// Default implementation of UnmarshalExpected (shared by all parsers)
type baseParser struct{}

func (baseParser) UnmarshalExpected(data []byte) (any, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return v, nil
}
