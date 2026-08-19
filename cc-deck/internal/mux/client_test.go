package mux

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tempClientSocket(t *testing.T) string {
	t.Helper()
	dir := fmt.Sprintf("/tmp/mux-cli-%d", time.Now().UnixNano()%100000)
	os.MkdirAll(dir, 0o700)
	sock := dir + "/t.sock"
	t.Cleanup(func() { os.RemoveAll(dir) })
	return sock
}

func TestSend_Success(t *testing.T) {
	sock := tempClientSocket(t)
	addr := &net.UnixAddr{Name: sock, Net: "unix"}
	ln, err := net.ListenUnix("unix", addr)
	require.NoError(t, err)
	defer ln.Close()

	received := make(chan Message, 1)
	go func() {
		conn, err := ln.AcceptUnix()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		msg, _ := UnmarshalMessage(buf[:n-1]) // strip newline
		received <- msg
	}()

	msg := Message{SessionName: "s1", PipeName: "cc-deck:hook", Args: `{"test":true}`}
	err = Send(sock, msg)
	require.NoError(t, err)

	select {
	case got := <-received:
		assert.Equal(t, msg.SessionName, got.SessionName)
		assert.Equal(t, msg.PipeName, got.PipeName)
		assert.Equal(t, msg.Args, got.Args)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestSend_ErrorWhenNoBroker(t *testing.T) {
	err := Send("/tmp/nonexistent-mux-sock-12345.sock", Message{SessionName: "s", PipeName: "p", Args: "a"})
	assert.Error(t, err)
}

func TestSendOrStart_SendsToRunningBroker(t *testing.T) {
	sock := tempClientSocket(t)
	addr := &net.UnixAddr{Name: sock, Net: "unix"}
	ln, err := net.ListenUnix("unix", addr)
	require.NoError(t, err)
	defer ln.Close()

	go func() {
		conn, _ := ln.AcceptUnix()
		if conn != nil {
			conn.Close()
		}
	}()

	msg := Message{SessionName: "s1", PipeName: "p", Args: "a"}
	err = SendOrStart(sock, msg)
	assert.NoError(t, err)
}

func TestSendOrStart_FailsWhenNoBrokerAndNoExecutable(t *testing.T) {
	sock := "/tmp/mux-no-broker-test.sock"
	os.Remove(sock)

	msg := Message{SessionName: "s1", PipeName: "p", Args: "a"}
	err := SendOrStart(sock, msg)
	assert.Error(t, err)
}

func TestSendOrStart_CleansStaleSocket(t *testing.T) {
	sock := tempClientSocket(t)
	// Create a stale socket file (not a real listener)
	os.WriteFile(sock, []byte("stale"), 0o600)

	msg := Message{SessionName: "s1", PipeName: "p", Args: "a"}
	err := SendOrStart(sock, msg)
	// Will fail (no real broker to start), but should have cleaned the stale file
	assert.Error(t, err)
	_, statErr := os.Stat(sock)
	assert.True(t, os.IsNotExist(statErr), "stale socket should have been removed")
}

func TestSend_ConnectionClosedAfterSend(t *testing.T) {
	sock := tempClientSocket(t)
	addr := &net.UnixAddr{Name: sock, Net: "unix"}
	ln, err := net.ListenUnix("unix", addr)
	require.NoError(t, err)
	defer ln.Close()

	connClosed := make(chan bool, 1)
	go func() {
		conn, err := ln.AcceptUnix()
		if err != nil {
			return
		}
		buf := make([]byte, 4096)
		for {
			_, err := conn.Read(buf)
			if err != nil {
				connClosed <- true
				conn.Close()
				return
			}
		}
	}()

	msg := Message{SessionName: "s1", PipeName: "p", Args: "a"}
	err = Send(sock, msg)
	require.NoError(t, err)

	select {
	case <-connClosed:
		// Client closed connection after send
	case <-time.After(2 * time.Second):
		t.Fatal("client did not close connection after send")
	}
}
