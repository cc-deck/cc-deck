package mux

import (
	"bufio"
	"context"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// FlushFunc is the function called to deliver a message during flush.
// Replaceable for testing (default: invoke zellij pipe).
type FlushFunc func(msg Message) error

// Broker is the pipe mux daemon that accepts messages on a Unix socket,
// deduplicates them, and flushes to zellij pipe at regular intervals.
type Broker struct {
	SocketPath    string
	FlushInterval time.Duration
	IdleTimeout   time.Duration
	QueueSize     int
	Dedup         bool
	Logger        Logger
	FlushFn       FlushFunc

	listener  *net.UnixListener
	mu        sync.Mutex
	dedupMap  map[string]Message
	dedupKeys []string
	queue     []Message
	stopCh    chan struct{}
	connWg    sync.WaitGroup
	idleMu    sync.Mutex
	lastMsg   time.Time
}

// NewBroker creates a Broker with the given configuration.
func NewBroker(socketPath string, flushInterval, idleTimeout time.Duration, queueSize int, dedup bool, logger Logger) *Broker {
	if logger == nil {
		logger = NoopLogger{}
	}
	return &Broker{
		SocketPath:    socketPath,
		FlushInterval: flushInterval,
		IdleTimeout:   idleTimeout,
		QueueSize:     queueSize,
		Dedup:         dedup,
		Logger:        logger,
		FlushFn:       defaultFlushFn,
		dedupMap:      make(map[string]Message),
		stopCh:        make(chan struct{}),
		lastMsg:       time.Now(),
	}
}

func defaultFlushFn(msg Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "zellij", "pipe",
		"--session", msg.SessionName,
		"--name", msg.PipeName,
		"--", msg.Args)
	return cmd.Run()
}

// Run starts the broker: binds the socket, runs the accept loop, flush timer,
// and idle timer. Blocks until shutdown.
func (b *Broker) Run() error {
	addr := &net.UnixAddr{Name: b.SocketPath, Net: "unix"}
	ln, err := net.ListenUnix("unix", addr)
	if err != nil {
		return err
	}
	b.listener = ln

	b.Logger.Lifecycle("broker started")

	var wg sync.WaitGroup

	// Accept loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.acceptLoop()
	}()

	// Flush loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.flushLoop()
	}()

	// Idle check loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.idleLoop()
	}()

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		select {
		case sig := <-sigCh:
			b.Logger.Lifecycle("received signal: " + sig.String())
			b.Stop()
		case <-b.stopCh:
		}
		signal.Stop(sigCh)
	}()

	<-b.stopCh

	b.listener.Close()
	wg.Wait()
	b.connWg.Wait()

	b.finalFlush()
	os.Remove(b.SocketPath)
	b.Logger.Lifecycle("broker stopped")
	return nil
}

// Stop signals the broker to shut down.
func (b *Broker) Stop() {
	select {
	case <-b.stopCh:
	default:
		close(b.stopCh)
	}
}

func (b *Broker) acceptLoop() {
	for {
		b.listener.SetDeadline(time.Now().Add(500 * time.Millisecond))
		conn, err := b.listener.AcceptUnix()
		if err != nil {
			select {
			case <-b.stopCh:
				return
			default:
				continue
			}
		}
		b.connWg.Add(1)
		go func() {
			defer b.connWg.Done()
			b.handleConn(conn)
		}()
	}
}

const maxMessageSize = 256 * 1024

func (b *Broker) handleConn(conn *net.UnixConn) {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxMessageSize)
	if scanner.Scan() {
		msg, err := UnmarshalMessage(scanner.Bytes())
		if err != nil {
			return
		}
		b.enqueue(msg)
	}
	if err := scanner.Err(); err != nil {
		b.Logger.FlushError(Message{}, err)
	}
}

func (b *Broker) enqueue(msg Message) {
	b.idleMu.Lock()
	b.lastMsg = time.Now()
	b.idleMu.Unlock()

	b.Logger.Incoming(msg)

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.Dedup {
		key := msg.DedupKey()
		if _, exists := b.dedupMap[key]; exists {
			b.Logger.DedupHit(key, msg)
		} else {
			b.dedupKeys = append(b.dedupKeys, key)
		}
		b.dedupMap[key] = msg
		if len(b.dedupMap) > b.QueueSize {
			b.dropOldestDedup()
		}
	} else {
		b.queue = append(b.queue, msg)
		if len(b.queue) > b.QueueSize {
			dropped := b.queue[0]
			b.queue = b.queue[1:]
			b.Logger.Drop(dropped)
		}
	}
}

func (b *Broker) dropOldestDedup() {
	if len(b.dedupKeys) == 0 {
		return
	}
	key := b.dedupKeys[0]
	b.dedupKeys = b.dedupKeys[1:]
	if v, ok := b.dedupMap[key]; ok {
		b.Logger.Drop(v)
		delete(b.dedupMap, key)
	}
}

func (b *Broker) flushLoop() {
	ticker := time.NewTicker(b.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.flush()
		}
	}
}

func (b *Broker) flush() {
	b.mu.Lock()
	var msgs []Message
	if b.Dedup {
		for _, key := range b.dedupKeys {
			if m, ok := b.dedupMap[key]; ok {
				msgs = append(msgs, m)
			}
		}
		b.dedupMap = make(map[string]Message)
		b.dedupKeys = nil
	} else {
		msgs = b.queue
		b.queue = nil
	}
	b.mu.Unlock()

	if len(msgs) == 0 {
		return
	}

	start := time.Now()
	for _, msg := range msgs {
		if err := b.FlushFn(msg); err != nil {
			b.Logger.FlushError(msg, err)
		}
	}
	b.Logger.Flush(len(msgs), time.Since(start))
}

func (b *Broker) finalFlush() {
	b.flush()
}

func (b *Broker) idleLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.idleMu.Lock()
			idle := time.Since(b.lastMsg)
			b.idleMu.Unlock()
			if idle >= b.IdleTimeout {
				b.Logger.Lifecycle("idle timeout reached")
				b.Stop()
				return
			}
		}
	}
}
