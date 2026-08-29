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
	if err := seedComps(orders, capacities, now); err != nil {
		t.Fatalf("seedComps: %v", err)
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

	// The seeded deals are there, with buyers to see, and one comp
	// beside them that its guest owes nothing for.
	placed, err := orders.ForEvent("jazz-trio")
	if err != nil {
		t.Fatalf("ForEvent: %v", err)
	}
	if len(placed) != 3 {
		t.Fatalf("seeded orders = %d, want 2 deals and 1 comp", len(placed))
	}
	comps := 0
	for _, o := range placed {
		if o.Attendee().Name == "" {
			t.Errorf("seeded order %s has no named attendee", o.ID())
		}
		if o.OutstandingCents() != o.TotalCents() {
			t.Errorf("seeded order %s owes %d of %d", o.ID(), o.OutstandingCents(), o.TotalCents())
		}
		if o.Comped() {
			comps++
			if o.TotalCents() != 0 {
				t.Errorf("seeded comp %s costs %d, want nothing", o.ID(), o.TotalCents())
			}
		}
	}
	if comps != 1 {
		t.Errorf("seeded comps = %d, want 1", comps)
	}

	// The ledger's count matches the deals struck and the comp given:
	// seven general seats spoken for out of sixty, two front-row out
	// of twelve, and nothing left hanging in a hold.
	c, err := capacities.Find("jazz-trio")
	if err != nil {
		t.Fatalf("Find capacity: %v", err)
	}
	general, err := c.Count(generalTier, now)
	if err != nil {
		t.Fatalf("Count general: %v", err)
	}
	if general.Seats != 60 || general.SpokenFor != 7 || general.Held != 0 || general.Remaining != 53 {
		t.Errorf("general = %+v, want 60 seats with 7 spoken for", general)
	}
	front, err := c.Count(frontRowTier, now)
	if err != nil {
		t.Fatalf("Count front-row: %v", err)
	}
	if front.Seats != 12 || front.SpokenFor != 2 || front.Held != 0 || front.Remaining != 10 {
		t.Errorf("front-row = %+v, want 12 seats with 2 spoken for", front)
	}
}
