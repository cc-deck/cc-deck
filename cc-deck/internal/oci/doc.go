// Package oci provides functions for extracting files from OCI container images
// and managing image labels. It supports both local daemon images (via the
// Docker-compatible API used by podman) and remote registry images.
//
// The primary use case is extracting policy files from OpenShell sandbox images
// at runtime, enabling sandbox creation without requiring the original build
// directory on the host.
package oci
