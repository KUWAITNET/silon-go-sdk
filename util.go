package silon

import (
	crand "crypto/rand"
	"fmt"
)

// newUUID returns a random (version 4, variant 1) UUID string, used for
// auto-generated Idempotency-Key headers.
func newUUID() string {
	var b [16]byte
	// crypto/rand.Read never fails on supported platforms (Go 1.24+).
	if _, err := crand.Read(b[:]); err != nil {
		panic("silon: crypto/rand failed: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// String returns a pointer to v, for optional string params.
func String(v string) *string { return &v }

// Int returns a pointer to v, for optional int params.
func Int(v int) *int { return &v }

// Bool returns a pointer to v, for optional bool params.
func Bool(v bool) *bool { return &v }

// Float returns a pointer to v, for optional float64 params.
func Float(v float64) *float64 { return &v }
