package orderbook

import (
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	decimal "github.com/geseq/udecimal"
)

func BenchmarkLatency(b *testing.B) {
	var totalAddHist, totalCancelHist []float64
	var mu sync.Mutex

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			addHist, cancelHist := runBenchmarkLatency(b)

			mu.Lock()
			totalAddHist = append(totalAddHist, addHist...)
			totalCancelHist = append(totalCancelHist, cancelHist...)
			mu.Unlock()
		}
	})

	printResultsWithPercentiles(b, "Add Order", totalAddHist)
	printResultsWithPercentiles(b, "Cancel Order", totalCancelHist)
}

func runBenchmarkLatency(b *testing.B) ([]float64, []float64) {
	seed := time.Now().UnixNano()
	duration := 30 * time.Second
	lowerBound := decimal.MustParse("50.0")
	upperBound := decimal.MustParse("100.0")
	minSpread := decimal.MustParse("0.25")
	sched := true

	ob := getOrderBook()
	bid, ask, bidQty, askQty := getInitialVars(lowerBound, upperBound, minSpread)

	var tok, buyID, sellID uint64
	rand := rand.New(rand.NewSource(seed))

	addHist := make([]float64, 0, 1000)
	cancelHist := make([]float64, 0, 1000)

	b.ReportAllocs()

	endTime := time.Now().Add(duration)

	b.ResetTimer()

	for time.Now().Before(endTime) {
		var r = rand.Intn(10)
		dec := r < 5

		bid, ask = getPrice(bid, ask, minSpread, dec)
		if bid.LessThan(lowerBound) {
			bid, ask = getPrice(bid, ask, minSpread, false)
		} else if bid.GreaterThan(upperBound) {
			bid, ask = getPrice(bid, ask, minSpread, true)
		}

		tok++
		if sched {
			runtime.Gosched()
		}

		start := time.Now()
		ob.CancelOrder(tok, buyID)
		elapsed := time.Since(start).Nanoseconds()
		cancelHist = append(cancelHist, float64(elapsed))

		tok++
		if sched {
			runtime.Gosched()
		}

		start = time.Now()
		ob.CancelOrder(tok, sellID)
		elapsed = time.Since(start).Nanoseconds()
		cancelHist = append(cancelHist, float64(elapsed))

		tok++
		buyID = tok
		tok++
		sellID = tok

		if sched {
			runtime.Gosched()
		}

		start = time.Now()
		ob.AddOrder(buyID, buyID, Limit, Buy, bidQty, bid, decimal.Zero, None)
		elapsed = time.Since(start).Nanoseconds()
		addHist = append(addHist, float64(elapsed))

		if sched {
			runtime.Gosched()
		}

		start = time.Now()
		ob.AddOrder(sellID, sellID, Limit, Sell, askQty, ask, decimal.Zero, None)
		elapsed = time.Since(start).Nanoseconds()
		addHist = append(addHist, float64(elapsed))
	}

	b.StopTimer()

	return addHist, cancelHist
}

func BenchmarkThroughput(b *testing.B) {
	var totalThroughputHist []float64
	var mu sync.Mutex

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			throughputHist := runBenchmarkThroughput(b)

			mu.Lock()
			totalThroughputHist = append(totalThroughputHist, throughputHist...)
			mu.Unlock()
		}
	})

	printResultsWithPercentiles(b, "Throughput", totalThroughputHist)
}

func runBenchmarkThroughput(b *testing.B) []float64 {
	seed := time.Now().UnixNano()
	duration := 30 * time.Second
	lowerBound := decimal.MustParse("50.0")
	upperBound := decimal.MustParse("100.0")
	minSpread := decimal.MustParse("0.25")

	ob := getOrderBook()
	bid, ask, bidQty, askQty := getInitialVars(lowerBound, upperBound, minSpread)

	var tok, buyID, sellID uint64
	var operations int
	rand := rand.New(rand.NewSource(seed))

	throughputHist := make([]float64, 0, 1000)

	b.ReportAllocs()

	b.ResetTimer()

	start := time.Now()
	end := start.Add(duration)

	for time.Now().Before(end) {
		var r = rand.Intn(10)
		dec := r < 5

		bid, ask = getPrice(bid, ask, minSpread, dec)
		if bid.LessThan(lowerBound) {
			bid, ask = getPrice(bid, ask, minSpread, false)
		} else if bid.GreaterThan(upperBound) {
			bid, ask = getPrice(bid, ask, minSpread, true)
		}

		tok = tok + 1
		ob.CancelOrder(tok, buyID)
		tok = tok + 1
		ob.CancelOrder(tok, sellID)
		tok = tok + 1
		buyID = tok
		tok = tok + 1
		sellID = tok

		ob.AddOrder(buyID, buyID, Limit, Buy, bidQty, bid, decimal.Zero, None)
		ob.AddOrder(sellID, sellID, Limit, Sell, askQty, ask, decimal.Zero, None)
		operations += 4

		elapsed := time.Since(start).Nanoseconds()
		if elapsed > 0 {
			throughput := float64(operations) / float64(elapsed)
			throughputHist = append(throughputHist, throughput)
		}
	}
	b.StopTimer()

	return throughputHist
}

func printResultsWithPercentiles(b *testing.B, operationName string, data []float64) {
	percentiles := []float64{50, 75, 90, 95, 99, 99.9}

	b.Logf("Operation: %s", operationName)
	for _, p := range percentiles {
		value := calculatePercentile(data, p)
		b.Logf("%v: %f ms", p, value)
	}
}

func calculatePercentile(data []float64, percentile float64) float64 {
	if len(data) == 0 {
		return 0
	}
	index := int((percentile / 100) * float64(len(data)-1))
	return data[index]
}

func BenchmarkOrderbook(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			benchmarkOrderbookLimitCreate(30*time.Second, b)
		}
	})
}

func benchmarkOrderbookLimitCreate(duration time.Duration, b *testing.B) {
	tok := uint64(1)
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
			TrigPrice: decimal.Zero,
		}
	}

	b.ReportAllocs()

	var latencies []float64

	stopwatch := time.Now()
	endTime := stopwatch.Add(duration)

	b.ResetTimer()

	for time.Now().Before(endTime) {
		start := time.Now()

		order := orders[rand.Intn(999_999)]
		ob.AddOrder(tok, uint64(tok), order.Class, order.Side, order.Price, order.Qty, order.TrigPrice, order.Flag)

		elapsed := time.Since(start).Nanoseconds()
		latencies = append(latencies, float64(elapsed))

		tok++
	}

	b.StopTimer()

	printResultsWithPercentiles(b, "Add Order Latencies", latencies)
}

func getOrderBook() *OrderBook {
	on := &EmptyNotification{}
	ob := NewOrderBook(on)

	runtime.GC()
	runtime.Gosched()
	return ob
}

func getInitialVars(lowerBound, upperBound, minSpread decimal.Decimal) (bid decimal.Decimal, ask decimal.Decimal, bidQty decimal.Decimal, askQty decimal.Decimal) {
	bid = lowerBound.Add(upperBound).Div(decimal.NewI(2, 0))
	ask = bid.Sub(minSpread)
	bidQty = decimal.NewI(10, 0)
	askQty = decimal.NewI(10, 0)

	return
}

func getPrice(bid, ask, diff decimal.Decimal, dec bool) (decimal.Decimal, decimal.Decimal) {
	if dec {
		bid = bid.Sub(diff)
		ask = ask.Sub(diff)
		return bid, ask
	}

	bid = bid.Add(diff)
	ask = ask.Add(diff)
	return bid, ask
}
