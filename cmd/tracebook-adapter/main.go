// Command tracebook-adapter exposes geseq/orderbook over Tracebook's NDJSON
// conformance protocol. Stdout is reserved for protocol frames.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"sort"
	"strconv"
	"strings"

	orderbook "github.com/geseq/orderbook"
	decimal "github.com/geseq/udecimal"
)

const (
	protocolName    = "tracebook.conformance"
	protocolVersion = 1
	enginePlaces    = 8
)

var engineVersion = "development"

type configWire struct {
	MatchingAlgorithm     string `json:"matching_algorithm"`
	TickSize              string `json:"tick_size"`
	SelfTradePolicy       string `json:"self_trade_policy"`
	QuantityDecimalPlaces int    `json:"quantity_decimal_places"`
}

type helloFrame struct {
	Type            string     `json:"type"`
	Protocol        string     `json:"protocol"`
	ProtocolVersion int        `json:"protocol_version"`
	Config          configWire `json:"config"`
}

type marketEvent struct {
	Op          string       `json:"op"`
	Symbol      string       `json:"symbol"`
	OrderID     *json.Number `json:"order_id"`
	Side        *string      `json:"side"`
	OrderType   string       `json:"order_type"`
	Price       *json.Number `json:"price"`
	Quantity    *json.Number `json:"quantity"`
	Owner       int64        `json:"owner"`
	TimestampNS *uint64      `json:"timestamp_ns"`
}

type eventFrame struct {
	Type  string      `json:"type"`
	Index uint64      `json:"index"`
	Event marketEvent `json:"event"`
}

type indexedFrame struct {
	Type  string `json:"type"`
	Index uint64 `json:"index"`
}

type finishFrame struct {
	Type       string `json:"type"`
	EventCount uint64 `json:"event_count"`
}

type engineMetadata struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Language string `json:"language"`
}

type readyFrame struct {
	Type            string         `json:"type"`
	Protocol        string         `json:"protocol"`
	ProtocolVersion int            `json:"protocol_version"`
	Engine          engineMetadata `json:"engine"`
}

type outcome struct {
	Status  string  `json:"status"`
	Reason  *string `json:"reason"`
	Message *string `json:"message"`
}

type tradeFill struct {
	Symbol      string `json:"symbol"`
	BuyOrderID  uint64 `json:"buy_order_id"`
	SellOrderID uint64 `json:"sell_order_id"`
	Price       string `json:"price"`
	Quantity    string `json:"quantity"`
}

type observationFrame struct {
	Type              string      `json:"type"`
	Index             uint64      `json:"index"`
	Outcome           outcome     `json:"outcome"`
	Trades            []tradeFill `json:"trades"`
	StateHash         string      `json:"state_hash"`
	RestingOrderCount int         `json:"resting_order_count"`
}

type restingOrder struct {
	OrderID           uint64 `json:"order_id"`
	Price             string `json:"price"`
	RemainingQuantity string `json:"remaining_quantity"`
	Owner             int64  `json:"owner"`
	OrderType         string `json:"order_type"`
}

type bookSnapshot struct {
	Symbol string         `json:"symbol"`
	Bids   []restingOrder `json:"bids"`
	Asks   []restingOrder `json:"asks"`
}

type bookState struct {
	Books []bookSnapshot `json:"books"`
}

type snapshotFrame struct {
	Type  string    `json:"type"`
	Index uint64    `json:"index"`
	State bookState `json:"state"`
}

type completeFrame struct {
	Type       string `json:"type"`
	EventCount uint64 `json:"event_count"`
}

type errorFrame struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func applied() outcome {
	return outcome{Status: "applied"}
}

func rejected(code, detail string) outcome {
	return outcome{Status: "rejected", Reason: &code, Message: &detail}
}

type nativeTrade struct {
	makerID   uint64
	takerID   uint64
	quantity  decimal.Decimal
	price     decimal.Decimal
	takerSide orderbook.SideType
}

type orderNotice struct {
	message orderbook.MsgType
	status  orderbook.OrderStatus
	err     error
}

type bookHarness struct {
	book      *orderbook.OrderBook
	token     uint64
	owners    map[uint64]int64
	trades    []nativeTrade
	notices   []orderNotice
	takerSide orderbook.SideType
}

