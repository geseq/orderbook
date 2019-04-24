package orderbook

// Market or Limit
const (
	None FlagType = iota
	IoC
	AoN
	FoK
	Cancel
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
	case Cancel:
		return "cancel"
	case Snapshot:
		return "snapshot"
	default:
		return ""
	}
}
