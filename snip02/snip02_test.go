package snip02

import (
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	original := &Hashrate{
		Address:       "fc1qxyz123",
		TotalHashrate: "5000000000",
		MeanSharenote: "33z53",
		Workers: []Worker{
			{
				Name:                    "antminer-s19",
				Hashrate:                "3000000000",
				Sharenote:               "33z53",
				MeanSharenote:           "34z10",
				CountSharenotes:         150,
				CountRejectedSharenotes: 3,
				MeanTimeSec:             "12.50",
				LastAcceptedUnix:        1712345678,
				UserAgent:               "bmminer/2.0.0",
			},
			{
				Name:             "whatsminer-m30",
				Hashrate:         "2000000000",
				Sharenote:        "33z53",
				MeanSharenote:    "33z80",
				CountSharenotes:  100,
				MeanTimeSec:      "15.00",
				LastAcceptedUnix: 1712345600,
				UserAgent:        "btminer/3.1",
			},
		},
	}

	tags, err := MarshalTags(original)
	if err != nil {
		t.Fatalf("MarshalTags: %v", err)
	}

	parsed, err := UnmarshalTags(tags)
	if err != nil {
		t.Fatalf("UnmarshalTags: %v", err)
	}

	if parsed.Address != original.Address {
		t.Errorf("Address: got %s, want %s", parsed.Address, original.Address)
	}
	if parsed.TotalHashrate != original.TotalHashrate {
		t.Errorf("TotalHashrate: got %s, want %s", parsed.TotalHashrate, original.TotalHashrate)
	}
	if parsed.MeanSharenote != original.MeanSharenote {
		t.Errorf("MeanSharenote: got %s, want %s", parsed.MeanSharenote, original.MeanSharenote)
	}
	if len(parsed.Workers) != 2 {
		t.Fatalf("Workers count: got %d, want 2", len(parsed.Workers))
	}

	w := parsed.Workers[0]
	if w.Name != "antminer-s19" {
		t.Errorf("Worker[0].Name: got %s, want antminer-s19", w.Name)
	}
	if w.Hashrate != "3000000000" {
		t.Errorf("Worker[0].Hashrate: got %s, want 3000000000", w.Hashrate)
	}
	if w.CountSharenotes != 150 {
		t.Errorf("Worker[0].CountSharenotes: got %d, want 150", w.CountSharenotes)
	}
	if w.CountRejectedSharenotes != 3 {
		t.Errorf("Worker[0].CountRejectedSharenotes: got %d, want 3", w.CountRejectedSharenotes)
	}
	if w.LastAcceptedUnix != 1712345678 {
		t.Errorf("Worker[0].LastAcceptedUnix: got %d, want 1712345678", w.LastAcceptedUnix)
	}
}

func TestSingleWorkerMode(t *testing.T) {
	hr := &Hashrate{
		Address:       "fc1qaddr",
		Hashrate:      "1500000000",
		MeanSharenote: "33z53",
	}

	tags, err := MarshalTags(hr)
	if err != nil {
		t.Fatalf("MarshalTags: %v", err)
	}

	// Should have "h" tag, not "all".
	foundH := false
	foundAll := false
	for _, tag := range tags {
		if tag[0] == "h" {
			foundH = true
		}
		if tag[0] == "all" {
			foundAll = true
		}
	}
	if !foundH {
		t.Error("expected 'h' tag for single-worker mode")
	}
	if foundAll {
		t.Error("unexpected 'all' tag in single-worker mode")
	}

	parsed, err := UnmarshalTags(tags)
	if err != nil {
		t.Fatalf("UnmarshalTags: %v", err)
	}
	if parsed.Hashrate != "1500000000" {
		t.Errorf("Hashrate: got %s, want 1500000000", parsed.Hashrate)
	}
	if parsed.MeanSharenote != "33z53" {
		t.Errorf("MeanSharenote: got %s, want 33z53", parsed.MeanSharenote)
	}
}

func TestMarshalMissingAddress(t *testing.T) {
	_, err := MarshalTags(&Hashrate{})
	if err == nil {
		t.Fatal("expected error for missing address")
	}
}

