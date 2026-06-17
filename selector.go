package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ========================================
// Unified Selector Service
// Consolidates all element finding and selector logic
// Used by: Recording, Workflow, UI Inspector
// ========================================

// BoundsRect represents parsed bounds coordinates
type BoundsRect struct {
	X1, Y1, X2, Y2 int
}

// reBounds matches Android bounds strings "[x1,y1][x2,y2]".
// Package-level so per-node hot paths don't recompile it.
var reBounds = regexp.MustCompile(`\[(\d+),(\d+)\]\[(\d+),(\d+)\]`)

// ParseBounds parses Android bounds string "[x1,y1][x2,y2]" into BoundsRect
func ParseBounds(bounds string) (*BoundsRect, error) {
	matches := reBounds.FindStringSubmatch(bounds)
	if len(matches) != 5 {
		return nil, fmt.Errorf("invalid bounds format: %s", bounds)
	}

	x1, _ := strconv.Atoi(matches[1])
	y1, _ := strconv.Atoi(matches[2])
	x2, _ := strconv.Atoi(matches[3])
	y2, _ := strconv.Atoi(matches[4])

	return &BoundsRect{X1: x1, Y1: y1, X2: x2, Y2: y2}, nil
}

// Center returns the center point of the bounds
func (b *BoundsRect) Center() (int, int) {
	return b.X1 + (b.X2-b.X1)/2, b.Y1 + (b.Y2-b.Y1)/2
}

// Contains checks if point (x, y) is inside the bounds
func (b *BoundsRect) Contains(x, y int) bool {
	return x >= b.X1 && x <= b.X2 && y >= b.Y1 && y <= b.Y2
}

// Area returns the area of the bounds rectangle
func (b *BoundsRect) Area() int {
	return (b.X2 - b.X1) * (b.Y2 - b.Y1)
}

// ========================================
// Unified Element Finding
// ========================================

// FindElementBySelector finds an element using the given selector
// Returns the first matching element, or nil if not found
func (a *App) FindElementBySelector(root *UINode, selector *ElementSelector) *UINode {
	if selector == nil || root == nil {
		return nil
	}

	switch selector.Type {
	case "text":
		return a.findElementByText(root, selector.Value, selector.Index)
	case "id":
		return a.findElementByID(root, selector.Value, selector.Index)
	case "desc", "description":
		return a.findElementByDesc(root, selector.Value, selector.Index)
	case "class":
		return a.findElementByClass(root, selector.Value, selector.Index)
	case "contains":
		return a.findElementByContains(root, selector.Value, selector.Index)
	case "xpath":
		results := a.SearchElementsXPath(root, selector.Value)
		if len(results) > selector.Index {
			return results[selector.Index].Node
		}
		return nil
	case "bounds":
		// Direct bounds match
		node := &UINode{Bounds: selector.Value}
		return node
	case "coordinates":
		// Parse coordinates and find element at point
		parts := strings.Split(selector.Value, ",")
		if len(parts) == 2 {
			x, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			y, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
			return a.FindElementAtPoint(root, x, y)
		}
		return nil
	case "advanced":
		// Advanced query syntax: "attr:value", "attr~value", "cond1 AND cond2"
		return a.findElementByAdvanced(root, selector.Value, selector.Index)
	default:
		return nil
	}
}

// FindAllElementsBySelector finds all elements matching the selector
func (a *App) FindAllElementsBySelector(root *UINode, selector *ElementSelector) []*UINode {
	if selector == nil || root == nil {
		return nil
	}

	switch selector.Type {
	case "text":
		return a.collectMatchingNodes(root, func(n *UINode) bool {
			return n.Text == selector.Value || n.ContentDesc == selector.Value
		})
	case "id":
		return a.collectMatchingNodes(root, func(n *UINode) bool {
			return n.ResourceID == selector.Value || strings.HasSuffix(n.ResourceID, ":id/"+selector.Value)
		})
	case "desc", "description":
		return a.collectMatchingNodes(root, func(n *UINode) bool {
			return n.ContentDesc == selector.Value
		})
	case "class":
		return a.collectMatchingNodes(root, func(n *UINode) bool {
			return n.Class == selector.Value
		})
	case "contains":
		return a.collectMatchingNodes(root, func(n *UINode) bool {
			return strings.Contains(n.Text, selector.Value) || strings.Contains(n.ContentDesc, selector.Value)
		})
	case "xpath":
		results := a.SearchElementsXPath(root, selector.Value)
		nodes := make([]*UINode, len(results))
		for i, r := range results {
			nodes[i] = r.Node
		}
		return nodes
	case "advanced":
		return a.collectMatchingNodes(root, func(n *UINode) bool {
			return a.matchAdvancedQuery(n, selector.Value)
		})
	default:
		return nil
	}
}

// Helper functions for finding elements by specific criteria

func (a *App) findElementByText(root *UINode, text string, index int) *UINode {
	nodes := a.collectMatchingNodes(root, func(n *UINode) bool {
		return n.Text == text || n.ContentDesc == text
	})
	if index < len(nodes) {
		return nodes[index]
	}
	return nil
}

