// Code generated by gotemplate. DO NOT EDIT.

// Copyright (c) 2015, Emir Pasic. All rights reserved.
// Copyright (c) 2021, E Sequeira. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package redblacktree implements a red-black tree.
//
// Used by TreeSet and TreeMap.
//
// Structure is not thread safe.
//
// References: http://en.wikipedia.org/wiki/Red%E2%80%93black_tree
package orderbook

import (
	"fmt"

	"github.com/geseq/udecimal"
)

type colorTree bool

type comparatorTree func(a, b udecimal.Decimal) int

const (
	blackTree, redTree colorTree = true, false
)

// Tree holds elements of the red-black tree
type tree struct {
	Root       *nodeTree
	size       int
	Comparator comparatorTree
	Min        *nodeTree
	Max        *nodeTree
}

// template type Tree(KeyType, ValueType)

// Node is a single element within the tree
type nodeTree struct {
	Key    udecimal.Decimal
	Value  *orderQueue
	color  colorTree
	Left   *nodeTree
	Right  *nodeTree
	Parent *nodeTree
}

// NewWith instantiates a red-black tree with the custom comparator.
func newWithTree(comparator comparatorTree) *tree {
	return &tree{Comparator: comparator}
}

func newNodeTree(key udecimal.Decimal, value *orderQueue, color colorTree) *nodeTree {
	nt := ntPool.Get()
	nt.Key = key
	nt.Value = value
	nt.color = color
	nt.Left = nil
	nt.Right = nil
	nt.Parent = nil

	return nt
}

func (n *nodeTree) Release() {
	ntPool.Put(n)
}

