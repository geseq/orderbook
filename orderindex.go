package orderbook

import "math/bits"

// fibHash is the 64-bit Fibonacci hashing multiplier (2^64 / golden ratio).
const fibHash = 0x9E3779B97F4A7C15

// orderIndexNode is a chained bucket entry mapping an order id to its *Order.
type orderIndexNode struct {
	key  uint64
	val  *Order
	next *orderIndexNode
}

// orderIndex is a chained hash map specialized for uint64 -> *Order keyed by
// order id. It uses Fibonacci (multiplicative) hashing to avoid modulo, and a
// node free-list to recycle removed nodes and avoid GC churn. This is the Go
// port of the C++ Fibonacci-hashed pooled-node map.
type orderIndex struct {
	buckets []*orderIndexNode
	mask    uint64
	shift   uint
	count   int
	free    *orderIndexNode
}

// nextPow2 rounds n up to the next power of two (minimum 1).
func nextPow2(n uint64) uint64 {
	if n < 2 {
		return 1
	}
	return uint64(1) << uint(bits.Len64(n-1))
}

// newOrderIndex creates an orderIndex sized to hold roughly hint entries before
// the first grow. The bucket count is a power of two.
func newOrderIndex(hint uint64) *orderIndex {
	// Size buckets so that hint entries stay under the 0.7 load factor.
	n := nextPow2(hint)
	if n < 2 {
		n = 2
	}
	oi := &orderIndex{}
	oi.setBuckets(make([]*orderIndexNode, n))
	return oi
}

func (oi *orderIndex) setBuckets(b []*orderIndexNode) {
	oi.buckets = b
	oi.mask = uint64(len(b) - 1)
	oi.shift = uint(64 - bits.TrailingZeros64(uint64(len(b))))
}

func (oi *orderIndex) index(key uint64) uint64 {
	return (key * fibHash) >> oi.shift
}

// get returns the *Order for key and whether it was present.
func (oi *orderIndex) get(key uint64) (*Order, bool) {
	for n := oi.buckets[oi.index(key)]; n != nil; n = n.next {
		if n.key == key {
			return n.val, true
		}
	}
	return nil, false
}

// put inserts or updates the value for key. It is safe to call for an existing
// key (the value is overwritten without inserting a duplicate node).
func (oi *orderIndex) put(key uint64, val *Order) {
	idx := oi.index(key)
	for n := oi.buckets[idx]; n != nil; n = n.next {
		if n.key == key {
			n.val = val
			return
		}
	}

	n := oi.free
	if n != nil {
		oi.free = n.next
	} else {
		n = &orderIndexNode{}
	}
	n.key = key
	n.val = val
	n.next = oi.buckets[idx]
	oi.buckets[idx] = n
	oi.count++

	// Grow at load factor ~0.7.
	if uint64(oi.count)*10 >= uint64(len(oi.buckets))*7 {
		oi.grow()
	}
}

// remove unlinks key from its bucket chain, recycles the node onto the
// free-list, and returns the removed value and whether it was present.
func (oi *orderIndex) remove(key uint64) (*Order, bool) {
	idx := oi.index(key)
	prev := (*orderIndexNode)(nil)
	for n := oi.buckets[idx]; n != nil; n = n.next {
		if n.key == key {
			if prev == nil {
				oi.buckets[idx] = n.next
			} else {
				prev.next = n.next
			}
			val := n.val
			n.val = nil
			n.next = oi.free
			oi.free = n
			oi.count--
			return val, true
		}
		prev = n
	}
	return nil, false
}

// grow doubles the bucket count and rehashes the existing nodes in place,
// reusing the node objects (no allocation per entry).
func (oi *orderIndex) grow() {
	old := oi.buckets
	oi.setBuckets(make([]*orderIndexNode, len(old)*2))
	for _, n := range old {
		for n != nil {
			next := n.next
			idx := oi.index(n.key)
			n.next = oi.buckets[idx]
			oi.buckets[idx] = n
			n = next
		}
	}
}
