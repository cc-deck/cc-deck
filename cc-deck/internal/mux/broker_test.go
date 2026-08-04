package mux

import (
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tempSocket(t *testing.T) string {
	t.Helper()
	dir := fmt.Sprintf("/tmp/mux-test-%d", time.Now().UnixNano()%100000)
	os.MkdirAll(dir, 0o700)
	sock := dir + "/t.sock"
	t.Cleanup(func() { os.RemoveAll(dir) })
	return sock
}

func startTestBroker(t *testing.T, opts ...func(*Broker)) (*Broker, []Message) {
	t.Helper()
	var mu sync.Mutex
	var flushed []Message

	b := NewBroker(tempSocket(t), 50*time.Millisecond, 10*time.Second, 1000, true, nil)
	b.FlushFn = func(msg Message) error {
		mu.Lock()
		flushed = append(flushed, msg)
		mu.Unlock()
		return nil
	}
	for _, opt := range opts {
		opt(b)
	}

	go func() {
		if err := b.Run(); err != nil {
			t.Logf("broker.Run error: %v", err)
		}
	}()

	t.Cleanup(func() {
		b.Stop()
		time.Sleep(100 * time.Millisecond)
	})

	// Wait for socket to be available
	for i := 0; i < 50; i++ {
		conn, err := net.Dial("unix", b.SocketPath)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	return b, flushed
}

func getFlushed(mu *sync.Mutex, flushed *[]Message) []Message {
	mu.Lock()
	defer mu.Unlock()
	result := make([]Message, len(*flushed))
	copy(result, *flushed)
	return result
}

func TestBroker_AcceptsConnections(t *testing.T) {
	b, _ := startTestBroker(t)

	conn, err := net.Dial("unix", b.SocketPath)
	require.NoError(t, err)
	conn.Close()
}

func TestBroker_DedupCollapsesIdentical(t *testing.T) {
	var mu sync.Mutex
	var flushed []Message

	b, _ := startTestBroker(t, func(b *Broker) {
		b.FlushFn = func(msg Message) error {
			mu.Lock()
			flushed = append(flushed, msg)
			mu.Unlock()
			return nil
		}
	})

	msg := Message{SessionName: "s1", PipeName: "cc-deck:hook", Args: `{"e":"test"}`}
	for i := 0; i < 5; i++ {
		require.NoError(t, Send(b.SocketPath, msg))
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count := len(flushed)
	mu.Unlock()
	assert.Equal(t, 1, count, "5 identical messages should deduplicate to 1")
}

func TestBroker_FlushDeliversUniqueMessages(t *testing.T) {
	var mu sync.Mutex
	var flushed []Message

	b, _ := startTestBroker(t, func(b *Broker) {
		b.FlushFn = func(msg Message) error {
			mu.Lock()
			flushed = append(flushed, msg)
			mu.Unlock()
			return nil
		}
	})

	for i := 0; i < 3; i++ {
		msg := Message{SessionName: fmt.Sprintf("s%d", i), PipeName: "cc-deck:hook", Args: `{"x":1}`}
		require.NoError(t, Send(b.SocketPath, msg))
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count := len(flushed)
	mu.Unlock()
	assert.Equal(t, 3, count, "3 unique messages should all be flushed")
}

func TestBroker_QueueSizeDropsOldest(t *testing.T) {
	var mu sync.Mutex
	var flushed []Message

	b, _ := startTestBroker(t, func(b *Broker) {
		b.QueueSize = 3
		b.FlushFn = func(msg Message) error {
			mu.Lock()
			flushed = append(flushed, msg)
			mu.Unlock()
			return nil
		}
	})

	for i := 0; i < 5; i++ {
		msg := Message{SessionName: fmt.Sprintf("s%d", i), PipeName: "p", Args: fmt.Sprintf("%d", i)}
		require.NoError(t, Send(b.SocketPath, msg))
		time.Sleep(5 * time.Millisecond)
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count := len(flushed)
	mu.Unlock()
	assert.Equal(t, 3, count, "should only flush 3 messages (queue_size limit)")
}

func TestBroker_NoDedupPreservesAll(t *testing.T) {
	var mu sync.Mutex
	var flushed []Message

	b, _ := startTestBroker(t, func(b *Broker) {
		b.Dedup = false
		b.FlushFn = func(msg Message) error {
			mu.Lock()
			flushed = append(flushed, msg)
			mu.Unlock()
			return nil
		}
	})

	msg := Message{SessionName: "s1", PipeName: "p", Args: "same"}
	for i := 0; i < 5; i++ {
		require.NoError(t, Send(b.SocketPath, msg))
		time.Sleep(5 * time.Millisecond)
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count := len(flushed)
	mu.Unlock()
	assert.Equal(t, 5, count, "dedup=false should preserve all messages")
}

func TestBroker_NoDedupQueueSizeDropsOldest(t *testing.T) {
	var mu sync.Mutex
	var flushed []Message

	b, _ := startTestBroker(t, func(b *Broker) {
		b.Dedup = false
		b.QueueSize = 3
		b.FlushFn = func(msg Message) error {
			mu.Lock()
			flushed = append(flushed, msg)
			mu.Unlock()
			return nil
		}
	})

	for i := 0; i < 5; i++ {
		msg := Message{SessionName: "s1", PipeName: "p", Args: fmt.Sprintf("%d", i)}
		require.NoError(t, Send(b.SocketPath, msg))
		time.Sleep(5 * time.Millisecond)
	}

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	result := make([]Message, len(flushed))
	copy(result, flushed)
	mu.Unlock()

	assert.Equal(t, 3, len(result), "dedup=false with queue_size=3 should keep only 3 messages")
	assert.Equal(t, "2", result[0].Args, "oldest messages should be dropped, keeping 2,3,4")
	assert.Equal(t, "3", result[1].Args)
	assert.Equal(t, "4", result[2].Args)
}

func TestBroker_ConcurrentWriteAndFlush(t *testing.T) {
	var mu sync.Mutex
	var flushed []Message

	b, _ := startTestBroker(t, func(b *Broker) {
		b.FlushFn = func(msg Message) error {
			mu.Lock()
			flushed = append(flushed, msg)
			mu.Unlock()
			return nil
		}
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			msg := Message{SessionName: fmt.Sprintf("s%d", n), PipeName: "p", Args: fmt.Sprintf("%d", n)}
			_ = Send(b.SocketPath, msg)
		}(i)
	}
	wg.Wait()

	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	count := len(flushed)
	mu.Unlock()
	assert.Equal(t, 10, count, "all 10 unique concurrent messages should be flushed")
}

func TestBroker_FlushErrorDiscardsMessage(t *testing.T) {
	var mu sync.Mutex
	var errorCount int

	b, _ := startTestBroker(t, func(b *Broker) {
		b.FlushFn = func(msg Message) error {
			mu.Lock()
			errorCount++
			mu.Unlock()
			return fmt.Errorf("zellij pipe failed")
		}
	})

	msg := Message{SessionName: "s1", PipeName: "p", Args: "test"}
	require.NoError(t, Send(b.SocketPath, msg))

	time.Sleep(200 * time.Millisecond)

	// Send another message to verify broker didn't crash from the error
	msg2 := Message{SessionName: "s2", PipeName: "p", Args: "test2"}
	err := Send(b.SocketPath, msg2)
	assert.NoError(t, err, "broker should still accept messages after flush error")
}

func TestBroker_IdleTimeout(t *testing.T) {
	sock := tempSocket(t)
	b := NewBroker(sock, 50*time.Millisecond, 500*time.Millisecond, 1000, true, nil)
	b.FlushFn = func(msg Message) error { return nil }

	done := make(chan struct{})
	go func() {
		b.Run()
		close(done)
	}()

	// Wait for socket
	for i := 0; i < 50; i++ {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	select {
	case <-done:
		// Broker exited due to idle timeout
	case <-time.After(3 * time.Second):
		b.Stop()
		t.Fatal("broker did not self-terminate after idle timeout")
	}
}
