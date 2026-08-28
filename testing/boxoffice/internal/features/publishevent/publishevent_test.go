package publishevent_test

import (
	"boxoffice/internal/entities/event"
	"boxoffice/internal/features/publishevent"
	"errors"
	"testing"
)

type events map[string]event.Event

func (e events) Save(ev event.Event) error { e[ev.ID()] = ev; return nil }

func (e events) Find(id string) (event.Event, error) {
	ev, ok := e[id]
	if !ok {
		return event.Event{}, event.ErrEventUnknown
	}
	return ev, nil
}

func (e events) All() ([]event.Event, error) { return nil, nil }

func draft(t *testing.T, price event.Price) event.Event {
	t.Helper()
	ev, err := event.New("jazz-trio", "Jazz Trio")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := ev.AddTier("general", price); err != nil {
		t.Fatalf("AddTier: %v", err)
	}
	return ev
}

func TestPublishPersistsThePublishedEvent(t *testing.T) {
	repo := events{}
	if err := repo.Save(draft(t, 2200)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	published, err := publishevent.Feature{Events: repo}.Do("jazz-trio")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !published.OnSale() {
		t.Fatal("Do returned an unpublished event")
	}
	stored, err := repo.Find("jazz-trio")
	if err != nil || !stored.OnSale() {
		t.Fatalf("the flip was not persisted: %v, on sale=%v", err, stored.OnSale())
	}
}

func TestPublishRefusesUnpricedDraftAndKeepsItDraft(t *testing.T) {
	repo := events{}
	if err := repo.Save(draft(t, 0)); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := (publishevent.Feature{Events: repo}).Do("jazz-trio"); !errors.Is(err, event.ErrTierUnpriced) {
		t.Fatalf("Do = %v, want ErrTierUnpriced", err)
	}
	stored, err := repo.Find("jazz-trio")
	if err != nil || !stored.Draft() {
		t.Fatalf("a refused publish must leave the stored event a draft: %v, status=%v", err, stored.Status())
	}
}

func TestPublishUnknownEvent(t *testing.T) {
	if _, err := (publishevent.Feature{Events: events{}}).Do("nope"); !errors.Is(err, event.ErrEventUnknown) {
		t.Fatalf("Do = %v, want ErrEventUnknown", err)
	}
}
