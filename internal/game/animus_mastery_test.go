package game

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// The blob is a bare sequence of PropStream strings: uint16 little-endian length
// followed by the bytes, with no count header. This is what
// AnimusMastery::serialize writes and what the C++ server reads back, so the
// exact bytes are the compatibility contract.
func TestAnimusMasterySerializeGolden(t *testing.T) {
	am := NewAnimusMastery()
	am.Add(1, "Rat") // stored lowercase
	am.Add(2, "dragon")

	// sorted: "dragon" then "rat"
	// 06 00 "dragon"  03 00 "rat"
	want, _ := hex.DecodeString("0600" + hex.EncodeToString([]byte("dragon")) +
		"0300" + hex.EncodeToString([]byte("rat")))

	got := am.Serialize()
	if !bytes.Equal(got, want) {
		t.Errorf("got %x want %x", got, want)
	}
}

func TestAnimusMasteryRoundTrip(t *testing.T) {
	am := NewAnimusMastery()
	am.Add(10, "dragon lord")
	am.Add(20, "Demon")
	am.Add(30, "rat")

	lookup := func(name string) (uint16, bool) {
		switch name {
		case "dragon lord":
			return 10, true
		case "demon":
			return 20, true
		case "rat":
			return 30, true
		}
		return 0, false
	}

	back := UnserializeAnimusMastery(am.Serialize(), lookup)
	if back.Count() != 3 {
		t.Fatalf("expected 3 masteries, got %d", back.Count())
	}
	for _, raceID := range []uint16{10, 20, 30} {
		if !back.Has(raceID) {
			t.Errorf("race %d missing after round trip", raceID)
		}
	}
	// Re-serializing must be byte-stable.
	if !bytes.Equal(am.Serialize(), back.Serialize()) {
		t.Errorf("re-serialization differs:\n%x\n%x", am.Serialize(), back.Serialize())
	}
}

// A name the registry does not know must be preserved rather than dropped, so a
// blob written by the C++ server survives a Go load/save cycle intact.
func TestAnimusMasteryUnknownNamePreserved(t *testing.T) {
	src := NewAnimusMastery()
	src.Add(0, "some unregistered monster")
	blob := src.Serialize()

	back := UnserializeAnimusMastery(blob, func(string) (uint16, bool) { return 0, false })
	if got := back.Serialize(); !bytes.Equal(got, blob) {
		t.Errorf("blob not preserved: got %x want %x", got, blob)
	}
}

func TestAnimusMasteryEmpty(t *testing.T) {
	if got := NewAnimusMastery().Serialize(); len(got) != 0 {
		t.Errorf("expected empty blob, got %x", got)
	}
	if am := UnserializeAnimusMastery(nil, nil); am.Count() != 0 {
		t.Errorf("expected empty tracker, got %d", am.Count())
	}
}

// A truncated blob must stop cleanly instead of panicking, matching the
// read-until-false loop in C++.
func TestAnimusMasteryTruncatedBlob(t *testing.T) {
	// Claims a 9-byte string but supplies 3.
	blob, _ := hex.DecodeString("0900" + hex.EncodeToString([]byte("rat")))
	am := UnserializeAnimusMastery(blob, nil)
	if am.Count() != 0 {
		t.Errorf("expected the truncated entry to be dropped, got %d", am.Count())
	}
}
