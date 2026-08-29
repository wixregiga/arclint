// The runnable-server proof: the whole stack through real HTTP,
// with the clock under test control.
package app_test

import (
	"boxoffice/internal/app"
	"boxoffice/internal/app/memory"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const organizerToken = "test-organizer"

type env struct {
	t   *testing.T
	srv *httptest.Server
	now *time.Time
}

func start(t *testing.T) *env {
	t.Helper()
	now := time.Date(2026, 8, 28, 19, 0, 0, 0, time.UTC)
	e := &env{t: t, now: &now}
	handler := app.New(app.Config{
		OrganizerToken: organizerToken,
		HoldFor:        5 * time.Minute,
		Now:            func() time.Time { return *e.now },
	}, app.Deps{
		Events:     memory.NewEvents(),
		Orders:     memory.NewOrders(),
		Capacities: memory.NewCapacities(),
	})
	e.srv = httptest.NewServer(handler)
	t.Cleanup(e.srv.Close)
	return e
}

// call sends one JSON request and decodes the JSON answer.
func (e *env) call(method, path, token string, body any) (int, any) {
	e.t.Helper()
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			e.t.Fatalf("marshal: %v", err)
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(e.t.Context(), method, e.srv.URL+path, payload)
	if err != nil {
		e.t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := e.srv.Client().Do(req)
	if err != nil {
		e.t.Fatalf("do %s %s: %v", method, path, err)
	}
	defer func() {
		if cerr := res.Body.Close(); cerr != nil {
			e.t.Errorf("close body: %v", cerr)
		}
	}()
	var decoded any
	if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil && err != io.EOF {
		e.t.Fatalf("decode %s %s: %v", method, path, err)
	}
	return res.StatusCode, decoded
}

func obj(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected an object, got %T: %v", v, v)
	}
	return m
}

func list(t *testing.T, v any) []any {
	t.Helper()
	l, ok := v.([]any)
	if !ok {
		t.Fatalf("expected a list, got %T: %v", v, v)
	}
	return l
}

var jazzTrio = map[string]any{
	"title": "Jazz Trio",
	"story": "Standards, played close.",
	"when":  "Saturday 20:00",
	"where": "The Garage Stage",
	"tiers": []map[string]any{
		{"name": "general", "priceCents": 2200, "seats": 3},
		{"name": "front-row", "priceCents": 3000, "seats": 2},
	},
}

func TestBoxOfficeKeepsItsPromises(t *testing.T) {
	e := start(t)

	// The organizer creates a draft; the public sees nothing.
	status, created := e.call("POST", "/api/organizer/events", organizerToken, jazzTrio)
	if status != http.StatusCreated {
		t.Fatalf("create: %d %v", status, created)
	}
	id := obj(t, created)["id"].(string)
	if id != "jazz-trio" {
		t.Fatalf("id = %q, want jazz-trio", id)
	}
	if status, body := e.call("GET", "/api/events", "", nil); status != http.StatusOK || len(list(t, body)) != 0 {
		t.Fatalf("drafts leaked to the public list: %d %v", status, body)
	}
	if status, _ := e.call("GET", "/api/events/jazz-trio", "", nil); status != http.StatusNotFound {
		t.Fatalf("a draft must be invisible to strangers, got %d", status)
	}

	// Published, it goes on sale.
	if status, body := e.call("POST", "/api/organizer/events/jazz-trio/publish", organizerToken, nil); status != http.StatusOK {
		t.Fatalf("publish: %d %v", status, body)
	}
	status, body := e.call("GET", "/api/events", "", nil)
	if status != http.StatusOK || len(list(t, body)) != 1 {
		t.Fatalf("public list after publish: %d %v", status, body)
	}

	// Hold two seats; a second hold cannot outrun the room.
	status, held := e.call("POST", "/api/events/jazz-trio/holds", "", map[string]any{"tier": "general", "seats": 2})
	if status != http.StatusCreated {
		t.Fatalf("hold: %d %v", status, held)
	}
	holdID := obj(t, held)["holdId"].(string)
	if status, refused := e.call("POST", "/api/events/jazz-trio/holds", "", map[string]any{"tier": "general", "seats": 2}); status != http.StatusConflict {
		t.Fatalf("overhold = %d %v, want 409", status, refused)
	}

	// The deal is struck at the prices as they stand.
	status, placed := e.call("POST", "/api/orders", "", map[string]any{
		"eventId":  "jazz-trio",
		"holdIds":  []string{holdID},
		"attendee": map[string]any{"name": "Sam", "email": "sam@example.com"},
	})
	if status != http.StatusCreated {
		t.Fatalf("place order: %d %v", status, placed)
	}
	placedOrder := obj(t, placed)
	if placedOrder["totalCents"].(float64) != 4400 {
		t.Fatalf("totalCents = %v, want 4400", placedOrder["totalCents"])
	}
	orderID := placedOrder["id"].(string)
	if status, _ := e.call("GET", "/api/orders/"+orderID, "", nil); status != http.StatusOK {
		t.Fatalf("get order: %d", status)
	}

	// One seat left on the page.
	status, detail := e.call("GET", "/api/events/jazz-trio", "", nil)
	if status != http.StatusOK {
		t.Fatalf("get event: %d", status)
	}
	tier := obj(t, list(t, obj(t, detail)["tiers"])[0])
	if tier["remaining"].(float64) != 1 {
		t.Fatalf("remaining = %v, want 1", tier["remaining"])
	}
}

// sellJazzTrio puts the Jazz Trio on sale if it is not already, then
// strikes one deal for that many tickets on one tier and returns the
// order's id.
func (e *env) sellJazzTrio(t *testing.T, name, email, tier string, seats int) string {
	t.Helper()
	if _, ok := e.findEvent("jazz-trio"); !ok {
		if status, body := e.call("POST", "/api/organizer/events", organizerToken, jazzTrio); status != http.StatusCreated {
			t.Fatalf("create: %d %v", status, body)
		}
		if status, body := e.call("POST", "/api/organizer/events/jazz-trio/publish", organizerToken, nil); status != http.StatusOK {
			t.Fatalf("publish: %d %v", status, body)
		}
	}
	status, held := e.call("POST", "/api/events/jazz-trio/holds", "", map[string]any{"tier": tier, "seats": seats})
	if status != http.StatusCreated {
		t.Fatalf("hold: %d %v", status, held)
	}
	status, placed := e.call("POST", "/api/orders", "", map[string]any{
		"eventId":  "jazz-trio",
		"holdIds":  []string{obj(t, held)["holdId"].(string)},
		"attendee": map[string]any{"name": name, "email": email},
	})
	if status != http.StatusCreated {
		t.Fatalf("place order: %d %v", status, placed)
	}
	return obj(t, placed)["id"].(string)
}

// findEvent reads one event from the organizer's list, if it is there.
func (e *env) findEvent(id string) (map[string]any, bool) {
	status, body := e.call("GET", "/api/organizer/events", organizerToken, nil)
	if status != http.StatusOK {
		return nil, false
	}
	for _, raw := range body.([]any) {
		if ev, ok := raw.(map[string]any); ok && ev["id"] == id {
			return ev, true
		}
	}
	return nil, false
}

// tierOf pulls one tier out of an event view by name.
func tierOf(t *testing.T, view map[string]any, name string) map[string]any {
	t.Helper()
	for _, raw := range list(t, view["tiers"]) {
		tier := obj(t, raw)
		if tier["name"] == name {
			return tier
		}
	}
	t.Fatalf("no tier %q in %v", name, view)
	return nil
}

// num reads one number out of a decoded JSON object.
func num(t *testing.T, o map[string]any, key string) float64 {
	t.Helper()
	got, ok := o[key].(float64)
	if !ok {
		t.Fatalf("%s = %v (%T), want a number", key, o[key], o[key])
	}
	return got
}

func TestOrganizerSeesPublishedAmountsAndBuyers(t *testing.T) {
	e := start(t)
	e.sellJazzTrio(t, "Sam", "sam@example.com", "general", 2)
	// A third seat is held by someone still deciding.
	if status, held := e.call("POST", "/api/events/jazz-trio/holds", "", map[string]any{"tier": "general", "seats": 1}); status != http.StatusCreated {
		t.Fatalf("hold: %d %v", status, held)
	}

	status, body := e.call("GET", "/api/organizer/events/jazz-trio", organizerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("organizer view: %d %v", status, body)
	}
	view := obj(t, body)
	if view["status"] != "published" {
		t.Fatalf("status = %v, want published", view["status"])
	}

	// The seats as published, next to what has become of them.
	tier := tierOf(t, view, "general")
	if got := num(t, tier, "seats"); got != 3 {
		t.Errorf("seats = %v, want the 3 published", got)
	}
	if got := num(t, tier, "spokenFor"); got != 2 {
		t.Errorf("spokenFor = %v, want 2", got)
	}
	if got := num(t, tier, "held"); got != 1 {
		t.Errorf("held = %v, want 1", got)
	}
	if got := num(t, tier, "remaining"); got != 0 {
		t.Errorf("remaining = %v, want 0", got)
	}

	// And who bought tickets.
	orders := list(t, view["orders"])
	if len(orders) != 1 {
		t.Fatalf("orders = %v, want the one deal struck", orders)
	}
	buyer := obj(t, orders[0])
	if obj(t, buyer["attendee"])["name"] != "Sam" {
		t.Errorf("buyer = %v, want Sam", buyer["attendee"])
	}
	line := obj(t, list(t, buyer["lines"])[0])
	if num(t, line, "quantity") != 2 || num(t, line, "refunded") != 0 {
		t.Errorf("line = %v, want 2 bought and none refunded", line)
	}
	if num(t, buyer, "totalCents") != 4400 || num(t, buyer, "outstandingCents") != 4400 {
		t.Errorf("buyer totals = %v", buyer)
	}
}

func TestOrganizerViewIsTheOrganizersAlone(t *testing.T) {
	e := start(t)
	e.sellJazzTrio(t, "Sam", "sam@example.com", "general", 1)
	if status, _ := e.call("GET", "/api/organizer/events/jazz-trio", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("organizer view without a token = %d, want 401", status)
	}
	if status, _ := e.call("GET", "/api/organizer/events/nope", organizerToken, nil); status != http.StatusNotFound {
		t.Fatalf("organizer view of an unknown event = %d, want 404", status)
	}
}

func TestOrganizerRefundsOneBuyersTicket(t *testing.T) {
	e := start(t)
	orderID := e.sellJazzTrio(t, "Sam", "sam@example.com", "general", 2)

	status, body := e.call("POST", "/api/organizer/orders/"+orderID+"/refunds", organizerToken,
		map[string]any{"tier": "general", "quantity": 1})
	if status != http.StatusOK {
		t.Fatalf("refund: %d %v", status, body)
	}
	refunded := obj(t, body)
	line := obj(t, list(t, refunded["lines"])[0])
	if num(t, line, "quantity") != 2 || num(t, line, "refunded") != 1 {
		t.Fatalf("line after refund = %v, want 2 bought and 1 refunded", line)
	}
	// The deal as struck never moves; what is still owed does.
	if num(t, refunded, "totalCents") != 4400 {
		t.Errorf("totalCents = %v, want the 4400 struck", refunded["totalCents"])
	}
	if num(t, refunded, "outstandingCents") != 2200 {
		t.Errorf("outstandingCents = %v, want 2200", refunded["outstandingCents"])
	}

	// The counts are right everywhere anyone can see them.
	_, organizerView := e.call("GET", "/api/organizer/events/jazz-trio", organizerToken, nil)
	tier := tierOf(t, obj(t, organizerView), "general")
	if num(t, tier, "spokenFor") != 1 || num(t, tier, "remaining") != 2 {
		t.Errorf("organizer tier after refund = %v, want 1 spoken for and 2 left", tier)
	}
	_, publicView := e.call("GET", "/api/events/jazz-trio", "", nil)
	if got := num(t, tierOf(t, obj(t, publicView), "general"), "remaining"); got != 2 {
		t.Errorf("public remaining after refund = %v, want 2", got)
	}
	_, listed := e.call("GET", "/api/events", "", nil)
	if got := num(t, tierOf(t, obj(t, list(t, listed)[0]), "general"), "remaining"); got != 2 {
		t.Errorf("listed remaining after refund = %v, want 2", got)
	}
	_, orderView := e.call("GET", "/api/orders/"+orderID, "", nil)
	orderLine := obj(t, list(t, obj(t, orderView)["lines"])[0])
	if num(t, orderLine, "refunded") != 1 {
		t.Errorf("the buyer's own order page hides the refund: %v", orderLine)
	}

	// The freed seat really is sellable again.
	if status, held := e.call("POST", "/api/events/jazz-trio/holds", "", map[string]any{"tier": "general", "seats": 2}); status != http.StatusCreated {
		t.Fatalf("holding the refunded seats = %d %v, want 201", status, held)
	}
}

func TestRefundRefusals(t *testing.T) {
	e := start(t)
	orderID := e.sellJazzTrio(t, "Sam", "sam@example.com", "general", 2)
	cases := []struct {
		name  string
		path  string
		token string
		body  map[string]any
		want  int
	}{
		{
			"without the organizer token", "/api/organizer/orders/" + orderID + "/refunds", "",
			map[string]any{"tier": "general", "quantity": 1},
			http.StatusUnauthorized,
		},
		{
			"an order nobody placed", "/api/organizer/orders/nope/refunds", organizerToken,
			map[string]any{"tier": "general", "quantity": 1},
			http.StatusNotFound,
		},
		{
			"no tickets asked for", "/api/organizer/orders/" + orderID + "/refunds", organizerToken,
			map[string]any{"tier": "general", "quantity": 0},
			http.StatusUnprocessableEntity,
		},
		{
			"a tier the order never bought", "/api/organizer/orders/" + orderID + "/refunds", organizerToken,
			map[string]any{"tier": "balcony", "quantity": 1},
			http.StatusUnprocessableEntity,
		},
		{
			"more than the deal struck", "/api/organizer/orders/" + orderID + "/refunds", organizerToken,
			map[string]any{"tier": "general", "quantity": 3},
			http.StatusConflict,
		},
	}
	for _, c := range cases {
		if status, body := e.call("POST", c.path, c.token, c.body); status != c.want {
			t.Errorf("%s = %d %v, want %d", c.name, status, body, c.want)
		}
	}
	// None of the refused refunds reached the counts.
	_, view := e.call("GET", "/api/organizer/events/jazz-trio", organizerToken, nil)
	tier := tierOf(t, obj(t, view), "general")
	if num(t, tier, "spokenFor") != 2 || num(t, tier, "remaining") != 1 {
		t.Fatalf("a refused refund stuck: %v", tier)
	}
}

func TestOrganizerCompsTicketsAndSeesWhoGotThem(t *testing.T) {
	e := start(t)
	e.sellJazzTrio(t, "Sam", "sam@example.com", "general", 1)

	status, comped := e.call("POST", "/api/organizer/events/jazz-trio/comps", organizerToken, map[string]any{
		"attendee": map[string]any{"name": "Rae Mensah", "email": "rae@example.com"},
		"tickets":  []map[string]any{{"tier": "general", "quantity": 1}, {"tier": "front-row", "quantity": 2}},
	})
	if status != http.StatusCreated {
		t.Fatalf("comp: %d %v", status, comped)
	}
	gift := obj(t, comped)
	if gift["comped"] != true {
		t.Errorf("comped = %v, want true", gift["comped"])
	}
	if num(t, gift, "totalCents") != 0 || num(t, gift, "outstandingCents") != 0 {
		t.Errorf("a comped order costs %v and owes %v, want nothing", gift["totalCents"], gift["outstandingCents"])
	}

	// The seats are promised like any other: spoken for, not held.
	_, view := e.call("GET", "/api/organizer/events/jazz-trio", organizerToken, nil)
	general := tierOf(t, obj(t, view), "general")
	if num(t, general, "spokenFor") != 2 || num(t, general, "held") != 0 || num(t, general, "remaining") != 1 {
		t.Errorf("general after comping = %v, want 2 spoken for and 1 left", general)
	}
	front := tierOf(t, obj(t, view), "front-row")
	if num(t, front, "spokenFor") != 2 || num(t, front, "remaining") != 0 {
		t.Errorf("front-row after comping = %v, want both seats spoken for", front)
	}

	// The organizer can tell who was comped from who bought.
	comps := map[string]bool{}
	for _, raw := range list(t, obj(t, view)["orders"]) {
		o := obj(t, raw)
		comps[obj(t, o["attendee"])["name"].(string)] = o["comped"] == true
	}
	if len(comps) != 2 || !comps["Rae Mensah"] || comps["Sam"] {
		t.Errorf("the organizer's list of who was comped = %v", comps)
	}

	// Buyers see none of it: the public page shows seats and prices,
	// never who holds them or what anyone paid.
	_, page := e.call("GET", "/api/events/jazz-trio", "", nil)
	public := obj(t, page)
	if _, leaked := public["orders"]; leaked {
		t.Errorf("the public page lists orders: %v", public)
	}
	if got := num(t, tierOf(t, public, "general"), "remaining"); got != 1 {
		t.Errorf("public remaining = %v, want the 1 left", got)
	}
	if got := num(t, tierOf(t, public, "general"), "priceCents"); got != 2200 {
		t.Errorf("comping moved the tier's price to %v, want 2200 for everyone else", got)
	}
	// And the next buyer still pays.
	nextOrder := e.sellJazzTrio(t, "Ada", "ada@example.com", "general", 1)
	_, bought := e.call("GET", "/api/orders/"+nextOrder, "", nil)
	if num(t, obj(t, bought), "totalCents") != 2200 || obj(t, bought)["comped"] != false {
		t.Errorf("the buyer after a comp = %v, want a paid order", bought)
	}
}

func TestCompedTicketsAreRefundedLikeAnyOther(t *testing.T) {
	e := start(t)
	e.sellJazzTrio(t, "Sam", "sam@example.com", "general", 1)
	_, comped := e.call("POST", "/api/organizer/events/jazz-trio/comps", organizerToken, map[string]any{
		"attendee": map[string]any{"name": "Rae Mensah", "email": "rae@example.com"},
		"tickets":  []map[string]any{{"tier": "general", "quantity": 2}},
	})
	compID := obj(t, comped)["id"].(string)

	// No comp-shaped door: the ordinary refund gives the seats back.
	status, body := e.call("POST", "/api/organizer/orders/"+compID+"/refunds", organizerToken,
		map[string]any{"tier": "general", "quantity": 1})
	if status != http.StatusOK {
		t.Fatalf("refunding a comped ticket: %d %v", status, body)
	}
	line := obj(t, list(t, obj(t, body)["lines"])[0])
	if num(t, line, "refunded") != 1 || num(t, line, "quantity") != 2 {
		t.Errorf("comped line after refund = %v, want 2 given and 1 back", line)
	}
	_, view := e.call("GET", "/api/organizer/events/jazz-trio", organizerToken, nil)
	tier := tierOf(t, obj(t, view), "general")
	if num(t, tier, "spokenFor") != 2 || num(t, tier, "remaining") != 1 {
		t.Errorf("general after the comp refund = %v, want 2 spoken for and 1 free", tier)
	}

	// Cancelling the show gives back comped tickets with the rest.
	if status, cancelled := e.call("POST", "/api/organizer/events/jazz-trio/cancel", organizerToken, nil); status != http.StatusOK {
		t.Fatalf("cancel: %d %v", status, cancelled)
	}
	_, after := e.call("GET", "/api/orders/"+compID, "", nil)
	compLine := obj(t, list(t, obj(t, after)["lines"])[0])
	if num(t, compLine, "refunded") != 2 {
		t.Errorf("cancelling left comped tickets outstanding: %v", compLine)
	}
	_, whole := e.call("GET", "/api/organizer/events/jazz-trio", organizerToken, nil)
	if got := num(t, tierOf(t, obj(t, whole), "general"), "spokenFor"); got != 0 {
		t.Errorf("spokenFor after cancelling = %v, want the room whole", got)
	}
}

func TestCompRefusals(t *testing.T) {
	e := start(t)
	e.sellJazzTrio(t, "Sam", "sam@example.com", "general", 1)
	guest := map[string]any{"name": "Rae Mensah", "email": "rae@example.com"}
	cases := []struct {
		name  string
		path  string
		token string
		body  map[string]any
		want  int
	}{
		{
			"without the organizer token", "/api/organizer/events/jazz-trio/comps", "",
			map[string]any{"attendee": guest, "tickets": []map[string]any{{"tier": "general", "quantity": 1}}},
			http.StatusUnauthorized,
		},
		{
			"an event nobody created", "/api/organizer/events/nope/comps", organizerToken,
			map[string]any{"attendee": guest, "tickets": []map[string]any{{"tier": "general", "quantity": 1}}},
			http.StatusNotFound,
		},
		{
			"no tickets at all", "/api/organizer/events/jazz-trio/comps", organizerToken,
			map[string]any{"attendee": guest, "tickets": []map[string]any{}},
			http.StatusUnprocessableEntity,
		},
		{
			"a tier the show never sold", "/api/organizer/events/jazz-trio/comps", organizerToken,
			map[string]any{"attendee": guest, "tickets": []map[string]any{{"tier": "balcony", "quantity": 1}}},
			http.StatusUnprocessableEntity,
		},
		{
			"nobody to comp them for", "/api/organizer/events/jazz-trio/comps", organizerToken,
			map[string]any{"attendee": map[string]any{"name": ""}, "tickets": []map[string]any{{"tier": "general", "quantity": 1}}},
			http.StatusUnprocessableEntity,
		},
		{
			"more than the room has", "/api/organizer/events/jazz-trio/comps", organizerToken,
			map[string]any{"attendee": guest, "tickets": []map[string]any{{"tier": "general", "quantity": 3}}},
			http.StatusConflict,
		},
	}
	for _, c := range cases {
		if status, body := e.call("POST", c.path, c.token, c.body); status != c.want {
			t.Errorf("%s = %d %v, want %d", c.name, status, body, c.want)
		}
	}
	// None of the refused comps reached the counts or the order book.
	_, view := e.call("GET", "/api/organizer/events/jazz-trio", organizerToken, nil)
	tier := tierOf(t, obj(t, view), "general")
	if num(t, tier, "spokenFor") != 1 || num(t, tier, "remaining") != 2 {
		t.Errorf("a refused comp stuck: %v", tier)
	}
	if got := len(list(t, obj(t, view)["orders"])); got != 1 {
		t.Errorf("orders after refused comps = %d, want the one real sale", got)
	}
}

func TestDraftsAndCancelledShowsCannotBeComped(t *testing.T) {
	e := start(t)
	guest := map[string]any{"name": "Rae Mensah", "email": "rae@example.com"}
	body := map[string]any{"attendee": guest, "tickets": []map[string]any{{"tier": "general", "quantity": 1}}}

	if status, created := e.call("POST", "/api/organizer/events", organizerToken, jazzTrio); status != http.StatusCreated {
		t.Fatalf("create: %d %v", status, created)
	}
	if status, refused := e.call("POST", "/api/organizer/events/jazz-trio/comps", organizerToken, body); status != http.StatusConflict {
		t.Fatalf("comping a draft = %d %v, want 409", status, refused)
	}
	e.call("POST", "/api/organizer/events/jazz-trio/publish", organizerToken, nil)
	if status, ok := e.call("POST", "/api/organizer/events/jazz-trio/comps", organizerToken, body); status != http.StatusCreated {
		t.Fatalf("comping a published show = %d %v, want 201", status, ok)
	}
	e.call("POST", "/api/organizer/events/jazz-trio/cancel", organizerToken, nil)
	if status, refused := e.call("POST", "/api/organizer/events/jazz-trio/comps", organizerToken, body); status != http.StatusConflict {
		t.Fatalf("comping a cancelled show = %d %v, want 409", status, refused)
	}
}

func TestOrganizerCancelsAPublishedEvent(t *testing.T) {
	e := start(t)
	samOrder := e.sellJazzTrio(t, "Sam", "sam@example.com", "general", 2)
	adaOrder := e.sellJazzTrio(t, "Ada", "ada@example.com", "front-row", 2)

	status, body := e.call("POST", "/api/organizer/events/jazz-trio/cancel", organizerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("cancel: %d %v", status, body)
	}
	if obj(t, body)["status"] != "cancelled" {
		t.Fatalf("status after cancel = %v, want cancelled", obj(t, body)["status"])
	}

	// Every ticket came back, on every order.
	for _, id := range []string{samOrder, adaOrder} {
		_, view := e.call("GET", "/api/orders/"+id, "", nil)
		o := obj(t, view)
		if num(t, o, "outstandingCents") != 0 {
			t.Errorf("order %s still owes %v cents", id, o["outstandingCents"])
		}
		line := obj(t, list(t, o["lines"])[0])
		if num(t, line, "refunded") != num(t, line, "quantity") {
			t.Errorf("order %s was not fully refunded: %v", id, line)
		}
	}

	// The room is whole again, on every tier, and the show is off sale.
	_, organizerView := e.call("GET", "/api/organizer/events/jazz-trio", organizerToken, nil)
	general := tierOf(t, obj(t, organizerView), "general")
	if num(t, general, "seats") != 3 || num(t, general, "spokenFor") != 0 || num(t, general, "remaining") != 3 {
		t.Errorf("general after cancelling = %v, want all 3 published seats free", general)
	}
	front := tierOf(t, obj(t, organizerView), "front-row")
	if num(t, front, "seats") != 2 || num(t, front, "spokenFor") != 0 || num(t, front, "remaining") != 2 {
		t.Errorf("front-row after cancelling = %v, want all 2 published seats free", front)
	}
	if status, listed := e.call("GET", "/api/events", "", nil); status != http.StatusOK || len(list(t, listed)) != 0 {
		t.Fatalf("a cancelled event is still on sale: %d %v", status, listed)
	}

	// Its page still tells the truth to anyone holding the link.
	status, page := e.call("GET", "/api/events/jazz-trio", "", nil)
	if status != http.StatusOK || obj(t, page)["status"] != "cancelled" {
		t.Fatalf("public page of a cancelled event = %d %v", status, page)
	}

	// Nothing sells, nothing changes, and it never comes back.
	refusals := []struct {
		name   string
		method string
		path   string
		token  string
		body   any
		want   int
	}{
		{"holding seats", "POST", "/api/events/jazz-trio/holds", "", map[string]any{"tier": "general", "seats": 1}, http.StatusConflict},
		{"cancelling twice", "POST", "/api/organizer/events/jazz-trio/cancel", organizerToken, nil, http.StatusConflict},
		{"publishing again", "POST", "/api/organizer/events/jazz-trio/publish", organizerToken, nil, http.StatusConflict},
		{"editing", "PUT", "/api/organizer/events/jazz-trio", organizerToken, map[string]any{
			"tiers": []map[string]any{{"name": "general", "priceCents": 1, "seats": 1}},
		}, http.StatusConflict},
		{
			"refunding again", "POST", "/api/organizer/orders/" + samOrder + "/refunds", organizerToken,
			map[string]any{"tier": "general", "quantity": 1},
			http.StatusConflict,
		},
	}
	for _, c := range refusals {
		if status, got := e.call(c.method, c.path, c.token, c.body); status != c.want {
			t.Errorf("%s on a cancelled event = %d %v, want %d", c.name, status, got, c.want)
		}
	}
}

func TestCancelRefusals(t *testing.T) {
	e := start(t)
	if status, body := e.call("POST", "/api/organizer/events", organizerToken, jazzTrio); status != http.StatusCreated {
		t.Fatalf("create: %d %v", status, body)
	}
	cases := []struct {
		name  string
		path  string
		token string
		want  int
	}{
		{"without the organizer token", "/api/organizer/events/jazz-trio/cancel", "", http.StatusUnauthorized},
		{"an event nobody created", "/api/organizer/events/nope/cancel", organizerToken, http.StatusNotFound},
		{"a draft that never went on sale", "/api/organizer/events/jazz-trio/cancel", organizerToken, http.StatusConflict},
	}
	for _, c := range cases {
		if status, body := e.call("POST", c.path, c.token, nil); status != c.want {
			t.Errorf("%s = %d %v, want %d", c.name, status, body, c.want)
		}
	}
	// The refused cancel left the draft a draft, invisible as ever.
	if status, _ := e.call("GET", "/api/events/jazz-trio", "", nil); status != http.StatusNotFound {
		t.Fatalf("a draft leaked after a refused cancel, got %d", status)
	}
	if ev, ok := e.findEvent("jazz-trio"); !ok || ev["status"] != "draft" {
		t.Fatalf("draft status after a refused cancel = %v", ev)
	}
}

func TestExpiredHoldRefusesTheOrder(t *testing.T) {
	e := start(t)
	e.call("POST", "/api/organizer/events", organizerToken, jazzTrio)
	e.call("POST", "/api/organizer/events/jazz-trio/publish", organizerToken, nil)

	_, held := e.call("POST", "/api/events/jazz-trio/holds", "", map[string]any{"tier": "general", "seats": 2})
	holdID := obj(t, held)["holdId"].(string)

	*e.now = e.now.Add(6 * time.Minute)

	status, refused := e.call("POST", "/api/orders", "", map[string]any{
		"eventId":  "jazz-trio",
		"holdIds":  []string{holdID},
		"attendee": map[string]any{"name": "Sam", "email": "sam@example.com"},
	})
	if status != http.StatusConflict {
		t.Fatalf("order on expired hold = %d %v, want 409", status, refused)
	}

	// The expired hold's seats are free again.
	_, detail := e.call("GET", "/api/events/jazz-trio", "", nil)
	tier := obj(t, list(t, obj(t, detail)["tiers"])[0])
	if tier["remaining"].(float64) != 3 {
		t.Fatalf("remaining = %v, want all 3 back", tier["remaining"])
	}
}

func TestOrganizerGate(t *testing.T) {
	e := start(t)
	if status, _ := e.call("POST", "/api/organizer/events", "", jazzTrio); status != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", status)
	}
	if status, _ := e.call("POST", "/api/organizer/events", "wrong", jazzTrio); status != http.StatusUnauthorized {
		t.Fatalf("wrong token = %d, want 401", status)
	}
}

