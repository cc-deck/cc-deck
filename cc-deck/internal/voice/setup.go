package voice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cc-deck/cc-deck/internal/xdg"
)

// ModelInfo describes a downloadable whisper model.
type ModelInfo struct {
	Name     string
	FileName string
	URL      string
}

var models = map[string]ModelInfo{
	"tiny.en": {
		Name:     "tiny.en",
		FileName: "ggml-tiny.en.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.en.bin",
	},
	"base.en": {
		Name:     "base.en",
		FileName: "ggml-base.en.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en.bin",
	},
	"small.en": {
		Name:     "small.en",
		FileName: "ggml-small.en.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.en.bin",
	},
	"medium": {
		Name:     "medium",
		FileName: "ggml-medium.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin",
	},
}

const hfTreeAPI = "https://huggingface.co/api/models/ggerganov/whisper.cpp/tree/main"

// ModelDir returns the path where whisper models are cached.
func ModelDir() string {
	return filepath.Join(xdg.CacheHome, "cc-deck", "models")
}

// ModelPath returns the full path for a named model.
func ModelPath(name string) string {
	info, ok := models[name]
	if !ok {
		safe := filepath.Base(name)
		return filepath.Join(ModelDir(), fmt.Sprintf("ggml-%s.bin", safe))
	}
	return filepath.Join(ModelDir(), info.FileName)
}

// RunSetup checks dependencies and downloads the model.
func RunSetup(modelName string) error {
	return RunSetupWithContext(context.Background(), modelName)
}

func RunSetupWithContext(ctx context.Context, modelName string) error {
	fmt.Println("Voice Relay Setup")
	fmt.Println()

	checkDependency("whisper-server")
	checkDependency("whisper-cli")
	fmt.Println()

	info, ok := models[modelName]
	if !ok {
		return fmt.Errorf("unknown model %q; available: tiny.en, base.en, small.en, medium", modelName)
	}

	modelPath := filepath.Join(ModelDir(), info.FileName)
	shaPath := modelPath + ".sha256"

	remoteSHA, remoteSize, err := fetchRemoteSHA(ctx, info.FileName)
	if err != nil {
		fmt.Printf("  [!] Could not fetch checksum from Hugging Face: %v\n", err)
		fmt.Println("      Falling back to download without verification.")
		remoteSHA = ""
	}

	if _, err := os.Stat(modelPath); err == nil {
		if remoteSHA != "" {
			if localSHA, err := readSHAFile(shaPath); err == nil && localSHA == remoteSHA {
				fmt.Printf("Model %s is up to date (%s)\n", modelName, modelPath)
				fmt.Println("\nSetup complete. Ready for voice relay.")
				return nil
			}
		} else {
			fmt.Printf("Model %s already downloaded (%s)\n", modelName, modelPath)
			fmt.Println("\nSetup complete. Ready for voice relay.")
			return nil
		}
		fmt.Printf("Model %s has a newer version available. Re-downloading.\n", modelName)
	}

	if err := downloadModel(ctx, info, modelPath, remoteSHA, remoteSize); err != nil {
		return fmt.Errorf("downloading model: %w", err)
	}

	fmt.Println("\nSetup complete. Ready for voice relay.")
	return nil
}

// ValidateModel checks that a model file exists.
func ValidateModel(modelName string) error {
	info, ok := models[modelName]
	if !ok {
		return fmt.Errorf("unknown model %q", modelName)
	}

	modelPath := filepath.Join(ModelDir(), info.FileName)
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model %q not found; run: cc-deck ws voice --setup", modelName)
	} else if err != nil {
		return fmt.Errorf("checking model: %w", err)
	}
	return nil
}

func checkDependency(name string) {
	path, err := exec.LookPath(name)
	if err != nil {
		fmt.Printf("  [!] %s: not found\n", name)
		fmt.Printf("      Install: brew install whisper-cpp\n")
	} else {
		fmt.Printf("  [+] %s: %s\n", name, path)
	}
}

type hfTreeEntry struct {
	Path string `json:"path"`
	LFS  *struct {
		OID  string `json:"oid"`
		Size int64  `json:"size"`
	} `json:"lfs"`
}

// fetchRemoteSHA queries the Hugging Face API for the LFS SHA256 and size of a model file.
func fetchRemoteSHA(ctx context.Context, fileName string) (sha string, size int64, err error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hfTreeAPI, nil)
	if err != nil {
		return "", 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var entries []hfTreeEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return "", 0, fmt.Errorf("parsing API response: %w", err)
	}

	for _, e := range entries {
		if e.Path == fileName && e.LFS != nil {
			return e.LFS.OID, e.LFS.Size, nil
		}
	}
	return "", 0, fmt.Errorf("file %q not found in repository listing", fileName)
}

func readSHAFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func writeSHAFile(path, sha string) error {
	return os.WriteFile(path, []byte(sha), 0o644)
}

func downloadModel(ctx context.Context, info ModelInfo, destPath, expectedSHA string, expectedSize int64) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("creating model directory: %w", err)
	}

	if expectedSize > 0 {
		fmt.Printf("Downloading %s (%d MB)...\n", info.Name, expectedSize/1024/1024)
	} else {
		fmt.Printf("Downloading %s...\n", info.Name)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.URL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	hasher := sha256.New()
	writer := io.MultiWriter(f, hasher)

	_, copyErr := io.Copy(writer, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("writing model: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing model file: %w", closeErr)
	}

	if expectedSHA != "" {
		actualSHA := hex.EncodeToString(hasher.Sum(nil))
		if actualSHA != expectedSHA {
			os.Remove(tmpPath)
			return fmt.Errorf("checksum mismatch: got %s, expected %s", actualSHA, expectedSHA)
		}
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("moving model file: %w", err)
	}

	if expectedSHA != "" {
		shaPath := destPath + ".sha256"
		writeSHAFile(shaPath, expectedSHA)
	}

	fmt.Printf("Model saved to %s\n", destPath)
	return nil
}
