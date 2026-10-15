package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const concurrentWriters = 20

// watchPaneMap decodes pane-map.json in a loop until stop is closed and
// counts reads that found a file that does not parse (SC-006).
func watchPaneMap(t *testing.T, stop <-chan struct{}) (wait func() (reads, bad int64)) {
	t.Helper()
	var reads, bad atomic.Int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			data, err := os.ReadFile(paneMapFile)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			reads.Add(1)
			var m map[string]paneMapEntry
			if err != nil || json.Unmarshal(data, &m) != nil {
				bad.Add(1)
			}
		}
	}()
	return func() (int64, int64) {
		<-done
		return reads.Load(), bad.Load()
	}
}

func assertAllSessions(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(paneMapFile)
	if err != nil {
		t.Fatalf("read pane map: %v", err)
	}
	var m map[string]paneMapEntry
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("pane map does not parse: %v", err)
	}
	for i := 0; i < concurrentWriters; i++ {
		id := fmt.Sprintf("sess-%02d", i)
		e, ok := m[id]
		if !ok {
			t.Errorf("missing %s (lost update)", id)
			continue
		}
		if e.PaneID != uint32(i) {
			t.Errorf("%s: pane %d, want %d", id, e.PaneID, i)
		}
	}
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(paneMapFile), "pane-map-*.json"))
	if len(matches) != 0 {
		t.Errorf("temp files left behind: %v", matches)
	}
}

func TestPaneMap_ConcurrentGoroutineWriters(t *testing.T) {
	newHookTestEnv(t)
	stop := make(chan struct{})
	wait := watchPaneMap(t, stop)

	var wg sync.WaitGroup
	for i := 0; i < concurrentWriters; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			recordPaneMapping(fmt.Sprintf("sess-%02d", i), uint32(i), "cc-deck-local")
		}(i)
	}
	wg.Wait()
	close(stop)
	if _, bad := wait(); bad != 0 {
		t.Errorf("%d reads saw an undecodable pane map", bad)
	}
	assertAllSessions(t)
}

// The real race is between hook processes, so the second variant runs each
// writer in its own process of this test binary.
func TestPaneMap_ConcurrentProcessWriters(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns subprocesses")
	}
	newHookTestEnv(t)
	self, err := os.Executable()
	if err != nil {
		t.Skip("no test executable")
	}
	stop := make(chan struct{})
	wait := watchPaneMap(t, stop)

	var wg sync.WaitGroup
	errs := make(chan error, concurrentWriters)
	for i := 0; i < concurrentWriters; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cmd := exec.Command(self, "-test.run=^TestPaneMapHelperProcess$", "-test.count=1")
			cmd.Env = append(os.Environ(),
				"CC_DECK_PANEMAP_HELPER=1",
				"CC_DECK_PANEMAP_FILE="+paneMapFile,
				fmt.Sprintf("CC_DECK_PANEMAP_SESSION=sess-%02d", i),
				fmt.Sprintf("CC_DECK_PANEMAP_PANE=%d", i),
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				errs <- fmt.Errorf("writer %d: %v: %s", i, err, out)
			}
		}(i)
	}
	wg.Wait()
	close(stop)
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if _, bad := wait(); bad != 0 {
		t.Errorf("%d reads saw an undecodable pane map", bad)
	}
	assertAllSessions(t)
}

// TestPaneMapHelperProcess is the body of one writer process in
// TestPaneMap_ConcurrentProcessWriters; it does nothing in a normal run.
func TestPaneMapHelperProcess(t *testing.T) {
	if os.Getenv("CC_DECK_PANEMAP_HELPER") != "1" {
		return
	}
	paneMapFile = os.Getenv("CC_DECK_PANEMAP_FILE")
	hookStateDir = filepath.Dir(paneMapFile)
	var pane uint32
	if _, err := fmt.Sscan(os.Getenv("CC_DECK_PANEMAP_PANE"), &pane); err != nil {
		t.Fatalf("bad pane: %v", err)
	}
	recordPaneMapping(os.Getenv("CC_DECK_PANEMAP_SESSION"), pane, "cc-deck-local")
}

// A lock held elsewhere past the deadline skips the write instead of
// blocking the agent.
func TestWithPaneMapLock_TimesOut(t *testing.T) {
	newHookTestEnv(t)
	orig := paneMapLockTimeout
	paneMapLockTimeout = 50 * time.Millisecond
	t.Cleanup(func() { paneMapLockTimeout = orig })

	release := make(chan struct{})
	held := make(chan struct{})
	go withPaneMapLock(func() {
		close(held)
		<-release
	})
	<-held
	ran := withPaneMapLock(func() {})
	close(release)
	if ran {
		t.Fatal("expected the second lock attempt to time out")
	}
	if !withPaneMapLock(func() {}) {
		// The holder may still be unlocking; retry once with the default.
		paneMapLockTimeout = orig
		if !withPaneMapLock(func() {}) {
			t.Fatal("lock not acquirable after release")
		}
	}
}
