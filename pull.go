// Package ociregistry pulls OCI images from Docker Hub and other OCI-compatible registries.
package ociregistry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ImageConfig holds the runtime config extracted from an OCI image.
type ImageConfig struct {
	Cmd        []string
	Entrypoint []string
	Env        []string
	WorkingDir string
}

// Layer describes a single extracted image layer.
type Layer struct {
	Digest string
	Dir    string
}

// PullResult is returned after a successful pull.
type PullResult struct {
	Config ImageConfig
	Layers []Layer // ordered bottom (index 0) to top
}

// Pull pulls image:tag from Docker Hub and extracts layers into dest.
// image: "alpine", "library/alpine", or "user/repo"
// tag:   "latest", a version tag, or a digest ("sha256:abc...")
func Pull(image, tag, dest string) (*PullResult, error) {
	if !strings.Contains(image, "/") {
		image = "library/" + image
	}

	token, err := getToken(image)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	m, err := getManifest(image, tag, token)
	if err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}

	cfg, err := getImageConfig(image, m.Config.Digest, token)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	layersDir := filepath.Join(dest, "layers")
	if err := os.MkdirAll(layersDir, 0755); err != nil {
		return nil, err
	}

	var layers []Layer
	for _, desc := range m.Layers {
		layerDir := filepath.Join(layersDir, desc.Digest)
		if err := extractLayer(image, desc.Digest, layerDir, token); err != nil {
			return nil, fmt.Errorf("layer %.12s: %w", desc.Digest, err)
		}
		layers = append(layers, Layer{Digest: desc.Digest, Dir: layerDir})
	}

	return &PullResult{Config: *cfg, Layers: layers}, nil
}
