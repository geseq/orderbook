package tree

import (
	"fmt"

	"github.com/geseq/orderbook/pkg/pool"
	"github.com/geseq/udecimal"
)

type colorTree bool

type Number interface {
	int | int64 | uint64 | udecimal.Decimal
}

type comparatorTree[K Number] func(a, b K) int

const (
	blackTree, redTree colorTree = true, false
)

// Tree holds elements of the red-black tree
type Tree[K Number, V any] struct {
	Root       *nodeTree[K, V]
	size       int
	Comparator comparatorTree[K]
	Min        *nodeTree[K, V]
	Max        *nodeTree[K, V]
	Pool       pool.PoolInterface[nodeTree[K, V]]
}

// template type Tree(KeyType, ValueType)
// Node is a single element within the tree
type nodeTree[K Number, V any] struct {
	Key    K
	Value  V
	color  colorTree
	Left   *nodeTree[K, V]
	Right  *nodeTree[K, V]
	Parent *nodeTree[K, V]
}

// NewWith instantiates a red-black tree with the custom comparator.
func NewWithTree[K Number, V any](comparator comparatorTree[K], maxSize uint64) *Tree[K, V] {
	return &Tree[K, V]{Comparator: comparator, Pool: pool.NewItemPoolV2[nodeTree[K, V]](maxSize)}
}

func newNodeTree[K Number, V any](key K, value V, color colorTree, local_pool pool.PoolInterface[nodeTree[K, V]]) *nodeTree[K, V] {
	nt := local_pool.Get()
	nt.Key = key
	nt.Value = value
	nt.color = color
	nt.Left = nil
	nt.Right = nil
	nt.Parent = nil

	return nt
}

func (n *nodeTree[K, V]) Release(local_pool pool.PoolInterface[nodeTree[K, V]]) {
	local_pool.Put(n)
}

// Put inserts node into the tree.
// Key should adhere to the comparator's type assertion, otherwise method panics.
func (tree *Tree[K, V]) Put(key K, value V) {
	var insertedNode *nodeTree[K, V]
	if tree.Root == nil {
		// Assert key is of comparator's type for initial tree
		tree.Comparator(key, key)
		tree.Root = newNodeTree(key, value, redTree, tree.Pool)
		insertedNode = tree.Root
		tree.Min = tree.Root
		tree.Max = tree.Root
	} else {
		node := tree.Root
		loop := true
		for loop {
			compare := tree.Comparator(key, node.Key)
			switch {
			case compare == 0:
				node.Key = key
				node.Value = value
				return
			case compare < 0:
				if node.Left == nil {
					node.Left = newNodeTree(key, value, redTree, tree.Pool)
					insertedNode = node.Left
					loop = false
				} else {
					node = node.Left
				}
			case compare > 0:
				if node.Right == nil {
					node.Right = newNodeTree(key, value, redTree, tree.Pool)
					insertedNode = node.Right
					loop = false
				} else {
					node = node.Right
				}
			}
		}
		insertedNode.Parent = node
	}
	tree.insertCase1(insertedNode)

	compare := tree.Comparator(insertedNode.Key, tree.Min.Key)
	if compare < 0 {
		tree.Min = insertedNode
	}
	compare = tree.Comparator(insertedNode.Key, tree.Max.Key)
	if compare > 0 {
		tree.Max = insertedNode
	}
	tree.size++
}

// Get searches the node in the tree by key and returns its value or nil if key is not found in tree.
// Second return parameter is true if key was found, otherwise false.
// Key should adhere to the comparator's type assertion, otherwise method panics.
func (tree *Tree[K, V]) Get(key K) (value V, found bool) {
	node := tree.lookup(key)
	if node != nil {
		return node.Value, true
	}
	var zeroValue V
	return zeroValue, false
}

// Remove remove the node from the tree by key.
// Key should adhere to the comparator's type assertion, otherwise method panics.
func (tree *Tree[K, V]) Remove(key K) {
	var child *nodeTree[K, V]
	node := tree.lookup(key)
	if node == nil {
		return
	}
	if node.Left != nil && node.Right != nil {
		pred := node.Left.maximumNode()
		node.Key = pred.Key
		node.Value = pred.Value
		node = pred
	}
	if node.Left == nil || node.Right == nil {
		if node.Right == nil {
			child = node.Left
		} else {
			child = node.Right
		}
		if node.color == blackTree {
			node.color = nodeColorTree(child)
			tree.deleteCase1(node)
		}
		tree.replaceNode(node, child)
		if node.Parent == nil && child != nil {
			child.color = blackTree
		}
	}
	if node == tree.Max {
		if node.Parent != nil {
			tree.Max, _ = tree.getMaxFromNode(node.Parent)
		} else {
			tree.Max, _ = tree.getMaxFromNode(tree.Root)
		}
	}
	if node == tree.Min {
		if node.Parent != nil {
			tree.Min, _ = tree.getMinFromNode(node.Parent)
		} else {
			tree.Min, _ = tree.getMinFromNode(tree.Root)
		}
	}

	node.Release(tree.Pool)
	tree.size--
}

