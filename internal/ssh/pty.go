package ssh

import (
	"context"
	"fmt"
	"io"
	"sync"

	"golang.org/x/crypto/ssh"
)

type PTYSession struct {
	session   *ssh.Session
	stdin     chan []byte
	stdout    chan []byte
	done      chan struct{}
	closed    chan struct{}
	closeOnce sync.Once
	nodeID    string
	client    *ssh.Client
	stdinPipe io.WriteCloser
	manager   *Manager
}

func (m *Manager) NewPTYSession(ctx context.Context, nodeID string, cols, rows uint16) (*PTYSession, error) {
	return m.newPTYWithCommand(ctx, nodeID, cols, rows, "")
}

func (m *Manager) NewContainerPTYSession(ctx context.Context, nodeID, containerID string, cols, rows uint16) (*PTYSession, error) {
	cmd := fmt.Sprintf(
		`DPATH=""; for p in /usr/bin/docker /usr/local/bin/docker /snap/bin/docker; do [ -x "$p" ] && DPATH="$p" && break; done; [ -z "$DPATH" ] && DPATH=docker; $DPATH exec -it %s bash 2>/dev/null || $DPATH exec -it %s sh`,
		containerID, containerID,
	)
	return m.newPTYWithCommand(ctx, nodeID, cols, rows, cmd)
}

func (m *Manager) newPTYWithCommand(ctx context.Context, nodeID string, cols, rows uint16, customCmd string) (*PTYSession, error) {
	client, err := m.getClient(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close() // Don't pool stale client
		return nil, err
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	term := "xterm-256color"
	if err := session.RequestPty(term, int(rows), int(cols), modes); err != nil {
		session.Close()
		m.releaseClient(nodeID, client)
		return nil, fmt.Errorf("request pty: %w", err)
	}

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		session.Close()
		m.releaseClient(nodeID, client)
		return nil, err
	}
	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		m.releaseClient(nodeID, client)
		return nil, err
	}
	stderrPipe, err := session.StderrPipe()
	if err != nil {
		session.Close()
		m.releaseClient(nodeID, client)
		return nil, err
	}

	if customCmd != "" {
		wrapped := fmt.Sprintf(`export LANG=C.UTF-8 LC_ALL=C.UTF-8 TERM=xterm-256color; %s`, customCmd)
		if err := session.Start(wrapped); err != nil {
			session.Close()
			m.releaseClient(nodeID, client)
			return nil, fmt.Errorf("start command: %w", err)
		}
	} else {
		session.Setenv("TERM", "xterm-256color")
		if err := session.Shell(); err != nil {
			session.Close()
			m.releaseClient(nodeID, client)
			return nil, fmt.Errorf("start shell: %w", err)
		}
	}

	ps := &PTYSession{
		session:   session,
		stdin:     make(chan []byte, 64),
		stdout:    make(chan []byte, 256),
		done:      make(chan struct{}),
		closed:    make(chan struct{}),
		nodeID:    nodeID,
		client:    client,
		stdinPipe: stdinPipe,
		manager:   m,
	}

	// stdout reader
	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				select {
				case ps.stdout <- data:
				case <-ps.closed:
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// stderr reader
	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := stderrPipe.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				select {
				case ps.stdout <- data:
				case <-ps.closed:
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// stdin writer
	go func() {
		for {
			select {
			case data, ok := <-ps.stdin:
				if !ok {
					return
				}
				_, err := stdinPipe.Write(data)
				if err != nil {
					return
				}
			case <-ps.closed:
				return
			}
		}
	}()

	// Session done detector — closes ps.done when the remote session ends
	go func() {
		_ = session.Wait()
		select {
		case <-ps.done:
		default:
			close(ps.done)
		}
	}()

	return ps, nil
}

func (ps *PTYSession) Write(data []byte) error {
	select {
	case ps.stdin <- data:
		return nil
	case <-ps.done:
		return fmt.Errorf("session closed")
	}
}

func (ps *PTYSession) Read() ([]byte, error) {
	select {
	case data := <-ps.stdout:
		return data, nil
	case <-ps.done:
		return nil, fmt.Errorf("session closed")
	}
}

func (ps *PTYSession) Resize(cols, rows uint16) error {
	return ps.session.WindowChange(int(rows), int(cols))
}

func (ps *PTYSession) Close() error {
	ps.closeOnce.Do(func() {
		// Signal all goroutines to stop
		select {
		case <-ps.closed:
		default:
			close(ps.closed)
		}
		// Send EOF to remote shell
		ps.stdinPipe.Write([]byte("\x04"))
		ps.stdinPipe.Close()
		// Close the SSH session
		ps.session.Close()
		// Signal done if not already
		select {
		case <-ps.done:
		default:
			close(ps.done)
		}
		// CRITICAL: Release the SSH client back to the connection pool
		if ps.manager != nil && ps.client != nil && ps.nodeID != "" {
			ps.manager.releaseClient(ps.nodeID, ps.client)
		}
	})
	return nil
}

func (ps *PTYSession) Done() <-chan struct{} {
	return ps.done
}
