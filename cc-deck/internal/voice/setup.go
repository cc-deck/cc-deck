package voice

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cc-deck/cc-deck/internal/xdg"
)

// ModelInfo describes a downloadable whisper model.
type ModelInfo struct {
	Name     string
	FileName string
	Size     int64
	URL      string
}

var models = map[string]ModelInfo{
	"tiny.en": {
		Name:     "tiny.en",
		FileName: "ggml-tiny.en.bin",
		Size:     77704715,
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.en.bin",
	},
	"base.en": {
		Name:     "base.en",
		FileName: "ggml-base.en.bin",
		Size:     147964211,
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en.bin",
	},
	"small.en": {
		Name:     "small.en",
		FileName: "ggml-small.en.bin",
		Size:     487601967,
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.en.bin",
	},
	"medium": {
		Name:     "medium",
		FileName: "ggml-medium.bin",
		Size:     1533774781,
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin",
	},
}

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
	if stat, err := os.Stat(modelPath); err == nil {
		if stat.Size() == info.Size {
			fmt.Printf("Model %s already downloaded (%s)\n", modelName, modelPath)
			fmt.Println("\nSetup complete. Ready for voice relay.")
			return nil
		}
		fmt.Printf("Model file exists but size mismatch (got %d, expected %d). Re-downloading.\n",
			stat.Size(), info.Size)
	}

	if err := downloadModel(info, modelPath); err != nil {
		return fmt.Errorf("downloading model: %w", err)
	}

	fmt.Println("\nSetup complete. Ready for voice relay.")
	return nil
}

// ValidateModel checks that a model exists and has the expected size.
func ValidateModel(modelName string) error {
	info, ok := models[modelName]
	if !ok {
		return fmt.Errorf("unknown model %q", modelName)
	}

	modelPath := filepath.Join(ModelDir(), info.FileName)
	stat, err := os.Stat(modelPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("model %q not found; run: cc-deck ws voice --setup", modelName)
	}
	if err != nil {
		return fmt.Errorf("checking model: %w", err)
	}
	if stat.Size() != info.Size {
		return fmt.Errorf("model %q is corrupted (size %d, expected %d); run: cc-deck ws voice --setup",
			modelName, stat.Size(), info.Size)
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

func downloadModel(info ModelInfo, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("creating model directory: %w", err)
	}

	fmt.Printf("Downloading %s (%d MB)...\n", info.Name, info.Size/1024/1024)

	resp, err := http.Get(info.URL)
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

	written, err := io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("writing model: %w", err)
	}

	if written != info.Size {
		os.Remove(tmpPath)
		return fmt.Errorf("incomplete download: got %d bytes, expected %d", written, info.Size)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("moving model file: %w", err)
	}

	fmt.Printf("Model saved to %s\n", destPath)
	return nil
}
