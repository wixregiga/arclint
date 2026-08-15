package store

import "context"

// Store is the storage API every part of the application depends on.
type Store interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, value []byte) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
	Watch(ctx context.Context, prefix string) (<-chan string, error)
	Close() error
}

// MemStore is the in-memory implementation.
type MemStore struct {
	data map[string][]byte
}

func NewMemStore() *MemStore {
	return &MemStore{data: map[string][]byte{}}
}

func (m *MemStore) Get(ctx context.Context, key string) ([]byte, error) {
	return m.data[key], nil
}

func (m *MemStore) Put(ctx context.Context, key string, value []byte) error {
	m.data[key] = value
	return nil
}

func (m *MemStore) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *MemStore) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	for k := range m.data {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (m *MemStore) Watch(ctx context.Context, prefix string) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}

func (m *MemStore) Close() error {
	m.data = nil
	return nil
}
