package lane

import (
	"log"
	"os"
)

type (
	diskLane struct {
		LogLane
		f *os.File
	}
)

func NewDiskLane(ctx OptionalContext, logFile string) (l Lane, err error) {

	createFn := func(parentLane Lane) (newLane Lane, ll LogLane, writer *log.Logger, err error) {
		newLane, ll, writer, err = createDiskLane(logFile, parentLane)
		return
	}

	return NewEmbeddedLogLane(createFn, ctx)
}

func createDiskLane(logFile string, parentLane Lane) (newLane Lane, ll LogLane, writer *log.Logger, err error) {
	dl := diskLane{}
	pdl, _ := parentLane.(*diskLane)

	if pdl == nil {
		var f *os.File
		f, err = os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return
		}

		dl.f = f
	} else {
		var f2 *os.File
		f2, err = dupFile(pdl.f)
		if err != nil {
			return
		}
		dl.f = f2
	}
	writer = log.New(dl.f, "", 0)

	ll = AllocEmbeddedLogLane()
	dl.LogLane = ll
	newLane = &dl
	return
}

func (dl *diskLane) Close() {
	if dl.f != nil {
		dl.f.Close()
	}
	dl.f = nil
}
