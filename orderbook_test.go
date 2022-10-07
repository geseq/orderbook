package orderbook

import (
	"bufio"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	decimal "github.com/geseq/udecimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func getError(notifications []Stringer) error {
	for _, notification := range notifications {
		if n, ok := notification.(orderNotification); ok && n.Error != nil {
			return n.Error
		}
	}

	return nil
}

func TestLimitOrder_Create(t *testing.T) {
	n, ob := getTestOrderBook()

	for i := 50; i < 100; i = i + 10 {
		n.Reset()
		processLine(ob, fmt.Sprintf("%d	L	B	2	%d	0	N", i, i))
		n.Verify(t, []string{fmt.Sprintf("CreateOrder Accepted %d 2", i)})
	}

	for i := 100; i < 150; i = i + 10 {
		n.Reset()
		processLine(ob, fmt.Sprintf("%d	L	S	2	%d	0	N", i, i))
		n.Verify(t, []string{fmt.Sprintf("CreateOrder Accepted %d 2", i)})
	}

	assert.Nil(t, ob.Order(999))
	assert.NotNil(t, ob.Order(100))
}

func TestLimitOrder_CreateBuy(t *testing.T) {
	n, ob := getTestOrderBook()
	addDepth(ob, 0)

	n.Reset()
	processLine(ob, "1100	L	B	1	100	0	N")
	n.Verify(t, []string{
		"CreateOrder Accepted 1100 1",
		"6 1100 FilledPartial FilledComplete 1 100",
	})

	n.Reset()
	processLine(ob, "1150	L	B	10	150	0	N")
	n.Verify(t, []string{
		"CreateOrder Accepted 1150 10",
		"6 1150 FilledComplete FilledPartial 1 100",
		"7 1150 FilledComplete FilledPartial 2 110",
		"8 1150 FilledComplete FilledPartial 2 120",
		"9 1150 FilledComplete FilledPartial 2 130",
		"10 1150 FilledComplete FilledPartial 2 140",
	})
}

func TestLimitOrder_CreateWithZeroQty(t *testing.T) {
	n, ob := getTestOrderBook()
	addDepth(ob, 0)
	n.Reset()

	processLine(ob, "170	L	S	0	40	0	N")
	n.Verify(t, []string{
		"CreateOrder Rejected 170 0 ErrInvalidQuantity",
	})
}

func TestLimitOrder_CreateWithZeroPrice(t *testing.T) {
	n, ob := getTestOrderBook()
	addDepth(ob, 0)
	n.Reset()

	processLine(ob, "170	L	S	10	0	0	N")
	n.Verify(t, []string{
		"CreateOrder Rejected 170 0 ErrInvalidPrice",
	})
}

func TestLimitOrder_CreateAndCancel(t *testing.T) {
	n, ob := getTestOrderBook()
	addDepth(ob, 0)
	n.Reset()

	processLine(ob, "170	L	S	10	1000	0	N")
	ob.CancelOrder(tok, 170)
	tok++

	n.Verify(t, []string{
		"CreateOrder Accepted 170 10",
		"CancelOrder Canceled 170 10",
	})
}

func TestLimitOrder_CancelNonExistent(t *testing.T) {
	n, ob := getTestOrderBook()
	addDepth(ob, 0)
	n.Reset()

	ob.CancelOrder(tok, 170)
	tok++

	n.Verify(t, []string{
		"CancelOrder Rejected 170 0 ErrOrderNotExists",
	})
}

func TestLimitOrder_CreateIOCWithNoMatches(t *testing.T) {
	n, ob := getTestOrderBook()
	addDepth(ob, 0)
	n.Reset()

	processLine(ob, "300	L	S	1	200	0	I")
	n.Verify(t, []string{
		"CreateOrder Accepted 300 1",
	})
}

func TestLimitOrder_CreateIOCWithMatches(t *testing.T) {
	n, ob := getTestOrderBook()
	addDepth(ob, 0)
	n.Reset()
	t.Log(ob)

	processLine(ob, "300	L	S	1	90	0	I")

	n.Verify(t, []string{
		"CreateOrder Accepted 300 1",
		"5 300 FilledPartial FilledComplete 1 90",
	})
}

func TestLimitOrder_CreateSell(t *testing.T) {
	n, ob := getTestOrderBook()
	addDepth(ob, 0)
	n.Reset()

	t.Log(ob)

	processLine(ob, "340	L	S	11	40	0	N")
	processLine(ob, "343	L	S	11	1	0	I")

	n.Verify(t, []string{
		"CreateOrder Accepted 340 11",
		"5 340 FilledComplete FilledPartial 2 90",
		"4 340 FilledComplete FilledPartial 2 80",
		"3 340 FilledComplete FilledPartial 2 70",
		"2 340 FilledComplete FilledPartial 2 60",
		"1 340 FilledComplete FilledPartial 2 50",
		"CreateOrder Accepted 343 11",
	})
}

