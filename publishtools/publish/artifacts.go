package main

import (
	"encoding/json"
	"os"
)

type Extra struct {
	Binary    string   `json:"Binary,omitempty"`
	Builder   string   `json:"Builder,omitempty"`
	Ext       string   `json:"Ext,omitempty"`
	ID        string   `json:"ID,omitempty"`
	Binaries  []string `json:"Binaries,omitempty"`
	Checksum  string   `json:"Checksum,omitempty"`
	Format    string   `json:"Format,omitempty"`
	WrappedIn string   `json:"WrappedIn,omitempty"`
}

type Artifact struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	OS    string `json:"goos,omitempty"`
	Arch  string `json:"goarch,omitempty"`
	Type  string `json:"type"`
	Extra Extra  `json:"extra"`
}

func loadArtifacts(filepath string) ([]Artifact, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var artifacts []Artifact
	err = json.Unmarshal(data, &artifacts)
	if err != nil {
		return nil, err
	}

	return artifacts, nil
}
