package lane

type (
	stubMetadata struct {
		l Lane
	}
)

func newStubMetadata(l Lane) LaneMetadata {
	return &stubMetadata{
		l: l,
	}
}

func (sm *stubMetadata) SetMetadata(key, value string) {
	tees := sm.l.Tees()
	for _, tee := range tees {
		tee.Metadata().SetMetadata(key, value)
	}
}
