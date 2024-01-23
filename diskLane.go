package lane

import (
	"context"
	"log"
	"os"
	"syscall"
)

type (
	diskLane struct {
		logLane
		f *os.File
	}
)

func NewDiskLane(ctx context.Context, logFile string) (l Lane, err error) {
	ll := deriveLogLane(nil, ctx, []Lane{}, "")

	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return
	}

	dl := diskLane{
		f: f,
	}
	ll.clone(&dl.logLane)

	dl.logLane.writer = log.New(f, "", 0)
	l = &dl
	return
}

func (dl *diskLane) Derive() Lane {
	ll := deriveLogLane(&dl.logLane, context.WithValue(dl.Context, parent_lane_id, dl.LaneId()), dl.tees, dl.cr)

	newFd, err := syscall.Dup(int(dl.f.Fd()))
	if err != nil {
		panic(err)
	}
	f2 := os.NewFile(uintptr(newFd), dl.f.Name())

	dl2 := diskLane{f: f2}
	ll.clone(&dl2.logLane)
	dl2.logLane.writer = log.New(f2, "", 0)
	return &dl2
}

func (dl *diskLane) Close() {
	if dl.f != nil {
		dl.f.Close()
	}
	dl.f = nil
}
