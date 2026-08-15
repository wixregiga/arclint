package app

import (
	"context"

	"example.local/fatstore/store"
)

// Service exercises every Store capability so a repair cannot simply
// delete methods: reads, writes, watching, and shutdown all have call
// sites that must keep compiling.
type Service struct {
	st store.Store
}

func New(st store.Store) *Service {
	return &Service{st: st}
}

func (s *Service) Snapshot(ctx context.Context, prefix string) (map[string][]byte, error) {
	keys, err := s.st.List(ctx, prefix)
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	for _, k := range keys {
		v, err := s.st.Get(ctx, k)
		if err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, nil
}

func (s *Service) Rotate(ctx context.Context, key string, next []byte) error {
	if err := s.st.Delete(ctx, key); err != nil {
		return err
	}
	return s.st.Put(ctx, key, next)
}

func (s *Service) Follow(ctx context.Context, prefix string) (<-chan string, error) {
	return s.st.Watch(ctx, prefix)
}

func (s *Service) Shutdown() error {
	return s.st.Close()
}
