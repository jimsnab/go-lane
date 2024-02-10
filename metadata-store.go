package lane

import "sync"

type (
	// Common implementation of metadata
	MetadataStore struct {
		mu       sync.Mutex
		l        Lane
		metadata map[string]string
	}
)

// Used in lane object creation to link the metadata interface to the owning lane
func (ms *MetadataStore) SetOwner(l Lane) {
	ms.l = l
}

// Sets the lane's metadata value, overwriting a prior value if one was set
func (ms *MetadataStore) SetMetadata(key, value string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.metadata == nil {
		ms.metadata = map[string]string{}
	}
	ms.metadata[key] = value

	tees := ms.l.Tees()
	for _, tee := range tees {
		tee.SetMetadata(key, value)
	}
}

// Retrieves the lane's metadata value if it is set
func (ms *MetadataStore) GetMetadata(key string) string {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	return ms.metadata[key]
}

// Returns a copy of the metadata map
func (ms *MetadataStore) Map() map[string]string {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	md := make(map[string]string, len(ms.metadata))
	for k, v := range ms.metadata {
		md[k] = v
	}

	return md
}
