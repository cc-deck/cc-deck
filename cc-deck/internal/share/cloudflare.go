package share

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
	"time"
)

const cloudflareReadyTimeout = 10 * time.Second

type CloudflareProvider struct {
	runner       CommandRunner
	readyTimeout time.Duration
	pollInterval time.Duration
}

func NewCloudflareProvider(r CommandRunner) *CloudflareProvider {
	return &CloudflareProvider{runner: r, readyTimeout: cloudflareReadyTimeout, pollInterval: 25 * time.Millisecond}
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
	return ProviderHandle{PID: proc.PID(), Metadata: map[string]string{"log_path": logPath}}, nil
}

var quickTunnelURL = regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)

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
func (p *CloudflareProvider) Status(ctx context.Context, h ProviderHandle) (ProviderStatus, error) {
	if h.PID <= 0 {
		return ProviderStatus{State: "stopped"}, nil
	}
	proc, e := os.FindProcess(h.PID)
	if e != nil {
		return ProviderStatus{State: "unknown", Diagnostic: e.Error()}, nil
	}
	if e = proc.Signal(syscall.Signal(0)); e != nil {
		return ProviderStatus{State: "stopped"}, nil
	}
	return ProviderStatus{State: "ready", EndpointURL: h.Metadata["endpoint"]}, nil
}
func (p *CloudflareProvider) Stop(ctx context.Context, h ProviderHandle) error {
	defer os.Remove(h.Metadata["log_path"])
	if h.PID <= 0 {
		return nil
	}
	proc, e := os.FindProcess(h.PID)
	if e != nil {
		return nil
	}
	if e = proc.Signal(os.Interrupt); e != nil && isProcessGone(e) {
		return nil
	}
	return e
}
func isProcessGone(err error) bool {
	return err != nil && (regexp.MustCompile(`(?i)(finished|not found|no such process)`).MatchString(err.Error()))
}
