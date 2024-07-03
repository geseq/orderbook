package pool

import (
	"unsafe"
)

// template type ItemPool(PoolItem)
type PoolItem struct {
	next uint64 // for free list management
}

// A simple object pool.
type ItemPool struct {
	items       []PoolItem
	freeList    int // Head of the free list
	maxSize     uint64
	currentSize uint64
	poolStart   uintptr // Start address of the pool
	poolEnd     uintptr // End address of the pool
}

func NewItemPool(maxSize uint64) *ItemPool {
	items := make([]PoolItem, maxSize)
	p := &ItemPool{
		items:       items,
		freeList:    -1, // -1 indicates an empty free list
		maxSize:     maxSize,
		currentSize: 0,
		poolStart:   uintptr(unsafe.Pointer(&items[0])),
		poolEnd:     uintptr(unsafe.Pointer(&items[maxSize-1])),
	}
	return p
}

func (p *ItemPool) Get() *PoolItem {
	if p.IsEmpty() {
		// If pool is empty we use Go's GC instead of the pool
		return &PoolItem{}
	}

	// Get the item from the head of the free list
	pos := p.freeList
	p.freeList = int(p.items[pos].next)
	p.currentSize++

	return &p.items[pos]
}

func (p *ItemPool) Put(o *PoolItem) {
	if o == nil {
		return
	}

	// Check if the object is from our pool
	if p.isFromPool(o) {
		pos := p.getPositionInPool(o)
		// Add it to the head of the free list
		p.items[pos].next = uint64(p.freeList)
		p.freeList = pos
		p.currentSize--
	}
}

func (p *ItemPool) IsEmpty() bool {
	return p.freeList == -1
}

func (p *ItemPool) isFromPool(o *PoolItem) bool {
	ptr := uintptr(unsafe.Pointer(o))
	return ptr >= p.poolStart && ptr <= p.poolEnd
}

func (p *ItemPool) getPositionInPool(o *PoolItem) int {
	ptr := uintptr(unsafe.Pointer(o))
	return int((ptr - p.poolStart) / unsafe.Sizeof(PoolItem{}))
}
