package ssh

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/crypto/ssh"
)

type PTYSession struct {
	session  *ssh.Session
	stdin    chan []byte
	stdout   chan []byte
	done     chan struct{}
	closeOnce sync.Once
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
		return nil, fmt.Errorf("new session: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	term := "xterm-256color"
	if err := session.RequestPty(term, int(rows), int(cols), modes); err != nil {
		session.Close()
		return nil, fmt.Errorf("request pty: %w", err)
	}

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return nil, err
	}
	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return nil, err
	}
	stderrPipe, err := session.StderrPipe()
	if err != nil {
		session.Close()
		return nil, err
	}

	if customCmd != "" {
		// Wrap command with UTF-8 locale for container exec
		wrapped := fmt.Sprintf(
			`export LANG=C.UTF-8 LC_ALL=C.UTF-8 TERM=xterm-256color; %s`,
			customCmd,
		)
		if err := session.Start(wrapped); err != nil {
			session.Close()
			return nil, fmt.Errorf("start command: %w", err)
		}
	} else {
		// Set UTF-8 environment variables before starting the shell
		session.Setenv("LANG", "C.UTF-8")
		session.Setenv("LC_ALL", "C.UTF-8")
		session.Setenv("LANGUAGE", "en_US.UTF-8")
		session.Setenv("TERM", "xterm-256color")
		if err := session.Shell(); err != nil {
			session.Close()
			return nil, fmt.Errorf("start shell: %w", err)
		}
	}

	ps := &PTYSession{
		session: session,
		stdin:   make(chan []byte, 64),
		stdout:  make(chan []byte, 256),
		done:    make(chan struct{}),
	}

	go func() {
		defer close(ps.done)
		buf := make([]byte, 8192)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				select {
				case ps.stdout <- data:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := stderrPipe.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				select {
				case ps.stdout <- data:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

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
			case <-ctx.Done():
				return
			}
		}
	}()

	return ps, nil
}

func (ps *PTYSession) Write(data []byte) error {
	ps.stdin <- data
	return nil
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
		ps.session.Close()
	})
	return nil
}

func (ps *PTYSession) Done() <-chan struct{} {
	return ps.done
}
