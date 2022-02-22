package orderbook

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	decimal "github.com/geseq/udecimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type OrderHandler struct {
	n []OrderNotification
}

func (o *OrderHandler) Put(on OrderNotification) {
	o.n = append(o.n, on)
}

type Trades struct {
	n []Trade
}

func (o *Trades) Put(on Trade) {
	o.n = append(o.n, on)
}

type EmptyOrderHandler struct {
}

func (e *EmptyOrderHandler) Put(o OrderNotification) {
}

type EmptyTrades struct {
}

func (e *EmptyTrades) Put(o Trade) {
}

var owg sync.WaitGroup
var twg sync.WaitGroup

var tok uint64

func addDepth(ob *OrderBook, prefix uint64, quantity decimal.Decimal) {
	var i uint64
	for i = 50; i < 100; i = i + 10 {
		ob.ProcessOrder(tok, prefix*1000+i, Limit, Buy, quantity, decimal.New(uint64(i), 0), decimal.Zero, None)
		tok++
	}

	for i = 100; i < 150; i = i + 10 {
		ob.ProcessOrder(tok, prefix*1000+i, Limit, Sell, quantity, decimal.New(uint64(i), 0), decimal.Zero, None)
		tok++
	}
}

func getQtyProcessed(trades *[]Trade) decimal.Decimal {
	qty := decimal.Zero
	for _, trade := range *trades {
		qty = qty.Add(trade.Qty)
	}

	return qty
}

func getError(notifications *[]OrderNotification) error {
	for _, notification := range *notifications {
		if notification.Error != nil {
			return notification.Error
		}
	}

	return nil
}

func TestLimitOrder_Create(t *testing.T) {
	done, trades, ob := getTestOrderBook()

	quantity := decimal.New(2, 0)
	for i := 50; i < 100; i = i + 10 {
		resetTest(done, trades)
		ob.ProcessOrder(tok, uint64(i), Limit, Buy, quantity, decimal.New(uint64(i), 0), decimal.Zero, None)
		tok++
		assert.Len(t, *done, 1)
		assert.Len(t, *trades, 0)
		require.NoError(t, getError(done))
	}

	for i := 100; i < 150; i = i + 10 {
		resetTest(done, trades)
		ob.ProcessOrder(tok, uint64(i), Limit, Sell, quantity, decimal.New(uint64(i), 0), decimal.Zero, None)
		tok++
		assert.Len(t, *done, 1)
		assert.Len(t, *trades, 0)
		require.NoError(t, getError(done))
	}

	assert.Nil(t, ob.Order(999))
	assert.NotNil(t, ob.Order(100))
}

func TestLimitOrder_CreateBuy(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0, decimal.New(2, 0))

	resetTest(done, trades)

	ob.ProcessOrder(tok, 100100, Limit, Buy, decimal.New(1, 0), decimal.New(100, 0), decimal.Zero, None)
	tok++

	require.NoError(t, getError(done))
	tr := *trades
	require.Len(t, *trades, 1)
	assert.Equal(t, OrderFilledPartial, tr[0].MakerStatus)
	assert.EqualValues(t, uint64(100100), tr[0].TakerOrderID)
	assert.Equal(t, OrderFilledComplete, tr[0].TakerStatus)

	quantityProcessed := getQtyProcessed(trades)
	require.Len(t, *trades, 1)
	assert.True(t, quantityProcessed.Equal(decimal.New(1, 0)))

	resetTest(done, trades)

	ob.ProcessOrder(tok, 100150, Limit, Buy, decimal.New(10, 0), decimal.New(150, 0), decimal.Zero, None)
	tok++

	require.NoError(t, getError(done))
	require.Len(t, *done, 1)

	d := *done
	assert.Equal(t, OrderQueued, d[0].Status)
	assert.True(t, d[0].Qty.Equal(decimal.New(1, 0)))

	quantityProcessed = getQtyProcessed(trades)
	require.Len(t, *trades, 5)
	require.True(t, quantityProcessed.Equal(decimal.New(9, 0)))
}

func TestLimitOrder_CreateWithZeroQty(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0, decimal.New(2, 0))
	resetTest(done, trades)

	ob.ProcessOrder(tok, 100070, Limit, Sell, decimal.New(0, 0), decimal.New(40, 0), decimal.Zero, None)
	tok++

	require.Len(t, *done, 1)
	require.Error(t, getError(done))
	require.Len(t, *trades, 0)
}

func TestLimitOrder_CreateWithZeroPrice(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0, decimal.New(2, 0))
	resetTest(done, trades)

	ob.ProcessOrder(tok, 100070, Limit, Sell, decimal.New(10, 0), decimal.New(0, 0), decimal.Zero, None)
	tok++

	require.Len(t, *done, 1)
	require.Error(t, getError(done))
	require.Len(t, *trades, 0)
}

