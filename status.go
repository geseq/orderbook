package orderbook

// OrderStatus of the order
type OrderStatus byte

// String implements Stringer interface
func (o OrderStatus) String() string {
	switch o {
	case Rejected:
		return "Rejected"
	case Canceled:
		return "Canceled"
	case FilledPartial:
		return "FilledPartial"
	case FilledComplete:
		return "FilledComplete"
	case Accepted:
		return "Accepted"
	}

	return ""
}

const (
	Rejected OrderStatus = iota
	Canceled
	FilledPartial
	FilledComplete
	Accepted
)