// Empty returns true if tree does not contain any nodes
func (tree *Tree[K, V]) Empty() bool {
	return tree.size == 0
}

// Size returns number of nodes in the tree.
func (tree *Tree[K, V]) Size() int {
	return tree.size
}

// Keys returns all keys in-order
func (tree *Tree[K, V]) Keys() []K {
	keys := make([]K, tree.size)
	it := tree.Iterator()
	for i := 0; it.Next(); i++ {
		keys[i] = it.Key()
	}
	return keys
}

// Values returns all values in-order based on the key.
func (tree *Tree[K, V]) Values() []V {
	values := make([]V, tree.size)
	it := tree.Iterator()
	for i := 0; it.Next(); i++ {
		values[i] = it.Value()
	}
	return values
}

// Left returns the left-most (min) node or nil if tree is empty.
func (tree *Tree[K, V]) Left() *nodeTree[K, V] {
	var parent *nodeTree[K, V]
	current := tree.Root
	for current != nil {
		parent = current
		current = current.Left
	}
	return parent
}

// Right returns the right-most (max) node or nil if tree is empty.
func (tree *Tree[K, V]) Right() *nodeTree[K, V] {
	var parent *nodeTree[K, V]
	current := tree.Root
	for current != nil {
		parent = current
		current = current.Right
	}
	return parent
}

// Floor Finds floor node of the input key, return the floor node or nil if no floor is found.
// Second return parameter is true if floor was found, otherwise false.
//
// Floor node is defined as the largest node that is smaller than or equal to the given node.
// A floor node may not be found, either because the tree is empty, or because
// all nodes in the tree are larger than the given node.
//
// Key should adhere to the comparator's type assertion, otherwise method panics.
func (tree *Tree[K, V]) Floor(key K) (floor *nodeTree[K, V], found bool) {
	found = false
	node := tree.Root
	for node != nil {
		compare := tree.Comparator(key, node.Key)
		switch {
		case compare == 0:
			return node, true
		case compare < 0:
			node = node.Left
		case compare > 0:
			floor, found = node, true
			node = node.Right
		}
	}
	if found {
		return floor, true
	}
	return nil, false
}

// Ceiling finds ceiling node of the input key, return the ceiling node or nil if no ceiling is found.
// Second return parameter is true if ceiling was found, otherwise false.
//
// Ceiling node is defined as the smallest node that is larger than or equal to the given node.
// A ceiling node may not be found, either because the tree is empty, or because
// all nodes in the tree are smaller than the given node.
//
// Key should adhere to the comparator's type assertion, otherwise method panics.
func (tree *Tree[K, V]) Ceiling(key K) (ceiling *nodeTree[K, V], found bool) {
	found = false
	node := tree.Root
	for node != nil {
		compare := tree.Comparator(key, node.Key)
		switch {
		case compare == 0:
			return node, true
		case compare < 0:
			ceiling, found = node, true
			node = node.Left
		case compare > 0:
			node = node.Right
		}
	}
	if found {
		return ceiling, true
	}
	return nil, false
}

// GreatestLessThan finds largest node that is smaller than the given node.
// A node may not be found, either because the tree is empty, or because
// all nodes in the tree are larger than or equal to the given node.
//
// Key should adhere to the comparator's type assertion, otherwise method panics.
func (tree *Tree[K, V]) LargestLessThan(key K) (floor *nodeTree[K, V], found bool) {
	found = false
	node := tree.Root
	for node != nil {
		if tree.Comparator(key, node.Key) > 0 {
			floor, found = node, true
			node = node.Right
		} else {
			node = node.Left
		}
	}
	if found {
		return floor, true
	}
	return nil, false
}

