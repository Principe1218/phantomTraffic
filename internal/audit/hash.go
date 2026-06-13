package audit

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sort"
)

// genesisHash is the PrevHash of the first record (Seq 0). It is the hex SHA-256
// of the fixed domain-separation string below — a constant, not a secret.
var genesisHash = hex.EncodeToString(func() []byte {
	h := sha256.Sum256([]byte("phantomtraffic.audit.genesis.v1"))
	return h[:]
}())

// writeField appends a length-prefixed field to the running hash. The 8-byte
// big-endian length prefix makes the encoding unambiguous: ("a","bc") and
// ("ab","c") hash differently because the boundaries are explicit.
func writeField(h interface{ Write([]byte) (int, error) }, b []byte) {
	var lp [8]byte
	binary.BigEndian.PutUint64(lp[:], uint64(len(b)))
	_, _ = h.Write(lp[:])
	_, _ = h.Write(b)
}

// recordHash computes the canonical SHA-256 digest of a record over ALL fields,
// including PrevHash, in a fixed order with Detail keys sorted. This is the chain
// link: any byte change to any field (or to the previous record, via PrevHash)
// yields a different digest (design §5.5).
func recordHash(r Record) string {
	h := sha256.New()

	var seqBuf [8]byte
	binary.BigEndian.PutUint64(seqBuf[:], r.Seq)
	writeField(h, seqBuf[:])

	// UnixNano fully captures the instant; encode as fixed 8 bytes.
	var timeBuf [8]byte
	binary.BigEndian.PutUint64(timeBuf[:], uint64(r.Time.UTC().UnixNano()))
	writeField(h, timeBuf[:])

	writeField(h, []byte(r.AgentID))
	writeField(h, []byte(r.Actor))
	writeField(h, []byte(r.Action))
	writeField(h, []byte(r.Resource))

	// Detail: sort keys for order-independence; prefix the count.
	var countBuf [8]byte
	binary.BigEndian.PutUint64(countBuf[:], uint64(len(r.Detail)))
	writeField(h, countBuf[:])
	keys := make([]string, 0, len(r.Detail))
	for k := range r.Detail {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		writeField(h, []byte(k))
		writeField(h, []byte(r.Detail[k]))
	}

	writeField(h, []byte(r.PrevHash))

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

// HashRecordForTest exposes recordHash for white-box tests in the audit_test
// package. It is not part of the production API surface.
func HashRecordForTest(r Record) string { return recordHash(r) }
