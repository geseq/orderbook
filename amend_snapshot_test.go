package orderbook

import (
	"testing"

	decimal "github.com/geseq/udecimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAmendOrderReductionPreservesFIFO(t *testing.T) {
	n, ob := getTestOrderBook()
	processLine(ob, "1\tL\tS\t5\t100\t0\tN")
	processLine(ob, "2\tL\tS\t5\t100\t0\tN")
	n.Reset()

	ob.AmendOrder(tok, 1, decimal.New(2, 0), decimal.New(100, 0))
	tok++
	processLine(ob, "3\tL\tB\t3\t100\t0\tN")

	n.Verify(t, []string{
		"AmendOrder Accepted 1 2",
		"CreateOrder Accepted 3 3",
		"1 3 FilledComplete FilledPartial 2 100",
		"2 3 FilledPartial FilledComplete 1 100",
	})
}

func TestAmendOrderIncreaseLosesFIFO(t *testing.T) {
	n, ob := getTestOrderBook()
	processLine(ob, "1\tL\tS\t2\t100\t0\tN")
	processLine(ob, "2\tL\tS\t2\t100\t0\tN")
	n.Reset()

	ob.AmendOrder(tok, 1, decimal.New(3, 0), decimal.New(100, 0))
	tok++
	processLine(ob, "3\tL\tB\t1\t100\t0\tN")

	n.Verify(t, []string{
		"AmendOrder Accepted 1 3",
		"CreateOrder Accepted 3 1",
		"2 3 FilledPartial FilledComplete 1 100",
	})
}

func TestAmendOrderRepriceLosesPriorityAndMayMatch(t *testing.T) {
	n, ob := getTestOrderBook()
	processLine(ob, "1\tL\tS\t2\t100\t0\tN")
	processLine(ob, "2\tL\tB\t2\t90\t0\tN")
	n.Reset()

	ob.AmendOrder(tok, 2, decimal.New(2, 0), decimal.New(100, 0))
	tok++

	n.Verify(t, []string{
		"AmendOrder Accepted 2 2",
		"1 2 FilledComplete FilledComplete 2 100",
	})
	assert.Empty(t, ob.Snapshot().Bids)
	assert.Empty(t, ob.Snapshot().Asks)
}

func TestAmendOrderRejectsInvalidChangeAtomically(t *testing.T) {
	n, ob := getTestOrderBook()
	processLine(ob, "1\tL\tB\t5\t100\t0\tN")
	n.Reset()

	ob.AmendOrder(tok, 1, decimal.Zero, decimal.New(100, 0))
	tok++
	ob.AmendOrder(tok, 1, decimal.New(3, 0), decimal.Zero)
	tok++

	n.Verify(t, []string{
		"AmendOrder Rejected 1 0 ErrInvalidQuantity",
		"AmendOrder Rejected 1 3 ErrInvalidPrice",
	})
	snapshot := ob.Snapshot()
	require.Len(t, snapshot.Bids, 1)
	assert.Equal(t, uint64(1), snapshot.Bids[0].ID)
	assert.True(t, snapshot.Bids[0].Qty.Equal(decimal.New(5, 0)))
	assert.True(t, snapshot.Bids[0].Price.Equal(decimal.New(100, 0)))
}

func TestSnapshotUsesConsumptionOrderAndIsDetached(t *testing.T) {
	_, ob := getTestOrderBook()
	processLine(ob, "1\tL\tB\t1\t100\t0\tN")
	processLine(ob, "2\tL\tB\t2\t101\t0\tN")
	processLine(ob, "3\tL\tB\t3\t101\t0\tN")
	processLine(ob, "4\tL\tS\t4\t103\t0\tN")
	processLine(ob, "5\tL\tS\t5\t102\t0\tN")
	processLine(ob, "6\tL\tS\t6\t102\t0\tN")

	snapshot := ob.Snapshot()
	assert.Equal(t, []uint64{2, 3, 1}, snapshotIDs(snapshot.Bids))
	assert.Equal(t, []uint64{5, 6, 4}, snapshotIDs(snapshot.Asks))

	snapshot.Bids[0].Qty = decimal.Zero
	assert.True(t, ob.Order(2).Qty.Equal(decimal.New(2, 0)))
}

func TestPartialFillUpdatesAggregateVolume(t *testing.T) {
	_, ob := getTestOrderBook()
	processLine(ob, "1\tL\tS\t5\t100\t0\tN")
	processLine(ob, "2\tL\tB\t2\t100\t0\tN")

	assert.True(t, ob.asks.Volume().Equal(decimal.New(3, 0)))
	assert.True(t, ob.asks.GetQueue().TotalQty().Equal(decimal.New(3, 0)))
}

func snapshotIDs(orders []RestingOrder) []uint64 {
	ids := make([]uint64, len(orders))
	for i, order := range orders {
		ids[i] = order.ID
	}
	return ids
}