// Ceiling finds the smallest node that is larger than to the given node.
// A node may not be found, either because the tree is empty, or because
// all nodes in the tree are smaller than the given node.
//
// Key should adhere to the comparator's type assertion, otherwise method panics.
func (tree *Tree[K, V]) SmallestGreaterThan(key K) (ceiling *nodeTree[K, V], found bool) {
	found = false
	node := tree.Root
	for node != nil {
		if tree.Comparator(key, node.Key) < 0 {
			ceiling, found = node, true
			node = node.Left
		} else {
			node = node.Right
		}
	}

	if found {
		return ceiling, true
	}
	return nil, false
}

// GetMin gets the min value and flag if found
func (tree *Tree[K, V]) GetMin() (node *nodeTree[K, V], found bool) {
	return tree.Min, tree.Min != nil
}

// GetMax gets the max value and flag if found
func (tree *Tree[K, V]) GetMax() (node *nodeTree[K, V], found bool) {
	return tree.Max, tree.Max != nil
}

func (tree *Tree[K, V]) getMinFromNode(node *nodeTree[K, V]) (foundNode *nodeTree[K, V], found bool) {
	if node == nil {
		return nil, false

	}
	if node.Left == nil {
		return node, true

	}
	return tree.getMinFromNode(node.Left)

}

func (tree *Tree[K, V]) getMaxFromNode(node *nodeTree[K, V]) (foundNode *nodeTree[K, V], found bool) {
	if node == nil {
		return nil, false

	}
	if node.Right == nil {
		return node, true

	}
	return tree.getMaxFromNode(node.Right)

}

// Clear removes all nodes from the tree.
func (tree *Tree[K, V]) Clear() {
	tree.Root = nil
	tree.size = 0
	tree.Max = nil
	tree.Min = nil
}

// String returns a string representation of container
func (tree *Tree[K, V]) String() string {
	str := "RedBlackTree\n"
	if !tree.Empty() {
		outputTree(tree.Root, "", true, &str)
	}
	return str
}

func (node *nodeTree[K, V]) String() string {
	return fmt.Sprintf("%v", node.Key)
}

func outputTree[K Number, V any](node *nodeTree[K, V], prefix string, isTail bool, str *string) {
	if node.Right != nil {
		newPrefix := prefix
		if isTail {
			newPrefix += "│   "
		} else {
			newPrefix += "    "
		}
		outputTree(node.Right, newPrefix, false, str)
	}
	*str += prefix
	if isTail {
		*str += "└── "
	} else {
		*str += "┌── "
	}
	*str += node.String() + "\n"
	if node.Left != nil {
		newPrefix := prefix
		if isTail {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}
		outputTree(node.Left, newPrefix, true, str)
	}
}

func (tree *Tree[K, V]) lookup(key K) *nodeTree[K, V] {
	node := tree.Root
	for node != nil {
		compare := tree.Comparator(key, node.Key)
		switch {
		case compare == 0:
			return node
		case compare < 0:
			node = node.Left
		case compare > 0:
			node = node.Right
		}
	}
	return nil
}

func (node *nodeTree[K, V]) grandparent() *nodeTree[K, V] {
	if node != nil && node.Parent != nil {
		return node.Parent.Parent
	}
	return nil
}

func (node *nodeTree[K, V]) uncle() *nodeTree[K, V] {
	if node == nil || node.Parent == nil || node.Parent.Parent == nil {
		return nil
	}
	return node.Parent.sibling()
}

func (node *nodeTree[K, V]) sibling() *nodeTree[K, V] {
	if node == nil || node.Parent == nil {
		return nil
	}
	if node == node.Parent.Left {
		return node.Parent.Right
	}
	return node.Parent.Left
}

func (tree *Tree[K, V]) rotateLeft(node *nodeTree[K, V]) {
	right := node.Right
	tree.replaceNode(node, right)
	node.Right = right.Left
	if right.Left != nil {
		right.Left.Parent = node
	}
	right.Left = node
	node.Parent = right
}

func (tree *Tree[K, V]) rotateRight(node *nodeTree[K, V]) {
	left := node.Left
	tree.replaceNode(node, left)
	node.Left = left.Right
	if left.Right != nil {
		left.Right.Parent = node
	}
	left.Right = node
	node.Parent = left
}

func (tree *Tree[K, V]) replaceNode(old *nodeTree[K, V], new *nodeTree[K, V]) {
	if old.Parent == nil {
		tree.Root = new
	} else {
		if old == old.Parent.Left {
			old.Parent.Left = new
		} else {
			old.Parent.Right = new
		}
	}
	if new != nil {
		new.Parent = old.Parent
	}
}

func (tree *Tree[K, V]) insertCase1(node *nodeTree[K, V]) {
	if node.Parent == nil {
		node.color = blackTree
	} else {
		tree.insertCase2(node)
	}
}

