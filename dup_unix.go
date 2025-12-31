//go:build !windows

package lane

import (
	"os"
	"syscall"
)

func dupFile(f *os.File) (*os.File, error) {
	newFd, err := syscall.Dup(int(f.Fd()))
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(newFd), f.Name()), nil
}