func newBookHarness() *bookHarness {
	harness := &bookHarness{owners: make(map[uint64]int64)}
	harness.book = orderbook.NewOrderBook(harness)
	return harness
}

func (h *bookHarness) PutOrder(
	message orderbook.MsgType,
	status orderbook.OrderStatus,
	_ uint64,
	_ decimal.Decimal,
	err error,
) {
	h.notices = append(h.notices, orderNotice{message: message, status: status, err: err})
}

func (h *bookHarness) PutTrade(
	makerID, takerID uint64,
	_, _ orderbook.OrderStatus,
	quantity, price decimal.Decimal,
) {
	h.trades = append(h.trades, nativeTrade{
		makerID: makerID, takerID: takerID, quantity: quantity,
		price: price, takerSide: h.takerSide,
	})
}

func (h *bookHarness) begin(side orderbook.SideType) {
	h.trades = h.trades[:0]
	h.notices = h.notices[:0]
	h.takerSide = side
}

func (h *bookHarness) nextToken() uint64 {
	h.token++
	return h.token
}

func (h *bookHarness) reconcileOwners() {
	active := make(map[uint64]struct{})
	snapshot := h.book.Snapshot()
	for _, order := range snapshot.Bids {
		active[order.ID] = struct{}{}
	}
	for _, order := range snapshot.Asks {
		active[order.ID] = struct{}{}
	}
	for orderID := range h.owners {
		if _, ok := active[orderID]; !ok {
			delete(h.owners, orderID)
		}
	}
}

type adapterConfig struct {
	tickSize              float64
	tickPlaces            int
	quantityDecimalPlaces int
}

func newAdapterConfig(wire configWire) (adapterConfig, error) {
	if wire.MatchingAlgorithm != "fifo" {
		return adapterConfig{}, errors.New("matching_algorithm must be 'fifo'")
	}
	if wire.SelfTradePolicy != "NONE" {
		return adapterConfig{}, errors.New("self_trade_policy must be NONE")
	}
	if wire.QuantityDecimalPlaces < 0 || wire.QuantityDecimalPlaces > 18 {
		return adapterConfig{}, errors.New("quantity_decimal_places must be between 0 and 18")
	}
	tickSize, err := strconv.ParseFloat(wire.TickSize, 64)
	if err != nil || math.IsNaN(tickSize) || math.IsInf(tickSize, 0) || tickSize <= 0 {
		return adapterConfig{}, errors.New("tick_size must be a positive finite decimal")
	}
	tickPlaces, err := decimalPlaces(wire.TickSize)
	if err != nil || tickPlaces > enginePlaces {
		return adapterConfig{}, fmt.Errorf("tick_size exceeds the engine's %d-decimal price precision", enginePlaces)
	}
	return adapterConfig{
		tickSize: tickSize, tickPlaces: tickPlaces,
		quantityDecimalPlaces: wire.QuantityDecimalPlaces,
	}, nil
}

func (c adapterConfig) price(value *json.Number) (decimal.Decimal, error) {
	if value == nil {
		return decimal.Zero, errors.New("price is required")
	}
	parsed, err := strconv.ParseFloat(value.String(), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return decimal.Zero, errors.New("price must be finite")
	}
	ticks := math.RoundToEven(parsed / c.tickSize)
	if ticks <= 0 || math.IsInf(ticks, 0) {
		return decimal.Zero, errors.New("price snaps to a non-positive or unsupported tick")
	}
	snapped := strconv.FormatFloat(ticks*c.tickSize, 'f', c.tickPlaces, 64)
	price, err := decimal.Parse(snapped)
	if err != nil || price.IsZero() {
		return decimal.Zero, errors.New("price exceeds the engine's supported range")
	}
	return price, nil
}

func (c adapterConfig) quantity(value *json.Number) (decimal.Decimal, error) {
	if value == nil {
		return decimal.Zero, errors.New("quantity is required")
	}
	units, err := quantizedUnits(value.String(), c.quantityDecimalPlaces)
	if err != nil || units.Sign() <= 0 {
		return decimal.Zero, errors.New("quantity must be a positive finite decimal")
	}
	if c.quantityDecimalPlaces < enginePlaces {
		units.Mul(units, pow10(enginePlaces-c.quantityDecimalPlaces))
	}
	quantity, err := decimal.Parse(formatFixed(units, enginePlaces))
	if err != nil || quantity.IsZero() {
		return decimal.Zero, errors.New("quantity exceeds the engine's supported range")
	}
	return quantity, nil
}

