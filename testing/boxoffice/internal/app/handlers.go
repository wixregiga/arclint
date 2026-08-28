// The handlers: translate HTTP to feature calls and domain errors
// back to statuses.

package app

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/event"
	"boxoffice/internal/entities/order"
	"boxoffice/internal/features/editevent"
	"boxoffice/internal/features/holdseats"
	"boxoffice/internal/features/placeorder"
	"boxoffice/internal/shared/httpkit"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *server) listPublished(w http.ResponseWriter, _ *http.Request) {
	evs, err := s.deps.Events.All()
	if err != nil {
		s.fail(w, err)
		return
	}
	now := s.cfg.Now()
	views := []eventView{}
	for _, ev := range evs {
		// What's on sale: drafts have not started selling and
		// cancelled shows have stopped.
		if !ev.OnSale() {
			continue
		}
		views = append(views, s.viewEvent(ev, now))
	}
	httpkit.Respond(w, http.StatusOK, views)
}

func (s *server) listAll(w http.ResponseWriter, _ *http.Request) {
	evs, err := s.deps.Events.All()
	if err != nil {
		s.fail(w, err)
		return
	}
	now := s.cfg.Now()
	views := []eventView{}
	for _, ev := range evs {
		views = append(views, s.viewEvent(ev, now))
	}
	httpkit.Respond(w, http.StatusOK, views)
}

func (s *server) getEvent(w http.ResponseWriter, r *http.Request) {
	ev, err := s.deps.Events.Find(chi.URLParam(r, "eventID"))
	if err != nil {
		s.fail(w, err)
		return
	}
	// A draft Event is visible to its Organizer alone. A cancelled
	// one was public once and stays readable, so an Attendee who
	// holds its link learns the show is off instead of meeting a
	// page that denies it ever existed.
	if ev.Draft() && !s.isOrganizer(r) {
		s.fail(w, event.ErrEventUnknown)
		return
	}
	httpkit.Respond(w, http.StatusOK, s.viewEvent(ev, s.cfg.Now()))
}

// organizerEvent is the Event from behind the counter: the published
// seats per tier next to what has become of them, and who bought
// tickets.
func (s *server) organizerEvent(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "eventID")
	ev, err := s.deps.Events.Find(eventID)
	if err != nil {
		s.fail(w, err)
		return
	}
	orders, err := s.deps.Orders.ForEvent(eventID)
	if err != nil {
		s.fail(w, err)
		return
	}
	httpkit.Respond(w, http.StatusOK, s.viewOrganizerEvent(ev, orders, s.cfg.Now()))
}