func TestLimitOrder_CreateAndCancel(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0, decimal.New(2, 0))
	resetTest(done, trades)

	ob.ProcessOrder(tok, 101070, Limit, Sell, decimal.New(10, 0), decimal.New(1000, 0), decimal.Zero, None)
	tok++
	ob.CancelOrder(tok, 101070)
	tok++

	d := *done
	assert.Len(t, d, 2)
	assert.Equal(t, OrderCanceled, d[1].Status)
	assert.Equal(t, uint64(101070), d[1].OrderID)
}

func TestLimitOrder_CancelNonExistent(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0, decimal.New(2, 0))
	resetTest(done, trades)

	ob.CancelOrder(tok, 108100)
	tok++

	d := *done
	assert.Len(t, d, 1)
	assert.Equal(t, OrderCancelRejected, d[0].Status)
	assert.Equal(t, uint64(108100), d[0].OrderID)
}

func TestLimitOrder_CreateAndCancelWithinProcess(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0, decimal.New(2, 0))
	resetTest(done, trades)

	ob.ProcessOrder(tok, 101071, Limit, Sell, decimal.New(10, 0), decimal.New(1000, 0), decimal.Zero, None)
	tok++
	ob.ProcessOrder(tok, 101071, Limit, Sell, decimal.New(10, 0), decimal.New(1000, 0), decimal.Zero, Cancel)
	tok++

	d := *done
	assert.Len(t, d, 2)
	assert.Equal(t, OrderCanceled, d[1].Status)
	assert.Equal(t, uint64(101071), d[1].OrderID)
}

func TestLimitOrder_CancelNonExistentWithinProcess(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0, decimal.New(2, 0))
	resetTest(done, trades)

	ob.ProcessOrder(tok, 108100, Limit, Sell, decimal.New(10, 0), decimal.New(1000, 0), decimal.Zero, Cancel)
	tok++

	d := *done
	assert.Len(t, d, 1)
	assert.Equal(t, OrderCancelRejected, d[0].Status)
	assert.Equal(t, uint64(108100), d[0].OrderID)
}

func TestLimitOrder_CreateIOCWithNoMatches(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0, decimal.New(2, 0))
	resetTest(done, trades)

	ob.ProcessOrder(tok, 103100, Limit, Sell, decimal.New(1, 0), decimal.New(200, 0), decimal.Zero, IoC)
	tok++

	require.NoError(t, getError(done))
	d := *done
	assert.Len(t, d, 0)

	quantityProcessed := getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(0, 0)))
}

func TestLimitOrder_CreateIOCWithMatches(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0, decimal.New(2, 0))
	resetTest(done, trades)
	t.Log(ob)

	ob.ProcessOrder(tok, 103100, Limit, Sell, decimal.New(1, 0), decimal.New(90, 0), decimal.Zero, IoC)
	tok++

	require.NoError(t, getError(done))
	d := *done
	assert.Len(t, d, 0)
	assert.Len(t, *trades, 1)

	quantityProcessed := getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(1, 0)))
}

func TestLimitOrder_CreateSell(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0, decimal.New(2, 0))
	resetTest(done, trades)

	t.Log(ob)

	ob.ProcessOrder(tok, 103140, Limit, Sell, decimal.New(11, 0), decimal.New(40, 0), decimal.Zero, None)
	tok++

	require.NoError(t, getError(done))
	assert.Len(t, *done, 1)

	quantityProcessed := getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(10, 0)))

	resetTest(done, trades)
	ob.ProcessOrder(tok, 103143, Limit, Sell, decimal.New(11, 0), decimal.New(1, 0), decimal.Zero, IoC)
	tok++

	require.NoError(t, getError(done))
	assert.Len(t, *done, 0)

	quantityProcessed = getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(0, 0)))
}

func TestLimitOrder_ClearSellBestPriceFirst(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0, decimal.New(2, 0))
	resetTest(done, trades)

	ob.ProcessOrder(tok, 108900, Limit, Buy, decimal.New(11, 0), decimal.New(1, 0), decimal.Zero, None)
	tok++

	require.NoError(t, getError(done))
	assert.Len(t, *done, 1)

	quantityProcessed := getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(0, 0)))

	resetTest(done, trades)

	ob.ProcessOrder(tok, 113900, Limit, Sell, decimal.New(11, 0), decimal.New(1, 0), decimal.Zero, None)
	tok++

	require.NoError(t, getError(done))
	assert.Len(t, *done, 1)

	quantityProcessed = getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(11, 0)))
}

