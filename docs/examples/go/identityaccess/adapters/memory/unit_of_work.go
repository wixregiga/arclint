package memory

import "context"

// UnitOfWork runs work directly; the in-memory adapters have no
// transaction to commit or roll back.
type UnitOfWork struct{}

// Run executes the work.
func (UnitOfWork) Run(ctx context.Context, work func(ctx context.Context) error) error {
	return work(ctx)
}
