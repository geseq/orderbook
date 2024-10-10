package pool

// PoolInterface defines the common interface for item pools
type PoolInterface[T any] interface {
	Get() *T     // Retrieves an item from the pool
	Put(item *T) // Returns an item to the pool
}

// Ensure that ItemPoolV2 implements PoolInterface
var _ PoolInterface[any] = (*ItemPoolV2[any])(nil)
