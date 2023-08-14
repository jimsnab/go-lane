package lane

import (
	"context"
	"log"
	"os"
	"sync/atomic"
	"syscall"
)

type (
	diskLane struct {
		logLane
		f *os.File
	}
)

func NewDiskLane(ctx context.Context, logFile string) (l Lane, err error) {
	ll := newLogLane(ctx)

	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return
	}

	dl := diskLane{logLane: *ll, f: f}
	dl.logLane.writer = log.New(f, "", 0)
	l = &dl
	return
}

func (dl *diskLane) Derive() Lane {
	ll := newLogLane(context.WithValue(dl.Context, parent_lane_id, dl.LaneId()))
	ll.SetLogLevel(LaneLogLevel(atomic.LoadInt32(&ll.level)))

	newFd, err := syscall.Dup(int(dl.f.Fd()))
	if err != nil {
		panic(err)
	}
	f2 := os.NewFile(uintptr(newFd), dl.f.Name())

	dl2 := diskLane{logLane: *ll, f: f2}
	dl2.logLane.writer = log.New(f2, "", 0)
	return &dl2
}

func (dl *diskLane) Close() {
	if dl.f != nil {
		dl.f.Close()
	}
	dl.f = nil
}
