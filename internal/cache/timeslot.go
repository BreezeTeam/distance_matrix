package cache

import (
	"strconv"
	"time"
)

// TimeSlotWMH returns weekday + month + hour, matching Python time_slot_wmh.
func TimeSlotWMH(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	weekday := strconv.Itoa(int(t.Weekday()) + 1)
	month := strconv.Itoa(int(t.Month()))
	if t.Month() < 10 {
		month = "0" + month
	}
	hour := strconv.Itoa(t.Hour())
	if t.Hour() < 10 {
		hour = "0" + hour
	}
	return weekday + month + hour
}

// HourBucket returns two-digit hour for fuzzy matching.
func HourBucket(slot string) string {
	if len(slot) >= 2 {
		return slot[len(slot)-2:]
	}
	return TimeSlotWMH(time.Now())[len(TimeSlotWMH(time.Now()))-2:]
}

// AdjacentSlots returns current slot and ±1 hour variants for HMGET.
func AdjacentSlots(slot string) []string {
	if len(slot) < 5 {
		slot = TimeSlotWMH(time.Now())
	}
	out := []string{slot}
	h, err := strconv.Atoi(slot[len(slot)-2:])
	if err != nil {
		return out
	}
	prefix := slot[:len(slot)-2]
	for _, delta := range []int{-1, 1} {
		nh := h + delta
		if nh < 0 {
			nh = 23
		}
		if nh > 23 {
			nh = 0
		}
		hs := strconv.Itoa(nh)
		if nh < 10 {
			hs = "0" + hs
		}
		out = append(out, prefix+hs)
	}
	return uniqueStrings(out)
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
