# boxoffice — a small box office for real events

*Never promise a seat the room doesn't have.*

boxoffice is a small box office for live events: organizers publish events with ticket tiers and honest prices, attendees strike deals the box office keeps, and a capacity ledger makes sure no promise outruns the room. When a tier sells out, boxoffice says so before you order, not after.

---

Priya puts on small shows: an open-mic night, a jazz trio, a winter recital. Each event gets its story on the page — a title, a date, a venue, and ticket tiers with their prices: general, front-row, kids. The page is hers to curate; an event stays a draft until she publishes it, and a tier she hasn't priced yet isn't on sale, no matter how big the room is.

When someone wants in, they pick their tickets and place their order. The order remembers the deal as it was struck: which tiers, how many, and at what price that night. If Priya raises the front-row price next week, yesterday's order doesn't change. An order placed is a promise made, and the box office keeps its promises.

Promises can still be given back. Priya can refund a ticket for one buyer, and when a show cannot happen at all she cancels it: every ticket sold comes back at once and the show leaves the sale for good. A refund never rewrites the deal. What was struck stays on the order at the price paid, with the refund recorded beside it, and the seats it returns go back to the room where anyone can buy them again.

The capacity ledger keeps the count. The room is counted in when a tier opens: eighty seats, twenty of them front-row. Every order placed sets seats aside as spoken for; someone still deciding holds their seats only for a while before the ledger lets them go. The ledger has no opinion on stories or prices — only how many seats exist and how many are spoken for. When every seat is spoken for, boxoffice stops promising seats.

Three conversations, one honest box office: Priya deciding what's on sale, attendees striking deals that will be kept, and the ledger making sure no promise outruns the room. On show night, someone at the door checks tickets against what the box office sold.

## What you see

Anyone can walk up. The front page lists what's on sale — published events only, never drafts. An event's page tells its story and offers its tiers; pick your tickets, and while you're deciding your seats are held for a little while. Place the order and the deal is struck; your order page shows what you bought at the prices you paid.

Priya's side of the counter is hers alone. As the organizer she also sees her drafts, writes the stories, prices the tiers, publishes when ready, and watches the counts. Each published show opens its own page for her: the seats every tier went on sale with, next to how many are sold, held, and left, and the list of who bought tickets. From there she refunds a single buyer's ticket or calls the whole show off. Someone who only buys tickets can't create events and never sees a draft — for them, boxoffice is simply what's on sale, and a show that was called off says so instead of vanishing.

## The shape

One Feature-Sliced Design grammar governs both runtimes. On the Go side, `cmd/boxoffice` composes, `internal/app` speaks HTTP (chi, DTOs, the organizer gate), `internal/features` holds one use-case slice per thing that changes the world (publishevent, cancelevent, holdseats, placeorder, refundticket), `internal/entities` holds the aggregates — Event, Order, Capacity — as pure domain logic, and `internal/shared` is kit. On the web, `web/src/app` routes (TanStack Router), `web/src/pages` are screens, `web/src/features` are interactions, and `web/src/shared` carries the api client with one file per aggregate plus the ui kit. The domain is recorded in `ubiquitous-language.yaml`, its invariants map line for line into `requirements.ears`, and `rules.yaml` enforces the whole arrangement — including deriving required structure from the recorded aggregates in both runtimes.

How to work in here — adding a feature, fixing a bug, moving a rule — is written down in [AGENTS.md](AGENTS.md).

## Running

```bash
make run
```

rebuilds everything — npm install, typecheck, vite build, then the single binary with the web app embedded — replaces any running instance, and serves on `:8080`, seeded with Priya's shows and the deals already struck for them, organizer token `dev-organizer`. Run it again after any change; the port always carries the latest build. While working on the web side, `npm --prefix web run dev` serves it on `:5173` with `/api` proxied to the Go server.

The gates: `make check` runs vet, golangci-lint, the Go tests, `arclint check`, and `arclint rules test`. `make web` is the web side on its own — npm install, `tsc --noEmit`, vite build — and `make build-full` embeds that build in the single binary.

## Why this repo exists

boxoffice is also the end-to-end proving ground for [arclint](../../README.md): a real two-runtime app (Go server, React web) whose architecture is governed by arclint throughout — its bounded contexts recorded in `ubiquitous-language.yaml`, its structure and dependencies enforced by `rules.yaml`. The app comes first; the governance demonstrates itself on top of it.
