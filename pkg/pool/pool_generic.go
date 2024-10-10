package pool

import (
	"runtime"
	"sync/atomic"

	"golang.org/x/sys/cpu"
)

// ItemPoolV2 is a generic item pool with the V2 suffix
type ItemPoolV2[T any] struct {
	ch *ItemChanV2[T]
}

// NewItemPoolV2 creates a new pool with a maximum size using generics and the V2 suffix
func NewItemPoolV2[T any](maxSize uint64) *ItemPoolV2[T] {
	ch := NewItemChanV2[T](maxSize)
	p := &ItemPoolV2[T]{
		ch: ch,
	}

	// Prepopulate the pool
	for !ch.IsFull() {
		//println("Here1")
		ch.Put(new(T))
	}

	return p
}

// Get retrieves an item from the pool, or creates a new one if the pool is empty
func (p *ItemPoolV2[T]) Get() *T {
	if p.ch.IsEmpty() {
		return new(T)
	}

	return p.ch.Read()
}

// Put returns an item back to the pool
func (p *ItemPoolV2[T]) Put(o *T) {
	if o == nil {
		return
	}

	// If the pool is full, let the garbage collector handle the item
	if p.ch.IsFull() {
		return
	}

	p.ch.Put(o)
}

// ItemChanV2 is a generic channel implementation with the V2 suffix
type ItemChanV2[T any] struct {
	_                  cpu.CacheLinePad
	indexMask          uint64
	_                  cpu.CacheLinePad
	lastCommittedIndex uint64
	_                  cpu.CacheLinePad
	nextFreeIndex      uint64
	_                  cpu.CacheLinePad
	readerIndex        uint64
	_                  cpu.CacheLinePad
	contents           []*T
	_                  cpu.CacheLinePad
}

// NewItemChanV2 creates a new generic channel with the V2 suffix
func NewItemChanV2[T any](size uint64) *ItemChanV2[T] {
	size = roundUpNextPowerOfTwoItemChanV2(size)
	return &ItemChanV2[T]{
		lastCommittedIndex: 0,
		nextFreeIndex:      0,
		readerIndex:        0,
		indexMask:          size - 1,
		contents:           make([]*T, size),
	}
}

// Put inserts an item into the channel at the next free position
func (c *ItemChanV2[T]) Put(value *T) {
	// Wait for the reader to catch up if the channel is full
	for atomic.LoadUint64(&c.nextFreeIndex)+1 > (atomic.LoadUint64(&c.readerIndex) + c.indexMask) {
		println("Here2")
		runtime.Gosched()
	}

	// Add the item to the next available slot
	var myIndex = atomic.AddUint64(&c.nextFreeIndex, 1)
	c.contents[myIndex&c.indexMask] = value

	// Update the last committed index to make the item available for reading
	for !atomic.CompareAndSwapUint64(&c.lastCommittedIndex, myIndex-1, myIndex) {
		println("Here3")
		runtime.Gosched()
	}
}

// Read removes and returns an item from the channel
func (c *ItemChanV2[T]) Read() *T {
	// Wait for a committed item if the reader has outpaced the writer
	for atomic.LoadUint64(&c.readerIndex)+1 > atomic.LoadUint64(&c.lastCommittedIndex) {
		println("Here4")
		runtime.Gosched()
	}

	// Retrieve the item from the next slot
	var myIndex = atomic.AddUint64(&c.readerIndex, 1)
	return c.contents[myIndex&c.indexMask]
}

// Empty resets the channel
func (c *ItemChanV2[T]) Empty() {
	c.lastCommittedIndex = 0
	c.nextFreeIndex = 0
	c.readerIndex = 0
}

// Size returns the number of items in the channel
func (c *ItemChanV2[T]) Size() uint64 {
	return atomic.LoadUint64(&c.lastCommittedIndex) - atomic.LoadUint64(&c.readerIndex)
}

// IsEmpty checks whether the channel is empty
func (c *ItemChanV2[T]) IsEmpty() bool {
	return atomic.LoadUint64(&c.readerIndex) >= atomic.LoadUint64(&c.lastCommittedIndex)
}

// IsFull checks whether the channel is full
func (c *ItemChanV2[T]) IsFull() bool {
	return atomic.LoadUint64(&c.nextFreeIndex) >= (atomic.LoadUint64(&c.readerIndex) + c.indexMask)
}

// roundUpNextPowerOfTwoItemChanV2 rounds up a number to the next power of two
func roundUpNextPowerOfTwoItemChanV2(v uint64) uint64 {
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v |= v >> 32
	v++
	return v
}
