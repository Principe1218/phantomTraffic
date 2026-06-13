package idgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentID_CreatesThenPersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.id")

	first, err := AgentID(path)
	if err != nil {
		t.Fatalf("first AgentID: %v", err)
	}
	if len(first) != 22 { // 16 bytes RawURLEncoding = 22 chars
		t.Fatalf("AgentID len = %d, want 22 (id=%q)", len(first), first)
	}
	if !isURLSafe(first) {
		t.Fatalf("AgentID %q is not URL-safe", first)
	}

	// File must exist and contain exactly the id (trailing newline tolerated).
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted file: %v", err)
	}
	if strings.TrimSpace(string(raw)) != first {
		t.Fatalf("persisted file = %q, want %q", strings.TrimSpace(string(raw)), first)
	}

	// Second call on the same path must reload the SAME id (stable per host).
	second, err := AgentID(path)
	if err != nil {
		t.Fatalf("second AgentID: %v", err)
	}
	if second != first {
		t.Fatalf("AgentID not stable: first=%q second=%q", first, second)
	}
}

func TestAgentID_DistinctPathsDiffer(t *testing.T) {
	dir := t.TempDir()
	a, err := AgentID(filepath.Join(dir, "a.id"))
	if err != nil {
		t.Fatalf("AgentID a: %v", err)
	}
	b, err := AgentID(filepath.Join(dir, "b.id"))
	if err != nil {
		t.Fatalf("AgentID b: %v", err)
	}
	if a == b {
		t.Fatalf("distinct hosts produced identical AgentID %q", a)
	}
}

func TestAgentID_RejectsCorruptFile(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		content string
	}{
		{"empty", ""},
		{"whitespace", "   \n"},
		{"bad charset", "not/url+safe=="},
		{"too short", "abc"},
		{"too long", strings.Repeat("A", 64)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".id")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatalf("seed corrupt file: %v", err)
			}
			if _, err := AgentID(path); err == nil {
				t.Fatalf("AgentID accepted corrupt file %q", tc.content)
			}
		})
	}
}

func TestAgentID_AtomicWriteLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.id")
	if _, err := AgentID(path); err != nil {
		t.Fatalf("AgentID: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected exactly the id file, got %d entries: %v", len(entries), names)
	}
	if entries[0].Name() != "agent.id" {
		t.Fatalf("unexpected file left behind: %q", entries[0].Name())
	}
}
