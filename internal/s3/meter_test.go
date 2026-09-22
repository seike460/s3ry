package s3

import "testing"

func TestMeterAddClampsToTotal(t *testing.T) {
	var events []Progress
	m := &meter{
		base: Progress{Total: 100},
		fn:   func(p Progress) { events = append(events, p) },
	}

	m.add(80)
	m.add(80)
	if got := m.n.Load(); got != 100 {
		t.Fatalf("transferred = %d, want clamped to total 100", got)
	}

	m.finish(nil)
	last := events[len(events)-1]
	if !last.Done || last.Transferred != 100 {
		t.Fatalf("final event = %#v, want Done at 100", last)
	}
}

func TestMeterSetTotalClampsOvershoot(t *testing.T) {
	m := &meter{base: Progress{Total: -1}}
	m.add(200)
	m.setTotal(150)
	if got := m.n.Load(); got != 150 {
		t.Fatalf("transferred = %d, want clamped to late total 150", got)
	}
}
