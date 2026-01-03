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

// ParseBounds parses Android bounds string "[x1,y1][x2,y2]" into BoundsRect
func ParseBounds(bounds string) (*BoundsRect, error) {
	re := regexp.MustCompile(`\[(\d+),(\d+)\]\[(\d+),(\d+)\]`)
	matches := re.FindStringSubmatch(bounds)
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
// ========================================

// GenerateSelectorSuggestions generates multiple selector options for an element
func (a *App) GenerateSelectorSuggestions(node *UINode, root *UINode) []SelectorSuggestion {
	suggestions := make([]SelectorSuggestion, 0)

	// 1. Text selector (highest priority when available and unique)
	if node.Text != "" {
		priority := 5
		desc := fmt.Sprintf("Text: \"%s\"", node.Text)
		if isGenericText(node.Text) {
			priority = 3
			desc += " (generic text)"
		} else if !a.isSelectorUnique(root, "text", node.Text) {
			priority = 3
			desc += " (not unique)"
		}
		suggestions = append(suggestions, SelectorSuggestion{
			Type:        "text",
			Value:       node.Text,
			Priority:    priority,
			Description: desc,
		})
	}

	// 2. Resource ID selector
	if node.ResourceID != "" {
		priority := 5
		desc := fmt.Sprintf("Resource ID: %s", node.ResourceID)
		if !a.isSelectorUnique(root, "id", node.ResourceID) {
			priority = 3
			desc += " (not unique)"
		}
		suggestions = append(suggestions, SelectorSuggestion{
			Type:        "id",
			Value:       node.ResourceID,
			Priority:    priority,
			Description: desc,
		})
	}

	// 3. Content description selector
	if node.ContentDesc != "" {
		priority := 4
		desc := fmt.Sprintf("Content Description: \"%s\"", node.ContentDesc)
		if !a.isSelectorUnique(root, "desc", node.ContentDesc) {
			priority = 3
			desc += " (not unique)"
		}
		suggestions = append(suggestions, SelectorSuggestion{
			Type:        "desc",
			Value:       node.ContentDesc,
			Priority:    priority,
			Description: desc,
		})
	}

	// 4. Class selector (lower priority)
	if node.Class != "" {
		shortClass := node.Class
		if parts := strings.Split(node.Class, "."); len(parts) > 0 {
			shortClass = parts[len(parts)-1]
		}
		suggestions = append(suggestions, SelectorSuggestion{
			Type:        "class",
			Value:       node.Class,
			Priority:    2,
			Description: fmt.Sprintf("Class: %s (usually matches multiple)", shortClass),
		})
	}

	// 5. XPath selector (fallback, fragile)
	xpath := a.buildXPath(root, node)
	if xpath != "" {
		suggestions = append(suggestions, SelectorSuggestion{
			Type:        "xpath",
			Value:       xpath,
			Priority:    2,
			Description: "XPath (specific but fragile)",
		})
	}

	// 6. Bounds selector
	if node.Bounds != "" {
		suggestions = append(suggestions, SelectorSuggestion{
			Type:        "bounds",
			Value:       node.Bounds,
			Priority:    1,
			Description: fmt.Sprintf("Bounds: %s (position dependent)", node.Bounds),
		})
	}

	// Sort by priority (descending)
	for i := 0; i < len(suggestions)-1; i++ {
		for j := i + 1; j < len(suggestions); j++ {
			if suggestions[j].Priority > suggestions[i].Priority {
				suggestions[i], suggestions[j] = suggestions[j], suggestions[i]
			}
		}
	}

	return suggestions
}

// GetBestSelector returns the best selector for an element
func (a *App) GetBestSelector(node *UINode, root *UINode) *ElementSelector {
	// Priority: unique text > unique id > desc > xpath > bounds
	if node.Text != "" && a.isSelectorUnique(root, "text", node.Text) && !isGenericText(node.Text) {
		return &ElementSelector{Type: "text", Value: node.Text}
	}
	if node.ResourceID != "" && a.isSelectorUnique(root, "id", node.ResourceID) {
		return &ElementSelector{Type: "id", Value: node.ResourceID}
	}
	if node.ContentDesc != "" && a.isSelectorUnique(root, "desc", node.ContentDesc) {
		return &ElementSelector{Type: "desc", Value: node.ContentDesc}
	}
	// Fallback to xpath
	xpath := a.buildXPath(root, node)
	if xpath != "" {
		return &ElementSelector{Type: "xpath", Value: xpath}
	}
	// Last resort: bounds
	if node.Bounds != "" {
		return &ElementSelector{Type: "bounds", Value: node.Bounds}
	}
	return nil
}

// isSelectorUnique checks if a selector value is unique in the hierarchy
func (a *App) isSelectorUnique(root *UINode, selectorType, value string) bool {
	count := a.countMatchingNodes(root, selectorType, value)
	return count == 1
}

// GetSelectorMatchCount returns the number of elements matching a selector
func (a *App) GetSelectorMatchCount(root *UINode, selector *ElementSelector) int {
	if selector == nil {
		return 0
	}
	return a.countMatchingNodes(root, selector.Type, selector.Value)
}
