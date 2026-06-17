package proxy

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// BreakpointRule defines when to pause a request/response for inspection
type BreakpointRule struct {
	ID         string `json:"id"`
	URLPattern string `json:"urlPattern"`
	Method     string `json:"method"` // empty = match all
	Phase      string `json:"phase"`  // "request", "response", "both"
}

// BreakpointResolution is the user's decision for a paused breakpoint
type BreakpointResolution struct {
	Action string `json:"action"` // "forward", "drop"

	// Request phase modifications
	ModifiedMethod  string            `json:"modifiedMethod,omitempty"`
	ModifiedURL     string            `json:"modifiedUrl,omitempty"`
	ModifiedHeaders map[string]string `json:"modifiedHeaders,omitempty"`
	ModifiedBody    string            `json:"modifiedBody,omitempty"`

	// Response phase modifications
	ModifiedStatusCode  int               `json:"modifiedStatusCode,omitempty"`
	ModifiedRespHeaders map[string]string `json:"modifiedRespHeaders,omitempty"`
	ModifiedRespBody    string            `json:"modifiedRespBody,omitempty"`
}

// PendingBreakpointInfo is the serializable info sent to the frontend
type PendingBreakpointInfo struct {
	ID     string `json:"id"`
	RuleID string `json:"ruleId"`
	Phase  string `json:"phase"` // "request" or "response"

	// Request info
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Headers map[string][]string `json:"headers,omitempty"`
	Body    string              `json:"body,omitempty"`

	// Response info (only for response phase)
	StatusCode  int                 `json:"statusCode,omitempty"`
	RespHeaders map[string][]string `json:"respHeaders,omitempty"`
	RespBody    string              `json:"respBody,omitempty"`

	CreatedAt int64 `json:"createdAt"` // unix ms
}

// pendingBreakpoint is the internal representation with resolution channel
type pendingBreakpoint struct {
	Info PendingBreakpointInfo
	Ch   chan BreakpointResolution
}

// breakpointTimeout is how long we wait before auto-forwarding
const breakpointTimeout = 120 * time.Second

// maxPendingBreakpoints limits concurrent paused requests to prevent proxy exhaustion
const maxPendingBreakpoints = 20

// breakpointState groups breakpoint-related fields (embedded in ProxyServer)
type breakpointState struct {
	rules      map[string]*BreakpointRule
	pending    map[string]*pendingBreakpoint
	mu         sync.Mutex // protects pending map
	onHit      func(PendingBreakpointInfo)
	onResolved func(id string, reason string)  // called when breakpoint is resolved/timed out
	onCapacity func(ruleID, url, phase string) // called when a matching request passes through because the pool is full

	capacitySkips   atomic.Int64 // matching requests passed through due to maxPendingBreakpoints
	timeoutForwards atomic.Int64 // breakpoints auto-forwarded after breakpointTimeout
}

// AddBreakpointRule adds a breakpoint rule to the proxy engine
func (p *ProxyServer) AddBreakpointRule(id, urlPattern, method, phase string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.bp.rules == nil {
		p.bp.rules = make(map[string]*BreakpointRule)
	}
	p.bp.rules[id] = &BreakpointRule{
		ID:         id,
		URLPattern: urlPattern,
		Method:     method,
		Phase:      phase,
	}
}

// RemoveBreakpointRule removes a breakpoint rule
func (p *ProxyServer) RemoveBreakpointRule(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.bp.rules != nil {
		delete(p.bp.rules, id)
	}
}

// matchBreakpointRule checks if a request matches any breakpoint rule for the given phase.
// Must NOT hold p.mu when calling this (it acquires it internally).
func (p *ProxyServer) matchBreakpointRule(r *http.Request, phase string) *BreakpointRule {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.bp.rules) == 0 {
		return nil
	}

	method := r.Method
	url := r.URL.String()

	for _, rule := range p.bp.rules {
		// Check phase: "both" matches either
		if rule.Phase != "both" && rule.Phase != phase {
			continue
		}
		// Check method (empty = match all)
		if rule.Method != "" && rule.Method != method {
			continue
		}
		// Check URL pattern
		if !MatchPattern(url, rule.URLPattern) {
			continue
		}
		return rule
	}
	return nil
}

// hasBreakpointRules returns true if any breakpoint rules are registered
func (p *ProxyServer) hasBreakpointRules() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.bp.rules) > 0
}

// addPendingBreakpoint stores a pending breakpoint
func (p *ProxyServer) addPendingBreakpoint(bp *pendingBreakpoint) {
	p.bp.mu.Lock()
	defer p.bp.mu.Unlock()
	if p.bp.pending == nil {
		p.bp.pending = make(map[string]*pendingBreakpoint)
	}
	p.bp.pending[bp.Info.ID] = bp
}