func (a *App) findElementByID(root *UINode, id string, index int) *UINode {
	nodes := a.collectMatchingNodes(root, func(n *UINode) bool {
		return n.ResourceID == id || strings.HasSuffix(n.ResourceID, ":id/"+id)
	})
	if index < len(nodes) {
		return nodes[index]
	}
	return nil
}

func (a *App) findElementByDesc(root *UINode, desc string, index int) *UINode {
	nodes := a.collectMatchingNodes(root, func(n *UINode) bool {
		return n.ContentDesc == desc
	})
	if index < len(nodes) {
		return nodes[index]
	}
	return nil
}

func (a *App) findElementByClass(root *UINode, class string, index int) *UINode {
	nodes := a.collectMatchingNodes(root, func(n *UINode) bool {
		return n.Class == class
	})
	if index < len(nodes) {
		return nodes[index]
	}
	return nil
}

func (a *App) findElementByContains(root *UINode, text string, index int) *UINode {
	nodes := a.collectMatchingNodes(root, func(n *UINode) bool {
		return strings.Contains(n.Text, text) || strings.Contains(n.ContentDesc, text)
	})
	if index < len(nodes) {
		return nodes[index]
	}
	return nil
}

// findElementByAdvanced finds element using advanced query syntax
// Supports: "attr:value", "attr~value" (contains), "attr=value" (exact)
// Boolean: "cond1 AND cond2", "cond1 OR cond2"
func (a *App) findElementByAdvanced(root *UINode, query string, index int) *UINode {
	nodes := a.collectMatchingNodes(root, func(n *UINode) bool {
		return a.matchAdvancedQuery(n, query)
	})
	if index < len(nodes) {
		return nodes[index]
	}
	return nil
}

// matchAdvancedQuery evaluates an advanced query against a node
func (a *App) matchAdvancedQuery(node *UINode, query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return false
	}

	// Handle OR (lower precedence)
	orParts := splitAdvancedQuery(query, " OR ")
	if len(orParts) > 1 {
		for _, part := range orParts {
			if a.matchAdvancedQuery(node, part) {
				return true
			}
		}
		return false
	}

	// Handle AND (higher precedence)
	andParts := splitAdvancedQuery(query, " AND ")
	if len(andParts) > 1 {
		for _, part := range andParts {
			if !a.matchAdvancedQuery(node, part) {
				return false
			}
		}
		return true
	}

	// Single condition: "attr:value", "attr~value", "attr=value", "attr^value", "attr$value"
	return a.evaluateAdvancedCondition(node, query)
}

// splitAdvancedQuery splits query by separator (case insensitive)
func splitAdvancedQuery(query, sep string) []string {
	// Case insensitive split
	lowerQuery := strings.ToLower(query)
	lowerSep := strings.ToLower(sep)

	var parts []string
	start := 0
	for {
		idx := strings.Index(lowerQuery[start:], lowerSep)
		if idx == -1 {
			parts = append(parts, strings.TrimSpace(query[start:]))
			break
		}
		parts = append(parts, strings.TrimSpace(query[start:start+idx]))
		start += idx + len(sep)
	}
	return parts
}

// evaluateAdvancedCondition evaluates a single condition
func (a *App) evaluateAdvancedCondition(node *UINode, condition string) bool {
	condition = strings.TrimSpace(condition)

	// Find operator: ~, ^, $, =, :
	operators := []string{"~", "^", "$", "=", ":"}
	var attr, op, value string

	for _, operator := range operators {
		idx := strings.Index(condition, operator)
		if idx != -1 {
			attr = strings.TrimSpace(condition[:idx])
			op = operator
			value = strings.TrimSpace(condition[idx+1:])
			break
		}
	}

	// No operator found - treat as text contains search
	if attr == "" {
		lowerCond := strings.ToLower(condition)
		return strings.Contains(strings.ToLower(node.Text), lowerCond) ||
			strings.Contains(strings.ToLower(node.ContentDesc), lowerCond) ||
			strings.Contains(strings.ToLower(node.ResourceID), lowerCond)
	}

	// Get attribute value from node
	attrValue := a.getNodeAttribute(node, attr)
	lowerAttrValue := strings.ToLower(attrValue)
	lowerValue := strings.ToLower(value)

	// Evaluate based on operator
	switch op {
	case "=":
		return lowerAttrValue == lowerValue
	case ":", "~":
		return strings.Contains(lowerAttrValue, lowerValue)
	case "^":
		return strings.HasPrefix(lowerAttrValue, lowerValue)
	case "$":
		return strings.HasSuffix(lowerAttrValue, lowerValue)
	default:
		return false
	}
}

// Note: getNodeAttribute is defined in automation.go and reused here

// collectMatchingNodes traverses the tree and collects nodes matching the predicate
func (a *App) collectMatchingNodes(node *UINode, predicate func(*UINode) bool) []*UINode {
	if node == nil {
		return nil
	}

	var results []*UINode
	if predicate(node) {
		results = append(results, node)
	}

	for i := range node.Nodes {
		results = append(results, a.collectMatchingNodes(&node.Nodes[i], predicate)...)
	}

	return results
}

// ========================================
// Selector Generation & Analysis
