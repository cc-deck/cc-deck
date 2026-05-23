package oci

import (
	"archive/tar"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
)

// PolicyLayerLabel is the OCI label key used to record which layer contains
// the policy file. The value is the layer's diff ID as "sha256:<hex>".
const PolicyLayerLabel = "dev.cc-deck.policy-layer"

// FindLayerContaining walks image layers in reverse order (topmost first) and
// returns the diff ID of the first layer that contains the specified file path.
// The reverse order matches OCI overlay filesystem semantics where later layers
// override earlier ones.
func FindLayerContaining(img v1.Image, filePath string) (v1.Hash, error) {
	layers, err := img.Layers()
	if err != nil {
		return v1.Hash{}, fmt.Errorf("reading image layers: %w", err)
	}

	// Normalize the file path: strip leading slash for tar header comparison.
	normalizedPath := strings.TrimPrefix(filePath, "/")

	// Walk layers in reverse order (topmost layer first).
	for i := len(layers) - 1; i >= 0; i-- {
		layer := layers[i]
		diffID, err := layer.DiffID()
		if err != nil {
			log.Printf("DEBUG: oci: skipping layer %d: cannot read diff ID: %v", i, err)
			continue
		}

		found, err := layerContainsFile(layer, normalizedPath)
		if err != nil {
			log.Printf("DEBUG: oci: skipping layer %d (%s): %v", i, diffID, err)
			continue
		}
		if found {
			log.Printf("DEBUG: oci: found %s in layer %d (%s)", filePath, i, diffID)
			return diffID, nil
		}
		log.Printf("DEBUG: oci: layer %d (%s) does not contain %s", i, diffID, filePath)
	}

	return v1.Hash{}, fmt.Errorf("file %s not found in any image layer", filePath)
}

// layerContainsFile checks whether a single layer's tar archive contains
// the specified file path.
func layerContainsFile(layer v1.Layer, normalizedPath string) (bool, error) {
	rc, err := layer.Uncompressed()
	if err != nil {
		return false, fmt.Errorf("opening layer: %w", err)
	}
	defer rc.Close()

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, fmt.Errorf("reading tar: %w", err)
		}

		entryPath := strings.TrimPrefix(hdr.Name, "./")
		entryPath = strings.TrimPrefix(entryPath, "/")

		if entryPath == normalizedPath && hdr.Typeflag == tar.TypeReg {
			return true, nil
		}
	}
	return false, nil
}

// StampPolicyLabel loads an image from the local daemon, finds the layer
// containing the policy file, and stamps the image with the PolicyLayerLabel.
// The mutated image is written back to the local daemon. Returns nil if the
// policy file is not found in the image (logs a warning instead of failing).
func StampPolicyLabel(imageRef, filePath string) error {
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return fmt.Errorf("parsing image reference %q: %w", imageRef, err)
	}

	img, err := daemon.Image(ref)
	if err != nil {
		return fmt.Errorf("loading image %s from local daemon: %w", imageRef, err)
	}

	layerHash, err := FindLayerContaining(img, filePath)
	if err != nil {
		log.Printf("WARNING: oci: policy file %s not found in image %s, skipping label stamping", filePath, imageRef)
		return nil
	}

	labeled, err := AddLabel(img, PolicyLayerLabel, layerHash.String())
	if err != nil {
		return fmt.Errorf("adding label to image: %w", err)
	}

	tag, ok := ref.(name.Tag)
	if !ok {
		return fmt.Errorf("image reference %s is not a tag, cannot write back to daemon", imageRef)
	}

	if _, err := daemon.Write(tag, labeled); err != nil {
		return fmt.Errorf("writing labeled image back to daemon: %w", err)
	}

	log.Printf("INFO: oci: stamped %s with %s=%s", imageRef, PolicyLayerLabel, layerHash)
	return nil
}

// AddLabel adds a key-value label to an OCI image's config. The mutated image
// replaces the original in memory. This function operates on an in-memory
// v1.Image and returns the modified image. The caller is responsible for
// writing the image back to the daemon or registry.
func AddLabel(img v1.Image, key, value string) (v1.Image, error) {
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("reading image config: %w", err)
	}

	if cfg.Config.Labels == nil {
		cfg.Config.Labels = make(map[string]string)
	}
	cfg.Config.Labels[key] = value

	mutated, err := mutate.Config(img, cfg.Config)
	if err != nil {
		return nil, fmt.Errorf("mutating image config: %w", err)
	}

	return mutated, nil
}
