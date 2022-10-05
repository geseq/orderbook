package orderbook

// Market or Limit
const (
	None FlagType = iota
	IoC
	AoN
	FoK
	Snapshot
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
	case Snapshot:
		return "snapshot"
	case None:
		return "none"
	default:
		return ""
	}
}