func TestMarketProcess(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0, decimal.New(2, 0))
	resetTest(done, trades)

	ob.ProcessOrder(tok, 100800, Market, Buy, decimal.New(3, 0), decimal.Zero, decimal.Zero, None)
	tok++

	require.NoError(t, getError(done))

	quantityProcessed := getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(3, 0)))

	resetTest(done, trades)

	ob.ProcessOrder(tok, 100802, Market, Buy, decimal.New(0, 0), decimal.Zero, decimal.Zero, None)
	tok++

	require.Error(t, getError(done))

	resetTest(done, trades)

	ob.ProcessOrder(tok, 100901, Market, Sell, decimal.New(12, 0), decimal.Zero, decimal.Zero, None)
	tok++

	require.NoError(t, getError(done))

	assert.Len(t, *done, 0)

	quantityProcessed = getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(10, 0)))

	resetTest(done, trades)
	ob.ProcessOrder(tok, 101803, Market, Buy, decimal.New(12, 0), decimal.Zero, decimal.Zero, AoN)
	tok++

	require.NoError(t, getError(done))
	assert.Len(t, *done, 0)

	quantityProcessed = getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(0, 0)))

	resetTest(done, trades)

	ob.ProcessOrder(tok, 101804, Market, Buy, decimal.New(12, 0), decimal.Zero, decimal.Zero, None)
	tok++

	require.NoError(t, getError(done))
	assert.Len(t, *done, 0)

	quantityProcessed = getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(7, 0)))

	resetTest(done, trades)
}

func TestMarketProcess2(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0, decimal.New(2, 0))
	addDepth(ob, 1, decimal.New(2, 0))
	resetTest(done, trades)

	ob.ProcessOrder(tok, 100801, Market, Buy, decimal.New(6, 0), decimal.Zero, decimal.Zero, None)
	tok++

	require.NoError(t, getError(done))

	quantityProcessed := getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(6, 0)))
}

func TestStopPlace(t *testing.T) {
	done, trades, ob := getTestOrderBook()

	quantity := decimal.New(2, 0)
	for i := 50; i < 100; i = i + 10 {
		resetTest(done, trades)
		ob.ProcessOrder(tok, uint64(i), Limit, Buy, quantity, decimal.New(uint64(i), 0), decimal.New(uint64(10), 0), None)
		tok++

		require.Len(t, *done, 0)
		require.NoError(t, getError(done))

		quantityProcessed := getQtyProcessed(trades)
		require.True(t, quantityProcessed.Equal(decimal.Zero))
	}

	for i := 100; i < 150; i = i + 10 {
		resetTest(done, trades)
		ob.ProcessOrder(tok, uint64(i), Limit, Sell, quantity, decimal.New(uint64(i), 0), decimal.New(uint64(200), 0), None)
		tok++

		require.Len(t, *done, 0)
		require.NoError(t, getError(done))

		quantityProcessed := getQtyProcessed(trades)
		require.True(t, quantityProcessed.Equal(decimal.Zero))
	}

	for i := 150; i < 200; i = i + 10 {
		resetTest(done, trades)
		ob.ProcessOrder(tok, uint64(i), Market, Buy, quantity, decimal.Zero, decimal.New(uint64(5), 0), None)
		tok++

		require.Len(t, *done, 0)
		require.NoError(t, getError(done))

		quantityProcessed := getQtyProcessed(trades)
		require.True(t, quantityProcessed.Equal(decimal.Zero))
	}

	for i := 200; i < 250; i = i + 10 {
		resetTest(done, trades)

		ob.ProcessOrder(tok, uint64(i), Limit, Sell, quantity, decimal.Zero, decimal.New(uint64(210), 0), None)
		tok++
		ob.ProcessOrder(tok, uint64(i), Market, Buy, quantity, decimal.Zero, decimal.New(uint64(5), 0), None)
		tok++

		require.Len(t, *done, 1)
		require.Error(t, getError(done))

		quantityProcessed := getQtyProcessed(trades)
		require.True(t, quantityProcessed.Equal(decimal.Zero))
	}

	assert.Nil(t, ob.Order(999))
	assert.NotNil(t, ob.Order(100))
}

