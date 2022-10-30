package orderbook

type Option func(*OrderBook)

type options []Option

func (l options) applyTo(p *OrderBook) {
	for _, opt := range l {
		opt(p)
	}
}

// defaultOpts provides list of options
var defaultOpts = []Option{
	WithMatching(true),
	WithOrderPoolSize(1e6),
	WithNodeTreePoolSize(1e6),
	WithOrderQueuePoolSize(1e5),
}

// WithMatching enables or disables matching
func WithMatching(b bool) Option {
	return func(o *OrderBook) { o.matching = b }
}

// WithOrderPoolSize sets the size of the order pool
func WithOrderPoolSize(size uint64) Option {
	return func(o *OrderBook) { o.orderPoolSize = size }
}

// WithNodeTreePoolSize sets the size of the node tree pool
func WithNodeTreePoolSize(size uint64) Option {
	return func(o *OrderBook) { o.nodeTreePoolSize = size }
}

// WithOrderQueuePoolSize sets the size of the order queue pool
func WithOrderQueuePoolSize(size uint64) Option {
	return func(o *OrderBook) { o.orderQueuePoolSize = size }
}
