package main

import (
	"maps"
	"slices"
)

type VersionMeta struct {
	Version   string     `json:"version"`
	Protocols []string   `json:"protocols"`
	Platforms []Platform `json:"platforms"`
}

type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

func createVersionMeta(version string, artifacts []Artifact) (*VersionMeta, error) {
	platformsSet := make(map[Platform]struct{})

	for _, artifact := range artifacts {
		if artifact.Type == "Archive" {
			platform := Platform{
				OS:   artifact.OS,
				Arch: artifact.Arch,
			}
			platformsSet[platform] = struct{}{}
		}
	}

	platforms := slices.Collect(maps.Keys(platformsSet))

	return &VersionMeta{
		Version:   version,
		Protocols: []string{"6.0"},
		Platforms: platforms,
	}, nil
}
