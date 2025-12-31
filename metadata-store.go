package lane

import "sync"

type (
	// LaneMetadata is the interface for managing metadata associated with a lane.
	// Metadata consists of key-value pairs that provide additional context.
	// These pairs are often used by structured logging lanes (like OpenSearch)
	// to include extra fields in the log records.
	LaneMetadata interface {
		// SetOwner links the metadata to the owning lane.
		SetOwner(l Lane)
		// SetMetadata sets a key-value pair in the metadata.
		// This value will be propagated to any attached tee lanes.
		SetMetadata(key, value string)
		// GetMetadata retrieves the value for a given key.
		GetMetadata(key string) string
		// MetadataMap returns a copy of all metadata.
		MetadataMap() map[string]string
	}

	// Common implementation of metadata
	MetadataStore struct {
		mu       sync.Mutex
		l        Lane
		metadata map[string]string
	}
)

// SetOwner is used in lane object creation to link the metadata interface to the owning lane
func (ms *MetadataStore) SetOwner(l Lane) {
	ms.l = l
}

// SetMetadata sets the lane's metadata value, overwriting a prior value if one was set
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

// GetMetadata retrieves the lane's metadata value if it is set
func (ms *MetadataStore) GetMetadata(key string) string {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	return ms.metadata[key]
}

// MetadataMap returns a copy of the metadata map
func (ms *MetadataStore) MetadataMap() map[string]string {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	md := make(map[string]string, len(ms.metadata))
	for k, v := range ms.metadata {
		md[k] = v
	}

	return md
}
