package idgen

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// agentIDBytes is 128 bits of entropy for the stable per-host AgentID.
const agentIDBytes = 16

// agentIDLen is the RawURLEncoding length of agentIDBytes (ceil(16*8/6) = 22).
const agentIDLen = 22

// AgentID returns this host's stable per-agent identity, loading it from path if a
// valid id is already persisted there, otherwise minting a fresh crypto/rand id and
// writing it atomically. The id is stamped on every audit/stat record so a
// multi-agent run is reconstructable post-hoc (design §7).
func AgentID(path string) (string, error) {
	if existing, ok, err := loadAgentID(path); err != nil {
		return "", err
	} else if ok {
		return existing, nil
	}

	id, err := token(agentIDBytes)
	if err != nil {
		return "", err
	}
	if err := writeAtomic(path, id); err != nil {
		return "", err
	}
	return id, nil
}

// loadAgentID reads and validates a persisted AgentID. ok==false with nil error
// means "no file yet" (mint a new one); a present-but-invalid file is a hard error
// so a corrupt id is never silently trusted.
func loadAgentID(path string) (id string, ok bool, err error) {
	// #nosec G304 -- path is the controlled AgentID persistence location passed by
	// the caller (an API parameter), never user-supplied request data.
	raw, readErr := os.ReadFile(path)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("idgen: read agent id %q: %w", path, readErr)
	}
	candidate := strings.TrimSpace(string(raw))
	if err := validateAgentID(candidate); err != nil {
		return "", false, fmt.Errorf("idgen: persisted agent id %q is invalid: %w", path, err)
	}
	return candidate, true, nil
}

// validateAgentID enforces the exact length and URL-safe charset of a minted id,
// rejecting anything that did not come from token(agentIDBytes).
func validateAgentID(s string) error {
	if len(s) != agentIDLen {
		return fmt.Errorf("length %d, want %d", len(s), agentIDLen)
	}
	if _, err := base64.RawURLEncoding.DecodeString(s); err != nil {
		return fmt.Errorf("not URL-safe base64: %w", err)
	}
	return nil
}

// writeAtomic persists id durably: it creates a temp file in the SAME directory,
// fsyncs it, then renames it over path. A crash or concurrent run can therefore
// only ever observe the old file or the fully-written new one, never a torn id.
func writeAtomic(path, id string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("idgen: create agent id dir %q: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".agentid-*.tmp")
	if err != nil {
		return fmt.Errorf("idgen: create temp agent id: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if we return before the rename succeeds.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.WriteString(id + "\n"); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("idgen: write temp agent id: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("idgen: fsync temp agent id: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("idgen: close temp agent id: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil { // #nosec G703 -- tmpName is the os.CreateTemp result in the same dir; path is the controlled AgentID persistence location, not user-supplied input
		return fmt.Errorf("idgen: rename agent id into place: %w", err)
	}
	return nil
}
