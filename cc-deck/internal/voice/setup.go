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
	"sort"
	"strings"
	"time"

	"github.com/cc-deck/cc-deck/internal/xdg"
)

// ModelInfo describes a downloadable whisper model.
type ModelInfo struct {
	Name     string
	FileName string
	URL      string
	TreeAPI  string // HF tree API endpoint for SHA lookup; empty means hfTreeAPI
}

// TdrzModelName is the model name for tinydiarize turn detection.
const TdrzModelName = "small.en-tdrz"

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
	"large-v3-turbo": {
		Name:     "large-v3-turbo",
		FileName: "ggml-large-v3-turbo.bin",
		URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin",
	},
	TdrzModelName: {
		Name:     TdrzModelName,
		FileName: "ggml-small.en-tdrz.bin",
		URL:      "https://huggingface.co/akashmjn/tinydiarize-whisper.cpp/resolve/main/ggml-small.en-tdrz.bin",
		TreeAPI:  "https://huggingface.co/api/models/akashmjn/tinydiarize-whisper.cpp/tree/main",
	},
}

const hfTreeAPI = "https://huggingface.co/api/models/ggerganov/whisper.cpp/tree/main"

// speechFilterModel is the Silero voice activity model that whisper-server
// and whisper-cli use (--vad) to skip non-speech audio before transcribing.
// Without it, large models turn keyboard noise or breathing into phantom
// words such as "Thank you." It is not a transcription model, so it is kept
// out of the models map and cannot be selected with --model.
var speechFilterModel = ModelInfo{
	Name:     "silero-vad",
	FileName: "ggml-silero-v6.2.0.bin",
	URL:      "https://huggingface.co/ggml-org/whisper-vad/resolve/main/ggml-silero-v6.2.0.bin",
	TreeAPI:  "https://huggingface.co/api/models/ggml-org/whisper-vad/tree/main",
}

// SpeechFilterModelPath returns where the speech filter model is installed.
func SpeechFilterModelPath() string {
	return filepath.Join(ModelDir(), speechFilterModel.FileName)
}

// ResolveSpeechFilter returns the speech filter model path to pass to the
// whisper tools, or "" when the filter is disabled or not installed.
// missing reports that the filter is enabled but its model is absent.
func ResolveSpeechFilter(enabled bool) (modelPath string, missing bool) {
	return resolveSpeechFilter(enabled, SpeechFilterModelPath())
}

func resolveSpeechFilter(enabled bool, path string) (string, bool) {
	if !enabled {
		return "", false
	}
	if _, err := os.Stat(path); err != nil {
		return "", true
	}
	return path, false
}

// ModelNames returns the names of all downloadable models, sorted.
func ModelNames() []string {
	names := make([]string, 0, len(models))
	for name := range models {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
		return fmt.Errorf("unknown model %q; available: %s", modelName, strings.Join(ModelNames(), ", "))
	}

	if err := installModel(ctx, info); err != nil {
		return err
	}
	// The speech filter is small and shared by all models, so setup always
	// installs it. The relay uses it to skip non-speech audio.
	if err := installModel(ctx, speechFilterModel); err != nil {
		return fmt.Errorf("installing speech filter: %w", err)
	}

	fmt.Println("\nSetup complete. Ready for voice relay.")
	return nil
}

