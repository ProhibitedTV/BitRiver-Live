package storage

// NewJSONRepository opens the JSON-backed datastore and returns it as a
// Repository.
func NewJSONRepository(path string, opts ...Option) (Repository, error) {
	return NewStorage(path, opts...)
}