func TestOrganizerTakesADraftToPublished(t *testing.T) {
	e := start(t)

	// A fresh draft the way the web app creates one: title only, one
	// unpriced general tier. Publishing it is rightly refused.
	blank := map[string]any{
		"title": "Winter Recital",
		"tiers": []map[string]any{{"name": "general", "priceCents": 0, "seats": 50}},
	}
	if status, body := e.call("POST", "/api/organizer/events", organizerToken, blank); status != http.StatusCreated {
		t.Fatalf("create: %d %v", status, body)
	}
	if status, _ := e.call("POST", "/api/organizer/events/winter-recital/publish", organizerToken, nil); status != http.StatusUnprocessableEntity {
		t.Fatalf("publish unpriced = %d, want 422", status)
	}

	// The organizer fills the page in: story, when, where, priced
	// tiers with their seats; one tier added, seats recounted.
	edit := map[string]any{
		"story": "The students' season finale.",
		"when":  "December 12, 19:00",
		"where": "Main Hall",
		"tiers": []map[string]any{
			{"name": "general", "priceCents": 1200, "seats": 80},
			{"name": "front-row", "priceCents": 2000, "seats": 10},
		},
	}
	status, edited := e.call("PUT", "/api/organizer/events/winter-recital", organizerToken, edit)
	if status != http.StatusOK {
		t.Fatalf("edit: %d %v", status, edited)
	}
	view := obj(t, edited)
	if view["story"] != "The students' season finale." || view["where"] != "Main Hall" {
		t.Fatalf("edit not applied: %v", view)
	}
	tiers := list(t, view["tiers"])
	if len(tiers) != 2 {
		t.Fatalf("tiers after edit = %v", tiers)
	}
	front := obj(t, tiers[1])
	if front["priceCents"].(float64) != 2000 || front["remaining"].(float64) != 10 {
		t.Fatalf("front-row after edit = %v", front)
	}

	// The edit stayed the organizer's secret; then publishing works.
	if status, _ := e.call("GET", "/api/events/winter-recital", "", nil); status != http.StatusNotFound {
		t.Fatalf("edited draft leaked, got %d", status)
	}
	if status, body := e.call("POST", "/api/organizer/events/winter-recital/publish", organizerToken, nil); status != http.StatusOK {
		t.Fatalf("publish: %d %v", status, body)
	}
	if status, _ := e.call("GET", "/api/events/winter-recital", "", nil); status != http.StatusOK {
		t.Fatalf("published event not public, got %d", status)
	}

	// Published means done: no more edits, from anyone.
	if status, _ := e.call("PUT", "/api/organizer/events/winter-recital", organizerToken, edit); status != http.StatusConflict {
		t.Fatalf("edit after publish = %d, want 409", status)
	}
	if status, _ := e.call("PUT", "/api/organizer/events/winter-recital", "", edit); status != http.StatusUnauthorized {
		t.Fatalf("edit without token = %d, want 401", status)
	}
}

