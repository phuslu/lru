//go:build linux && amd64
// +build linux,amd64

package lru

import (
	"fmt"
	"os"
	"syscall"
)

type bytesfile struct {
	location string
	file     *os.File
	size     int
}

func (m *bytesfile) close() error {
	return m.file.Close()
}

func (m *bytesfile) open() error {
	var openingFlag = os.O_RDWR
	if _, err := os.Stat(m.location); os.IsNotExist(err) {
		openingFlag = openingFlag | os.O_CREATE
	}
	var err error
	if m.file, err = os.OpenFile(m.location, openingFlag, 0644); err != nil {
		return fmt.Errorf("failed to create file %v: %w", m.location, err)
	}
	return m.allocate()
}

func (m *bytesfile) assign(offset int64, target *[]byte) error {
	buffer, err := syscall.Mmap(int(m.file.Fd()), offset, m.size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return fmt.Errorf("failed to map memory %v: %w", m.location, err)
	}
	*target = buffer
	return nil
}

func (m *bytesfile) allocate() error {
	info, err := os.Stat(m.location)
	if err != nil {
		return err
	}
	if info.Size() < int64(m.size) {
		_, err := m.file.Seek(int64(m.size-1), 0)
		if err != nil {
			return fmt.Errorf("Failed to seek file %v", err)
		}
		_, err = m.file.Write([]byte{0})
		if err != nil {
			return fmt.Errorf("failed to resize %v: %w", m.location, err)
		}
	}
	return nil
}
