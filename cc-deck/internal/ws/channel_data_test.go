package ws

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cc-deck/cc-deck/internal/ssh"
)

func TestLocalDataChannel_Push(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "source.txt")
	dstFile := filepath.Join(dir, "dest.txt")
	os.WriteFile(srcFile, []byte("hello"), 0o644)

	ch := &localDataChannel{name: "test"}
	err := ch.Push(context.Background(), SyncOpts{
		LocalPath:  srcFile,
		RemotePath: dstFile,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(dstFile)
	if string(got) != "hello" {
		t.Errorf("content = %q, want %q", string(got), "hello")
	}
}

func TestLocalDataChannel_Pull(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "remote.txt")
	dstDir := filepath.Join(dir, "local")
	os.MkdirAll(dstDir, 0o755)
	os.WriteFile(srcFile, []byte("data"), 0o644)

	ch := &localDataChannel{name: "test"}
	err := ch.Pull(context.Background(), SyncOpts{
		RemotePath: srcFile,
		LocalPath:  dstDir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(dstDir, "remote.txt"))
	if string(got) != "data" {
		t.Errorf("content = %q, want %q", string(got), "data")
	}
}

func TestLocalDataChannel_PushBytes(t *testing.T) {
	dir := t.TempDir()
	dstFile := filepath.Join(dir, "output.bin")

	ch := &localDataChannel{name: "test"}
	err := ch.PushBytes(context.Background(), []byte("binary data"), dstFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(dstFile)
	if string(got) != "binary data" {
		t.Errorf("content = %q, want %q", string(got), "binary data")
	}
}

func TestLocalDataChannel_PushBytes_EmptyData(t *testing.T) {
	dir := t.TempDir()
	dstFile := filepath.Join(dir, "empty.bin")

	ch := &localDataChannel{name: "test"}
	err := ch.PushBytes(context.Background(), []byte{}, dstFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, _ := os.Stat(dstFile)
	if info.Size() != 0 {
		t.Errorf("file size = %d, want 0", info.Size())
	}
}

func TestLocalDataChannel_PushBytes_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	dstFile := filepath.Join(dir, "subdir", "nested", "file.txt")

	ch := &localDataChannel{name: "test"}
	err := ch.PushBytes(context.Background(), []byte("nested"), dstFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(dstFile)
	if string(got) != "nested" {
		t.Errorf("content = %q, want %q", string(got), "nested")
	}
}

func TestLocalDataChannel_Push_EmptyLocalPath(t *testing.T) {
	ch := &localDataChannel{name: "test"}
	err := ch.Push(context.Background(), SyncOpts{})
	if err == nil {
		t.Fatal("expected error for empty local path")
	}
	var chErr *ChannelError
	if !errors.As(err, &chErr) {
		t.Fatalf("expected ChannelError, got %T", err)
	}
	if chErr.Op != "push" {
		t.Errorf("Op = %q, want %q", chErr.Op, "push")
	}
}

func TestLocalDataChannel_Pull_EmptyRemotePath(t *testing.T) {
	ch := &localDataChannel{name: "test"}
	err := ch.Pull(context.Background(), SyncOpts{})
	if err == nil {
		t.Fatal("expected error for empty remote path")
	}
	var chErr *ChannelError
	if !errors.As(err, &chErr) {
		t.Fatalf("expected ChannelError, got %T", err)
	}
	if chErr.Op != "pull" {
		t.Errorf("Op = %q, want %q", chErr.Op, "pull")
	}
}

func TestLocalDataChannel_PushBytes_EmptyRemotePath(t *testing.T) {
	ch := &localDataChannel{name: "test"}
	err := ch.PushBytes(context.Background(), []byte("data"), "")
	if err == nil {
		t.Fatal("expected error for empty remote path")
	}
	var chErr *ChannelError
	if !errors.As(err, &chErr) {
		t.Fatalf("expected ChannelError, got %T", err)
	}
}

func TestLocalDataChannel_Push_DirectoryCopy(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	dstDir := filepath.Join(dir, "dst")
	os.MkdirAll(filepath.Join(srcDir, "sub"), 0o755)
	os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("file-a"), 0o644)
	os.WriteFile(filepath.Join(srcDir, "sub", "b.txt"), []byte("file-b"), 0o644)

	ch := &localDataChannel{name: "test"}
	err := ch.Push(context.Background(), SyncOpts{
		LocalPath:  srcDir,
		RemotePath: dstDir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotA, _ := os.ReadFile(filepath.Join(dstDir, "a.txt"))
	if string(gotA) != "file-a" {
		t.Errorf("a.txt content = %q, want %q", string(gotA), "file-a")
	}
	gotB, _ := os.ReadFile(filepath.Join(dstDir, "sub", "b.txt"))
	if string(gotB) != "file-b" {
		t.Errorf("sub/b.txt content = %q, want %q", string(gotB), "file-b")
	}
}

func TestPodmanDataChannel_Push_EmptyLocalPath(t *testing.T) {
	ch := &podmanDataChannel{
		name:          "test",
		containerName: func() string { return "cc-deck-test" },
	}
	err := ch.Push(context.Background(), SyncOpts{})
	if err == nil {
		t.Fatal("expected error for empty local path")
	}
	var chErr *ChannelError
	if !errors.As(err, &chErr) {
		t.Fatalf("expected ChannelError, got %T", err)
	}
}

func TestPodmanDataChannel_Pull_EmptyRemotePath(t *testing.T) {
	ch := &podmanDataChannel{
		name:          "test",
		containerName: func() string { return "cc-deck-test" },
	}
	err := ch.Pull(context.Background(), SyncOpts{})
	if err == nil {
		t.Fatal("expected error for empty remote path")
	}
	var chErr *ChannelError
	if !errors.As(err, &chErr) {
		t.Fatalf("expected ChannelError, got %T", err)
	}
}

func TestPodmanDataChannel_PushBytes_EmptyRemotePath(t *testing.T) {
	ch := &podmanDataChannel{
		name:          "test",
		containerName: func() string { return "cc-deck-test" },
	}
	err := ch.PushBytes(context.Background(), []byte("data"), "")
	if err == nil {
		t.Fatal("expected error for empty remote path")
	}
}

func TestK8sDataChannel_PushBytes_EmptyRemotePath(t *testing.T) {
	ch := &k8sDataChannel{
		name:    "test",
		ns:      "default",
		podName: "pod-0",
	}
	err := ch.PushBytes(context.Background(), []byte("data"), "")
	if err == nil {
		t.Fatal("expected error for empty remote path")
	}
}

func TestSshDataChannel_Push_EmptyLocalPath(t *testing.T) {
	ch := &sshDataChannel{
		name:     "test",
		clientFn: func() *ssh.Client { return ssh.NewClient("host", 0, "", "", "") },
		workspace: func(_ context.Context) (string, error) {
			return "/workspace", nil
		},
	}
	err := ch.Push(context.Background(), SyncOpts{})
	if err == nil {
		t.Fatal("expected error for empty local path")
	}
	var chErr *ChannelError
	if !errors.As(err, &chErr) {
		t.Fatalf("expected ChannelError, got %T", err)
	}
	if chErr.Op != "push" {
		t.Errorf("Op = %q, want %q", chErr.Op, "push")
	}
}

func TestSshDataChannel_Pull_EmptyRemotePath(t *testing.T) {
	ch := &sshDataChannel{
		name:     "test",
		clientFn: func() *ssh.Client { return ssh.NewClient("host", 0, "", "", "") },
		workspace: func(_ context.Context) (string, error) {
			return "/workspace", nil
		},
	}
	err := ch.Pull(context.Background(), SyncOpts{})
	if err == nil {
		t.Fatal("expected error for empty remote path")
	}
	var chErr *ChannelError
	if !errors.As(err, &chErr) {
		t.Fatalf("expected ChannelError, got %T", err)
	}
	if chErr.Op != "pull" {
		t.Errorf("Op = %q, want %q", chErr.Op, "pull")
	}
}

func TestSshDataChannel_PushBytes_EmptyRemotePath(t *testing.T) {
	ch := &sshDataChannel{
		name:     "test",
		clientFn: func() *ssh.Client { return ssh.NewClient("host", 0, "", "", "") },
	}
	err := ch.PushBytes(context.Background(), []byte("data"), "")
	if err == nil {
		t.Fatal("expected error for empty remote path")
	}
	var chErr *ChannelError
	if !errors.As(err, &chErr) {
		t.Fatalf("expected ChannelError, got %T", err)
	}
}