func (s *server) createEvent(w http.ResponseWriter, r *http.Request) {
	var in createEventInput
	if err := httpkit.Decode(r, &in); err != nil {
		httpkit.Error(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	id := slugify(in.Title)
	if _, err := s.deps.Events.Find(id); err == nil {
		httpkit.Error(w, http.StatusConflict, "an event with that title already exists")
		return
	}
	ev, err := event.New(id, in.Title)
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := ev.Tell(in.Story, in.When, in.Where); err != nil {
		s.fail(w, err)
		return
	}
	c, err := capacity.New(id)
	if err != nil {
		s.fail(w, err)
		return
	}
	for _, t := range in.Tiers {
		if err := ev.AddTier(t.Name, event.Price(t.PriceCents)); err != nil {
			s.fail(w, err)
			return
		}
		if err := c.OpenTier(t.Name, t.Seats); err != nil {
			s.fail(w, err)
			return
		}
	}
	if err := s.deps.Events.Save(ev); err != nil {
		s.fail(w, err)
		return
	}
	if err := s.deps.Capacities.Save(c); err != nil {
		s.fail(w, err)
		return
	}
	httpkit.Respond(w, http.StatusCreated, s.viewEvent(ev, s.cfg.Now()))
}

func (s *server) editEvent(w http.ResponseWriter, r *http.Request) {
	var in editEventInput
	if err := httpkit.Decode(r, &in); err != nil {
		httpkit.Error(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	tiers := make([]editevent.Tier, 0, len(in.Tiers))
	for _, t := range in.Tiers {
		tiers = append(tiers, editevent.Tier{Name: t.Name, Price: event.Price(t.PriceCents), Seats: t.Seats})
	}
	ev, err := s.edit.Do(chi.URLParam(r, "eventID"), in.Story, in.When, in.Where, tiers)
	if err != nil {
		s.fail(w, err)
		return
	}
	httpkit.Respond(w, http.StatusOK, s.viewEvent(ev, s.cfg.Now()))
}

func (s *server) publishEvent(w http.ResponseWriter, r *http.Request) {
	ev, err := s.publish.Do(chi.URLParam(r, "eventID"))
	if err != nil {
		s.fail(w, err)
		return
	}
	httpkit.Respond(w, http.StatusOK, s.viewEvent(ev, s.cfg.Now()))
}

// cancelEvent calls a published show off: it goes off sale for good
// and every ticket sold for it is given back.
func (s *server) cancelEvent(w http.ResponseWriter, r *http.Request) {
	ev, err := s.cancel.Do(chi.URLParam(r, "eventID"))
	if err != nil {
		s.fail(w, err)
		return
	}
	httpkit.Respond(w, http.StatusOK, s.viewEvent(ev, s.cfg.Now()))
}

// refundTicket gives one buyer's tickets back on one tier.
func (s *server) refundTicket(w http.ResponseWriter, r *http.Request) {
	var in refundInput
	if err := httpkit.Decode(r, &in); err != nil {
		httpkit.Error(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	o, err := s.refund.Do(chi.URLParam(r, "orderID"), in.Tier, in.Quantity)
	if err != nil {
		s.fail(w, err)
		return
	}
	httpkit.Respond(w, http.StatusOK, viewOrder(o))
}

func (s *server) createHold(w http.ResponseWriter, r *http.Request) {
	var in holdInput
	if err := httpkit.Decode(r, &in); err != nil {
		httpkit.Error(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	held, err := s.hold.Do(newID(), chi.URLParam(r, "eventID"), in.Tier, in.Seats, s.cfg.Now())
	if err != nil {
		s.fail(w, err)
		return
	}
	httpkit.Respond(w, http.StatusCreated, holdView{
		HoldID:   held.HoldID,
		Tier:     held.TierName,
		Seats:    held.Seats,
		Deadline: held.Deadline,
	})
}

func (s *server) placeOrder(w http.ResponseWriter, r *http.Request) {
	var in placeOrderInput
	if err := httpkit.Decode(r, &in); err != nil {
		httpkit.Error(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	o, err := s.place.Do(newID(), in.EventID, in.HoldIDs,
		order.Attendee{Name: in.Attendee.Name, Email: in.Attendee.Email}, s.cfg.Now())
	if err != nil {
		s.fail(w, err)
		return
	}
	httpkit.Respond(w, http.StatusCreated, viewOrder(o))
}

func (s *server) getOrder(w http.ResponseWriter, r *http.Request) {
	o, err := s.deps.Orders.Find(chi.URLParam(r, "orderID"))
	if err != nil {
		s.fail(w, err)
		return
	}
	httpkit.Respond(w, http.StatusOK, viewOrder(o))
}

// fail maps domain refusals to statuses: unknown things are 404,
// promises that cannot be kept right now are 409, deals that were
// never valid are 422, anything else is a 500 kept vague.
func (s *server) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, event.ErrEventUnknown),
		errors.Is(err, order.ErrOrderUnknown),
		errors.Is(err, capacity.ErrCapacityUnknown):
		httpkit.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, capacity.ErrSeatsExhausted),
		errors.Is(err, capacity.ErrHoldExpiredOrUnknown),
		errors.Is(err, capacity.ErrHoldDuplicate),
		errors.Is(err, capacity.ErrTierOpenTwice),
		errors.Is(err, capacity.ErrSeatsNotSpokenFor),
		errors.Is(err, event.ErrAlreadyPublished),
		errors.Is(err, event.ErrAlreadyCancelled),
		errors.Is(err, event.ErrEventPublished),
		errors.Is(err, event.ErrEventCancelled),
		errors.Is(err, event.ErrEventNotPublished),
		errors.Is(err, event.ErrTierDuplicate),
		errors.Is(err, order.ErrRefundTooMany),
		errors.Is(err, holdseats.ErrEventNotOnSale),
		errors.Is(err, placeorder.ErrEventNotOnSale):
		httpkit.Error(w, http.StatusConflict, err.Error())
	case errors.Is(err, event.ErrIdentityMissing),
		errors.Is(err, event.ErrNothingToSell),
		errors.Is(err, event.ErrTierUnpriced),
		errors.Is(err, event.ErrTierNameMissing),
		errors.Is(err, event.ErrPriceNegative),
		errors.Is(err, order.ErrIdentityMissing),
		errors.Is(err, order.ErrAttendeeMissing),
		errors.Is(err, order.ErrLinesMissing),
		errors.Is(err, order.ErrLineInvalid),
		errors.Is(err, order.ErrLineDuplicate),
		errors.Is(err, order.ErrRefundQuantityUnreal),
		errors.Is(err, order.ErrRefundTierUnsold),
		errors.Is(err, capacity.ErrEventMissing),
		errors.Is(err, capacity.ErrTierNotOpen),
		errors.Is(err, capacity.ErrSeatsInvalid),
		errors.Is(err, capacity.ErrHoldIDMissing),
		errors.Is(err, capacity.ErrDeadlinePassed),
		errors.Is(err, placeorder.ErrTierNotSold):
		httpkit.Error(w, http.StatusUnprocessableEntity, err.Error())
	default:
		slog.Error("unexpected failure", "error", err)
		httpkit.Error(w, http.StatusInternalServerError, "something went wrong")
	}
}