func (c adapterConfig) formatQuantity(value decimal.Decimal) (string, error) {
	units, err := quantizedUnits(value.String(), enginePlaces)
	if err != nil {
		return "", err
	}
	if c.quantityDecimalPlaces < enginePlaces {
		divisor := pow10(enginePlaces - c.quantityDecimalPlaces)
		quotient, remainder := new(big.Int), new(big.Int)
		quotient.QuoRem(units, divisor, remainder)
		if remainder.Sign() != 0 {
			return "", errors.New("engine quantity cannot be represented at configured precision")
		}
		units = quotient
	}
	return canonicalFixed(units, c.quantityDecimalPlaces), nil
}

type adapter struct {
	config adapterConfig
	books  map[string]*bookHarness
}

func newAdapter(config configWire) (*adapter, error) {
	parsed, err := newAdapterConfig(config)
	if err != nil {
		return nil, err
	}
	return &adapter{config: parsed, books: make(map[string]*bookHarness)}, nil
}

func (a *adapter) apply(event marketEvent, index uint64) (observationFrame, error) {
	var result outcome
	if event.Op == "clear" {
		a.books[event.Symbol] = newBookHarness()
		result = applied()
	} else if _, ok := a.books[event.Symbol]; !ok && isLifecycle(event.Op) {
		result = rejected("ORDER_NOT_ACTIVE", "order is not active")
	} else {
		harness := a.books[event.Symbol]
		if harness == nil {
			harness = newBookHarness()
			a.books[event.Symbol] = harness
		}
		var err error
		result, err = a.applyToBook(harness, event)
		if err != nil {
			return observationFrame{}, err
		}
		harness.reconcileOwners()
	}

	trades, err := a.tradeFills(event.Symbol)
	if err != nil {
		return observationFrame{}, err
	}
	state, err := a.snapshot()
	if err != nil {
		return observationFrame{}, err
	}
	digest, err := state.digest()
	if err != nil {
		return observationFrame{}, err
	}
	return observationFrame{
		Type: "observation", Index: index, Outcome: result, Trades: trades,
		StateHash: digest, RestingOrderCount: state.orderCount(),
	}, nil
}

func isLifecycle(operation string) bool {
	return operation == "cancel" || operation == "reduce" || operation == "replace"
}

func (a *adapter) applyToBook(h *bookHarness, event marketEvent) (outcome, error) {
	// Every source event owns an independent notification window, including
	// events rejected by adapter-side validation before the engine is called.
	h.begin(orderbook.Buy)
	switch event.Op {
	case "new":
		return a.applyNew(h, event)
	case "cancel":
		return a.applyCancel(h, event)
	case "reduce":
		return a.applyReduce(h, event)
	case "replace":
		return a.applyReplace(h, event)
	default:
		return rejected("INVALID_ORDER", "unsupported event operation"), nil
	}
}

func (a *adapter) applyNew(h *bookHarness, event marketEvent) (outcome, error) {
	orderID, err := requiredOrderID(event.OrderID)
	if err != nil {
		return rejected("INVALID_ORDER", err.Error()), nil
	}
	if event.OrderType != "LIMIT" {
		return rejected("INVALID_ORDER", "fifo-limit-v1 accepts LIMIT orders only"), nil
	}
	side, err := parseSide(event.Side)
	if err != nil {
		return rejected("INVALID_ORDER", err.Error()), nil
	}
	price, err := a.config.price(event.Price)
	if err != nil {
		return rejected("INVALID_ORDER", err.Error()), nil
	}
	quantity, err := a.config.quantity(event.Quantity)
	if err != nil {
		return rejected("INVALID_ORDER", err.Error()), nil
	}

	h.begin(side)
	h.book.AddOrder(h.nextToken(), orderID, orderbook.Limit, side, quantity, price, decimal.Zero, orderbook.None)
	if notice := lastNotice(h); notice != nil && notice.status == orderbook.Rejected {
		if errors.Is(notice.err, orderbook.ErrOrderExists) {
			return rejected("DUPLICATE_ORDER_ID", "order is already active"), nil
		}
		return rejected("INVALID_ORDER", notice.err.Error()), nil
	}
	h.owners[orderID] = event.Owner
	return applied(), nil
}

