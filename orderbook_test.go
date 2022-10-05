package orderbook

import (
	"bufio"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
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

func (o *OrderHandler) String() string {
	res := make([]string, len(o.n))
	for _, n := range o.n {
		res = append(res, n.String())
	}

	return strings.Join(res, "\n")
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

var depth = `
# add depth to the orderbook
1	L	B	2	50	0	N
2	L	B	2	60	0	N
3	L	B	2	70	0	N
4	L	B	2	80	0	N
5	L	B	2	90	0	N
6	L	S	2	100	0	N
7	L	S	2	110	0	N
8	L	S	2	120	0	N
9	L	S	2	130	0	N
10	L	S	2	140	0	N
`

var re = regexp.MustCompile("#.*")

func addPrefix(input, prefix string) string {
	lines := []string{}
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if len(line) == 0 || line[0] == '#' {
			continue
		}

		lines = append(lines, prefix+line)
	}

	return strings.Join(lines, "\n")
}

func processOrders(ob *OrderBook, input string) {
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if len(line) == 0 || line[0] == '#' {
			continue
		}

		processLine(ob, line)
	}
}

func processLine(ob *OrderBook, line string) {
	line = string(re.ReplaceAll([]byte(line), nil))

	parts := strings.Split(line, "\t")
	if len(parts) == 0 {
		return
	}

	oid, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	class := Market
	if strings.TrimSpace(parts[1]) == "L" {
		class = Limit
	}

	side := Buy
	if strings.TrimSpace(parts[2]) == "S" {
		side = Sell
	}

	qty, _ := decimal.Parse(strings.TrimSpace(parts[3]))
	price, _ := decimal.Parse(strings.TrimSpace(parts[4]))
	stopPrice, _ := decimal.Parse(strings.TrimSpace(parts[5]))

	flag := None
	switch strings.TrimSpace(parts[6]) {
	case "I":
		flag = IoC
	case "A":
		flag = AoN
	case "F":
		flag = FoK
	case "S":
		flag = Snapshot
	}

	ob.ProcessOrder(tok, uint64(oid), class, side, qty, price, stopPrice, flag)
	tok++
}

func addDepth(ob *OrderBook, prefix int) {
	d := depth
	if prefix > 0 {
		d = addPrefix(d, strconv.Itoa(prefix))
	}

	processOrders(ob, d)
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

	for i := 50; i < 100; i = i + 10 {
		resetTest(done, trades)
		processLine(ob, fmt.Sprintf("%d	L	B	2	%d	0	N", i, i))
		assert.Len(t, *done, 1)
		assert.Len(t, *trades, 0)
		require.NoError(t, getError(done))
	}

	for i := 100; i < 150; i = i + 10 {
		resetTest(done, trades)
		processLine(ob, fmt.Sprintf("%d	L	S	2	%d	0	N", i, i))
		assert.Len(t, *done, 1)
		assert.Len(t, *trades, 0)
		require.NoError(t, getError(done))
	}

	assert.Nil(t, ob.Order(999))
	assert.NotNil(t, ob.Order(100))
}

func TestLimitOrder_CreateBuy(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0)

	resetTest(done, trades)
	processLine(ob, "1100	L	B	1	100	0	N")

	require.NoError(t, getError(done))
	tr := *trades
	require.Len(t, *trades, 1)
	assert.Equal(t, FilledPartial, tr[0].MakerStatus)
	assert.EqualValues(t, uint64(1100), tr[0].TakerOrderID)
	assert.Equal(t, FilledComplete, tr[0].TakerStatus)

	quantityProcessed := getQtyProcessed(trades)
	require.Len(t, *trades, 1)
	assert.True(t, quantityProcessed.Equal(decimal.New(1, 0)))

	resetTest(done, trades)
	processLine(ob, "1150	L	B	10	150	0	N")

	require.NoError(t, getError(done))
	require.Len(t, *done, 1)

	d := *done
	assert.Equal(t, Accepted, d[0].Status)
	assert.True(t, d[0].Qty.Equal(decimal.New(10, 0)))

	quantityProcessed = getQtyProcessed(trades)
	require.Len(t, *trades, 5)
	require.True(t, quantityProcessed.Equal(decimal.New(9, 0)))
}

