package capacity_test

import (
	"boxoffice/internal/entities/capacity"
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 8, 28, 19, 0, 0, 0, time.UTC)

func openRoom(t *testing.T) capacity.Capacity {
	t.Helper()
	c, err := capacity.New("jazz-trio")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.OpenTier("general", 3); err != nil {
		t.Fatalf("OpenTier: %v", err)
	}
	return c
}

func TestHoldsNeverOutrunTheRoom(t *testing.T) {
	c := openRoom(t)
	deadline := t0.Add(5 * time.Minute)

	if err := c.PlaceHold("h1", "general", 2, deadline, t0); err != nil {
		t.Fatalf("PlaceHold h1: %v", err)
	}
	if err := c.PlaceHold("h2", "general", 2, deadline, t0); !errors.Is(err, capacity.ErrSeatsExhausted) {
		t.Fatalf("overhold = %v, want ErrSeatsExhausted", err)
	}
	if err := c.PlaceHold("h2", "general", 1, deadline, t0); err != nil {
		t.Fatalf("PlaceHold h2 within room: %v", err)
	}
	left, err := c.Remaining("general", t0)
	if err != nil || left != 0 {
		t.Fatalf("Remaining = %d, %v; want 0 seats left", left, err)
	}
}

func TestExpiredHoldFreesItsSeats(t *testing.T) {
	c := openRoom(t)
	deadline := t0.Add(5 * time.Minute)
	if err := c.PlaceHold("h1", "general", 3, deadline, t0); err != nil {
		t.Fatalf("PlaceHold: %v", err)
	}

	later := deadline.Add(time.Second)
	left, err := c.Remaining("general", later)
	if err != nil || left != 3 {
		t.Fatalf("Remaining after expiry = %d, %v; want all 3 back", left, err)
	}
	if _, err := c.CommitAll([]string{"h1"}, later); !errors.Is(err, capacity.ErrHoldExpiredOrUnknown) {
		t.Fatalf("committing an expired hold = %v, want ErrHoldExpiredOrUnknown", err)
	}
}

func TestCommitIsAllOrNothing(t *testing.T) {
	c := openRoom(t)
	shortDeadline := t0.Add(1 * time.Minute)
	longDeadline := t0.Add(10 * time.Minute)
	if err := c.PlaceHold("short", "general", 1, shortDeadline, t0); err != nil {
		t.Fatalf("PlaceHold short: %v", err)
	}
	if err := c.PlaceHold("long", "general", 1, longDeadline, t0); err != nil {
		t.Fatalf("PlaceHold long: %v", err)
	}

	between := shortDeadline.Add(time.Second)
	if _, err := c.CommitAll([]string{"long", "short"}, between); !errors.Is(err, capacity.ErrHoldExpiredOrUnknown) {
		t.Fatalf("mixed commit = %v, want ErrHoldExpiredOrUnknown", err)
	}
	left, err := c.Remaining("general", between)
	if err != nil || left != 2 {
		t.Fatalf("Remaining after refused commit = %d, %v; want 2 (long still held, nothing spoken for)", left, err)
	}
}

func TestCommittedSeatsStaySpokenFor(t *testing.T) {
	c := openRoom(t)
	deadline := t0.Add(5 * time.Minute)
	if err := c.PlaceHold("h1", "general", 2, deadline, t0); err != nil {
		t.Fatalf("PlaceHold: %v", err)
	}
	committed, err := c.CommitAll([]string{"h1"}, t0)
	if err != nil || len(committed) != 1 || committed[0].Seats != 2 {
		t.Fatalf("CommitAll = %+v, %v", committed, err)
	}

	farFuture := deadline.Add(time.Hour)
	left, err := c.Remaining("general", farFuture)
	if err != nil || left != 1 {
		t.Fatalf("Remaining long after = %d, %v; spoken-for seats come back only by refund", left, err)
	}
}

func TestRefundFreesSpokenForSeats(t *testing.T) {
	c := openRoom(t)
	deadline := t0.Add(5 * time.Minute)
	if err := c.PlaceHold("h1", "general", 2, deadline, t0); err != nil {
		t.Fatalf("PlaceHold: %v", err)
	}
	if _, err := c.CommitAll([]string{"h1"}, t0); err != nil {
		t.Fatalf("CommitAll: %v", err)
	}

	if err := c.Refund("general", 1); err != nil {
		t.Fatalf("Refund: %v", err)
	}
	count, err := c.Count("general", t0)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	want := capacity.Count{Seats: 3, SpokenFor: 1, Held: 0, Remaining: 2}
	if count != want {
		t.Fatalf("Count after refund = %+v, want %+v", count, want)
	}

	// The freed seat can be promised again.
	if err := c.PlaceHold("h2", "general", 2, deadline, t0); err != nil {
		t.Fatalf("PlaceHold on refunded seats: %v", err)
	}
}

