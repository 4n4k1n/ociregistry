package ociregistry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	mediaTypeManifest         = "application/vnd.oci.image.manifest.v1+json"
	mediaTypeIndex            = "application/vnd.oci.image.index.v1+json"
	mediaTypeDockerManifest   = "application/vnd.docker.distribution.manifest.v2+json"
	mediaTypeDockerManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
)

type manifest struct {
	MediaType string       `json:"mediaType"`
	Config    descriptor   `json:"config"`
	Layers    []descriptor `json:"layers"`
	Manifests []struct {
		Digest   string `json:"digest"`
		Platform struct {
			OS   string `json:"os"`
			Arch string `json:"architecture"`
		} `json:"platform"`
	} `json:"manifests"`
}

type descriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type imageConfigJSON struct {
	Config struct {
		Cmd        []string `json:"Cmd"`
		Entrypoint []string `json:"Entrypoint"`
		Env        []string `json:"Env"`
		WorkingDir string   `json:"WorkingDir"`
	} `json:"config"`
}

func getManifest(image, ref, token string) (*manifest, error) {
	url := fmt.Sprintf("https://registry-1.docker.io/v2/%s/manifests/%s", image, ref)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", strings.Join([]string{
		mediaTypeManifest,
		mediaTypeIndex,
		mediaTypeDockerManifest,
		mediaTypeDockerManifestList,
	}, ", "))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("registry returned %d", resp.StatusCode)
	}

	var m manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}

	// Manifest list — pick linux/amd64
	if m.MediaType == mediaTypeIndex || m.MediaType == mediaTypeDockerManifestList {
		for _, entry := range m.Manifests {
			if entry.Platform.OS == "linux" && entry.Platform.Arch == "amd64" {
				return getManifest(image, entry.Digest, token)
			}
		}
		return nil, fmt.Errorf("no linux/amd64 manifest in index")
	}

	return &m, nil
}

func getImageConfig(image, digest, token string) (*ImageConfig, error) {
	url := fmt.Sprintf("https://registry-1.docker.io/v2/%s/blobs/%s", image, digest)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var cfg imageConfigJSON
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, err
	}

	return &ImageConfig{
		Cmd:        cfg.Config.Cmd,
		Entrypoint: cfg.Config.Entrypoint,
		Env:        cfg.Config.Env,
		WorkingDir: cfg.Config.WorkingDir,
	}, nil
}
