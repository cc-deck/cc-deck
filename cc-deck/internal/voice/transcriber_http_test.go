package voice

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPTranscriber_PromptFieldIncluded(t *testing.T) {
	var gotPrompt string
	var gotFile bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		mediaType, params, err := mime.ParseMediaType(ct)
		if err != nil {
			t.Errorf("parsing content type: %v", err)
			http.Error(w, "bad content type", 400)
			return
		}
		if !strings.HasPrefix(mediaType, "multipart/") {
			t.Errorf("expected multipart, got %q", mediaType)
			http.Error(w, "not multipart", 400)
			return
		}

		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("reading multipart: %v", err)
				break
			}
			if part.FormName() == "prompt" {
				data, _ := io.ReadAll(part)
				gotPrompt = string(data)
			}
			if part.FormName() == "file" {
				gotFile = true
				_, _ = io.ReadAll(part) // drain
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"hello"}`))
	}))
	defer server.Close()

	transcriber := NewHTTPTranscriber(server.URL, nil).(*httpTranscriber)
	transcriber.SetPrompt("Kubernetes, gRPC, Helm")

	audio := make([]int16, 100)
	_, err := transcriber.Transcribe(context.Background(), audio, 16000)
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}

	if !gotFile {
		t.Error("expected file part in multipart request")
	}
	if gotPrompt != "Kubernetes, gRPC, Helm" {
		t.Errorf("prompt = %q, want %q", gotPrompt, "Kubernetes, gRPC, Helm")
	}
}

func TestHTTPTranscriber_PromptFieldOmittedWhenEmpty(t *testing.T) {
	var gotPrompt bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		_, params, err := mime.ParseMediaType(ct)
		if err != nil {
			http.Error(w, "bad content type", 400)
			return
		}

		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}
			if part.FormName() == "prompt" {
				gotPrompt = true
			}
			_, _ = io.ReadAll(part) // drain
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"hello"}`))
	}))
	defer server.Close()

	// No SetPrompt call - prompt should be empty
	transcriber := NewHTTPTranscriber(server.URL, nil).(*httpTranscriber)

	audio := make([]int16, 100)
	_, err := transcriber.Transcribe(context.Background(), audio, 16000)
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}

	if gotPrompt {
		t.Error("expected prompt field to be omitted when no glossary is set")
	}
}

func TestHTTPTranscriber_SetPrompt(t *testing.T) {
	transcriber := NewHTTPTranscriber("http://localhost:1234", nil).(*httpTranscriber)

	if transcriber.prompt != "" {
		t.Errorf("initial prompt = %q, want empty", transcriber.prompt)
	}

	transcriber.SetPrompt("Kubernetes, Helm")
	if transcriber.prompt != "Kubernetes, Helm" {
		t.Errorf("prompt after SetPrompt = %q, want %q", transcriber.prompt, "Kubernetes, Helm")
	}

	transcriber.SetPrompt("")
	if transcriber.prompt != "" {
		t.Errorf("prompt after SetPrompt('') = %q, want empty", transcriber.prompt)
	}
}
