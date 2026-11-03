package event_test

import (
	"fmt"

	"github.com/nbd-wtf/go-nostr"
	"github.com/soprinter/go-sharenote/event"
)

func ExampleInvoice_UnpaidEvent() {
	inv := event.Invoice{
		Height:    843000,
		BlockHash: "000000000000000000023a1f1c9d00000000000000000000000000000000abe9",
		Amount:    312500000,
		Workers:   128,
		Shares:    "58Z42",
	}
	ev, _ := inv.UnpaidEvent()
	// ev is ready for the caller to set CreatedAt and sign.
	fmt.Println(ev.Kind)
	// Output: 35500
}

func ExampleParseShare() {
	s := event.Share{
		Address:   "fc1qminer",
		BlockHash: "000000000000000000023a1f1c9d00000000000000000000000000000000abe9",
		Workers:   []string{"rig-01"},
		Height:    843000,
		Amount:    2441406,
		Shares:    "34Z10",
	}
	ev, _ := s.PendingEvent()

	got, err := event.ParseShare(ev)
	if err != nil {
		fmt.Println("parse error:", err)
		return
	}
	fmt.Println(got.Address, got.Workers[0])
	// Output: fc1qminer rig-01
}

func ExampleParseMinerIdentity() {
	// A relay delivers an untrusted event; verify kind and required tags.
	ev := &nostr.Event{
		Kind: event.KindMinerIdentity,
		Tags: nostr.Tags{
			{"a", "15", "fc1qminer"},
			{"payout", "pplns"},
		},
	}
	m, _ := event.ParseMinerIdentity(ev)
	fmt.Println(m.Addresses[0].ChainID, m.Addresses[0].Address, m.Payout)
	// Output: 15 fc1qminer pplns
}
