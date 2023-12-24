package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type authResponse struct {
	Token       string    `json:"token"`
	AccessToken string    `json:"access_toke"`
	ExpiresIn   int       `json:"expires_in"`
	IssuedAt    time.Time `json:"issued_at"`
}

// type manifestDescription struct {
// 	Digest    string `json:"digest"`
// 	MediaType string `json:"mediaType"`
// 	Platform  struct {
// 		Architecture string `json:"architecture"`
// 		OS           string `json:"OS"`
// 	} `json:"platform"`
// }

type fatManifest struct {
	Manifests []struct {
		Digest string `json:"digest"`
	} `json:"manifests"`
}

type imageManifest struct {
	MediaType string `json:"application/vnd.docker.distribution.manifest.v2+json"`
	Layers    []struct {
		Digest string `json:"digest"`
	} `json:"layers"`
}

type DockerRegistry struct {
	auth         authResponse
	imageName    string
	imageVersion string
}

func NewRegistry(imageReference string) DockerRegistry {
	//
	imageNameParts := strings.Split(imageReference, ":")
	var imageName, imageVersion string
	imageName = imageNameParts[0]
	if len(imageNameParts) == 1 {
		imageVersion = "latest"
	} else if len(imageNameParts) == 2 {
		imageVersion = imageNameParts[1]
	}

	return DockerRegistry{
		imageName:    imageName,
		imageVersion: imageVersion,
	}
}

func (r *DockerRegistry) Authenticate() error {
	authUrl := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/%s:pull", r.imageName)
	resp, err := http.Get(authUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&r.auth)
	if err != nil {
		return err
	}
	return nil
}

func (r *DockerRegistry) getManifestKind() (string, string, error) {
	// Finds out which kind of manifest is going to be returned
	client := http.Client{}

	fatManifestURL := fmt.Sprintf("https://registry.hub.docker.com/v2/library/%s/manifests/%s", r.imageName, r.imageVersion)
	req, err := http.NewRequest("HEAD", fatManifestURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", r.auth.Token))
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	return resp.Header.Get("Content-Type"), resp.Header.Get("Docker-Content-Digest"), nil
}

func (r *DockerRegistry) getFromManifestList() (imageManifest, error) {
	// Gather and parse all multi-architecture image manifests from Fat Manifest and returns the first one

	client := http.Client{}

	fatManifestURL := fmt.Sprintf("https://registry.hub.docker.com/v2/library/%s/manifests/%s", r.imageName, r.imageVersion)
	req, err := http.NewRequest("GET", fatManifestURL, nil)
	if err != nil {
		return imageManifest{}, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", r.auth.Token))
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err := client.Do(req)
	if err != nil {
		return imageManifest{}, err
	}
	defer resp.Body.Close()

	var manifest fatManifest
	err = json.NewDecoder(resp.Body).Decode(&manifest)
	if err != nil {
		return imageManifest{}, err
	}
	return r.getManifestByDigest(manifest.Manifests[0].Digest)
}

func (r *DockerRegistry) getManifestByDigest(digest string) (imageManifest, error) {
	// Pull and parse a single manifest extracted from Fat Manifest
	client := http.Client{}

	manifestURL := fmt.Sprintf("https://registry.hub.docker.com/v2/library/%s/manifests/%s", r.imageName, digest)
	req, err := http.NewRequest("GET", manifestURL, nil)
	if err != nil {
		return imageManifest{}, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", r.auth.Token))
	req.Header.Add("Accept", "application/vnd.oci.image.manifest.v1+json")

	resp, err := client.Do(req)
	if err != nil {
		return imageManifest{}, err
	}
	defer resp.Body.Close()

	var manifest imageManifest
	err = json.NewDecoder(resp.Body).Decode(&manifest)
	if err != nil {
		return imageManifest{}, err
	}
	return manifest, nil
}

func (r *DockerRegistry) getLayers(manifest imageManifest, root string) error {
	client := http.Client{}

	for _, layer := range manifest.Layers {
		// fmt.Printf("Pulling layer [%s]", layer.Digest)

		layerURL := fmt.Sprintf("https://registry.hub.docker.com/v2/library/%s/blobs/%s", r.imageName, layer.Digest)
		req, err := http.NewRequest("GET", layerURL, nil)
		if err != nil {
			return err
		}
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", r.auth.Token))
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		tmpTarFile, err := os.CreateTemp("", "layer-tar")
		if err != nil {
			return err
		}
		defer tmpTarFile.Close()
		if _, err := io.Copy(tmpTarFile, resp.Body); err != nil {
			return err
		}
		if err := extractTarArchive(tmpTarFile.Name(), root); err != nil {
			return err
		}
		os.Remove(tmpTarFile.Name())

	}
	return nil
}

func (r *DockerRegistry) Pull(root string) error {
	manifestKind, manifestDigest, err := r.getManifestKind()
	if err != nil {
		return err
	}

	var manifest imageManifest
	switch manifestKind {
	case "application/vnd.oci.image.index.v1+json":
		manifest, err = r.getFromManifestList()
	case "application/vnd.docker.distribution.manifest.v2+json":
		manifest, err = r.getManifestByDigest(manifestDigest)
	default:
		return errors.New(fmt.Sprintf("Unknown manifest kind: %s", manifestKind))
	}
	err = r.getLayers(manifest, root)
	if err != nil {
		return err
	}
	return nil
}
