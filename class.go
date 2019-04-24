package orderbook

// Market or Limit
const (
	Market ClassType = iota
	Limit
)

// String implements fmt.Stringer interface
func (c ClassType) String() string {
	switch c {
	case Limit:
		return "limit"
	default:
		return "market"
	}
}
