package orderbook

// Sell (asks) or Buy (bids)
const (
	Sell SideType = iota
	Buy
)

// String implements fmt.Stringer interface
func (s SideType) String() string {
	switch s {
	case Buy:
		return "buy"
	default:
		return "sell"
	}
}
