package event

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
)

const workerPrefix = "w:"

// Worker is one mining worker's telemetry snapshot, carried as a "w:<name>" tag
// of "key:value" sub-fields. Hashrate ("h:") is required; the rest are optional,
// and CountRejected is omitted when zero.
type Worker struct {
	// Name is the worker identifier, used as the tag key suffix.
	Name string
	// Hashrate is the worker's EMA-adjusted hash rate in H/s, as a string.
	Hashrate string
	// MinDenomination is the minimum difficulty actively assigned ("sn:").
	MinDenomination string
	// MeanSharenote is the mean difficulty achieved ("msn:").
	MeanSharenote string
	// CountAccepted is the count of accepted sharenotes ("csn:").
	CountAccepted uint64
	// CountRejected is the count of rejected sharenotes ("crsn:"); omitted when 0.
	CountRejected uint64
	// MeanTimeSec is the mean seconds between accepted shares ("mt:").
	MeanTimeSec string
	// LastAcceptedUnix is the Unix timestamp of the last valid share ("lsn:").
	LastAcceptedUnix int64
	// UserAgent is the mining software's user agent ("ua:").
	UserAgent string
}

// Telemetry is a miner's live hashrate snapshot (kind 35502). All data travels
// in tags, so the event content is empty; the event carries no "d" tag, so a
// miner's latest telemetry replaces the prior one. Exactly one of TotalHashrate
// (aggregate "all" tag) or SingleHashrate (single-worker "h" tag) must be set.
type Telemetry struct {
	// Address is the chain address of the mining operation ("a" tag, required).
	Address string
	// TotalHashrate is the aggregate EMA-adjusted hash rate across all workers,
	// in H/s. When set it produces the "all" tag.
	TotalHashrate string
	// SingleHashrate is one worker's hash rate when there is no aggregate. When
	// TotalHashrate is empty and this is set it produces the "h" tag.
	SingleHashrate string
	// MeanSharenote is the inline mean-sharenote label on the "all"/"h" tag.
	MeanSharenote string
	// Workers are the per-worker snapshots, emitted in the given order.
	Workers []Worker
}

// Event builds the kind 35502 event. Content is empty. It errors if Address is
// missing or if neither TotalHashrate nor SingleHashrate is set.
func (t Telemetry) Event() (*nostr.Event, error) {
	tags, err := t.tags()
	if err != nil {
		return nil, err
	}
	return &nostr.Event{Kind: KindHashrate, Tags: tags}, nil
}

func (t Telemetry) tags() (nostr.Tags, error) {
	if t.Address == "" {
		return nil, errors.New("event: telemetry address is required")
	}
	if t.TotalHashrate == "" && t.SingleHashrate == "" {
		return nil, errors.New("event: telemetry needs either a total or a single hashrate")
	}
	tags := make(nostr.Tags, 0, 2+len(t.Workers))
	tags = append(tags, nostr.Tag{"a", t.Address})

	var rate nostr.Tag
	if t.TotalHashrate != "" {
		rate = nostr.Tag{"all", t.TotalHashrate}
	} else {
		rate = nostr.Tag{"h", t.SingleHashrate}
	}
	if t.MeanSharenote != "" {
		rate = append(rate, "msn:"+t.MeanSharenote)
	}
	tags = append(tags, rate)

	// Worker tags are sorted alphabetically by name for determinism (NIP-SN rule
	// A15 / decision D6). The input slice is left untouched.
	workers := append([]Worker(nil), t.Workers...)
	sort.SliceStable(workers, func(i, j int) bool { return workers[i].Name < workers[j].Name })
	for _, w := range workers {
		if tag := w.tag(); tag != nil {
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

func (w Worker) tag() nostr.Tag {
	name := strings.TrimSpace(w.Name)
	if name == "" {
		return nil
	}
	tag := nostr.Tag{workerPrefix + name}
	if w.Hashrate != "" {
		tag = append(tag, "h:"+w.Hashrate)
	}
	if w.MinDenomination != "" {
		tag = append(tag, "sn:"+w.MinDenomination)
	}
	if w.MeanSharenote != "" {
		tag = append(tag, "msn:"+w.MeanSharenote)
	}
	tag = append(tag, "csn:"+strconv.FormatUint(w.CountAccepted, 10))
	if w.CountRejected > 0 {
		tag = append(tag, "crsn:"+strconv.FormatUint(w.CountRejected, 10))
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

// ParseTelemetry extracts a [Telemetry] from a kind 35502 event, checking the
// kind and that the required "a" tag is present.
func ParseTelemetry(ev *nostr.Event) (Telemetry, error) {
	if ev == nil {
		return Telemetry{}, ErrNilEvent
	}
	if ev.Kind != KindHashrate {
		return Telemetry{}, fmt.Errorf("event: expected kind %d, got %d", KindHashrate, ev.Kind)
	}
	var t Telemetry
	for _, tag := range ev.Tags {
		if len(tag) == 0 {
			continue
		}
		key := tag[0]
		switch {
		case key == "a" && len(tag) >= 2:
			t.Address = tag[1]
		case key == "all" && len(tag) >= 2:
			t.TotalHashrate = tag[1]
			if msn := inlineField(tag[2:], "msn:"); msn != "" {
				t.MeanSharenote = msn
			}
		case key == "h" && len(tag) >= 2:
			t.SingleHashrate = tag[1]
			if t.MeanSharenote == "" {
				t.MeanSharenote = inlineField(tag[2:], "msn:")
			}
		case strings.HasPrefix(key, workerPrefix):
			name := strings.TrimPrefix(key, workerPrefix)
			if name == "" {
				continue
			}
			t.Workers = append(t.Workers, parseWorker(name, tag[1:]))
		}
	}
	if t.Address == "" {
		return Telemetry{}, errors.New("event: telemetry is missing its \"a\" tag")
	}
	return t, nil
}

func parseWorker(name string, fields []string) Worker {
	w := Worker{Name: name}
	for _, f := range fields {
		k, v, ok := strings.Cut(f, ":")
		if !ok {
			continue
		}
		switch k {
		case "h":
			w.Hashrate = v
		case "sn":
			w.MinDenomination = v
		case "msn":
			w.MeanSharenote = v
		case "csn":
			w.CountAccepted, _ = strconv.ParseUint(v, 10, 64)
		case "crsn":
			w.CountRejected, _ = strconv.ParseUint(v, 10, 64)
		case "mt":
			w.MeanTimeSec = v
		case "lsn":
			w.LastAcceptedUnix, _ = strconv.ParseInt(v, 10, 64)
		case "ua":
			w.UserAgent = v
		}
	}
	return w
}

func inlineField(fields []string, prefix string) string {
	for _, f := range fields {
		if strings.HasPrefix(f, prefix) {
			return strings.TrimPrefix(f, prefix)
		}
	}
	return ""
}

// Validate applies NIP-SN rule A15: exactly one "a" tag and exactly one of the
// "all" or "h" hashrate tags, and every worker must report a hashrate.
func (t Telemetry) Validate() error {
	if t.Address == "" {
		return errors.New("event: telemetry address is required")
	}
	if (t.TotalHashrate == "") == (t.SingleHashrate == "") {
		return errors.New("event: telemetry needs exactly one of a total or single hashrate")
	}
	for _, w := range t.Workers {
		if w.Hashrate == "" {
			return fmt.Errorf("event: worker %q is missing its hashrate", w.Name)
		}
	}
	return nil
}
