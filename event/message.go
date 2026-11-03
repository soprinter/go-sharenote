package event

import (
	"errors"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
)

// OperatorMessage is an optional encrypted operator<->miner direct message (kind
// 35515). Its content is an already-encrypted payload (NIP-44, or legacy
// NIP-04) that this package passes through unchanged — encryption is the
// caller's responsibility. The event is addressable by a conversation thread so
// a console can group and replace the current message in a thread.
//
// NIP-SN rule A17 makes this kind optional: a consumer that does not implement
// operator messaging must ignore it rather than fail.
type OperatorMessage struct {
	// Content is the encrypted ciphertext of the message, carried verbatim.
	Content string
	// Counterparty is the recipient's pubkey ("p" tag).
	Counterparty string
	// ThreadID is the stable conversation identifier; the addressable "d" tag.
	ThreadID string
	// ChainID and Address optionally name the miner address the thread concerns
	// ("a" context tag). Both must be set together, or neither.
	ChainID string
	Address string
	// Subject is an optional thread subject ("subject" tag).
	Subject string
}

// Event builds the kind 35515 event. The Content is set as the (already
// encrypted) event content. It errors if the content, counterparty, or thread
// ID is empty.
func (m OperatorMessage) Event() (*nostr.Event, error) {
	if m.Content == "" {
		return nil, errors.New("event: operator message content must be encrypted and non-empty")
	}
	if m.Counterparty == "" {
		return nil, errors.New("event: operator message needs a counterparty")
	}
	if m.ThreadID == "" {
		return nil, errors.New("event: operator message needs a thread ID")
	}
	tags := nostr.Tags{
		{"p", m.Counterparty},
		{"d", m.ThreadID},
	}
	if m.ChainID != "" || m.Address != "" {
		tags = append(tags, nostr.Tag{"a", m.ChainID, m.Address})
	}
	if m.Subject != "" {
		tags = append(tags, nostr.Tag{"subject", m.Subject})
	}
	return &nostr.Event{Kind: KindOperatorMessage, Content: m.Content, Tags: tags}, nil
}

// ParseOperatorMessage extracts an [OperatorMessage] from a kind 35515 event,
// checking the kind and the required single "p" and "d" tags.
func ParseOperatorMessage(ev *nostr.Event) (OperatorMessage, error) {
	if ev == nil {
		return OperatorMessage{}, ErrNilEvent
	}
	if ev.Kind != KindOperatorMessage {
		return OperatorMessage{}, fmt.Errorf("event: expected kind %d, got %d", KindOperatorMessage, ev.Kind)
	}
	var (
		m   OperatorMessage
		err error
	)
	m.Content = ev.Content
	if m.Counterparty, err = requireString(ev.Tags, "p"); err != nil {
		return OperatorMessage{}, err
	}
	if m.ThreadID, err = requireString(ev.Tags, "d"); err != nil {
		return OperatorMessage{}, err
	}
	if a := findTag(ev.Tags, "a"); a != nil {
		if len(a) > 1 {
			m.ChainID = a[1]
		}
		if len(a) > 2 {
			m.Address = a[2]
		}
	}
	m.Subject = optionalString(ev.Tags, "subject")
	return m, nil
}

// Validate applies NIP-SN rule A17: the content must be present (encrypted), and
// exactly one counterparty and one thread ID must be set.
func (m OperatorMessage) Validate() error {
	if m.Content == "" {
		return errors.New("event: operator message content must be encrypted and non-empty")
	}
	if m.Counterparty == "" {
		return errors.New("event: operator message needs a counterparty")
	}
	if m.ThreadID == "" {
		return errors.New("event: operator message needs a thread ID")
	}
	return nil
}
