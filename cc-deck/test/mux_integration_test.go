package test

import (
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cc-deck/cc-deck/internal/mux"
)

func integrationSocket(t *testing.T) string {
	t.Helper()
	dir := fmt.Sprintf("/tmp/mux-int-%d", time.Now().UnixNano()%100000)
	os.MkdirAll(dir, 0o700)
	sock := dir + "/t.sock"
	t.Cleanup(func() { os.RemoveAll(dir) })
	return sock
}

func waitForSocket(t *testing.T, path string) {
	t.Helper()
	for i := 0; i < 100; i++ {
		conn, err := net.Dial("unix", path)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("socket not available after 1s")
}

func TestIntegration_ConcurrentMultiSession(t *testing.T) {
	sock := integrationSocket(t)

	var mu sync.Mutex
	var flushed []mux.Message

	broker := mux.NewBroker(sock, 100*time.Millisecond, 5*time.Second, 1000, true, nil)
	broker.FlushFn = func(msg mux.Message) error {
		mu.Lock()
		flushed = append(flushed, msg)
		mu.Unlock()
		return nil
	}

	go broker.Run()
	t.Cleanup(func() {
		broker.Stop()
		time.Sleep(200 * time.Millisecond)
	})
	waitForSocket(t, sock)

	sessions := []string{"session-a", "session-b", "session-c"}
	var wg sync.WaitGroup

	for _, sess := range sessions {
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(s string, n int) {
				defer wg.Done()
				// Each goroutine sends a unique message per (session, index)
				msg := mux.Message{
					SessionName: s,
					PipeName:    "cc-deck:hook",
					Args:        fmt.Sprintf(`{"session":"%s","index":%d}`, s, n),
				}
				_ = mux.Send(sock, msg)
			}(sess, i)
		}
	}

	// Also send duplicates that should be deduped
	for _, sess := range sessions {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			msg := mux.Message{
				SessionName: s,
				PipeName:    "cc-deck:hook",
				Args:        fmt.Sprintf(`{"session":"%s","index":0}`, s),
			}
			_ = mux.Send(sock, msg)
		}(sess)
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	count := len(flushed)
	mu.Unlock()

	// 3 sessions * 5 unique messages = 15 unique, 3 duplicates should be collapsed
	assert.Equal(t, 15, count, "expected 15 unique messages after dedup (3 duplicates collapsed)")

	// Verify all sessions represented
	sessionCounts := map[string]int{}
	mu.Lock()
	for _, msg := range flushed {
		sessionCounts[msg.SessionName]++
	}
	mu.Unlock()
	for _, sess := range sessions {
		assert.Equal(t, 5, sessionCounts[sess], "session %s should have 5 messages", sess)
	}
}

func TestIntegration_IdleTimeoutShutdown(t *testing.T) {
	sock := integrationSocket(t)

	broker := mux.NewBroker(sock, 50*time.Millisecond, 500*time.Millisecond, 1000, true, nil)
	broker.FlushFn = func(msg mux.Message) error { return nil }

	done := make(chan struct{})
	go func() {
		broker.Run()
		close(done)
	}()

	waitForSocket(t, sock)

	// Send one message to prove broker is active
	msg := mux.Message{SessionName: "s", PipeName: "p", Args: "a"}
	require.NoError(t, mux.Send(sock, msg))

	select {
	case <-done:
		// Broker exited due to idle timeout
	case <-time.After(3 * time.Second):
		broker.Stop()
		t.Fatal("broker did not self-terminate after idle timeout")
	}

	// Verify socket file is cleaned up
	_, err := os.Stat(sock)
	assert.True(t, os.IsNotExist(err), "socket file should be removed after shutdown")
}

func TestIntegration_StaleSocketRecovery(t *testing.T) {
	sock := integrationSocket(t)

	// Create a stale socket file
	os.WriteFile(sock, []byte("stale"), 0o600)

	// SendOrStart should detect and clean the stale socket
	msg := mux.Message{SessionName: "s1", PipeName: "p", Args: "a"}
	err := mux.SendOrStart(sock, msg)

	// Will fail because no real broker binary, but should have cleaned stale socket
	assert.Error(t, err)
	_, statErr := os.Stat(sock)
	assert.True(t, os.IsNotExist(statErr), "stale socket should have been cleaned up")
}

func TestIntegration_FlushErrorDoesNotCrash(t *testing.T) {
	sock := integrationSocket(t)

	var mu sync.Mutex
	var errors int

	broker := mux.NewBroker(sock, 50*time.Millisecond, 5*time.Second, 1000, true, nil)
	broker.FlushFn = func(msg mux.Message) error {
		mu.Lock()
		errors++
		mu.Unlock()
		return fmt.Errorf("simulated zellij pipe failure")
	}

	go broker.Run()
	t.Cleanup(func() {
		broker.Stop()
		time.Sleep(200 * time.Millisecond)
	})
	waitForSocket(t, sock)

	// Send messages that will fail during flush
	for i := 0; i < 3; i++ {
		msg := mux.Message{SessionName: fmt.Sprintf("s%d", i), PipeName: "p", Args: "a"}
		require.NoError(t, mux.Send(sock, msg))
	}

	time.Sleep(300 * time.Millisecond)

	// Broker should still be accepting connections after flush errors
	msg := mux.Message{SessionName: "post-error", PipeName: "p", Args: "a"}
	err := mux.Send(sock, msg)
	assert.NoError(t, err, "broker should still accept connections after flush errors")
}

func TestIntegration_NoDedupPreservesOrder(t *testing.T) {
	sock := integrationSocket(t)

	var mu sync.Mutex
	var flushed []mux.Message

	broker := mux.NewBroker(sock, 100*time.Millisecond, 5*time.Second, 1000, false, nil)
	broker.FlushFn = func(msg mux.Message) error {
		mu.Lock()
		flushed = append(flushed, msg)
		mu.Unlock()
		return nil
	}

	go broker.Run()
	t.Cleanup(func() {
		broker.Stop()
		time.Sleep(200 * time.Millisecond)
	})
	waitForSocket(t, sock)

	// Send identical messages
	msg := mux.Message{SessionName: "s1", PipeName: "p", Args: "same"}
	for i := 0; i < 5; i++ {
		require.NoError(t, mux.Send(sock, msg))
		time.Sleep(5 * time.Millisecond)
	}

	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	count := len(flushed)
	mu.Unlock()
	assert.Equal(t, 5, count, "dedup=false should preserve all 5 identical messages")
}
