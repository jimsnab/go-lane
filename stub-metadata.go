package lane

type (
	stubMetadata struct{}
)

var nullMetadata = &stubMetadata{}

func (sm *stubMetadata) SetMetadata(key, value string) {}
