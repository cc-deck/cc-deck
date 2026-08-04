package mux

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// Send connects to the broker at socketPath, writes the message as a JSON line,
// and closes the connection. Fire-and-forget: no response is expected.
func Send(socketPath string, msg Message) error {
	conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))

	line, err := msg.MarshalLine()
	if err != nil {
		return err
	}

	_, err = conn.Write(line)
	return err
}

// SendOrStart tries to send a message to the broker. If the broker is not
// running, it starts one and retries. Falls back with an error if all
// attempts fail (caller should use direct zellij pipe).
func SendOrStart(socketPath string, msg Message) error {
	sendErr := Send(socketPath, msg)
	if sendErr == nil {
		return nil
	}

	// Only remove the socket if the file exists AND the failure was a dial
	// error (nobody listening). Write errors on a live connection mean the
	// broker is running; removing its socket would break it.
	if _, statErr := os.Stat(socketPath); statErr == nil {
		if isDialFailure(sendErr) {
			os.Remove(socketPath)
		}
	}

	if err := startBroker(); err != nil {
		return fmt.Errorf("failed to start broker: %w", err)
	}

	for i := 0; i < 3; i++ {
		time.Sleep(10 * time.Millisecond)
		if err := Send(socketPath, msg); err == nil {
			return nil
		}
	}

	return fmt.Errorf("broker not reachable after 3 retries")
}

// isDialFailure returns true if the error originated from the dial phase
// (connection refused, socket not found), as opposed to a write error on
// an established connection.
func isDialFailure(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr.Op == "dial"
	}
	return true
}

func startBroker() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, "mux")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Process.Release()
}
