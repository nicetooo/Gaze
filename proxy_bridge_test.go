package main

import (
	"os"
	"path/filepath"
	"testing"
)

// resetMockRulesState restores the package-level mock rules state so the test
// cannot leak into other tests in this package.
func resetMockRulesState(t *testing.T) {
	t.Helper()
	mockRulesMu.Lock()
	mockRules = make(map[string]*MockRule)
	mockRulesLoadFailed = false
	mockRulesMu.Unlock()
	t.Cleanup(func() {
		mockRulesMu.Lock()
		mockRules = make(map[string]*MockRule)
		mockRulesLoadFailed = false
		mockRulesMu.Unlock()
	})
}

func TestLoadMockRulesCorruptQuarantineAndRefuseSave(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	resetMockRulesState(t)

	dir := filepath.Join(tmpHome, ".adbGUI")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	path := filepath.Join(dir, "mock_rules.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	app := &App{}
	if err := app.LoadMockRules(); err == nil {
		t.Fatal("expected parse error from corrupt rules file")
	}

	// The corrupt file must be preserved under a .corrupt-<ts> name
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("corrupt file should have been quarantined, stat err: %v", err)
	}
	matches, _ := filepath.Glob(path + ".corrupt-*")
	if len(matches) != 1 {
		t.Fatalf("expected 1 quarantined file, got %d", len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil || string(data) != "{not valid json" {
		t.Errorf("quarantined content lost: %q, err: %v", data, err)
	}

	// Full-overwrite save must be refused until restart
	mockRulesMu.Lock()
	saveErr := saveMockRules()
	mockRulesMu.Unlock()
	if saveErr == nil {
		t.Error("expected saveMockRules to refuse after load failure")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("refused save must not create a rules file, stat err: %v", err)
	}
}

func TestSaveMockRulesAtomicWritesFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	resetMockRulesState(t)

	mockRulesMu.Lock()
	mockRules["r1"] = &MockRule{ID: "r1", URLPattern: "*/api/*", StatusCode: 200, CreatedAt: 1}
	saveErr := saveMockRules()
	mockRulesMu.Unlock()
	if saveErr != nil {
		t.Fatalf("saveMockRules failed: %v", saveErr)
	}

	app := &App{}
	mockRulesMu.Lock()
	mockRules = make(map[string]*MockRule)
	mockRulesMu.Unlock()
	if err := app.LoadMockRules(); err != nil {
		t.Fatalf("LoadMockRules failed: %v", err)
	}

	mockRulesMu.RLock()
	defer mockRulesMu.RUnlock()
	if len(mockRules) != 1 || mockRules["r1"] == nil {
		t.Errorf("expected rule r1 to round-trip, got %d rules", len(mockRules))
	}
}