func TestRefundNeverFreesMoreThanIsSpokenFor(t *testing.T) {
	c := openRoom(t)
	deadline := t0.Add(5 * time.Minute)
	if err := c.PlaceHold("h1", "general", 2, deadline, t0); err != nil {
		t.Fatalf("PlaceHold: %v", err)
	}
	if _, err := c.CommitAll([]string{"h1"}, t0); err != nil {
		t.Fatalf("CommitAll: %v", err)
	}

	cases := []struct {
		name  string
		tier  string
		seats int
		want  error
	}{
		{"no seats asked for", "general", 0, capacity.ErrSeatsInvalid},
		{"a negative refund", "general", -1, capacity.ErrSeatsInvalid},
		{"a tier the room never opened", "balcony", 1, capacity.ErrTierNotOpen},
		{"more seats than are spoken for", "general", 3, capacity.ErrSeatsNotSpokenFor},
	}
	for _, tc := range cases {
		if err := c.Refund(tc.tier, tc.seats); !errors.Is(err, tc.want) {
			t.Errorf("%s: Refund = %v, want %v", tc.name, err, tc.want)
		}
	}
	count, err := c.Count("general", t0)
	if err != nil || count.SpokenFor != 2 {
		t.Fatalf("a refused refund changed the count: %+v, %v", count, err)
	}
}

func TestReleaseHoldsFreesEveryPendingHold(t *testing.T) {
	c := openRoom(t)
	deadline := t0.Add(5 * time.Minute)
	if err := c.PlaceHold("h1", "general", 1, deadline, t0); err != nil {
		t.Fatalf("PlaceHold h1: %v", err)
	}
	if err := c.PlaceHold("h2", "general", 1, deadline, t0); err != nil {
		t.Fatalf("PlaceHold h2: %v", err)
	}

	c.ReleaseHolds()

	count, err := c.Count("general", t0)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	want := capacity.Count{Seats: 3, SpokenFor: 0, Held: 0, Remaining: 3}
	if count != want {
		t.Fatalf("Count after releasing holds = %+v, want %+v", count, want)
	}
	if _, err := c.CommitAll([]string{"h1"}, t0); !errors.Is(err, capacity.ErrHoldExpiredOrUnknown) {
		t.Fatalf("committing a released hold = %v, want ErrHoldExpiredOrUnknown", err)
	}
}

func TestCountSeesHoldsAndSeatsSpokenFor(t *testing.T) {
	c := openRoom(t)
	deadline := t0.Add(5 * time.Minute)
	if err := c.PlaceHold("h1", "general", 1, deadline, t0); err != nil {
		t.Fatalf("PlaceHold h1: %v", err)
	}
	if _, err := c.CommitAll([]string{"h1"}, t0); err != nil {
		t.Fatalf("CommitAll: %v", err)
	}
	if err := c.PlaceHold("h2", "general", 1, deadline, t0); err != nil {
		t.Fatalf("PlaceHold h2: %v", err)
	}

	count, err := c.Count("general", t0)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	want := capacity.Count{Seats: 3, SpokenFor: 1, Held: 1, Remaining: 1}
	if count != want {
		t.Fatalf("Count = %+v, want %+v", count, want)
	}

	// Once the hold expires its seat is free again, and the seats the
	// tier was given never move.
	later := deadline.Add(time.Second)
	count, err = c.Count("general", later)
	if err != nil {
		t.Fatalf("Count after expiry: %v", err)
	}
	want = capacity.Count{Seats: 3, SpokenFor: 1, Held: 0, Remaining: 2}
	if count != want {
		t.Fatalf("Count after expiry = %+v, want %+v", count, want)
	}
	if _, err := c.Count("balcony", t0); !errors.Is(err, capacity.ErrTierNotOpen) {
		t.Fatalf("Count on an unopened tier = %v, want ErrTierNotOpen", err)
	}
}

func TestCloneSharesNothing(t *testing.T) {
	c := openRoom(t)
	deadline := t0.Add(5 * time.Minute)
	if err := c.PlaceHold("h1", "general", 1, deadline, t0); err != nil {
		t.Fatalf("PlaceHold: %v", err)
	}

	clone := c.Clone()
	if _, err := clone.CommitAll([]string{"h1"}, t0); err != nil {
		t.Fatalf("CommitAll on clone: %v", err)
	}

	left, err := c.Remaining("general", t0)
	if err != nil || left != 2 {
		t.Fatalf("original Remaining = %d, %v; the clone's commit must not reach it", left, err)
	}
}
