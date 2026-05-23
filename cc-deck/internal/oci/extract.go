package oci

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// ExtractFileFromImage extracts the contents of a file at the given path from
// an OCI image.
//
// For local images it uses the container runtime's cp command (podman/docker),
// which reads from the overlay filesystem without exporting the full image.
//
// For remote registry images it uses go-containerregistry with lazy per-layer
// fetching, using the policy layer label for single-layer access when available.
func ExtractFileFromImage(imageRef, filePath string) ([]byte, error) {
	data, err := extractLocal(imageRef, filePath)
	if err == nil {
		return data, nil
	}
	log.Printf("DEBUG: oci: local extraction failed: %v, trying remote registry", err)

	return extractRemote(imageRef, filePath)
}

// extractLocal extracts a file from a local image by creating a temporary
// container and copying the file from its overlay filesystem. This never
// exports the full image, making it fast even for multi-GB images.
func extractLocal(imageRef, filePath string) ([]byte, error) {
	runtime, err := detectRuntime()
	if err != nil {
		return nil, err
	}

	// Verify the image exists locally before creating a container.
	checkCmd := exec.Command(runtime, "image", "exists", imageRef)
	if err := checkCmd.Run(); err != nil {
		return nil, fmt.Errorf("image %s not found in local %s daemon", imageRef, runtime)
	}

	// Create a container without starting it (instant, no image export).
	createCmd := exec.Command(runtime, "create", imageRef, "true")
	output, err := createCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s create failed: %s", runtime, strings.TrimSpace(string(output)))
	}
	cid := strings.TrimSpace(string(output))
	defer func() {
		rmCmd := exec.Command(runtime, "rm", cid)
		rmCmd.Run()
	}()

	log.Printf("INFO: oci: extracting %s from %s via %s cp", filePath, imageRef, runtime)

	tmpFile, err := os.CreateTemp("", "oci-extract-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	cpCmd := exec.Command(runtime, "cp", cid+":"+filePath, tmpPath)
	if cpOut, err := cpCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("file %s not found in image: %s", filePath, strings.TrimSpace(string(cpOut)))
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("reading extracted file: %w", err)
	}

	log.Printf("INFO: oci: extracted %s from local image %s (%d bytes)", filePath, imageRef, len(data))
	return data, nil
}

func detectRuntime() (string, error) {
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman", nil
	}
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker", nil
	}
	return "", fmt.Errorf("neither podman nor docker found")
}

// extractRemote extracts a file from a remote registry image using
// go-containerregistry. Layer access is lazy, so only inspected layers are
// downloaded. If the image has the policy layer label, only that single layer
// is fetched.
func extractRemote(imageRef, filePath string) ([]byte, error) {
	ref, err := name.ParseReference(imageRef, name.WithDefaultRegistry(""))
	if err != nil {
		return nil, fmt.Errorf("parsing image reference %q: %w", imageRef, err)
	}

	img, err := remote.Image(ref)
	if err != nil {
		return nil, fmt.Errorf("image %s not found in remote registry: %w\n\nTo provide the policy file manually, use the --policy flag", imageRef, err)
	}
	log.Printf("INFO: oci: resolved image %s from remote registry", imageRef)

	data, err := extractViaLabel(img, filePath)
	if err == nil {
		log.Printf("INFO: oci: extracted %s from labeled layer (remote)", filePath)
		return data, nil
	}
	log.Printf("DEBUG: oci: label-based extraction failed: %v, falling back to layer scan", err)

	data, err = extractViaLayerScan(img, filePath)
	if err != nil {
		return nil, fmt.Errorf("file %s not found in image %s: %w\n\nTo provide the policy file manually, use the --policy flag", filePath, imageRef, err)
	}
	log.Printf("INFO: oci: extracted %s via layer scan (remote, fallback)", filePath)
	return data, nil
}

// extractViaLabel attempts to extract a file from the layer identified by the
// PolicyLayerLabel. Returns an error if the label is missing, the labeled layer
// cannot be found, or the file is not in that layer.
func extractViaLabel(img v1.Image, filePath string) ([]byte, error) {
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("reading image config: %w", err)
	}

	labelValue, ok := cfg.Config.Labels[PolicyLayerLabel]
	if !ok || labelValue == "" {
		return nil, fmt.Errorf("image has no %s label", PolicyLayerLabel)
	}

	labelHash, err := v1.NewHash(labelValue)
	if err != nil {
		return nil, fmt.Errorf("invalid label value %q: %w", labelValue, err)
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("reading layers: %w", err)
	}

	for _, layer := range layers {
		diffID, err := layer.DiffID()
		if err != nil {
			continue
		}
		if diffID == labelHash {
			data, err := extractFileFromLayer(layer, filePath)
			if err != nil {
				return nil, fmt.Errorf("file not in labeled layer %s: %w", labelHash, err)
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("labeled layer %s not found in image (stale label)", labelHash)
}

// extractViaLayerScan walks all layers in reverse order and extracts the file
// from the topmost layer that contains it.
func extractViaLayerScan(img v1.Image, filePath string) ([]byte, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("reading layers: %w", err)
	}

	normalizedPath := strings.TrimPrefix(filePath, "/")

	for i := len(layers) - 1; i >= 0; i-- {
		layer := layers[i]
		diffID, _ := layer.DiffID()
		log.Printf("DEBUG: oci: scanning layer %d (%s) for %s", i, diffID, filePath)

		data, err := extractFileFromLayer(layer, filePath)
		if err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("file %s not found in any layer", normalizedPath)
}

// extractFileFromLayer reads a single file from a layer's tar archive.
func extractFileFromLayer(layer v1.Layer, filePath string) ([]byte, error) {
	normalizedPath := strings.TrimPrefix(filePath, "/")

	rc, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("opening layer: %w", err)
	}
	defer rc.Close()

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar: %w", err)
		}

		entryPath := strings.TrimPrefix(hdr.Name, "./")
		entryPath = strings.TrimPrefix(entryPath, "/")

		if entryPath == normalizedPath && hdr.Typeflag == tar.TypeReg {
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, tr); err != nil {
				return nil, fmt.Errorf("reading file from tar: %w", err)
			}
			return buf.Bytes(), nil
		}
	}

	return nil, fmt.Errorf("file %s not found in layer", normalizedPath)
}
