package main

import (
	"encoding/json"
	"os"
	"time"
)

type Runtime struct {
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
}

type Metadata struct {
	ProjectName string    `json:"project_name"`
	Tag         string    `json:"tag"`
	PreviousTag string    `json:"previous_tag"`
	Version     string    `json:"version"`
	Commit      string    `json:"commit"`
	Date        time.Time `json:"date"`
	Runtime     Runtime   `json:"runtime"`
}

func loadMetadata(filepath string) (*Metadata, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var metadata Metadata
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		return nil, err
	}

	return &metadata, nil
}
