package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuantityScalingRoundTripsTwelveDecimalPlaces(t *testing.T) {
	config, err := newAdapterConfig(testConfig())
	require.NoError(t, err)

	quantity, err := config.quantity(number("0.000000000001"))
	require.NoError(t, err)
	assert.Equal(t, "0.00000001", quantity.String())

	formatted, err := config.formatQuantity(quantity)
	require.NoError(t, err)
	assert.Equal(t, "0.000000000001", formatted)
}

func TestQuantityNormalizationUsesHalfEvenRounding(t *testing.T) {
	even, err := quantizedUnits("2.5", 0)
	require.NoError(t, err)
	assert.Equal(t, "2", even.String())

	odd, err := quantizedUnits("3.5", 0)
	require.NoError(t, err)
	assert.Equal(t, "4", odd.String())

	exponent, err := quantizedUnits("125e-3", 3)
	require.NoError(t, err)
	assert.Equal(t, "125", exponent.String())
}

func TestCanonicalStateHashMatchesTracebook(t *testing.T) {
	state := bookState{Books: []bookSnapshot{{
		Symbol: "TEST",
		Bids: []restingOrder{{
			OrderID: 10, Price: "100", RemainingQuantity: "2", Owner: 7, OrderType: "LIMIT",
		}},
		Asks: []restingOrder{},
	}}}

	digest, err := state.digest()
	require.NoError(t, err)
	assert.Equal(t, "18de97d9cb98a7ca4dbc2215a748f7fded7607f05782a33338fcc6f6f0b91c62", digest)
}

func TestReducePreservesAndReplaceLosesFIFO(t *testing.T) {
	adapter, err := newAdapter(testConfig())
	require.NoError(t, err)

	applyEvent(t, adapter, marketEvent{
		Op: "new", Symbol: "TEST", OrderID: number("1"), Side: text("SELL"),
		OrderType: "LIMIT", Price: number("100"), Quantity: number("5"), Owner: 1,
	}, 1)
	applyEvent(t, adapter, marketEvent{
		Op: "new", Symbol: "TEST", OrderID: number("2"), Side: text("SELL"),
		OrderType: "LIMIT", Price: number("100"), Quantity: number("5"), Owner: 2,
	}, 2)
	applyEvent(t, adapter, marketEvent{
		Op: "reduce", Symbol: "TEST", OrderID: number("1"), Quantity: number("3"),
	}, 3)
	applyEvent(t, adapter, marketEvent{
		Op: "replace", Symbol: "TEST", OrderID: number("1"),
		Price: number("100"), Quantity: number("2"),
	}, 4)
	observation := applyEvent(t, adapter, marketEvent{
		Op: "new", Symbol: "TEST", OrderID: number("3"), Side: text("BUY"),
		OrderType: "LIMIT", Price: number("100"), Quantity: number("5"), Owner: 3,
	}, 5)

	require.Len(t, observation.Trades, 1)
	assert.Equal(t, uint64(3), observation.Trades[0].BuyOrderID)
	assert.Equal(t, uint64(2), observation.Trades[0].SellOrderID)
	assert.Equal(t, "5", observation.Trades[0].Quantity)

	state, err := adapter.snapshot()
	require.NoError(t, err)
	require.Len(t, state.Books, 1)
	require.Len(t, state.Books[0].Asks, 1)
	assert.Equal(t, uint64(1), state.Books[0].Asks[0].OrderID)
	assert.Equal(t, "2", state.Books[0].Asks[0].RemainingQuantity)
}

func TestRejectedEventDoesNotRepeatPreviousTrades(t *testing.T) {
	adapter, err := newAdapter(testConfig())
	require.NoError(t, err)
	applyEvent(t, adapter, marketEvent{
		Op: "new", Symbol: "TEST", OrderID: number("1"), Side: text("SELL"),
		OrderType: "LIMIT", Price: number("100"), Quantity: number("1"), Owner: 1,
	}, 1)
	applyEvent(t, adapter, marketEvent{
		Op: "new", Symbol: "TEST", OrderID: number("2"), Side: text("BUY"),
		OrderType: "LIMIT", Price: number("100"), Quantity: number("1"), Owner: 2,
	}, 2)
	rejected := applyEvent(t, adapter, marketEvent{
		Op: "new", Symbol: "TEST", OrderID: number("3"), Side: text("BUY"),
		OrderType: "LIMIT", Quantity: number("1"), Owner: 3,
	}, 3)

	assert.Equal(t, "rejected", rejected.Outcome.Status)
	assert.Empty(t, rejected.Trades)
}

func applyEvent(t *testing.T, adapter *adapter, event marketEvent, index uint64) observationFrame {
	t.Helper()
	observation, err := adapter.apply(event, index)
	require.NoError(t, err)
	return observation
}

func testConfig() configWire {
	return configWire{
		MatchingAlgorithm: "fifo", TickSize: "0.01",
		SelfTradePolicy: "NONE", QuantityDecimalPlaces: 12,
	}
}

func number(value string) *json.Number {
	number := json.Number(value)
	return &number
}

func text(value string) *string {
	return &value
}
