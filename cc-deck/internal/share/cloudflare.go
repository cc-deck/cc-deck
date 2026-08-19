package share

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const cloudflareReadyTimeout = 10 * time.Second

type CloudflareProvider struct {
	runner       CommandRunner
	readyTimeout time.Duration
	pollInterval time.Duration
	stopTimeout  time.Duration
	mu           sync.Mutex
	processes    map[string]*cloudflareRuntime
	findProcess  func(int) (Process, error)
}
type cloudflareRuntime struct {
	process Process
	exited  chan error
}

func NewCloudflareProvider(r CommandRunner) *CloudflareProvider {
	return &CloudflareProvider{runner: r, readyTimeout: cloudflareReadyTimeout, pollInterval: 25 * time.Millisecond, stopTimeout: 500 * time.Millisecond, processes: map[string]*cloudflareRuntime{}, findProcess: func(pid int) (Process, error) {
		proc, err := os.FindProcess(pid)
		if err != nil {
			return nil, err
		}
		return &osProcessAdapter{Process: proc}, nil
	}}
}
func (p *CloudflareProvider) Name() string { return "cloudflare" }
func (p *CloudflareProvider) Validate(ctx context.Context) error {
	if _, e := p.runner.Run(ctx, "cloudflared", "--version"); e != nil {
		return fmt.Errorf("cloudflared is required: %w", e)
	}
	return nil
}
func (p *CloudflareProvider) Start(ctx context.Context, localURL string) (ProviderHandle, error) {
	logPath := filepath.Join(os.TempDir(), fmt.Sprintf("cc-deck-cloudflared-%d.log", time.Now().UnixNano()))
	proc, e := p.runner.Start(ctx, "cloudflared", "tunnel", "--url", localURL, "--logfile", logPath, "--no-autoupdate")
	if e != nil {
		return ProviderHandle{}, fmt.Errorf("start Cloudflare Quick Tunnel: %w", e)
	}
	rt := &cloudflareRuntime{process: proc, exited: make(chan error, 1)}
	p.mu.Lock()
	p.processes[logPath] = rt
	p.mu.Unlock()
	go func() { rt.exited <- proc.Wait(); close(rt.exited) }()
	fingerprint, err := p.processFingerprint(ctx, proc.PID())
	if err != nil {
		_ = proc.Kill()
		return ProviderHandle{}, fmt.Errorf("capture cloudflared process identity: %w", err)
	}
	return ProviderHandle{PID: proc.PID(), Metadata: map[string]string{"log_path": logPath, "identity": logPath, "process_fingerprint": fingerprint}}, nil
}

var quickTunnelURL = regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)
var errCloudflareProcessGone = errors.New("cloudflared process is gone")

