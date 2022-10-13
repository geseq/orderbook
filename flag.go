package orderbook

// Market or Limit
const (
	None       FlagType = 0
	IoC                 = 1
	AoN                 = 2
	FoK                 = 4
	StopLoss            = 8
	TakeProfit          = 16
	Snapshot            = 32
)

// String implements fmt.Stringer interface
func (f FlagType) String() string {
	switch f {
	case IoC:
		return "IoC"
	case AoN:
		return "AoN"
	case FoK:
		return "FoK"
	case StopLoss:
		return "StopLoss"
	case TakeProfit:
		return "TakeProfit"
	case Snapshot:
		return "snapshot"
	case None:
		return "none"
	default:
		return ""
	}
}
