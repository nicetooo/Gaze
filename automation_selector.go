package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// AnalyzeElementSelectors analyzes a UI element and generates multiple selector suggestions
// Returns a list of possible selectors ranked by priority
func (a *App) AnalyzeElementSelectors(deviceId string, x, y int, touchTime time.Time) ([]SelectorSuggestion, *ElementInfo, error) {
	// Use cached UI hierarchy if available and fresh
	uiHierarchyCacheMu.Lock()
	cached, exists := uiHierarchyCache[deviceId]
	uiHierarchyCacheMu.Unlock()

	var result *UIHierarchyResult
	// CRITICAL: Check if cache exists and was started BEFORE the action
	// If the dump started after the click, it likely contains the target screen (second page)
	if exists && cached.result != nil && cached.DumpStartTime.Before(touchTime) {
		result = cached.result
		fmt.Printf("[Automation] Using valid PRE-TOUCH cache for analysis (Started: %v, Action: %v)\n",
			cached.DumpStartTime.Format("15:04:05.000"), touchTime.Format("15:04:05.000"))
	} else {
		// Fallback to fresh dump if no valid pre-touch cache exists
		// This happens on the very first screen or if pre-capture failed
		fmt.Printf("[Automation] No valid PRE-TOUCH cache (started after action or missing). Performing fresh dump...\n")
		var err error
		result, err = a.GetUIHierarchy(deviceId)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get UI hierarchy: %w", err)
		}
	}

	// Find element at point
	node := a.FindElementAtPoint(result.Root, x, y)
	if node == nil {
		fmt.Printf("[Automation] No element found at scaled coordinates (%d, %d). Display size: %s\n", x, y, a.getDisplaySize(result.Root))
		return nil, nil, fmt.Errorf("no element found at coordinates (%d, %d)", x, y)
	}

	fmt.Printf("[Automation] Found element at (%d, %d): Class=%s, Text=%q, ID=%s, Bounds=%s\n",
		x, y, node.Class, node.Text, node.ResourceID, node.Bounds)

	// Try to find the "best" node for identification
	// If the leaf node has no text/ID but is part of a clickable container,
	// the container or its children might have the info we need.
	bestText := node.Text
	bestResID := node.ResourceID
	bestDesc := node.ContentDesc

	// If leaf node has no text, search children for any text that contains the point
	if bestText == "" {
		bestText = a.findTextAtPointInChildren(node, x, y)
	}

	// If still no identification, check ancestors and their children
	if bestText == "" || bestResID == "" {
		temp := node
		for i := 0; i < 4; i++ { // Check up to 4 levels
			parent := a.findParent(result.Root, temp)
			if parent == nil {
				break
			}

			// 1. Check parent itself
			if bestResID == "" && parent.ResourceID != "" {
				bestResID = parent.ResourceID
			}
			if bestText == "" && parent.Text != "" {
				bestText = parent.Text
			}
			if bestDesc == "" && parent.ContentDesc != "" {
				bestDesc = parent.ContentDesc
			}

			// 2. Search children of ancestor for text that is nearby or meaningful
			if bestText == "" {
				bestText = a.findTextAtPointInChildren(parent, x, y)
			}

			temp = parent
		}
	}

	// Build detailed element info
	elemInfo := &ElementInfo{
		X:      x,
		Y:      y,
		Class:  node.Class,
		Bounds: node.Bounds,
		Selector: &ElementSelector{
			Type:  "text", // Default to text, user choice will override this later
			Value: bestText,
			Index: 0,
		},
		Timestamp: 0,
	}

	// Update selector if ID or Desc is better/available
	if bestText == "" {
		if bestResID != "" {
			elemInfo.Selector.Type = "id"
			elemInfo.Selector.Value = bestResID
		} else if bestDesc != "" {
			elemInfo.Selector.Type = "desc"
			elemInfo.Selector.Value = bestDesc
		} else {
			// Fallback to XPath for leaf nodes with no identity
			xpath := a.buildXPath(result.Root, node)
			if xpath != "" {
				elemInfo.Selector.Type = "xpath"
				elemInfo.Selector.Value = xpath
			} else {
				// Absolute fallback to coordinates
				elemInfo.Selector.Type = "coordinates"
				elemInfo.Selector.Value = fmt.Sprintf("%d,%d", x, y)
			}
		}
	}

	// Generate selector suggestions
	suggestions := []SelectorSuggestion{}

	// 1. Text selector
	if bestText != "" {
		priority := 5
		desc := fmt.Sprintf("Text: \"%s\"", bestText)
		if node.Text == "" {
			desc += " (found in relative element)"
		}

		if isGenericText(bestText) {
			priority = 3
			desc += " (generic text)"
		} else if !a.isUniqueSelector(result.Root, "text", bestText) {
			priority = 3
			desc += " (not unique)"
		}

		suggestions = append(suggestions, SelectorSuggestion{
			Type:        "text",
			Value:       bestText,
			Priority:    priority,
			Description: desc,
		})
	}

	// 2. Resource ID selector
	if bestResID != "" {
		priority := 5
		desc := fmt.Sprintf("Resource ID: %s", bestResID)
		if node.ResourceID == "" {
			desc += " (found in parent)"
			priority = 4 // slightly lower since it's an ancestor
		}

		if !a.isUniqueSelector(result.Root, "id", bestResID) {
			priority = 3
			desc += " (not unique)"
		}

		suggestions = append(suggestions, SelectorSuggestion{
			Type:        "id",
			Value:       bestResID,
			Priority:    priority,
			Description: desc,
		})
	}

	// 3. Content-desc selector
	if node.ContentDesc != "" {
		priority := 4
		desc := fmt.Sprintf("Content Description: \"%s\"", node.ContentDesc)

		if !a.isUniqueSelector(result.Root, "desc", node.ContentDesc) {
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

	// 4. Class selector (lower priority, usually not unique)
	if node.Class != "" {
		shortClass := node.Class
		if parts := strings.Split(node.Class, "."); len(parts) > 0 {
			shortClass = parts[len(parts)-1]
		}

		suggestions = append(suggestions, SelectorSuggestion{
			Type:        "class",
			Value:       node.Class,
			Priority:    2,
			Description: fmt.Sprintf("Class: %s (usually matches multiple elements)", shortClass),
		})
	}

	// 5. XPath selector (fallback, most specific but fragile)
	xpath := a.buildXPath(result.Root, node)
	if xpath != "" {
		suggestions = append(suggestions, SelectorSuggestion{
			Type:        "xpath",
			Value:       xpath,
			Priority:    2,
			Description: "XPath (most specific but fragile to UI changes)",
		})
	}

	// 6. Coordinates fallback (lowest priority)
	suggestions = append(suggestions, SelectorSuggestion{
		Type:        "coordinates",
		Value:       fmt.Sprintf("%d,%d", x, y),
		Priority:    1,
		Description: fmt.Sprintf("Coordinates (%d, %d) - least reliable", x, y),
	})

	// Sort by priority (descending)
	for i := 0; i < len(suggestions)-1; i++ {
		for j := i + 1; j < len(suggestions); j++ {
			if suggestions[j].Priority > suggestions[i].Priority {
				suggestions[i], suggestions[j] = suggestions[j], suggestions[i]
			}
		}
	}

	return suggestions, elemInfo, nil
}

// buildXPath builds a simple XPath for the given node
func (a *App) buildXPath(root *UINode, target *UINode) string {
	// Simple implementation: build path from root to target
	path := a.findNodePath(root, target, "")
	if path == "" {
		return ""
	}
	return path
}

// findNodePath recursively finds the path to a target node
func (a *App) findNodePath(current *UINode, target *UINode, currentPath string) string {
	if current == target {
		return currentPath + "/" + getNodeXPathSegment(current)
	}

	for i := range current.Nodes {
		newPath := currentPath + "/" + getNodeXPathSegment(current)
		result := a.findNodePath(&current.Nodes[i], target, newPath)
		if result != "" {
			return result
		}
	}

	return ""
}

// getNodeXPathSegment gets the XPath segment for a node
func getNodeXPathSegment(node *UINode) string {
	className := node.Class
	if parts := strings.Split(className, "."); len(parts) > 0 {
		className = parts[len(parts)-1]
	}
	return className
}

// isUniqueSelector checks if a selector value is unique in the hierarchy
func (a *App) isUniqueSelector(root *UINode, selectorType, value string) bool {
	count := a.countMatchingNodes(root, selectorType, value)
	return count == 1
}

// countMatchingNodes counts how many nodes match the selector
func (a *App) countMatchingNodes(node *UINode, selectorType, value string) int {
	count := 0

	// Check current node
	switch selectorType {
	case "text":
		if node.Text == value {
			count++
		}
	case "id":
		if node.ResourceID == value {
			count++
		}
	case "desc":
		if node.ContentDesc == value {
			count++
		}
	case "class":
		if node.Class == value {
			count++
		}
	}

	// Check children
	for i := range node.Nodes {
		count += a.countMatchingNodes(&node.Nodes[i], selectorType, value)
	}

	return count
}

// findTextAtPointInChildren searches for text in descendants that contain the given point
func (a *App) findTextAtPointInChildren(node *UINode, x, y int) string {
	// Parse bounds to check if point is inside
	re := regexp.MustCompile(`\[(\d+),(\d+)\]\[(\d+),(\d+)\]`)
	matches := re.FindStringSubmatch(node.Bounds)
	if len(matches) >= 5 {
		x1, _ := strconv.Atoi(matches[1])
		y1, _ := strconv.Atoi(matches[2])
		x2, _ := strconv.Atoi(matches[3])
		y2, _ := strconv.Atoi(matches[4])

		if x < x1 || x > x2 || y < y1 || y > y2 {
			return "" // Point is not in this branch
		}
	}

	// Check this node
	if node.Text != "" {
		return node.Text
	}

	// Check children (only those that contain the point)
	for i := range node.Nodes {
		if t := a.findTextAtPointInChildren(&node.Nodes[i], x, y); t != "" {
			return t
		}
	}

	// If no text found exactly at point, as a desperate fallback,
	// return the first available text in this branch
	return a.findTextInChildren(node)
}

// findTextInChildren searches for any text in the subtree
func (a *App) findTextInChildren(node *UINode) string {
	for i := range node.Nodes {
		if node.Nodes[i].Text != "" {
			return node.Nodes[i].Text
		}
		if t := a.findTextInChildren(&node.Nodes[i]); t != "" {
			return t
		}
	}
	return ""
}

// findParent finds the parent of a target node in the hierarchy
func (a *App) findParent(root *UINode, target *UINode) *UINode {
	for i := range root.Nodes {
		if &root.Nodes[i] == target {
			return root
		}
		if p := a.findParent(&root.Nodes[i], target); p != nil {
			return p
		}
	}
	return nil
}

// isGenericText checks if text is too generic to be a good selector
func isGenericText(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	genericTexts := []string{
		"ok", "cancel", "yes", "no", "确定", "取消", "是", "否",
		"submit", "close", "done", "next", "back", "skip",
		"提交", "关闭", "完成", "下一步", "返回", "跳过",
	}

	for _, generic := range genericTexts {
		if text == generic {
			return true
		}
	}

	return false
}

// getDisplaySize returns the bounds of the root node for logging
func (a *App) getDisplaySize(root *UINode) string {
	if root == nil {
		return "unknown"
	}
	return root.Bounds
}
