package testcase

import "errors"

// type: acm || oi
type TestcaseMetadata struct {
	TimeLimit   int   `json:"time_limit"`
	MemoryLimit int   `json:"mem_limit"`
	Quantity    int   `json:"quantity"`
	Points      []int `json:"points"`
	Type        int   `json:"type"`
}

type TestcaseManager interface {
	GetTestcase(problemID string, subtestID int) ([]byte, []byte, error)
	GetTestcaseMetadata(problemID string) (TestcaseMetadata, error)
	GetTestcasePoints(problemID string) []int
}

type TestcaseStore struct {
}

var ErrTestcaseNotFound = errors.New("testcase not found")

func (tm *TestcaseStore) GetTestcase(problemID string, subtestID int) ([]byte, []byte, error) {
	return nil, nil, ErrTestcaseNotFound
}