func TestLimitOrder_ClearSellBestPriceFirst(t *testing.T) {
	n, ob := getTestOrderBook()
	addDepth(ob, 0)
	n.Reset()

	processLine(ob, "900	L	B	11	1	0	N")
	processLine(ob, "901	L	S	11	1	0	N")

	require.NoError(t, getError(n.n))
	n.Verify(t, []string{
		"CreateOrder Accepted 900 11",
		"CreateOrder Accepted 901 11",
		"5 901 FilledComplete FilledPartial 2 90",
		"4 901 FilledComplete FilledPartial 2 80",
		"3 901 FilledComplete FilledPartial 2 70",
		"2 901 FilledComplete FilledPartial 2 60",
		"1 901 FilledComplete FilledPartial 2 50",
		"900 901 FilledPartial FilledComplete 1 1",
	})
}

func TestMarketProcess(t *testing.T) {
	n, ob := getTestOrderBook()
	addDepth(ob, 0)
	n.Reset()

	processLine(ob, "800	M	B	3	0	0	N")
	processLine(ob, "801	M	B	0	0	0	N")
	processLine(ob, "802	M	S	12	0	0	N")
	processLine(ob, "803	M	B	12	0	0	A")
	processLine(ob, "804	M	B	12	0	0	N")

	n.Verify(t, []string{
		"CreateOrder Accepted 800 3",
		"6 800 FilledComplete FilledPartial 2 100",
		"7 800 FilledPartial FilledComplete 1 110",
		"CreateOrder Rejected 801 0 ErrInvalidQuantity",
		"CreateOrder Accepted 802 12",
		"5 802 FilledComplete FilledPartial 2 90",
		"4 802 FilledComplete FilledPartial 2 80",
		"3 802 FilledComplete FilledPartial 2 70",
		"2 802 FilledComplete FilledPartial 2 60",
		"1 802 FilledComplete FilledPartial 2 50",
		// TODO Add notification for market order unable to be filled fully
		"CreateOrder Accepted 803 12",
		"CreateOrder Accepted 804 12",
		"7 804 FilledComplete FilledPartial 1 110",
		"8 804 FilledComplete FilledPartial 2 120",
		"9 804 FilledComplete FilledPartial 2 130",
		"10 804 FilledComplete FilledPartial 2 140",
	})
}

func TestMarketProcess_PriceLevel_FIFO(t *testing.T) {
	n, ob := getTestOrderBook()
	addDepth(ob, 0)
	addDepth(ob, 1)
	n.Reset()

	processLine(ob, "801	M	B	6	0	0	N")
	n.Verify(t, []string{
		"CreateOrder Accepted 801 6",
		"6 801 FilledComplete FilledPartial 2 100",
		"16 801 FilledComplete FilledPartial 2 100",
		"7 801 FilledComplete FilledComplete 2 110",
	})
}

func TestStopPlace(t *testing.T) {
	n, ob := getTestOrderBook()

	for i := 50; i < 100; i = i + 10 {
		processLine(ob, fmt.Sprintf("%d	L	B	2	%d	10	N", i, i))
	}

	for i := 100; i < 150; i = i + 10 {
		processLine(ob, fmt.Sprintf("%d	L	S	2	%d	200	N", i, i))
	}

	for i := 150; i < 200; i = i + 10 {
		processLine(ob, fmt.Sprintf("%d	M	B	2	0	5	N", i))
	}

	for i := 200; i < 250; i = i + 10 {
		processLine(ob, fmt.Sprintf("%d1	L	S	2	0	210	N", i))
		processLine(ob, fmt.Sprintf("%d2	M	B	2	0	5	N", i))
	}

	n.Verify(t, []string{
		"CreateOrder Accepted 50 2",
		"CreateOrder Accepted 60 2",
		"CreateOrder Accepted 70 2",
		"CreateOrder Accepted 80 2",
		"CreateOrder Accepted 90 2",
		"CreateOrder Accepted 100 2",
		"CreateOrder Accepted 110 2",
		"CreateOrder Accepted 120 2",
		"CreateOrder Accepted 130 2",
		"CreateOrder Accepted 140 2",
		"CreateOrder Accepted 150 2",
		"CreateOrder Accepted 160 2",
		"CreateOrder Accepted 170 2",
		"CreateOrder Accepted 180 2",
		"CreateOrder Accepted 190 2",
		"CreateOrder Accepted 2001 2",
		"CreateOrder Accepted 2002 2",
		"CreateOrder Accepted 2101 2",
		"CreateOrder Accepted 2102 2",
		"CreateOrder Accepted 2201 2",
		"CreateOrder Accepted 2202 2",
		"CreateOrder Accepted 2301 2",
		"CreateOrder Accepted 2302 2",
		"CreateOrder Accepted 2401 2",
		"CreateOrder Accepted 2402 2",
	})
	assert.Nil(t, ob.Order(999))
	assert.NotNil(t, ob.Order(100))
}

