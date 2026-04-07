package snip02

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
)

const KindHashrate = 35502

const workerTagPrefix = "w:"

// Worker represents a single mining worker's telemetry snapshot.
type Worker struct {
	Name                    string
	Hashrate                string // H/s as decimal string
	Sharenote               string // minimum difficulty label
	MeanSharenote           string
	CountSharenotes         uint64
	CountRejectedSharenotes uint64
	MeanTimeSec             string // seconds between accepted shares
	LastAcceptedUnix        int64  // unix timestamp of last valid share
	UserAgent               string
}

// Hashrate represents a complete SNIP-02 telemetry broadcast.
type Hashrate struct {
	Address       string   // "a" tag
	TotalHashrate string   // "all" tag (empty if single-worker)
	MeanSharenote string  // inline msn label on "all" or "h" tag
	Hashrate      string   // "h" tag (single-worker mode)
	Workers       []Worker // "w:" prefixed worker entries
}

// NewHashrateEvent builds a nostr.Event of kind 35502.
func NewHashrateEvent(hr *Hashrate) (*nostr.Event, error) {
	t, err := MarshalTags(hr)
	if err != nil {
		return nil, err
	}
	return &nostr.Event{
		Kind: KindHashrate,
		Tags: t,
	}, nil
}

// MarshalTags converts a Hashrate into its tag representation.
func MarshalTags(hr *Hashrate) (nostr.Tags, error) {
	if hr == nil {
		return nil, fmt.Errorf("hashrate is nil")
	}
	if hr.Address == "" {
		return nil, fmt.Errorf("address is required")
	}

	t := make(nostr.Tags, 0, 2+len(hr.Workers))
	t = append(t, nostr.Tag{"a", hr.Address})

	if hr.TotalHashrate != "" {
		tag := nostr.Tag{"all", hr.TotalHashrate}
		if hr.MeanSharenote != "" {
			tag = append(tag, "msn:"+hr.MeanSharenote)
		}
		t = append(t, tag)
	} else if hr.Hashrate != "" {
		tag := nostr.Tag{"h", hr.Hashrate}
		if hr.MeanSharenote != "" {
			tag = append(tag, "msn:"+hr.MeanSharenote)
		}
		t = append(t, tag)
	}

	for _, w := range hr.Workers {
		tag := marshalWorker(w)
		if tag != nil {
			t = append(t, tag)
		}
	}

	return t, nil
}

func marshalWorker(w Worker) nostr.Tag {
	name := strings.TrimSpace(w.Name)
	if name == "" {
		return nil
	}

	tag := nostr.Tag{workerTagPrefix + name}
	if w.Hashrate != "" {
		tag = append(tag, "h:"+w.Hashrate)
	}
	if w.Sharenote != "" {
		tag = append(tag, "sn:"+w.Sharenote)
	}
	if w.MeanSharenote != "" {
		tag = append(tag, "msn:"+w.MeanSharenote)
	}
	tag = append(tag, "csn:"+strconv.FormatUint(w.CountSharenotes, 10))
	if w.CountRejectedSharenotes > 0 {
		tag = append(tag, "crsn:"+strconv.FormatUint(w.CountRejectedSharenotes, 10))
	}
	if w.MeanTimeSec != "" {
		tag = append(tag, "mt:"+w.MeanTimeSec)
	}
	if w.LastAcceptedUnix != 0 {
		tag = append(tag, "lsn:"+strconv.FormatInt(w.LastAcceptedUnix, 10))
	}
	if w.UserAgent != "" {
		tag = append(tag, "ua:"+w.UserAgent)
	}
	return tag
}

// ParseHashrateEvent extracts a Hashrate from a nostr.Event.
func ParseHashrateEvent(ev *nostr.Event) (*Hashrate, error) {
	if ev == nil {
		return nil, fmt.Errorf("event is nil")
	}
	if ev.Kind != KindHashrate {
		return nil, fmt.Errorf("expected kind %d, got %d", KindHashrate, ev.Kind)
	}
	return UnmarshalTags(ev.Tags)
}

// UnmarshalTags parses tags into a Hashrate.
func UnmarshalTags(t nostr.Tags) (*Hashrate, error) {
	hr := &Hashrate{}

	for _, tag := range t {
		if len(tag) == 0 {
			continue
		}
		key := tag[0]

		switch {
		case key == "a":
			if len(tag) < 2 {
				continue
			}
			hr.Address = tag[1]

		case key == "all":
			if len(tag) < 2 {
				continue
			}
			hr.TotalHashrate = tag[1]
			hr.MeanSharenote = extractInlineMSN(tag[2:])

		case key == "h":
			if len(tag) < 2 {
				continue
			}
			hr.Hashrate = tag[1]
			if hr.MeanSharenote == "" {
				hr.MeanSharenote = extractInlineMSN(tag[2:])
			}

		case strings.HasPrefix(key, workerTagPrefix):
			name := strings.TrimPrefix(key, workerTagPrefix)
			if name == "" {
				continue
			}
			w := Worker{Name: name}
			for _, field := range tag[1:] {
				parts := strings.SplitN(field, ":", 2)
				if len(parts) != 2 {
					continue
				}
				switch parts[0] {
				case "h":
					w.Hashrate = parts[1]
				case "sn":
					w.Sharenote = parts[1]
				case "msn":
					w.MeanSharenote = parts[1]
				case "csn":
					if v, err := strconv.ParseUint(parts[1], 10, 64); err == nil {
						w.CountSharenotes = v
					}
				case "crsn":
					if v, err := strconv.ParseUint(parts[1], 10, 64); err == nil {
						w.CountRejectedSharenotes = v
					}
				case "mt":
					w.MeanTimeSec = parts[1]
				case "lsn":
					if v, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
						w.LastAcceptedUnix = v
					}
				case "ua":
					w.UserAgent = parts[1]
				}
			}
			hr.Workers = append(hr.Workers, w)
		}
	}

	if hr.Address == "" {
		return nil, fmt.Errorf("missing address tag")
	}

	return hr, nil
}

func extractInlineMSN(fields []string) string {
	for _, f := range fields {
		if strings.HasPrefix(f, "msn:") {
			return strings.TrimPrefix(f, "msn:")
		}
	}
	return ""
}