// installModel downloads a model into ModelDir unless an up-to-date copy is
// already present, printing progress for the --setup command.
func installModel(ctx context.Context, info ModelInfo) error {
	modelPath := filepath.Join(ModelDir(), info.FileName)
	shaPath := modelPath + ".sha256"

	remoteSHA, remoteSize, err := fetchRemoteSHA(ctx, info)
	if err != nil {
		fmt.Printf("  [!] Could not fetch checksum for %s from Hugging Face: %v\n", info.Name, err)
		fmt.Println("      Falling back to download without verification.")
		remoteSHA = ""
	}

	if _, err := os.Stat(modelPath); err == nil {
		if remoteSHA != "" {
			if localSHA, err := readSHAFile(shaPath); err == nil && localSHA == remoteSHA {
				fmt.Printf("Model %s is up to date (%s)\n", info.Name, modelPath)
				return nil
			}
		} else {
			fmt.Printf("Model %s already downloaded (%s)\n", info.Name, modelPath)
			return nil
		}
		fmt.Printf("Model %s has a newer version available. Re-downloading.\n", info.Name)
	}

	if remoteSize >= 1_000_000 {
		fmt.Printf("Downloading %s (%d MB)...\n", info.Name, remoteSize/1_000_000)
	} else {
		fmt.Printf("Downloading %s...\n", info.Name)
	}
	if err := downloadModel(ctx, info, modelPath, remoteSHA, remoteSize, nil); err != nil {
		return fmt.Errorf("downloading model %s: %w", info.Name, err)
	}
	fmt.Printf("Model saved to %s\n", modelPath)
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
// It uses the model's own TreeAPI endpoint when set, falling back to hfTreeAPI.
func fetchRemoteSHA(ctx context.Context, info ModelInfo) (sha string, size int64, err error) {
	treeURL := info.TreeAPI
	if treeURL == "" {
		treeURL = hfTreeAPI
	}
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, treeURL, nil)
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
		if e.Path == info.FileName && e.LFS != nil {
			return e.LFS.OID, e.LFS.Size, nil
		}
	}
	return "", 0, fmt.Errorf("file %q not found in repository listing", info.FileName)
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

// TdrzStatus checks whether the prerequisites for tdrz turn detection are met.
// Returns toolErr if whisper-cli is not on PATH, and modelErr if the tdrz model
// file is missing.
func TdrzStatus() (toolErr, modelErr error) {
	if _, err := exec.LookPath("whisper-cli"); err != nil {
		toolErr = fmt.Errorf("whisper-cli not found; install whisper-cpp (brew install whisper-cpp)")
	}
	modelPath := ModelPath(TdrzModelName)
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		modelErr = fmt.Errorf("tdrz model not found at %s", modelPath)
	} else if err != nil {
		modelErr = fmt.Errorf("checking tdrz model: %w", err)
	}
	return toolErr, modelErr
}

// countingWriter wraps an io.Writer and reports bytes written through a
// progress callback.
type countingWriter struct {
	written  int64
	total    int64
	progress func(done, total int64)
	inner    io.Writer
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.inner.Write(p)
	cw.written += int64(n)
	if cw.progress != nil {
		cw.progress(cw.written, cw.total)
	}
	return n, err
}

// DownloadModel downloads a named model with SHA-256 verification.
// The progress callback, if non-nil, is called with bytes written and total.
func DownloadModel(ctx context.Context, name string, progress func(done, total int64)) error {
	info, ok := models[name]
	if !ok {
		return fmt.Errorf("unknown model %q", name)
	}

	destPath := filepath.Join(ModelDir(), info.FileName)

	remoteSHA, remoteSize, err := fetchRemoteSHA(ctx, info)
	if err != nil {
		remoteSHA = ""
		remoteSize = 0
	}

	return downloadModel(ctx, info, destPath, remoteSHA, remoteSize, progress)
}

func downloadModel(ctx context.Context, info ModelInfo, destPath, expectedSHA string, expectedSize int64, progress func(done, total int64)) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("creating model directory: %w", err)
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

	f, err := os.CreateTemp(filepath.Dir(destPath), filepath.Base(destPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := f.Name()

	hasher := sha256.New()
	var writer io.Writer
	if progress != nil {
		cw := &countingWriter{inner: io.MultiWriter(f, hasher), total: expectedSize, progress: progress}
		writer = cw
	} else {
		writer = io.MultiWriter(f, hasher)
	}

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

	// No stdout output here: the voice TUI calls this while bubbletea owns
	// the alternate screen. Callers report progress and results themselves.
	return nil
}