func TestStopProcess(t *testing.T) {
	n, ob := getTestOrderBook()
	addDepth(ob, 0)
	n.Reset()

	processLine(ob, "100	L	B	1	100	110	N")
	processLine(ob, "101	M	B	2	0	0	N")
	processLine(ob, "102	M	B	2	0	0	N")
	processLine(ob, "103	M	S	2	0	0	N")
	processLine(ob, "104	M	B	1	0	0	N")
	processLine(ob, "105	M	B	2	0	110	N") // @ LP 120. This should trigger immediately
	processLine(ob, "106	M	B	1	0	0	N")
	processLine(ob, "107	M	S	1	0	0	N")
	processLine(ob, "206	M	S	2	0	100	N") // @ LP 90. This should trigger immediately
	processLine(ob, "207	M	S	1	0	0	N")

	n.Verify(t, []string{
		"CreateOrder Accepted 100 1",
		"CreateOrder Accepted 101 2",
		"6 101 FilledComplete FilledComplete 2 100",
		"CreateOrder Accepted 102 2",
		"7 102 FilledComplete FilledComplete 2 110",
		"CreateOrder Accepted 103 2",
		"100 103 FilledComplete FilledPartial 1 100",
		"5 103 FilledPartial FilledComplete 1 90",
		"CreateOrder Accepted 104 1",
		"8 104 FilledPartial FilledComplete 1 120",
		"CreateOrder Accepted 105 2",
		"8 105 FilledComplete FilledPartial 1 120",
		"9 105 FilledPartial FilledComplete 1 130",
		"CreateOrder Accepted 106 1",
		"9 106 FilledComplete FilledComplete 1 130",
		"CreateOrder Accepted 107 1",
		"5 107 FilledComplete FilledComplete 1 90",
		"CreateOrder Accepted 206 2",
		"4 206 FilledComplete FilledComplete 2 80",
		"CreateOrder Accepted 207 1",
		"3 207 FilledPartial FilledComplete 1 70",
	})
}

var j uint64
var k uint64 = 100000

func BenchmarkOrderbook(b *testing.B) {
	benchmarkOrderbookLimitCreate(10000, b)
}

func benchmarkOrderbookLimitCreate(n int, b *testing.B) {
	tok = 1
	on := &EmptyNotification{}
	ob := NewOrderBook(on)

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

func getTestOrderBook() (*Notification, *OrderBook) {
	tok = 1
	on := &Notification{}
	ob := NewOrderBook(on)

	return on, ob
}

type orderNotification struct {
	MsgType MsgType
	Status  OrderStatus
	OrderID uint64
	Qty     decimal.Decimal
	Error   error
}

func (o orderNotification) String() string {
	if o.Error != nil {
		var errName string
		switch o.Error {
		case ErrOrderNotExists:
			errName = "ErrOrderNotExists"
		case ErrInvalidQuantity:
			errName = "ErrInvalidQuantity"
		case ErrInvalidPrice:
			errName = "ErrInvalidPrice"
		case ErrOrderID:
			errName = "ErrOrderID"
		case ErrOrderExists:
			errName = "ErrOrderExists"
		case ErrInsufficientQuantity:
			errName = "ErrInsufficientQuantity"
		case ErrNoMatching:
			errName = "ErrNoMatching"
		}

		return fmt.Sprintf("%s %s %d %s %s", o.MsgType, o.Status, o.OrderID, o.Qty.String(), errName)
	}
	return fmt.Sprintf("%s %s %d %s", o.MsgType, o.Status, o.OrderID, o.Qty.String())
}

type Stringer interface {
	String() string
}

type Notification struct {
	n []Stringer
}

func (o *Notification) Reset() {
	o.n = []Stringer{}
}

func (o *Notification) PutOrder(m MsgType, s OrderStatus, orderID uint64, qty decimal.Decimal, err error) {
	o.n = append(o.n, orderNotification{m, s, orderID, qty, err})
}

func (o *Notification) PutTrade(mID, tID uint64, mStatus, tStatus OrderStatus, qty, price decimal.Decimal) {
	o.n = append(o.n, Trade{mID, tID, mStatus, tStatus, qty, price})
}

func (o *Notification) Strings() []string {
	res := make([]string, 0, len(o.n))
	for _, n := range o.n {
		res = append(res, n.String())
	}

	return res
}

func (o *Notification) String() string {
	return strings.TrimSpace(strings.Join(o.Strings(), "\n"))
}

func (o *Notification) Verify(t *testing.T, expected []string) {
	assert.Equal(t, expected, o.Strings())
}

type EmptyNotification struct {
}

func (o *EmptyNotification) PutOrder(m MsgType, s OrderStatus, orderID uint64, qty decimal.Decimal, err error) {
}

func (e *EmptyNotification) PutTrade(mID, tID uint64, mStatus, tStatus OrderStatus, qty, price decimal.Decimal) {
}