func (a *adapter) applyCancel(h *bookHarness, event marketEvent) (outcome, error) {
	orderID, err := requiredOrderID(event.OrderID)
	if err != nil {
		return rejected("INVALID_CANCEL", err.Error()), nil
	}
	if h.book.Order(orderID) == nil {
		return rejected("ORDER_NOT_ACTIVE", "order is not active"), nil
	}
	h.begin(orderbook.Buy)
	h.book.CancelOrder(h.nextToken(), orderID)
	delete(h.owners, orderID)
	return applied(), nil
}

func (a *adapter) applyReduce(h *bookHarness, event marketEvent) (outcome, error) {
	orderID, err := requiredOrderID(event.OrderID)
	if err != nil {
		return rejected("INVALID_ORDER", err.Error()), nil
	}
	existing := h.book.Order(orderID)
	if existing == nil {
		return rejected("ORDER_NOT_ACTIVE", "order is not active"), nil
	}
	reduction, err := a.config.quantity(event.Quantity)
	if err != nil {
		return rejected("INVALID_ORDER", err.Error()), nil
	}
	if reduction.GreaterThan(existing.Qty) {
		return rejected("INVALID_ORDER", "reduction quantity cannot exceed remaining quantity"), nil
	}
	h.begin(existing.Side)
	if reduction.Equal(existing.Qty) {
		h.book.CancelOrder(h.nextToken(), orderID)
		delete(h.owners, orderID)
		return applied(), nil
	}
	h.book.AmendOrder(h.nextToken(), orderID, existing.Qty.Sub(reduction), existing.Price)
	if notice := lastNotice(h); notice == nil || notice.message != orderbook.MsgAmendOrder || notice.status != orderbook.Accepted {
		return outcome{}, errors.New("engine rejected a prevalidated quantity reduction")
	}
	return applied(), nil
}

func (a *adapter) applyReplace(h *bookHarness, event marketEvent) (outcome, error) {
	orderID, err := requiredOrderID(event.OrderID)
	if err != nil {
		return rejected("INVALID_REPLACEMENT", err.Error()), nil
	}
	existing := h.book.Order(orderID)
	if existing == nil {
		return rejected("ORDER_NOT_ACTIVE", "order is not active"), nil
	}
	price := existing.Price
	if event.Price != nil {
		price, err = a.config.price(event.Price)
		if err != nil {
			return rejected("INVALID_REPLACEMENT", err.Error()), nil
		}
	}
	quantity := existing.Qty
	if event.Quantity != nil {
		quantity, err = a.config.quantity(event.Quantity)
		if err != nil {
			return rejected("INVALID_REPLACEMENT", err.Error()), nil
		}
	}
	side := existing.Side
	owner := h.owners[orderID]

	h.begin(side)
	h.book.CancelOrder(h.nextToken(), orderID)
	h.notices = h.notices[:0]
	h.book.AddOrder(h.nextToken(), orderID, orderbook.Limit, side, quantity, price, decimal.Zero, orderbook.None)
	if notice := lastNotice(h); notice == nil || notice.status != orderbook.Accepted {
		return outcome{}, errors.New("engine rejected a prevalidated cancel-and-submit replacement")
	}
	h.owners[orderID] = owner
	return applied(), nil
}

func lastNotice(h *bookHarness) *orderNotice {
	if len(h.notices) == 0 {
		return nil
	}
	return &h.notices[len(h.notices)-1]
}

func (a *adapter) tradeFills(symbol string) ([]tradeFill, error) {
	harness := a.books[symbol]
	if harness == nil || len(harness.trades) == 0 {
		return []tradeFill{}, nil
	}
	trades := make([]tradeFill, 0, len(harness.trades))
	for _, trade := range harness.trades {
		quantity, err := a.config.formatQuantity(trade.quantity)
		if err != nil {
			return nil, err
		}
		fill := tradeFill{Symbol: symbol, Price: trade.price.String(), Quantity: quantity}
		if trade.takerSide == orderbook.Buy {
			fill.BuyOrderID, fill.SellOrderID = trade.takerID, trade.makerID
		} else {
			fill.BuyOrderID, fill.SellOrderID = trade.makerID, trade.takerID
		}
		trades = append(trades, fill)
	}
	return trades, nil
}