func TestEditRefusesBadTierLists(t *testing.T) {
	e := start(t)
	blank := map[string]any{
		"title": "Winter Recital",
		"tiers": []map[string]any{{"name": "general", "priceCents": 0, "seats": 50}},
	}
	e.call("POST", "/api/organizer/events", organizerToken, blank)

	cases := []struct {
		name string
		edit map[string]any
		want int
	}{
		{
			name: "zero seats",
			edit: map[string]any{"tiers": []map[string]any{{"name": "general", "priceCents": 1200, "seats": 0}}},
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "negative price",
			edit: map[string]any{"tiers": []map[string]any{{"name": "general", "priceCents": -1, "seats": 5}}},
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "nameless tier",
			edit: map[string]any{"tiers": []map[string]any{{"name": "", "priceCents": 1200, "seats": 5}}},
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "duplicate tiers",
			edit: map[string]any{"tiers": []map[string]any{
				{"name": "general", "priceCents": 1200, "seats": 5},
				{"name": "general", "priceCents": 2000, "seats": 5},
			}},
			want: http.StatusConflict,
		},
		{
			name: "unknown event",
			edit: map[string]any{"tiers": []map[string]any{}},
			want: http.StatusNotFound,
		},
	}
	for _, c := range cases {
		path := "/api/organizer/events/winter-recital"
		if c.name == "unknown event" {
			path = "/api/organizer/events/nope"
		}
		if status, body := e.call("PUT", path, organizerToken, c.edit); status != c.want {
			t.Fatalf("%s = %d %v, want %d", c.name, status, body, c.want)
		}
	}
	// None of the refused edits reached the stored draft.
	status, body := e.call("GET", "/api/events/winter-recital", organizerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("organizer view of draft: %d", status)
	}
	tier := obj(t, list(t, obj(t, body)["tiers"])[0])
	if tier["priceCents"].(float64) != 0 || tier["remaining"].(float64) != 50 {
		t.Fatalf("a refused edit stuck: %v", tier)
	}
}

func TestUnpricedTierRefusesPublish(t *testing.T) {
	e := start(t)
	draft := map[string]any{
		"title": "Winter Recital",
		"tiers": []map[string]any{{"name": "general", "priceCents": 0, "seats": 80}},
	}
	if status, body := e.call("POST", "/api/organizer/events", organizerToken, draft); status != http.StatusCreated {
		t.Fatalf("create: %d %v", status, body)
	}
	status, refused := e.call("POST", "/api/organizer/events/winter-recital/publish", organizerToken, nil)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("publish unpriced = %d %v, want 422", status, refused)
	}
}