func (tree *Tree[K, V]) insertCase2(node *nodeTree[K, V]) {
	if nodeColorTree(node.Parent) == blackTree {
		return
	}
	tree.insertCase3(node)
}

func (tree *Tree[K, V]) insertCase3(node *nodeTree[K, V]) {
	uncle := node.uncle()
	if nodeColorTree(uncle) == redTree {
		node.Parent.color = blackTree
		uncle.color = blackTree
		node.grandparent().color = redTree
		tree.insertCase1(node.grandparent())
	} else {
		tree.insertCase4(node)
	}
}

func (tree *Tree[K, V]) insertCase4(node *nodeTree[K, V]) {
	grandparent := node.grandparent()
	if node == node.Parent.Right && node.Parent == grandparent.Left {
		tree.rotateLeft(node.Parent)
		node = node.Left
	} else if node == node.Parent.Left && node.Parent == grandparent.Right {
		tree.rotateRight(node.Parent)
		node = node.Right
	}
	tree.insertCase5(node)
}

func (tree *Tree[K, V]) insertCase5(node *nodeTree[K, V]) {
	node.Parent.color = blackTree
	grandparent := node.grandparent()
	grandparent.color = redTree
	if node == node.Parent.Left && node.Parent == grandparent.Left {
		tree.rotateRight(grandparent)
	} else if node == node.Parent.Right && node.Parent == grandparent.Right {
		tree.rotateLeft(grandparent)
	}
}

func (node *nodeTree[K, V]) maximumNode() *nodeTree[K, V] {
	if node == nil {
		return nil
	}
	for node.Right != nil {
		node = node.Right
	}
	return node
}

func (tree *Tree[K, V]) deleteCase1(node *nodeTree[K, V]) {
	if node.Parent == nil {
		return
	}
	tree.deleteCase2(node)
}

func (tree *Tree[K, V]) deleteCase2(node *nodeTree[K, V]) {
	sibling := node.sibling()
	if nodeColorTree(sibling) == redTree {
		node.Parent.color = redTree
		sibling.color = blackTree
		if node == node.Parent.Left {
			tree.rotateLeft(node.Parent)
		} else {
			tree.rotateRight(node.Parent)
		}
	}
	tree.deleteCase3(node)
}

func (tree *Tree[K, V]) deleteCase3(node *nodeTree[K, V]) {
	sibling := node.sibling()
	if nodeColorTree(node.Parent) == blackTree &&
		nodeColorTree(sibling) == blackTree &&
		nodeColorTree(sibling.Left) == blackTree &&
		nodeColorTree(sibling.Right) == blackTree {
		sibling.color = redTree
		tree.deleteCase1(node.Parent)
	} else {
		tree.deleteCase4(node)
	}
}

func (tree *Tree[K, V]) deleteCase4(node *nodeTree[K, V]) {
	sibling := node.sibling()
	if nodeColorTree(node.Parent) == redTree &&
		nodeColorTree(sibling) == blackTree &&
		nodeColorTree(sibling.Left) == blackTree &&
		nodeColorTree(sibling.Right) == blackTree {
		sibling.color = redTree
		node.Parent.color = blackTree
	} else {
		tree.deleteCase5(node)
	}
}

func (tree *Tree[K, V]) deleteCase5(node *nodeTree[K, V]) {
	sibling := node.sibling()
	if node == node.Parent.Left &&
		nodeColorTree(sibling) == blackTree &&
		nodeColorTree(sibling.Left) == redTree &&
		nodeColorTree(sibling.Right) == blackTree {
		sibling.color = redTree
		sibling.Left.color = blackTree
		tree.rotateRight(sibling)
	} else if node == node.Parent.Right &&
		nodeColorTree(sibling) == blackTree &&
		nodeColorTree(sibling.Right) == redTree &&
		nodeColorTree(sibling.Left) == blackTree {
		sibling.color = redTree
		sibling.Right.color = blackTree
		tree.rotateLeft(sibling)
	}
	tree.deleteCase6(node)
}

func (tree *Tree[K, V]) deleteCase6(node *nodeTree[K, V]) {
	sibling := node.sibling()
	sibling.color = nodeColorTree(node.Parent)
	node.Parent.color = blackTree
	if node == node.Parent.Left && nodeColorTree(sibling.Right) == redTree {
		sibling.Right.color = blackTree
		tree.rotateLeft(node.Parent)
	} else if nodeColorTree(sibling.Left) == redTree {
		sibling.Left.color = blackTree
		tree.rotateRight(node.Parent)
	}
}