// Put inserts node into the tree.
// Key should adhere to the comparator's type assertion, otherwise method panics.
func (tree *tree) Put(key udecimal.Decimal, value *orderQueue) {
	var insertedNode *nodeTree
	if tree.Root == nil {
		// Assert key is of comparator's type for initial tree
		tree.Comparator(key, key)
		tree.Root = newNodeTree(key, value, redTree)
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
					node.Left = newNodeTree(key, value, redTree)
					insertedNode = node.Left
					loop = false
				} else {
					node = node.Left
				}
			case compare > 0:
				if node.Right == nil {
					node.Right = newNodeTree(key, value, redTree)
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
func (tree *tree) Get(key udecimal.Decimal) (value *orderQueue, found bool) {
	node := tree.lookup(key)
	if node != nil {
		return node.Value, true
	}
	return nil, false
}

// Remove remove the node from the tree by key.
// Key should adhere to the comparator's type assertion, otherwise method panics.
func (tree *tree) Remove(key udecimal.Decimal) {
	var child *nodeTree
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

	node.Release()
	tree.size--
}

// Empty returns true if tree does not contain any nodes
func (tree *tree) Empty() bool {
	return tree.size == 0
}

// Size returns number of nodes in the tree.
func (tree *tree) Size() int {
	return tree.size
}

// Keys returns all keys in-order
func (tree *tree) Keys() []udecimal.Decimal {
	keys := make([]udecimal.Decimal, tree.size)
	it := tree.Iterator()
	for i := 0; it.Next(); i++ {
		keys[i] = it.Key()
	}
	return keys
}

// Values returns all values in-order based on the key.
func (tree *tree) Values() []*orderQueue {
	values := make([]*orderQueue, tree.size)
	it := tree.Iterator()
	for i := 0; it.Next(); i++ {
		values[i] = it.Value()
	}
	return values
}

// Left returns the left-most (min) node or nil if tree is empty.
func (tree *tree) Left() *nodeTree {
	var parent *nodeTree
	current := tree.Root
	for current != nil {
		parent = current
		current = current.Left
	}
	return parent
}

// Right returns the right-most (max) node or nil if tree is empty.
func (tree *tree) Right() *nodeTree {
	var parent *nodeTree
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
func (tree *tree) Floor(key udecimal.Decimal) (floor *nodeTree, found bool) {
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
func (tree *tree) Ceiling(key udecimal.Decimal) (ceiling *nodeTree, found bool) {
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
func (tree *tree) LargestLessThan(key udecimal.Decimal) (floor *nodeTree, found bool) {
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
func (tree *tree) SmallestGreaterThan(key udecimal.Decimal) (ceiling *nodeTree, found bool) {
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
func (tree *tree) GetMin() (node *nodeTree, found bool) {
	return tree.Min, tree.Min != nil
}

// GetMax gets the max value and flag if found
func (tree *tree) GetMax() (node *nodeTree, found bool) {
	return tree.Max, tree.Max != nil
}

func (tree *tree) getMinFromNode(node *nodeTree) (foundNode *nodeTree, found bool) {
	if node == nil {
		return nil, false

	}
	if node.Left == nil {
		return node, true

	}
	return tree.getMinFromNode(node.Left)

}

func (tree *tree) getMaxFromNode(node *nodeTree) (foundNode *nodeTree, found bool) {
	if node == nil {
		return nil, false

	}
	if node.Right == nil {
		return node, true

	}
	return tree.getMaxFromNode(node.Right)

}

// Clear removes all nodes from the tree.
func (tree *tree) Clear() {
	tree.Root = nil
	tree.size = 0
	tree.Max = nil
	tree.Min = nil
}

// String returns a string representation of container
func (tree *tree) String() string {
	str := "RedBlackTree\n"
	if !tree.Empty() {
		outputTree(tree.Root, "", true, &str)
	}
	return str
}

func (node *nodeTree) String() string {
	return fmt.Sprintf("%v", node.Key)
}

func outputTree(node *nodeTree, prefix string, isTail bool, str *string) {
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

func (tree *tree) lookup(key udecimal.Decimal) *nodeTree {
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

func (node *nodeTree) grandparent() *nodeTree {
	if node != nil && node.Parent != nil {
		return node.Parent.Parent
	}
	return nil
}

func (node *nodeTree) uncle() *nodeTree {
	if node == nil || node.Parent == nil || node.Parent.Parent == nil {
		return nil
	}
	return node.Parent.sibling()
}

func (node *nodeTree) sibling() *nodeTree {
	if node == nil || node.Parent == nil {
		return nil
	}
	if node == node.Parent.Left {
		return node.Parent.Right
	}
	return node.Parent.Left
}

func (tree *tree) rotateLeft(node *nodeTree) {
	right := node.Right
	tree.replaceNode(node, right)
	node.Right = right.Left
	if right.Left != nil {
		right.Left.Parent = node
	}
	right.Left = node
	node.Parent = right
}

func (tree *tree) rotateRight(node *nodeTree) {
	left := node.Left
	tree.replaceNode(node, left)
	node.Left = left.Right
	if left.Right != nil {
		left.Right.Parent = node
	}
	left.Right = node
	node.Parent = left
}

func (tree *tree) replaceNode(old *nodeTree, new *nodeTree) {
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

func (tree *tree) insertCase1(node *nodeTree) {
	if node.Parent == nil {
		node.color = blackTree
	} else {
		tree.insertCase2(node)
	}
}

func (tree *tree) insertCase2(node *nodeTree) {
	if nodeColorTree(node.Parent) == blackTree {
		return
	}
	tree.insertCase3(node)
}

func (tree *tree) insertCase3(node *nodeTree) {
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

func (tree *tree) insertCase4(node *nodeTree) {
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

func (tree *tree) insertCase5(node *nodeTree) {
	node.Parent.color = blackTree
	grandparent := node.grandparent()
	grandparent.color = redTree
	if node == node.Parent.Left && node.Parent == grandparent.Left {
		tree.rotateRight(grandparent)
	} else if node == node.Parent.Right && node.Parent == grandparent.Right {
		tree.rotateLeft(grandparent)
	}
}

func (node *nodeTree) maximumNode() *nodeTree {
	if node == nil {
		return nil
	}
	for node.Right != nil {
		node = node.Right
	}
	return node
}

func (tree *tree) deleteCase1(node *nodeTree) {
	if node.Parent == nil {
		return
	}
	tree.deleteCase2(node)
}

func (tree *tree) deleteCase2(node *nodeTree) {
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

func (tree *tree) deleteCase3(node *nodeTree) {
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

func (tree *tree) deleteCase4(node *nodeTree) {
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

func (tree *tree) deleteCase5(node *nodeTree) {
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

func (tree *tree) deleteCase6(node *nodeTree) {
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

func nodeColorTree(node *nodeTree) colorTree {
	if node == nil {
		return blackTree
	}
	return node.color
}

// Iterator holding the iterator's state
type iteratorTree struct {
	tree     *tree
	node     *nodeTree
	position positionTree
}

type positionTree byte

const (
	beginTree, betweenTree, endTree positionTree = 0, 1, 2
)

// Iterator returns a stateful iterator whose elements are key/value pairs.
func (tree *tree) Iterator() iteratorTree {
	return iteratorTree{tree: tree, node: nil, position: beginTree}
}

// IteratorAt returns a stateful iterator whose elements are key/value pairs that is initialised at a particular node.
func (tree *tree) IteratorAt(node *nodeTree) iteratorTree {
	return iteratorTree{tree: tree, node: node, position: betweenTree}
}

// Next moves the iterator to the next element and returns true if there was a next element in the container.
// If Next() returns true, then next element's key and value can be retrieved by Key() and Value().
// If Next() was called for the first time, then it will point the iterator to the first element if it exists.
// Modifies the state of the iterator.
func (iterator *iteratorTree) Next() bool {
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
func (iterator *iteratorTree) Prev() bool {
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
func (iterator *iteratorTree) Value() *orderQueue {
	return iterator.node.Value
}

// Key returns the current element's key.
// Does not modify the state of the iterator.
func (iterator *iteratorTree) Key() udecimal.Decimal {
	return iterator.node.Key
}

// Begin resets the iterator to its initial state (one-before-first)
// Call Next() to fetch the first element if any.
func (iterator *iteratorTree) Begin() {
	iterator.node = nil
	iterator.position = beginTree
}

// End moves the iterator past the last element (one-past-the-end).
// Call Prev() to fetch the last element if any.
func (iterator *iteratorTree) End() {
	iterator.node = nil
	iterator.position = endTree
}

// First moves the iterator to the first element and returns true if there was a first element in the container.
// If First() returns true, then first element's key and value can be retrieved by Key() and Value().
// Modifies the state of the iterator
func (iterator *iteratorTree) First() bool {
	iterator.Begin()
	return iterator.Next()
}

// Last moves the iterator to the last element and returns true if there was a last element in the container.
// If Last() returns true, then last element's key and value can be retrieved by Key() and Value().
// Modifies the state of the iterator.
func (iterator *iteratorTree) Last() bool {
	iterator.End()
	return iterator.Prev()
}