func TestStopProcess(t *testing.T) {
	done, trades, ob := getTestOrderBook()

	addDepth(ob, 0, decimal.New(2, 0))

	ob.ProcessOrder(tok, 100100, Limit, Buy, decimal.New(1, 0), decimal.New(100, 0), decimal.New(110, 0), None)
	tok++

	require.Len(t, *trades, 0)
	require.NoError(t, getError(done))

	ob.ProcessOrder(tok, 100101, Market, Buy, decimal.New(2, 0), decimal.Zero, decimal.Zero, None)
	tok++

	ob.ProcessOrder(tok, 100102, Market, Buy, decimal.New(2, 0), decimal.Zero, decimal.Zero, None)
	tok++

	resetTest(done, trades)

	ob.ProcessOrder(tok, 100103, Market, Sell, decimal.New(2, 0), decimal.Zero, decimal.Zero, None)
	tok++

	require.Len(t, *trades, 2)
	tr := *trades

	assert.Equal(t, uint64(100100), tr[0].MakerOrderID)
	require.True(t, tr[0].Qty.Equal(decimal.New(1, 0)))

	// reset last price to over 110

	ob.ProcessOrder(tok, 100104, Market, Buy, decimal.New(1, 0), decimal.Zero, decimal.Zero, None)
	tok++

	resetTest(done, trades)
	ob.ProcessOrder(tok, 100105, Market, Buy, decimal.New(2, 0), decimal.Zero, decimal.New(110, 0), None)
	tok++
	require.Len(t, *trades, 0)

	require.NoError(t, getError(done))

	ob.ProcessOrder(tok, 100106, Market, Buy, decimal.New(1, 0), decimal.Zero, decimal.Zero, None)
	tok++

	resetTest(done, trades)

	ob.ProcessOrder(tok, 100107, Market, Sell, decimal.New(1, 0), decimal.Zero, decimal.Zero, None)
	tok++

	require.Len(t, *trades, 2)

	tr = *trades
	assert.Equal(t, uint64(100105), tr[1].TakerOrderID)
	require.True(t, tr[1].Qty.Equal(decimal.New(2, 0)))

	resetTest(done, trades)
	ob.ProcessOrder(tok, 100206, Market, Sell, decimal.New(2, 0), decimal.Zero, decimal.New(100, 0), None)
	tok++

	require.Len(t, *trades, 0)
	require.NoError(t, getError(done))

	resetTest(done, trades)

	ob.ProcessOrder(tok, 100207, Market, Sell, decimal.New(1, 0), decimal.Zero, decimal.Zero, None)
	tok++

	require.Len(t, *trades, 3)
	tr = *trades

	assert.Equal(t, uint64(100206), tr[1].TakerOrderID)
	require.True(t, tr[1].Qty.Equal(decimal.New(1, 0)))

	assert.Equal(t, uint64(100206), tr[2].TakerOrderID)
	require.True(t, tr[2].Qty.Equal(decimal.New(1, 0)))
}

var j uint64
var k uint64 = 100000

func addBenchDepth(ob *OrderBook, prefix uint64, quantity decimal.Decimal) {
	var i uint64
	for i = 50; i < 100; i = i + 10 {
		j++
		ob.ProcessOrder(tok, j, Limit, Buy, quantity, decimal.New(uint64(i), 0), decimal.Zero, None)
		tok++
	}

	for i = 100; i < 150; i = i + 10 {
		j++
		ob.ProcessOrder(tok, j, Limit, Sell, quantity, decimal.New(uint64(i), 0), decimal.Zero, None)
		tok++
	}

}

func BenchmarkOrderbook(b *testing.B) {
	benchmarkOrderbookLimitCreate(10000, b)
}

func benchmarkOrderbookLimitCreate(n int, b *testing.B) {
	tok = 1
	on := &EmptyOrderHandler{}
	tn := &EmptyTrades{}
	ob := NewOrderBook(on, tn)

	orders := make([]Order, b.N)
	for i := 0; i < b.N; i += 1 {
		side := Buy
		class := Limit
		if rand.Intn(10) < 5 {
			side = Sell
		}
		if rand.Intn(10) < 5 {
			class = Market
		}

		orders[i] = Order{
			ID:        uint64(i),
			Class:     class,
			Side:      side,
			Flag:      None,
			Qty:       decimal.NewI(uint64(rand.Intn(1000)), 0),
			Price:     decimal.NewI(uint64(rand.Intn(100000)), uint(rand.Intn(3))),
			StopPrice: decimal.Zero,
		}
	}

	b.ReportAllocs()

	stopwatch := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i += 1 {
		order := orders[i]
		ob.ProcessOrder(tok, order.ID, order.Class, order.Side, order.Price, order.Qty, order.StopPrice, order.Flag) // 1 ts
		tok++
	}
	b.StopTimer()
	elapsed := time.Since(stopwatch)
	fmt.Printf("\n\nElapsed: %s\nOrders per second (avg): %f\n", elapsed, float64(b.N)/elapsed.Seconds())
	b.StartTimer()
}

func getTestOrderBook() (*[]OrderNotification, *[]Trade, *OrderBook) {
	tok = 1
	on := &OrderHandler{}
	tn := &Trades{}
	ob := NewOrderBook(on, tn)

	return &on.n, &tn.n, ob
}

func resetTest(done *[]OrderNotification, trades *[]Trade) {
	*done = []OrderNotification{}
	*trades = []Trade{}
}