func (a *adapter) snapshot() (bookState, error) {
	symbols := make([]string, 0, len(a.books))
	for symbol := range a.books {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	state := bookState{Books: make([]bookSnapshot, 0, len(symbols))}
	for _, symbol := range symbols {
		harness := a.books[symbol]
		native := harness.book.Snapshot()
		bids, err := a.restingOrders(native.Bids, harness.owners)
		if err != nil {
			return bookState{}, err
		}
		asks, err := a.restingOrders(native.Asks, harness.owners)
		if err != nil {
			return bookState{}, err
		}
		state.Books = append(state.Books, bookSnapshot{Symbol: symbol, Bids: bids, Asks: asks})
	}
	return state, nil
}

func (a *adapter) restingOrders(native []orderbook.RestingOrder, owners map[uint64]int64) ([]restingOrder, error) {
	orders := make([]restingOrder, 0, len(native))
	for _, order := range native {
		owner, ok := owners[order.ID]
		if !ok {
			return nil, fmt.Errorf("adapter has no source owner for resting order %d", order.ID)
		}
		quantity, err := a.config.formatQuantity(order.Qty)
		if err != nil {
			return nil, err
		}
		orders = append(orders, restingOrder{
			OrderID: order.ID, Price: order.Price.String(), RemainingQuantity: quantity,
			Owner: owner, OrderType: "LIMIT",
		})
	}
	return orders, nil
}

func (s bookState) orderCount() int {
	total := 0
	for _, book := range s.Books {
		total += len(book.Bids) + len(book.Asks)
	}
	return total
}

func (s bookState) digest() (string, error) {
	books := make([]any, 0, len(s.Books))
	for _, book := range s.Books {
		bids := make([]any, 0, len(book.Bids))
		for _, order := range book.Bids {
			bids = append(bids, canonicalOrder(order))
		}
		asks := make([]any, 0, len(book.Asks))
		for _, order := range book.Asks {
			asks = append(asks, canonicalOrder(order))
		}
		books = append(books, map[string]any{"asks": asks, "bids": bids, "symbol": book.Symbol})
	}
	payload, err := compactJSON(map[string]any{"books": books})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func canonicalOrder(order restingOrder) map[string]any {
	return map[string]any{
		"order_id": order.OrderID, "order_type": order.OrderType, "owner": order.Owner,
		"price": order.Price, "remaining_quantity": order.RemainingQuantity,
	}
}

func compactJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

func requiredOrderID(value *json.Number) (uint64, error) {
	if value == nil {
		return 0, errors.New("order_id is required")
	}
	orderID, err := strconv.ParseUint(value.String(), 10, 64)
	if err != nil || orderID == 0 {
		return 0, errors.New("order_id must be a positive integer")
	}
	return orderID, nil
}

func parseSide(value *string) (orderbook.SideType, error) {
	if value == nil {
		return orderbook.Sell, errors.New("side is required")
	}
	switch *value {
	case "BUY":
		return orderbook.Buy, nil
	case "SELL":
		return orderbook.Sell, nil
	default:
		return orderbook.Sell, errors.New("side must be BUY or SELL")
	}
}

func decimalPlaces(value string) (int, error) {
	_, scale, err := decimalParts(value)
	if err != nil {
		return 0, err
	}
	if scale < 0 {
		return 0, nil
	}
	return scale, nil
}

func quantizedUnits(value string, places int) (*big.Int, error) {
	coefficient, scale, err := decimalParts(value)
	if err != nil {
		return nil, err
	}
	if scale <= places {
		return coefficient.Mul(coefficient, pow10(places-scale)), nil
	}

	divisor := pow10(scale - places)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(coefficient, divisor, remainder)
	twiceRemainder := new(big.Int).Lsh(new(big.Int).Set(remainder), 1)
	comparison := twiceRemainder.Cmp(divisor)
	if comparison > 0 || (comparison == 0 && quotient.Bit(0) == 1) {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient, nil
}

func decimalParts(value string) (*big.Int, int, error) {
	text := strings.TrimSpace(value)
	if text == "" || strings.HasPrefix(text, "-") {
		return nil, 0, errors.New("decimal must be non-negative")
	}
	text = strings.TrimPrefix(text, "+")
	exponent := 0
	if position := strings.IndexAny(text, "eE"); position >= 0 {
		parsed, err := strconv.Atoi(text[position+1:])
		if err != nil || parsed < -100 || parsed > 100 {
			return nil, 0, errors.New("invalid decimal exponent")
		}
		exponent = parsed
		text = text[:position]
	}
	parts := strings.Split(text, ".")
	if len(parts) > 2 || (len(parts) == 1 && parts[0] == "") {
		return nil, 0, errors.New("invalid decimal")
	}
	integer := parts[0]
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	digits := integer + fraction
	if digits == "" {
		return nil, 0, errors.New("invalid decimal")
	}
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return nil, 0, errors.New("invalid decimal")
		}
	}
	coefficient, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return nil, 0, errors.New("invalid decimal")
	}
	scale := len(fraction) - exponent
	if scale < 0 {
		coefficient.Mul(coefficient, pow10(-scale))
		scale = 0
	}
	return coefficient, scale, nil
}

