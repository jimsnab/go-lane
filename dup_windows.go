//go:build windows

package lane

import (
	"os"
	"syscall"
)

func dupFile(f *os.File) (*os.File, error) {
	p, err := syscall.GetCurrentProcess()
	if err != nil {
		return nil, err
	}
	var h syscall.Handle
	err = syscall.DuplicateHandle(p, syscall.Handle(f.Fd()), p, &h, 0, true, syscall.DUPLICATE_SAME_ACCESS)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), f.Name()), nil
}
