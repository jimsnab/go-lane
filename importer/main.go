package main

import (
	"context"

	"github.com/jimsnab/go-lane"
)

func main() {
	// Program to verify using the lane package works from a different package
	l := lane.NewLogLane(context.Background())
	l.Info("context package wants root level to be context.Background()")

	l2 := lane.NewLogLane(context.TODO())
	l2.Info("TODO can be used if desired")

	l3 := lane.NewLogLane(nil)
	l3.Info("nil is the same as context.Background()")

	dl := l3.DeriveReplaceContext(nil)
	dl.Info("nil is supported in replace context APIs also")
}
