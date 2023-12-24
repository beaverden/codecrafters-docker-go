package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/codeclysm/extract"
	log "github.com/sirupsen/logrus"
)

type ContentType string

const (
	ImageManifestV2 ContentType = "application/vnd.docker.distribution.manifest.v2+json"
	ImageManifestV1 ContentType = "application/vnd.oci.image.manifest.v1+json"
	ManifestList    ContentType = "application/vnd.oci.image.index.v1+json"
)

type registryAuth struct {
	Token       string    `json:"token"`
	AccessToken string    `json:"access_toke"`
	ExpiresIn   int       `json:"expires_in"`
	IssuedAt    time.Time `json:"issued_at"`
}

// https://distribution.github.io/distribution/spec/manifest-v2-2/
// Describes the common format of Manifest List and Image Manifest
//
// Extracted here are only the required fields
// The Manifests[] list is present in Manifest List (application/vnd.oci.image.index.v1+json)
// The Layers[] list is present in Image Manifest V2 (application/vnd.docker.distribution.manifest.v2+json)
// and Image manifest V1 (application/vnd.oci.image.manifest.v1+json)
type imageManifest struct {
	MediaType ContentType `json:"mediaType"`

	Manifests []struct {
		MediaType ContentType `json:"mediaType"`
		Digest    string      `json:"digest"`
		Platform  struct {
			Architecture string `json:"architecture"`
			OS           string `json:"OS"`
		} `json:"platform"`
	} `json:"manifests"`

	Layers []struct {
		Digest    string      `json:"digest"`
		MediaType ContentType `json:"mediaType"`
	} `json:"layers"`
}

type DockerRegistry struct {
	auth         registryAuth
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

	log.Infof("Registry created to pull %s:%s from initial reference %s", imageName, imageVersion, imageReference)
	return DockerRegistry{
		imageName:    imageName,
		imageVersion: imageVersion,
	}
}

func (r *DockerRegistry) Authenticate() error {
	authUrl := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:library/%s:pull", r.imageName)
	resp, err := http.Get(authUrl)
	if err != nil {
		return fmt.Errorf("Failed to create request for auth", err)
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&r.auth)
	if err != nil {
		return fmt.Errorf("Failed to parse auth response", err)
	}
	log.Infof("Successfully authenticated in the registry, token viable for %d seconds", r.auth.ExpiresIn)
	return nil
}

func (r *DockerRegistry) getManifest(reference string, expect ContentType) (imageManifest, error) {
	log.Infof("Requesting manifest for %s with reference: %s", r.imageName, reference)

	client := http.Client{}

	manifestURL := fmt.Sprintf("https://registry.hub.docker.com/v2/library/%s/manifests/%s", r.imageName, reference)
	req, err := http.NewRequest("GET", manifestURL, nil)
	if err != nil {
		return imageManifest{}, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", r.auth.Token))
	req.Header.Add("Accept", string(expect))

	resp, err := client.Do(req)
	if err != nil {
		return imageManifest{}, err
	}
	defer resp.Body.Close()

	var m imageManifest
	err = json.NewDecoder(resp.Body).Decode(&m)
	if err != nil {
		return imageManifest{}, err
	}

	log.Infof("Got manifest of type: %s", m.MediaType)

	return m, nil
}

func (r *DockerRegistry) getLayers(m imageManifest, root string) error {
	client := http.Client{}

	for _, layer := range m.Layers {
		log.Infof("Pulling layer [%s]", layer.Digest)

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
		if err := extract.Archive(context.Background(), resp.Body, root, nil); err != nil {
			return err
		}
	}
	return nil
}

func (r *DockerRegistry) Pull(root string) error {
	manifest, err := r.getManifest("latest", ImageManifestV2)
	if err != nil {
		return err
	}
	switch manifest.MediaType {
	case ManifestList:
		log.Infof("Got manifest list, requesting the first manifest in the list")
		manifest, err = r.getManifest(manifest.Manifests[0].Digest, manifest.Manifests[0].MediaType)
	case ImageManifestV1:
	case ImageManifestV2:
	default:
		return errors.New(fmt.Sprintf("Unknown manifest type: %s", manifest.MediaType))
	}

	err = r.getLayers(manifest, root)
	if err != nil {
		return err
	}
	return nil
}