func (p *CloudflareProvider) Ready(ctx context.Context, h ProviderHandle) (ProviderStatus, error) {
	t := time.NewTimer(p.readyTimeout)
	defer t.Stop()
	tick := time.NewTicker(p.pollInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ProviderStatus{}, ctx.Err()
		case <-t.C:
			return ProviderStatus{State: "failed", Diagnostic: "timed out waiting for Cloudflare Quick Tunnel"}, fmt.Errorf("Cloudflare Quick Tunnel readiness timeout")
		case err := <-p.exitChannel(h):
			return ProviderStatus{State: "failed", Diagnostic: "cloudflared exited before readiness"}, fmt.Errorf("cloudflared exited before readiness: %w", err)
		case <-tick.C:
			b, e := os.ReadFile(h.Metadata["log_path"])
			if e != nil && !os.IsNotExist(e) {
				return ProviderStatus{}, fmt.Errorf("read cloudflared readiness log: %w", e)
			}
			if u := quickTunnelURL.FindString(string(b)); u != "" {
				return ProviderStatus{State: "ready", EndpointURL: u}, nil
			}
		}
	}
}
func (p *CloudflareProvider) exitChannel(h ProviderHandle) <-chan error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if rt := p.processes[h.Metadata["identity"]]; rt != nil {
		return rt.exited
	}
	return nil
}
func (p *CloudflareProvider) Status(ctx context.Context, h ProviderHandle) (ProviderStatus, error) {
	if h.PID <= 0 {
		return ProviderStatus{State: "stopped"}, nil
	}
	if err := p.validateIdentity(ctx, h); err != nil {
		if errors.Is(err, errCloudflareProcessGone) {
			return ProviderStatus{State: "stopped"}, nil
		}
		return ProviderStatus{State: "unknown", Diagnostic: err.Error()}, nil
	}
	proc, e := p.findProcess(h.PID)
	if e != nil {
		return ProviderStatus{State: "stopped"}, nil
	}
	if e = proc.Signal(syscall.Signal(0)); e != nil {
		return ProviderStatus{State: "stopped"}, nil
	}
	return ProviderStatus{State: "ready", EndpointURL: h.Metadata["endpoint"]}, nil
}
func (p *CloudflareProvider) Stop(ctx context.Context, h ProviderHandle) error {
	if h.PID <= 0 {
		return nil
	}
	if err := p.validateIdentity(ctx, h); err != nil {
		if errors.Is(err, errCloudflareProcessGone) {
			return nil
		}
		return err
	}
	p.mu.Lock()
	rt := p.processes[h.Metadata["identity"]]
	p.mu.Unlock()
	var proc Process
	if rt != nil {
		proc = rt.process
	} else {
		external, err := p.findProcess(h.PID)
		if err != nil {
			return nil
		}
		proc = external
	}
	if e := proc.Signal(os.Interrupt); e != nil && !isProcessGone(e) {
		return e
	}
	if rt == nil {
		return p.waitExternalStop(ctx, h, proc)
	}
	select {
	case <-rt.exited:
		_ = os.Remove(h.Metadata["log_path"])
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(p.stopTimeout):
	}
	if e := proc.Kill(); e != nil && !isProcessGone(e) {
		return fmt.Errorf("kill cloudflared: %w", e)
	}
	select {
	case <-rt.exited:
		_ = os.Remove(h.Metadata["log_path"])
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(p.stopTimeout):
		return fmt.Errorf("cloudflared did not stop after forced termination")
	}
}

type osProcessAdapter struct{ *os.Process }

func (p *osProcessAdapter) PID() int    { return p.Pid }
func (p *osProcessAdapter) Wait() error { _, e := p.Process.Wait(); return e }
func (p *osProcessAdapter) Kill() error { return p.Process.Kill() }
func (p *CloudflareProvider) waitExternalStop(ctx context.Context, h ProviderHandle, proc Process) error {
	if p.waitExternalGone(ctx, h, p.stopTimeout) {
		return nil
	}
	if err := proc.Kill(); err != nil && !isProcessGone(err) {
		return fmt.Errorf("kill cloudflared: %w", err)
	}
	if p.waitExternalGone(ctx, h, p.stopTimeout) {
		return nil
	}
	return fmt.Errorf("cloudflared process closure could not be confirmed")
}
func (p *CloudflareProvider) waitExternalGone(ctx context.Context, h ProviderHandle, timeout time.Duration) bool {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-tick.C:
			_, err := p.processFingerprint(ctx, h.PID)
			if err != nil && isProcessGone(err) {
				_ = os.Remove(h.Metadata["log_path"])
				return true
			}
		}
	}
}
func (p *CloudflareProvider) validateIdentity(ctx context.Context, h ProviderHandle) error {
	marker := h.Metadata["identity"]
	if marker == "" || marker != h.Metadata["log_path"] {
		return fmt.Errorf("cloudflared process identity missing or mismatched")
	}
	p.mu.Lock()
	rt := p.processes[marker]
	p.mu.Unlock()
	current, err := p.processFingerprint(ctx, h.PID)
	if err != nil {
		if isProcessGone(err) {
			return errCloudflareProcessGone
		}
		return fmt.Errorf("cannot validate cloudflared process identity: %w", err)
	}
	want := h.Metadata["process_fingerprint"]
	if want == "" || current != want || !strings.Contains(current, "cloudflared") || !strings.Contains(current, marker) {
		return fmt.Errorf("refusing to signal PID %d: cloudflared process identity mismatch", h.PID)
	}
	if rt != nil && rt.process.PID() != h.PID {
		return fmt.Errorf("cloudflared controller identity mismatch")
	}
	return nil
}
func (p *CloudflareProvider) processFingerprint(ctx context.Context, pid int) (string, error) {
	out, err := p.runner.Run(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "lstart=,comm=,command=")
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "", fmt.Errorf("no such process")
	}
	return s, nil
}
func isProcessGone(err error) bool {
	return err != nil && (regexp.MustCompile(`(?i)(finished|not found|no such process|exit status 1)`).MatchString(err.Error()))
}
