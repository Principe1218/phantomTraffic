package scenario

import (
	"fmt"
	"os"

	yaml "go.yaml.in/yaml/v3"

	"github.com/Principe1218/phantomTraffic/internal/pterr"
)

// maxFileBytes is the hard size cap on a scenario file (1 MiB). It bounds the
// decode-time memory/CPU a single file may demand before any parsing begins
// (AGENTS.md §5.2: validate at the entry point).
const maxFileBytes = 1 << 20

// Load reads and strictly decodes a scenario file at path into a Raw.
//
// It rejects files larger than maxFileBytes, decodes with KnownFields(true) so
// unknown/typo'd keys are rejected, and wraps any size or decode failure as a
// *pterr.Error of class ClassConfig. A missing file is surfaced unwrapped so
// errors.Is(err, os.ErrNotExist) holds for the caller.
//
// Load does NOT validate the contents (required fields, enums, targets, caps):
// that is the pure Validate function in Module 5. Load only loads.
func Load(path string) (Raw, error) {
	info, err := os.Stat(path)
	if err != nil {
		// Includes the not-exist case; return it unmasked so errors.Is(os.ErrNotExist) holds.
		return Raw{}, err
	}
	if info.Size() > maxFileBytes {
		return Raw{}, pterr.New(
			pterr.ClassConfig,
			"scenario.file_too_large",
			"scenario.Load",
			fmt.Sprintf("scenario file exceeds the %d-byte limit", maxFileBytes),
		)
	}

	f, err := os.Open(path)
	if err != nil {
		// A file that vanished between Stat and Open: surface unmasked too.
		return Raw{}, err
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	var raw Raw
	if err := dec.Decode(&raw); err != nil {
		return Raw{}, pterr.Wrap(
			pterr.ClassConfig,
			"scenario.decode_failed",
			"scenario.Load",
			"scenario file failed to decode",
			err,
		)
	}
	return raw, nil
}