func TestLimitOrder_CreateWithZeroQty(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0)
	resetTest(done, trades)

	processLine(ob, "170	L	S	0	40	0	N")

	require.Len(t, *done, 1)
	require.Error(t, getError(done))
	require.Len(t, *trades, 0)
}

func TestLimitOrder_CreateWithZeroPrice(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0)
	resetTest(done, trades)

	processLine(ob, "170	L	S	10	0	0	N")

	require.Len(t, *done, 1)
	require.Error(t, getError(done))
	require.Len(t, *trades, 0)
}

func TestLimitOrder_CreateAndCancel(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0)
	resetTest(done, trades)

	processLine(ob, "170	L	S	10	1000	0	N")
	ob.CancelOrder(tok, 170)
	tok++

	d := *done
	assert.Len(t, d, 2)
	assert.Equal(t, Canceled, d[1].Status)
	assert.Equal(t, uint64(170), d[1].OrderID)
}

func TestLimitOrder_CancelNonExistent(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0)
	resetTest(done, trades)

	ob.CancelOrder(tok, 8100)
	tok++

	d := *done
	assert.Len(t, d, 1)
	assert.Equal(t, Rejected, d[0].Status)
	assert.Equal(t, uint64(8100), d[0].OrderID)
}

func TestLimitOrder_CreateIOCWithNoMatches(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0)
	resetTest(done, trades)

	processLine(ob, "300	L	S	1	200	0	I")

	require.NoError(t, getError(done))
	d := *done
	assert.Len(t, d, 1)

	quantityProcessed := getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(0, 0)))
}

func TestLimitOrder_CreateIOCWithMatches(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0)
	resetTest(done, trades)
	t.Log(ob)

	processLine(ob, "300	L	S	1	90	0	I")

	require.NoError(t, getError(done))
	d := *done
	assert.Len(t, d, 1)
	assert.Len(t, *trades, 1)

	quantityProcessed := getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(1, 0)))
}

func TestLimitOrder_CreateSell(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0)
	resetTest(done, trades)

	t.Log(ob)

	processLine(ob, "340	L	S	11	40	0	N")

	require.NoError(t, getError(done))
	assert.Len(t, *done, 1)

	quantityProcessed := getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(10, 0)))

	resetTest(done, trades)
	processLine(ob, "343	L	S	11	1	0	I")

	require.NoError(t, getError(done))
	assert.Len(t, *done, 1)

	quantityProcessed = getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(0, 0)))
}

func TestLimitOrder_ClearSellBestPriceFirst(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0)
	resetTest(done, trades)

	processLine(ob, "900	L	B	11	1	0	N")

	require.NoError(t, getError(done))
	assert.Len(t, *done, 1)

	quantityProcessed := getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(0, 0)))

	resetTest(done, trades)

	processLine(ob, "901	L	S	11	1	0	N")

	require.NoError(t, getError(done))
	assert.Len(t, *done, 1)

	quantityProcessed = getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(11, 0)))
}

func TestMarketProcess(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0)
	resetTest(done, trades)

	processLine(ob, "800	M	B	3	0	0	N")

	require.NoError(t, getError(done))

	quantityProcessed := getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(3, 0)))

	resetTest(done, trades)

	processLine(ob, "802	M	B	0	0	0	N")

	require.Error(t, getError(done))

	resetTest(done, trades)

	processLine(ob, "901	M	S	12	0	0	N")

	require.NoError(t, getError(done))

	assert.Len(t, *done, 1)

	quantityProcessed = getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(10, 0)))

	resetTest(done, trades)
	processLine(ob, "1803	M	B	12	0	0	A")

	require.NoError(t, getError(done))
	assert.Len(t, *done, 1)

	quantityProcessed = getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(0, 0)))

	resetTest(done, trades)

	processLine(ob, "1804	M	B	12	0	0	N")

	require.NoError(t, getError(done))
	assert.Len(t, *done, 1)

	quantityProcessed = getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(7, 0)))

	resetTest(done, trades)
}

func TestMarketProcess2(t *testing.T) {
	done, trades, ob := getTestOrderBook()
	addDepth(ob, 0)
	addDepth(ob, 1)
	resetTest(done, trades)

	processLine(ob, "801	M	B	6	0	0	N")

	require.NoError(t, getError(done))

	quantityProcessed := getQtyProcessed(trades)
	assert.True(t, quantityProcessed.Equal(decimal.New(6, 0)))
}

