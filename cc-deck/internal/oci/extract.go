package oci

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// ExtractFileFromImage extracts the contents of a file at the given path from
// an OCI image. It first tries the local container daemon, then falls back to
// a remote registry.
//
// If the image has a "dev.cc-deck.policy-layer" label, extraction targets that
// specific layer for faster retrieval. If the label is missing, invalid, or the
// labeled layer does not contain the file, it falls back to a reverse layer scan.
func ExtractFileFromImage(imageRef, filePath string) ([]byte, error) {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, fmt.Errorf("parsing image reference %q: %w", imageRef, err)
	}

	img, source, err := resolveImage(ref)
	if err != nil {
		return nil, fmt.Errorf("resolving image %s: %w\n\nTo provide the policy file manually, use the --policy flag", imageRef, err)
	}
	log.Printf("INFO: oci: resolved image %s from %s", imageRef, source)

	// Try fast path: check for the policy layer label.
	data, err := extractViaLabel(img, filePath)
	if err == nil {
		log.Printf("INFO: oci: extracted %s from labeled layer", filePath)
		return data, nil
	}
	log.Printf("DEBUG: oci: label-based extraction failed: %v, falling back to layer scan", err)

	// Fallback: scan all layers in reverse order.
	data, err = extractViaLayerScan(img, filePath)
	if err != nil {
		return nil, fmt.Errorf("file %s not found in image %s: %w\n\nTo provide the policy file manually, use the --policy flag", filePath, imageRef, err)
	}
	log.Printf("INFO: oci: extracted %s via layer scan (fallback)", filePath)
	return data, nil
}

// resolveImage attempts to load the image from the local daemon first,
// then falls back to a remote registry.
func resolveImage(ref name.Reference) (v1.Image, string, error) {
	// Try local daemon first.
	img, err := daemon.Image(ref)
	if err == nil {
		return img, "local daemon", nil
	}
	log.Printf("DEBUG: oci: local daemon lookup failed: %v, trying remote registry", err)

	// Fall back to remote registry.
	img, err = remote.Image(ref)
	if err != nil {
		return nil, "", fmt.Errorf("image not found locally or in remote registry: %w", err)
	}
	return img, "remote registry", nil
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

	// Find the layer matching the labeled diff ID.
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
