package main

import (
	"boxoffice/internal/app/memory"
	"boxoffice/internal/entities/event"
	"testing"
	"time"
)

func TestSeedingOpensAConsistentBoxOffice(t *testing.T) {
	events := memory.NewEvents()
	orders := memory.NewOrders()
	capacities := memory.NewCapacities()
	now := time.Date(2026, 8, 28, 19, 0, 0, 0, time.UTC)

	if err := seedShows(events, capacities); err != nil {
		t.Fatalf("seedShows: %v", err)
	}
	if err := seedOrders(orders, capacities, now); err != nil {
		t.Fatalf("seedOrders: %v", err)
	}

	// Two shows on sale and one draft, so both sides of the counter
	// have something to do.
	all, err := events.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	byStatus := map[event.Status]int{}
	for _, ev := range all {
		byStatus[ev.Status()]++
	}
	if byStatus[event.StatusPublished] != 2 || byStatus[event.StatusDraft] != 1 {
		t.Fatalf("seeded statuses = %v, want 2 published and 1 draft", byStatus)
	}

	// The seeded deals are there, with buyers to see.
	placed, err := orders.ForEvent("jazz-trio")
	if err != nil {
		t.Fatalf("ForEvent: %v", err)
	}
	if len(placed) != 2 {
		t.Fatalf("seeded orders = %d, want 2", len(placed))
	}
	for _, o := range placed {
		if o.Attendee().Name == "" || o.OutstandingCents() != o.TotalCents() {
			t.Errorf("seeded order %s = %+v, want a named buyer owing the whole deal", o.ID(), o.Attendee())
		}
	}

	// The ledger's count matches the deals struck: five general seats
	// spoken for out of sixty, one front-row out of twelve, and
	// nothing left hanging in a hold.
	c, err := capacities.Find("jazz-trio")
	if err != nil {
		t.Fatalf("Find capacity: %v", err)
	}
	general, err := c.Count(generalTier, now)
	if err != nil {
		t.Fatalf("Count general: %v", err)
	}
	if general.Seats != 60 || general.SpokenFor != 5 || general.Held != 0 || general.Remaining != 55 {
		t.Errorf("general = %+v, want 60 seats with 5 spoken for", general)
	}
	front, err := c.Count(frontRowTier, now)
	if err != nil {
		t.Fatalf("Count front-row: %v", err)
	}
	if front.Seats != 12 || front.SpokenFor != 1 || front.Held != 0 || front.Remaining != 11 {
		t.Errorf("front-row = %+v, want 12 seats with 1 spoken for", front)
	}
}