func TestStopPlace(t *testing.T) {
	done, trades, ob := getTestOrderBook()

	for i := 50; i < 100; i = i + 10 {
		resetTest(done, trades)
		processLine(ob, fmt.Sprintf("%d	L	B	2	%d	10	N", i, i))

		require.Len(t, *done, 0)
		require.NoError(t, getError(done))

		quantityProcessed := getQtyProcessed(trades)
		require.True(t, quantityProcessed.Equal(decimal.Zero))
	}

	for i := 100; i < 150; i = i + 10 {
		resetTest(done, trades)
		processLine(ob, fmt.Sprintf("%d	L	S	2	%d	200	N", i, i))

		require.Len(t, *done, 0)
		require.NoError(t, getError(done))

		quantityProcessed := getQtyProcessed(trades)
		require.True(t, quantityProcessed.Equal(decimal.Zero))
	}

	for i := 150; i < 200; i = i + 10 {
		resetTest(done, trades)
		processLine(ob, fmt.Sprintf("%d	M	B	2	0	5	N", i))

		require.Len(t, *done, 0)
		require.NoError(t, getError(done))

		quantityProcessed := getQtyProcessed(trades)
		require.True(t, quantityProcessed.Equal(decimal.Zero))
	}

	for i := 200; i < 250; i = i + 10 {
		resetTest(done, trades)

		processLine(ob, fmt.Sprintf("%d	L	S	2	0	210	N", i))
		processLine(ob, fmt.Sprintf("%d	M	B	2	0	5	N", i))

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

	addDepth(ob, 0)

	processLine(ob, "100	L	B	1	100	110	N")

	require.Len(t, *trades, 0)
	require.NoError(t, getError(done))

	processLine(ob, "101	M	B	2	0	0	N")
	processLine(ob, "102	M	B	2	0	0	N")

	resetTest(done, trades)
	processLine(ob, "103	M	S	2	0	0	N")
	require.Len(t, *trades, 2)

	tr := *trades

	assert.Equal(t, uint64(100), tr[0].MakerOrderID)
	require.True(t, tr[0].Qty.Equal(decimal.New(1, 0)))

	// reset last price to over 110

	processLine(ob, "104	M	B	1	0	0	N")

	resetTest(done, trades)
	processLine(ob, "105	M	B	2	0	110	N")
	require.Len(t, *trades, 0)

	require.NoError(t, getError(done))

	processLine(ob, "106	M	B	1	0	0	N")

	resetTest(done, trades)
	processLine(ob, "107	M	S	1	0	0	N")

	require.Len(t, *trades, 2)

	tr = *trades
	assert.Equal(t, uint64(105), tr[1].TakerOrderID)
	require.True(t, tr[1].Qty.Equal(decimal.New(2, 0)))

	resetTest(done, trades)
	processLine(ob, "206	M	S	2	0	100	N")

	require.Len(t, *trades, 0)
	require.NoError(t, getError(done))

	resetTest(done, trades)
	processLine(ob, "207	M	S	1	0	0	N")

	require.Len(t, *trades, 3)
	tr = *trades

	assert.Equal(t, uint64(206), tr[1].TakerOrderID)
	require.True(t, tr[1].Qty.Equal(decimal.New(1, 0)))

	assert.Equal(t, uint64(206), tr[2].TakerOrderID)
	require.True(t, tr[2].Qty.Equal(decimal.New(1, 0)))
}

var j uint64
var k uint64 = 100000

func BenchmarkOrderbook(b *testing.B) {
	benchmarkOrderbookLimitCreate(10000, b)
}

func benchmarkOrderbookLimitCreate(n int, b *testing.B) {
	tok = 1
	on := &EmptyOrderHandler{}
	tn := &EmptyTrades{}
	ob := NewOrderBook(on, tn)

	orders := make([]Order, 1000_000)
	for i := 0; i < 1000_000; i++ {
		side := Buy
		class := Limit
		if rand.Intn(10) < 5 {
			side = Sell
		}
		if rand.Intn(10) < 7 {
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
		order := orders[rand.Intn(999_999)]
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