// removePendingBreakpoint removes a pending breakpoint
func (p *ProxyServer) removePendingBreakpoint(id string) {
	p.bp.mu.Lock()
	defer p.bp.mu.Unlock()
	delete(p.bp.pending, id)
}

// pendingBreakpointCount returns the number of pending breakpoints
func (p *ProxyServer) pendingBreakpointCount() int {
	p.bp.mu.Lock()
	defer p.bp.mu.Unlock()
	return len(p.bp.pending)
}

// ResolveBreakpoint resolves a pending breakpoint with the user's decision
func (p *ProxyServer) ResolveBreakpoint(id string, resolution BreakpointResolution) error {
	p.bp.mu.Lock()
	bp, ok := p.bp.pending[id]
	p.bp.mu.Unlock()

	if !ok {
		return fmt.Errorf("breakpoint not found: %s", id)
	}

	// Send resolution to the waiting goroutine (non-blocking, channel has buffer of 1)
	select {
	case bp.Ch <- resolution:
		return nil
	default:
		return fmt.Errorf("breakpoint already resolved: %s", id)
	}
}

// GetPendingBreakpoints returns info about all pending breakpoints
func (p *ProxyServer) GetPendingBreakpoints() []PendingBreakpointInfo {
	p.bp.mu.Lock()
	defer p.bp.mu.Unlock()

	result := make([]PendingBreakpointInfo, 0, len(p.bp.pending))
	for _, bp := range p.bp.pending {
		result = append(result, bp.Info)
	}
	return result
}

// ForwardAllBreakpoints resolves all pending breakpoints with auto-forward
func (p *ProxyServer) ForwardAllBreakpoints() {
	p.bp.mu.Lock()
	pending := make([]*pendingBreakpoint, 0, len(p.bp.pending))
	for _, bp := range p.bp.pending {
		pending = append(pending, bp)
	}
	p.bp.mu.Unlock()

	for _, bp := range pending {
		select {
		case bp.Ch <- BreakpointResolution{Action: "forward"}:
		default:
		}
	}
}

// ClearBreakpointRules removes all breakpoint rules and forwards pending breakpoints
func (p *ProxyServer) ClearBreakpointRules() {
	p.ForwardAllBreakpoints()
	p.mu.Lock()
	p.bp.rules = make(map[string]*BreakpointRule)
	p.mu.Unlock()
}

// SetBreakpointHitCallback sets the callback for when a breakpoint is hit
func (p *ProxyServer) SetBreakpointHitCallback(cb func(PendingBreakpointInfo)) {
	p.bp.mu.Lock()
	defer p.bp.mu.Unlock()
	p.bp.onHit = cb
}

// SetBreakpointResolvedCallback sets the callback for when a breakpoint is resolved or times out
func (p *ProxyServer) SetBreakpointResolvedCallback(cb func(id string, reason string)) {
	p.bp.mu.Lock()
	defer p.bp.mu.Unlock()
	p.bp.onResolved = cb
}

// notifyBreakpointHit calls the hit callback in a goroutine
func (p *ProxyServer) notifyBreakpointHit(info PendingBreakpointInfo) {
	p.bp.mu.Lock()
	cb := p.bp.onHit
	p.bp.mu.Unlock()

	if cb != nil {
		go cb(info)
	}
}

// notifyBreakpointResolved calls the resolved callback in a goroutine
func (p *ProxyServer) notifyBreakpointResolved(id string, reason string) {
	p.bp.mu.Lock()
	cb := p.bp.onResolved
	p.bp.mu.Unlock()

	if cb != nil {
		go cb(id, reason)
	}
}

// SetBreakpointCapacityCallback sets the callback invoked when a request matching
// a breakpoint rule is passed through because maxPendingBreakpoints is reached.
func (p *ProxyServer) SetBreakpointCapacityCallback(cb func(ruleID, url, phase string)) {
	p.bp.mu.Lock()
	defer p.bp.mu.Unlock()
	p.bp.onCapacity = cb
}

// notifyBreakpointCapacity records a capacity pass-through and fires the callback
func (p *ProxyServer) notifyBreakpointCapacity(ruleID, url, phase string) {
	p.bp.capacitySkips.Add(1)
	p.bp.mu.Lock()
	cb := p.bp.onCapacity
	p.bp.mu.Unlock()

	if cb != nil {
		go cb(ruleID, url, phase)
	}
}

// BreakpointStats returns counters for capacity pass-throughs and timeout auto-forwards
func (p *ProxyServer) BreakpointStats() (capacitySkips, timeoutForwards int64) {
	return p.bp.capacitySkips.Load(), p.bp.timeoutForwards.Load()
}
