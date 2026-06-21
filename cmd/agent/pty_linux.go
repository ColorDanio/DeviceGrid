package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// Linux PTY constants
const (
	syscall_TIOCSPTLCK  = 0x40045431
	syscall_TIOCGPTPEER = 0x80045441
	syscall_TIOCSWINSZ  = 0x5414
)

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func openPty() (*os.File, *os.File, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open ptmx: %w", err)
	}

	// Unlock the slave PTY
	var unlock int32 = 0
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), uintptr(syscall_TIOCSPTLCK), uintptr(unsafe.Pointer(&unlock)))
	if errno != 0 {
		master.Close()
		return nil, nil, fmt.Errorf("unlock pty: %v", errno)
	}

	// Get the slave device name
	var ptsn int32
	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), uintptr(0x80045430), uintptr(unsafe.Pointer(&ptsn)))
	if errno != 0 {
		master.Close()
		return nil, nil, fmt.Errorf("get pts number: %v", errno)
	}

	slavePath := fmt.Sprintf("/dev/pts/%d", ptsn)
	slave, err := os.OpenFile(slavePath, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, nil, fmt.Errorf("open slave %s: %w", slavePath, err)
	}

	return master, slave, nil
}

func setPtyWinsize(fd uintptr, cols, rows uint16) {
	ws := &winsize{Row: rows, Col: cols}
	syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall_TIOCSWINSZ), uintptr(unsafe.Pointer(ws)))
}