func TestMarshalNilHashrate(t *testing.T) {
	_, err := MarshalTags(nil)
	if err == nil {
		t.Fatal("expected error for nil hashrate")
	}
}

func TestUnmarshalMissingAddress(t *testing.T) {
	tags := nostr.Tags{nostr.Tag{"all", "1000"}}
	_, err := UnmarshalTags(tags)
	if err == nil {
		t.Fatal("expected error for missing address")
	}
}

func TestWorkerSubfields(t *testing.T) {
	tags := nostr.Tags{
		nostr.Tag{"a", "fc1qaddr"},
		nostr.Tag{"all", "5000"},
		nostr.Tag{"w:rig01", "h:4123", "sn:33z55", "msn:34z10", "csn:50", "crsn:2", "mt:10.00", "lsn:1712345678", "ua:bmminer/2.0"},
	}

	hr, err := UnmarshalTags(tags)
	if err != nil {
		t.Fatalf("UnmarshalTags: %v", err)
	}
	if len(hr.Workers) != 1 {
		t.Fatalf("Workers count: got %d, want 1", len(hr.Workers))
	}

	w := hr.Workers[0]
	if w.Name != "rig01" {
		t.Errorf("Name: got %s, want rig01", w.Name)
	}
	if w.Hashrate != "4123" {
		t.Errorf("Hashrate: got %s, want 4123", w.Hashrate)
	}
	if w.Sharenote != "33z55" {
		t.Errorf("Sharenote: got %s, want 33z55", w.Sharenote)
	}
	if w.MeanSharenote != "34z10" {
		t.Errorf("MeanSharenote: got %s, want 34z10", w.MeanSharenote)
	}
	if w.CountSharenotes != 50 {
		t.Errorf("CountSharenotes: got %d, want 50", w.CountSharenotes)
	}
	if w.CountRejectedSharenotes != 2 {
		t.Errorf("CountRejectedSharenotes: got %d, want 2", w.CountRejectedSharenotes)
	}
	if w.MeanTimeSec != "10.00" {
		t.Errorf("MeanTimeSec: got %s, want 10.00", w.MeanTimeSec)
	}
	if w.LastAcceptedUnix != 1712345678 {
		t.Errorf("LastAcceptedUnix: got %d, want 1712345678", w.LastAcceptedUnix)
	}
	if w.UserAgent != "bmminer/2.0" {
		t.Errorf("UserAgent: got %s, want bmminer/2.0", w.UserAgent)
	}
}

func TestNewHashrateEvent(t *testing.T) {
	hr := &Hashrate{
		Address:       "fc1qaddr",
		TotalHashrate: "5000",
		Workers: []Worker{
			{Name: "rig01", Hashrate: "5000"},
		},
	}

	ev, err := NewHashrateEvent(hr)
	if err != nil {
		t.Fatalf("NewHashrateEvent: %v", err)
	}
	if ev.Kind != KindHashrate {
		t.Errorf("Kind: got %d, want %d", ev.Kind, KindHashrate)
	}
}

func TestParseHashrateEventWrongKind(t *testing.T) {
	ev := &nostr.Event{Kind: 99999, Tags: nostr.Tags{nostr.Tag{"a", "addr"}, nostr.Tag{"all", "100"}}}
	_, err := ParseHashrateEvent(ev)
	if err == nil {
		t.Fatal("expected error for wrong kind")
	}
}

func TestCrsnOmittedWhenZero(t *testing.T) {
	hr := &Hashrate{
		Address:       "fc1qaddr",
		TotalHashrate: "1000",
		Workers: []Worker{
			{Name: "rig01", Hashrate: "1000", CountSharenotes: 10, CountRejectedSharenotes: 0},
		},
	}

	tags, err := MarshalTags(hr)
	if err != nil {
		t.Fatalf("MarshalTags: %v", err)
	}

	for _, tag := range tags {
		for _, field := range tag {
			if field == "crsn:0" {
				t.Error("crsn:0 should be omitted when zero")
			}
		}
	}
}
