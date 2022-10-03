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
}

// WithMatching enables or disables matching
func WithMatching(b bool) Option {
	return func(o *OrderBook) { o.matching = b }
}