func pow10(power int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(power)), nil)
}

func formatFixed(units *big.Int, places int) string {
	digits := units.String()
	if places == 0 {
		return digits
	}
	if len(digits) <= places {
		digits = strings.Repeat("0", places-len(digits)+1) + digits
	}
	position := len(digits) - places
	return digits[:position] + "." + digits[position:]
}

func canonicalFixed(units *big.Int, places int) string {
	formatted := formatFixed(units, places)
	if strings.Contains(formatted, ".") {
		formatted = strings.TrimRight(formatted, "0")
		formatted = strings.TrimRight(formatted, ".")
	}
	if formatted == "" || formatted == "-0" {
		return "0"
	}
	return formatted
}

func decodeFrame(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("protocol frame contains trailing JSON")
		}
		return err
	}
	return nil
}

func writeFrame(output *bufio.Writer, frame any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(frame); err != nil {
		return err
	}
	return output.Flush()
}

func runProtocol(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	writer := bufio.NewWriter(output)
	if !scanner.Scan() {
		return errors.New("protocol ended before hello")
	}
	var hello helloFrame
	if err := decodeFrame(scanner.Bytes(), &hello); err != nil {
		return fmt.Errorf("invalid hello frame: %w", err)
	}
	if hello.Type != "hello" || hello.Protocol != protocolName || hello.ProtocolVersion != protocolVersion {
		return errors.New("unsupported conformance protocol handshake")
	}
	candidate, err := newAdapter(hello.Config)
	if err != nil {
		return err
	}
	if err := writeFrame(writer, readyFrame{
		Type: "ready", Protocol: protocolName, ProtocolVersion: protocolVersion,
		Engine: engineMetadata{Name: "geseq/orderbook", Version: engineVersion, Language: "Go"},
	}); err != nil {
		return err
	}

	var lastIndex uint64
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		var frameType struct {
			Type string `json:"type"`
		}
		if err := decodeFrame(line, &frameType); err != nil {
			return fmt.Errorf("invalid protocol frame: %w", err)
		}
		switch frameType.Type {
		case "event":
			var frame eventFrame
			if err := decodeFrame(line, &frame); err != nil {
				return err
			}
			if frame.Index != lastIndex+1 {
				return errors.New("event indices must be contiguous")
			}
			observation, err := candidate.apply(frame.Event, frame.Index)
			if err != nil {
				return err
			}
			if err := writeFrame(writer, observation); err != nil {
				return err
			}
			lastIndex = frame.Index
		case "snapshot":
			var frame indexedFrame
			if err := decodeFrame(line, &frame); err != nil {
				return err
			}
			if frame.Index != lastIndex {
				return errors.New("snapshot index does not match the last event")
			}
			state, err := candidate.snapshot()
			if err != nil {
				return err
			}
			if err := writeFrame(writer, snapshotFrame{Type: "snapshot", Index: frame.Index, State: state}); err != nil {
				return err
			}
		case "finish":
			var frame finishFrame
			if err := decodeFrame(line, &frame); err != nil {
				return err
			}
			if frame.EventCount != lastIndex {
				return errors.New("finish event_count does not match the last event")
			}
			return writeFrame(writer, completeFrame{Type: "complete", EventCount: lastIndex})
		default:
			return fmt.Errorf("unsupported protocol frame %q", frameType.Type)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return errors.New("protocol ended before finish")
}

func main() {
	if err := runProtocol(os.Stdin, os.Stdout); err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(errorFrame{Type: "error", Code: "PROTOCOL_ERROR", Message: err.Error()})
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
