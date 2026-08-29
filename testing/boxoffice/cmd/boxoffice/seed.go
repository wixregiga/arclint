// Seeding: Priya's three shows and the deals already struck for
// them, deterministic so demos and manual poking always start from
// the same box office.
package main

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/event"
	"boxoffice/internal/entities/order"
	"time"
)

// generalTier is the tier every seeded show sells.
const generalTier = "general"

// frontRowTier is the second tier of the jazz trio's room.
const frontRowTier = "front-row"

// seedShows records two shows on sale and one draft, so both sides
// of the counter have something to do.
func seedShows(events event.Repository, capacities capacity.Repository) error {
	type tier struct {
		name  string
		price event.Price
		seats int
	}
	shows := []struct {
		id, title, story, when, where string
		publish                       bool
		tiers                         []tier
	}{
		{
			id: "open-mic-night", title: "Open Mic Night",
			story: "Anyone with three minutes and some nerve.",
			when:  "Friday 19:00", where: "The Garage Stage",
			publish: true,
			tiers:   []tier{{generalTier, 1500, 40}},
		},
		{
			id: "jazz-trio", title: "Jazz Trio",
			story: "Standards, played close.",
			when:  "Saturday 20:00", where: "The Garage Stage",
			publish: true,
			tiers:   []tier{{generalTier, 2200, 60}, {frontRowTier, 3000, 12}},
		},
		{
			id: "winter-recital", title: "Winter Recital",
			story: "The students' season finale.",
			when:  "December, date to be told", where: "Main Hall",
			publish: false,
			tiers:   []tier{{generalTier, 0, 80}},
		},
	}
	for _, s := range shows {
		ev, err := event.New(s.id, s.title)
		if err != nil {
			return err
		}
		if err := ev.Tell(s.story, s.when, s.where); err != nil {
			return err
		}
		c, err := capacity.New(s.id)
		if err != nil {
			return err
		}
		for _, t := range s.tiers {
			if err := ev.AddTier(t.name, t.price); err != nil {
				return err
			}
			if err := c.OpenTier(t.name, t.seats); err != nil {
				return err
			}
		}
		if s.publish {
			if err := ev.Publish(); err != nil {
				return err
			}
		}
		if err := events.Save(ev); err != nil {
			return err
		}
		if err := capacities.Save(c); err != nil {
			return err
		}
	}
	return nil
}

// seedOrders strikes two deals for the jazz trio so the organizer's
// side of the counter opens with buyers to see and tickets to give
// back. The seats travel the honest route: held first, then spoken
// for, so the ledger's count matches the deals.
func seedOrders(orders order.Repository, capacities capacity.Repository, now time.Time) error {
	deals := []struct {
		id       string
		attendee order.Attendee
		lines    []order.Line
	}{
		{
			id:       "seed-order-sam",
			attendee: order.Attendee{Name: "Sam Okonkwo", Email: "sam@example.com"},
			lines: []order.Line{
				{TierName: generalTier, Quantity: 2, UnitCents: 2200},
				{TierName: frontRowTier, Quantity: 1, UnitCents: 3000},
			},
		},
		{
			id:       "seed-order-ada",
			attendee: order.Attendee{Name: "Ada Fletcher", Email: "ada@example.com"},
			lines: []order.Line{
				{TierName: generalTier, Quantity: 3, UnitCents: 2200},
			},
		},
	}
	c, err := capacities.Find("jazz-trio")
	if err != nil {
		return err
	}
	deadline := now.Add(time.Minute)
	for _, d := range deals {
		holdIDs := make([]string, 0, len(d.lines))
		for _, l := range d.lines {
			holdID := d.id + "-hold-" + l.TierName
			if err := c.PlaceHold(holdID, l.TierName, l.Quantity, deadline, now); err != nil {
				return err
			}
			holdIDs = append(holdIDs, holdID)
		}
		if _, err := c.CommitAll(holdIDs, now); err != nil {
			return err
		}
		o, err := order.New(d.id, "jazz-trio", d.attendee, d.lines)
		if err != nil {
			return err
		}
		if err := orders.Save(o); err != nil {
			return err
		}
	}
	return capacities.Save(c)
}

// seedComps gives one guest tickets to the jazz trio, so the
// organizer's side opens with a comp to see beside the deals. The
// seats are spoken for straight away, the way comping promises them,
// and the tickets cost their attendee nothing.
func seedComps(orders order.Repository, capacities capacity.Repository, now time.Time) error {
	c, err := capacities.Find("jazz-trio")
	if err != nil {
		return err
	}
	lines := []order.Line{
		{TierName: generalTier, Quantity: 2},
		{TierName: frontRowTier, Quantity: 1},
	}
	for _, l := range lines {
		if err := c.Commit(l.TierName, l.Quantity, now); err != nil {
			return err
		}
	}
	o, err := order.New("seed-comp-rae", "jazz-trio",
		order.Attendee{Name: "Rae Mensah", Email: "rae@example.com"}, lines)
	if err != nil {
		return err
	}
	if err := orders.Save(o); err != nil {
		return err
	}
	return capacities.Save(c)
}