func nodeColorTree[K Number, V any](node *nodeTree[K, V]) colorTree {
	if node == nil {
		return blackTree
	}
	return node.color
}

// Iterator holding the iterator's state
type iteratorTree[K Number, V any] struct {
	tree     *Tree[K, V]
	node     *nodeTree[K, V]
	position positionTree
}

type positionTree byte

const (
	beginTree, betweenTree, endTree positionTree = 0, 1, 2
)

// Iterator returns a stateful iterator whose elements are key/value pairs.
func (tree *Tree[K, V]) Iterator() iteratorTree[K, V] {
	return iteratorTree[K, V]{tree: tree, node: nil, position: beginTree}
}

// IteratorAt returns a stateful iterator whose elements are key/value pairs that is initialised at a particular node.
func (tree *Tree[K, V]) IteratorAt(node *nodeTree[K, V]) iteratorTree[K, V] {
	return iteratorTree[K, V]{tree: tree, node: node, position: betweenTree}
}

// Next moves the iterator to the next element and returns true if there was a next element in the container.
// If Next() returns true, then next element's key and value can be retrieved by Key() and Value().
// If Next() was called for the first time, then it will point the iterator to the first element if it exists.
// Modifies the state of the iterator.
func (iterator *iteratorTree[K, V]) Next() bool {
	if iterator.position == endTree {
		goto end
	}
	if iterator.position == beginTree {
		left := iterator.tree.Left()
		if left == nil {
			goto end
		}
		iterator.node = left
		goto between
	}
	if iterator.node.Right != nil {
		iterator.node = iterator.node.Right
		for iterator.node.Left != nil {
			iterator.node = iterator.node.Left
		}
		goto between
	}
	if iterator.node.Parent != nil {
		node := iterator.node
		for iterator.node.Parent != nil {
			iterator.node = iterator.node.Parent
			if iterator.tree.Comparator(node.Key, iterator.node.Key) <= 0 {
				goto between
			}
		}
	}

end:
	iterator.node = nil
	iterator.position = endTree
	return false

between:
	iterator.position = betweenTree
	return true
}

// Prev moves the iterator to the previous element and returns true if there was a previous element in the container.
// If Prev() returns true, then previous element's key and value can be retrieved by Key() and Value().
// Modifies the state of the iterator.
func (iterator *iteratorTree[K, V]) Prev() bool {
	if iterator.position == beginTree {
		goto begin
	}
	if iterator.position == endTree {
		right := iterator.tree.Right()
		if right == nil {
			goto begin
		}
		iterator.node = right
		goto between
	}
	if iterator.node.Left != nil {
		iterator.node = iterator.node.Left
		for iterator.node.Right != nil {
			iterator.node = iterator.node.Right
		}
		goto between
	}
	if iterator.node.Parent != nil {
		node := iterator.node
		for iterator.node.Parent != nil {
			iterator.node = iterator.node.Parent
			if iterator.tree.Comparator(node.Key, iterator.node.Key) >= 0 {
				goto between
			}
		}
	}

begin:
	iterator.node = nil
	iterator.position = beginTree
	return false

between:
	iterator.position = betweenTree
	return true
}

// Value returns the current element's value.
// Does not modify the state of the iterator.
func (iterator *iteratorTree[K, V]) Value() V {
	return iterator.node.Value
}

// Key returns the current element's key.
// Does not modify the state of the iterator.
func (iterator *iteratorTree[K, V]) Key() K {
	return iterator.node.Key
}

// Begin resets the iterator to its initial state (one-before-first)
// Call Next() to fetch the first element if any.
func (iterator *iteratorTree[K, V]) Begin() {
	iterator.node = nil
	iterator.position = beginTree
}

// End moves the iterator past the last element (one-past-the-end).
// Call Prev() to fetch the last element if any.
func (iterator *iteratorTree[K, V]) End() {
	iterator.node = nil
	iterator.position = endTree
}

// First moves the iterator to the first element and returns true if there was a first element in the container.
// If First() returns true, then first element's key and value can be retrieved by Key() and Value().
// Modifies the state of the iterator
func (iterator *iteratorTree[K, V]) First() bool {
	iterator.Begin()
	return iterator.Next()
}

// Last moves the iterator to the last element and returns true if there was a last element in the container.
// If Last() returns true, then last element's key and value can be retrieved by Key() and Value().
// Modifies the state of the iterator.
func (iterator *iteratorTree[K, V]) Last() bool {
	iterator.End()
	return iterator.Prev()
}
