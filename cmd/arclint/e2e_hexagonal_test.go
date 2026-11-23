package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// activeViolations runs check --format json in root and returns the
// rule id and path of every active violation, as "id path".
func activeViolations(t *testing.T, root string) (int, []string) {
	t.Helper()
	out, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
	var diagnostics []diagnosticDoc
	if err := json.Unmarshal([]byte(out), &diagnostics); err != nil {
		t.Fatalf("json: %v\nstdout: %s\nstderr: %s", err, out, stderr)
	}
	var found []string
	for _, d := range diagnostics {
		if d.Kind == "violation" && d.Status == "active" {
			found = append(found, d.RuleID+" "+d.Path)
		}
	}
	slices.Sort(found)
	return code, found
}

// writeHexagonalService writes a conforming Go service laid out the way
// arclint/hexagonal suggests, including cross-zone test wiring and
// non-I/O stdlib / comment / raw-string false-positive traps.
func writeHexagonalService(t *testing.T, root string) {
	t.Helper()
	write(t, root, "go.mod", "module example.com/orders\n\ngo 1.22\n\nrequire github.com/google/uuid v1.6.0\n")
	write(t, root, "internal/adapters/outbound/memory/ids.go", `package memory

import "github.com/google/uuid"

func NewID() string { return uuid.NewString() }
`)
	write(t, root, "internal/shared/log/log.go", `package log

func Info(string) {}
`)
	write(t, root, "internal/domain/order.go", `package domain

import (
	"fmt"
	"io"
	"net/netip"
	"net/url"
)

type OrderID string

// Order carries no `+"`json:\"id\"`"+` tag; `+"`Order`"+` must not emit db:"id".
type Order struct {
	ID    OrderID // no `+"`json:\"id\"`"+` tag here
	Total int     /* no `+"`xml:\"total\"`"+` tag here */
}

const wireDoc = `+"`format yaml:\"name\"`"+`

var _ io.Reader
var _ netip.Addr

func ParseLink(raw string) (*url.URL, error) { return url.Parse(raw) }

func TagHint() string { s := `+"`ok`"+`; return s + "json:\"x\"" + wireDoc }

func NewOrder(id OrderID, total int) (Order, error) {
	o := Order{ID: id, Total: total}
	if err := o.EnsureTotalNotNegative(); err != nil {
		return Order{}, err
	}
	return o, nil
}

func (o Order) EnsureTotalNotNegative() error {
	if o.Total < 0 {
		return fmt.Errorf("order %s: a total is never negative", o.ID)
	}
	return nil
}
`)
	write(t, root, "internal/domain/order_repository.go", `package domain

import "context"

type OrderRepository interface {
	Save(ctx context.Context, o Order) error
}
`)
	write(t, root, "internal/application/place_order.go", `package application

import (
	"context"

	"example.com/orders/internal/domain"
)

func PlaceOrder(ctx context.Context, repo domain.OrderRepository, id string, total int) error {
	o, err := domain.NewOrder(domain.OrderID(id), total)
	if err != nil {
		return err
	}
	return repo.Save(ctx, o)
}
`)
	write(t, root, "internal/application/place_order_test.go", `package application_test

import (
	"context"
	"testing"

	"example.com/orders/internal/adapters/outbound/memory"
	"example.com/orders/internal/application"
)

func TestPlaceOrder(t *testing.T) {
	if err := application.PlaceOrder(context.Background(), memory.New(), "o1", 3); err != nil {
		t.Fatal(err)
	}
}
`)
	write(t, root, "internal/adapters/outbound/memory/orders.go", `package memory

import (
	"context"
	"sync"

	"example.com/orders/internal/domain"
)

type Orders struct {
	mu     sync.Mutex
	orders map[domain.OrderID]domain.Order
}

func New() *Orders { return &Orders{orders: map[domain.OrderID]domain.Order{}} }

func (s *Orders) Save(_ context.Context, o domain.Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[o.ID] = o
	return nil
}
`)
	write(t, root, "internal/adapters/inbound/http/orders.go", `package http

import (
	"net/http"

	"example.com/orders/internal/application"
	"example.com/orders/internal/domain"
	"example.com/orders/internal/shared/log"
)

func PlaceOrder(repo domain.OrderRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := application.PlaceOrder(r.Context(), repo, r.URL.Query().Get("id"), 1); err != nil {
			log.Info(err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	}
}
`)
	write(t, root, "cmd/orders/main.go", `package main

import (
	"log"
	"net/http"

	"example.com/orders/internal/adapters/inbound/http"
	"example.com/orders/internal/adapters/outbound/memory"
)

func main() {
	http.Handle("/orders", http.PlaceOrder(memory.New()))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
`)
}

func TestHexagonalInitExtendsThePattern(t *testing.T) {
	root := t.TempDir()
	stdout, stderr, code := runBin(t, root, os.Environ(), "init", "--pattern", "hexagonal")
	if code != 0 {
		t.Fatalf("init --pattern hexagonal exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	ruleset, err := os.ReadFile(filepath.Join(root, "rules.arclint.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"extends:\n  - pattern: arclint/hexagonal@0.1.0-beta.2\n",
		"      domain:", "      application:", "      inbound:",
		"      outbound:", "      shared:", "      composition:",
	} {
		if !strings.Contains(string(ruleset), want) {
			t.Errorf("init --pattern hexagonal ruleset misses %q:\n%s", want, ruleset)
		}
	}
	stdout, stderr, code = runBin(t, root, os.Environ(), "rules")
	if code != 0 {
		t.Fatalf("rules exit %d\nstderr: %s", code, stderr)
	}
	if lines := strings.Split(strings.TrimSpace(stdout), "\n"); len(lines) != 10 {
		t.Errorf("rules listed = %d, want 10\n%s", len(lines), stdout)
	}
}

func TestHexagonalGoServiceConformsThenEachBoundaryFires(t *testing.T) {
	root := t.TempDir()
	if _, stderr, code := runBin(t, root, os.Environ(), "init", "--pattern", "hexagonal"); code != 0 {
		t.Fatalf("init exit %d\nstderr: %s", code, stderr)
	}
	writeHexagonalService(t, root)
	if code, found := activeViolations(t, root); code != 0 || len(found) != 0 {
		t.Fatalf("conforming service: exit %d, findings %v", code, found)
	}

	write(t, root, "internal/domain/leak.go", `package domain

import _ "example.com/orders/internal/adapters/outbound/memory"
`)
	write(t, root, "internal/domain/uuid.go", `package domain

import _ "github.com/google/uuid"
`)
	write(t, root, "internal/application/leak.go", `package application

import _ "example.com/orders/internal/adapters/inbound/http"
`)
	write(t, root, "internal/application/uuid.go", `package application

import _ "github.com/google/uuid"
`)
	write(t, root, "internal/adapters/inbound/http/leak.go", `package http

import _ "example.com/orders/internal/adapters/outbound/memory"
`)
	write(t, root, "internal/adapters/outbound/memory/leak.go", `package memory

import _ "example.com/orders/internal/adapters/inbound/http"
`)
	write(t, root, "internal/shared/log/leak.go", `package log

import _ "example.com/orders/internal/domain"
`)
	write(t, root, "internal/domain/shared.go", `package domain

import _ "example.com/orders/internal/shared/log"
`)
	write(t, root, "internal/application/shared.go", `package application

import _ "example.com/orders/internal/shared/log"
`)
	write(t, root, "internal/domain/clock.go", `package domain

import "os"

var Host, _ = os.Hostname()
`)
	write(t, root, "internal/domain/dial.go", `package domain

import "net"

var _ = net.Dial
`)
	write(t, root, "internal/domain/tls_dial.go", `package domain

import "crypto/tls"

var _ = tls.Dial
`)
	write(t, root, "internal/application/fetch.go", `package application

import _ "net/http/httputil"
`)
	write(t, root, "internal/domain/payload.go", `package domain

import _ "encoding/json"
`)
	write(t, root, "internal/domain/line.go", `package domain

type Line struct {
	SKU string `+"`"+`
		json:"sku"
	`+"`"+`
}
`)
	write(t, root, "internal/domain/row.go", `package domain

type Row struct {
	ID string "db:\"id\""
}
`)
	write(t, root, "internal/application/helpers.go", `package application
`)
	code, all := activeViolations(t, root)
	if code != 1 {
		t.Errorf("check exit = %d, want 1", code)
	}
	var found []string
	for _, f := range all {
		if !strings.HasPrefix(f, "arclint/hexagonal:dependencies/acyclic ") {
			found = append(found, f)
		}
	}
	want := []string{
		"arclint/hexagonal:application/core-libraries-only internal/application/uuid.go",
		"arclint/hexagonal:application/through-ports internal/application/leak.go",
		"arclint/hexagonal:application/through-ports internal/application/shared.go",
		"arclint/hexagonal:core/concept-named-files internal/application/helpers.go",
		"arclint/hexagonal:core/performs-no-io internal/application/fetch.go",
		"arclint/hexagonal:core/performs-no-io internal/domain/clock.go",
		"arclint/hexagonal:core/performs-no-io internal/domain/dial.go",
		"arclint/hexagonal:core/performs-no-io internal/domain/tls_dial.go",
		"arclint/hexagonal:domain/depends-on-nothing-outside internal/domain/leak.go",
		"arclint/hexagonal:domain/depends-on-nothing-outside internal/domain/shared.go",
		"arclint/hexagonal:domain/depends-on-nothing-outside internal/domain/uuid.go",
		"arclint/hexagonal:domain/no-wire-format internal/domain/line.go",
		"arclint/hexagonal:domain/no-wire-format internal/domain/payload.go",
		"arclint/hexagonal:domain/no-wire-format internal/domain/row.go",
		"arclint/hexagonal:inbound/calls-the-application internal/adapters/inbound/http/leak.go",
		"arclint/hexagonal:outbound/implements-ports internal/adapters/outbound/memory/leak.go",
		"arclint/hexagonal:shared/product-free internal/shared/log/leak.go",
	}
	if !slices.Equal(found, want) {
		t.Errorf("findings =\n  %s\nwant\n  %s", strings.Join(found, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestHexagonalCycleThroughComposition(t *testing.T) {
	root := t.TempDir()
	if _, stderr, code := runBin(t, root, os.Environ(), "init", "--pattern", "hexagonal"); code != 0 {
		t.Fatalf("init exit %d\nstderr: %s", code, stderr)
	}
	writeHexagonalService(t, root)
	write(t, root, "cmd/orders/wiring/wiring.go", `package wiring

import "example.com/orders/internal/adapters/outbound/memory"

func Orders() *memory.Orders { return memory.New() }
`)
	write(t, root, "internal/adapters/inbound/http/wired.go", `package http

import "example.com/orders/cmd/orders/wiring"

var orders = wiring.Orders()
`)
	code, found := activeViolations(t, root)
	if code != 1 {
		t.Errorf("check exit = %d, want 1", code)
	}
	if !slices.Contains(found, "arclint/hexagonal:inbound/calls-the-application internal/adapters/inbound/http/wired.go") {
		t.Errorf("default inbound/calls-the-application did not block composition import:\n  %s", strings.Join(found, "\n  "))
	}
	var sawCycle bool
	for _, f := range found {
		if strings.HasPrefix(f, "arclint/hexagonal:dependencies/acyclic ") {
			sawCycle = true
		} else if f != "arclint/hexagonal:inbound/calls-the-application internal/adapters/inbound/http/wired.go" {
			t.Errorf("unexpected finding %s", f)
		}
	}
	if !sawCycle {
		t.Fatal("a cycle through composition passed dependencies/acyclic")
	}

	ruleset, err := os.ReadFile(filepath.Join(root, "rules.arclint.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "rules.arclint.yaml", string(ruleset)+`
rules:
  arclint/hexagonal:inbound/calls-the-application:
    disable: "Allow composition import to prove dependencies/acyclic still stops round-trips."
  inbound/file-routes:
    rationale: "Allow composition alongside application, domain, and shared."
    on: inbound
    exclude:
      paths: ["**/*_test.go"]
      reason: "Tests wire fakes and adapters across zones."
    imports:
      internal: [application, domain, shared, composition]
      external: allow
`)
	code, found = activeViolations(t, root)
	if code != 1 {
		t.Errorf("override check exit = %d, want 1", code)
	}
	if len(found) == 0 {
		t.Fatal("a cycle through composition passed once inbound allowed composition")
	}
	for _, f := range found {
		if !strings.HasPrefix(f, "arclint/hexagonal:dependencies/acyclic ") {
			t.Errorf("unexpected finding %s; only the cycle is wrong", f)
		}
	}
}

func TestHexagonalEffectCoreAndFileRoutes(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"name": "orders", "private": true, "type": "module",
  "dependencies": {"effect": "4.0.2", "@effect/sql-pg": "4.0.2"}}
`)
	write(t, root, "rules.arclint.yaml", `runtime: [ts]
extends:
  - pattern: arclint/hexagonal@0.1.0-beta.2
    bind:
      domain: src/domain/**
      application: src/application/**
      inbound: src/routes/**
      outbound: src/adapters/**
      shared: src/shared/**
      composition: src/app/**
rules:
  arclint/hexagonal:application/core-libraries-only:
    disable: "The application declares its ports with Effect v4; application/effect-core replaces this rule."
  application/effect-core:
    rationale: "The application uses Effect's root module and no other third-party package."
    on: application
    exclude:
      paths: ["**/*.test.ts", "**/*.spec.ts"]
      reason: "Tests wire fakes and adapters across zones."
    uses: hexagonal/allowed-packages
    with:
      packages: [effect]
  arclint/hexagonal:inbound/calls-the-application:
    disable: "File-based route modules invoke the application through src/app/runtime.ts."
  inbound/file-routes:
    rationale: "Route modules import application, domain, shared, and the runtime module, never outbound."
    on: inbound
    exclude:
      paths: ["**/*.test.ts", "**/*.spec.ts"]
      reason: "Tests wire fakes and adapters across zones."
    imports:
      internal: [application, domain, shared, composition]
      external: allow
`)
	write(t, root, "tsconfig.json", `{
  // The alias new Vite, Next.js, and TanStack projects commonly configure.
  "compilerOptions": { "paths": { "@/*": ["./src/*"] } },
}
`)
	write(t, root, "src/domain/order.ts", `export interface Order {
  readonly id: string
  readonly total: number
}

export const placeOrder = (id: string, total: number): Order => {
  if (total < 0) throw new RangeError("an order total is never negative")
  return { id, total }
}
`)
	write(t, root, "src/application/order_repository.ts", `import { Context, Effect } from "effect"
import type { Order } from "../domain/order.ts"

export class OrderRepository extends Context.Service<OrderRepository, {
  readonly find: (id: string) => Effect.Effect<Order>
}>()("orders/application/OrderRepository") {}
`)
	write(t, root, "src/adapters/pg_order_repository.ts", `import { Effect, Layer } from "effect"
import { PgClient } from "@effect/sql-pg"
import { OrderRepository } from "../application/order_repository.ts"

export const PgOrderRepository = Layer.effect(OrderRepository, Effect.map(PgClient.PgClient.asEffect(), () =>
  OrderRepository.of({ find: (id) => Effect.die(id) })))
`)
	write(t, root, "src/app/runtime.ts", `import { ManagedRuntime } from "effect"
import { PgOrderRepository } from "../adapters/pg_order_repository.ts"

export const runtime = ManagedRuntime.make(PgOrderRepository)
`)
	write(t, root, "src/routes/orders/route.ts", `import { Effect } from "effect"
import { runtime } from "@/app/runtime"
import { OrderRepository } from "../../application/order_repository.ts"

export const GET = (id: string) =>
  runtime.runPromise(Effect.flatMap(OrderRepository.asEffect(), (orders) => orders.find(id)))
`)
	write(t, root, "src/application/order_repository.test.ts", `import { PgClient } from "@effect/sql-pg"

export const fixture = PgClient
`)
	write(t, root, "src/routes/orders/route.test.ts", `import { PgOrderRepository } from "../../adapters/pg_order_repository.ts"

export const fixture = PgOrderRepository
`)
	if code, found := activeViolations(t, root); code != 0 || len(found) != 0 {
		t.Fatalf("conforming Effect app: exit %d, findings %v", code, found)
	}

	write(t, root, "src/application/fetch_rates.ts", `import { HttpClient } from "effect/http"

export const client = HttpClient
`)
	write(t, root, "src/application/orders_db.ts", `import { PgClient } from "@effect/sql-pg"

export const client = PgClient
`)
	write(t, root, "src/domain/sql.ts", `import { PgClient } from "@effect/sql-pg"

export const client = PgClient
`)
	write(t, root, "src/domain/schema.ts", `import { Schema } from "effect"

export const OrderId = Schema.String
`)
	write(t, root, "src/domain/common.tsx", `export const x = 1
`)
	write(t, root, "src/application/export_orders.ts", `import { writeFile } from "node:fs/promises"

export const save = writeFile
`)
	write(t, root, "src/application/local_cache.ts", `import { DatabaseSync } from "node:sqlite"

export const db = DatabaseSync
`)
	write(t, root, "src/routes/orders/leak.ts", `import { PgOrderRepository } from "../../adapters/pg_order_repository.ts"

export const layer = PgOrderRepository
`)
	write(t, root, "src/routes/orders/leak_alias.ts", `import { PgOrderRepository } from "@/adapters/pg_order_repository"

export const layer = PgOrderRepository
`)
	_, found := activeViolations(t, root)
	want := []string{
		"application/effect-core src/application/fetch_rates.ts",
		"application/effect-core src/application/orders_db.ts",
		"arclint/hexagonal:core/concept-named-files src/domain/common.tsx",
		"arclint/hexagonal:core/performs-no-io src/application/export_orders.ts",
		"arclint/hexagonal:core/performs-no-io src/application/local_cache.ts",
		"arclint/hexagonal:domain/depends-on-nothing-outside src/domain/schema.ts",
		"arclint/hexagonal:domain/depends-on-nothing-outside src/domain/sql.ts",
		"inbound/file-routes src/routes/orders/leak.ts",
		"inbound/file-routes src/routes/orders/leak_alias.ts",
	}
	if !slices.Equal(found, want) {
		t.Errorf("findings =\n  %s\nwant\n  %s", strings.Join(found, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestHexagonalPythonMatchesDottedPathsAsWritten(t *testing.T) {
	root := t.TempDir()
	write(t, root, "pyproject.toml", `[project]
name = "orders"
dependencies = ["sqlalchemy>=2"]
`)
	ruleset := `runtime: [py]
extends:
  - pattern: arclint/hexagonal@0.1.0-beta.2
    bind:
      domain: orders/domain/**
      application: orders/service_layer/**
      inbound: orders/entrypoints/**
      outbound: orders/adapters/**
      shared: orders/shared/**
      composition: orders/bootstrap.py
`
	write(t, root, "rules.arclint.yaml", ruleset)
	write(t, root, "orders/__init__.py", "")
	write(t, root, "orders/domain/__init__.py", "")
	write(t, root, "orders/domain/model.py", `from dataclasses import dataclass


@dataclass(frozen=True)
class Order:
    id: str
    total: int
`)
	write(t, root, "orders/domain/status.py", `from http import HTTPStatus
from urllib import parse


def ok_url(path: str) -> tuple[int, str]:
    return HTTPStatus.OK, parse.quote(path)
`)
	write(t, root, "orders/domain/serialize.py", `import json


def encode(order_id: str) -> str:
    return json.dumps({"id": order_id})
`)
	write(t, root, "orders/domain/storage.py", `from pathlib import Path


def save(raw: str) -> None:
    Path("orders.json").write_text(raw)
`)
	write(t, root, "orders/service_layer/__init__.py", "")
	write(t, root, "orders/service_layer/unit_of_work.py", `from sqlalchemy.orm import Session

from orders.domain.model import Order


def place(session: Session, order: Order) -> None:
    session.add(order)
`)
	write(t, root, "orders/service_layer/notify.py", `import subprocess


def notify(message: str) -> None:
    subprocess.run(["notify-send", message], check=True)
`)
	write(t, root, "orders/service_layer/http_from.py", `from http import client


def fetch(host: str) -> client.HTTPConnection:
    return client.HTTPConnection(host)
`)
	write(t, root, "orders/service_layer/urllib_from.py", `from urllib import (
    parse,
    request as req,
)


def fetch(url: str):
    return req.urlopen(parse.quote(url))
`)
	_, found := activeViolations(t, root)
	want := []string{
		"arclint/hexagonal:application/core-libraries-only orders/service_layer/unit_of_work.py",
		"arclint/hexagonal:core/performs-no-io orders/domain/storage.py",
		"arclint/hexagonal:core/performs-no-io orders/service_layer/http_from.py",
		"arclint/hexagonal:core/performs-no-io orders/service_layer/notify.py",
		"arclint/hexagonal:core/performs-no-io orders/service_layer/urllib_from.py",
		"arclint/hexagonal:domain/no-wire-format orders/domain/serialize.py",
	}
	if !slices.Equal(found, want) {
		t.Fatalf("findings = %v, want %v", found, want)
	}

	write(t, root, "rules.arclint.yaml", ruleset+`rules:
  arclint/hexagonal:application/core-libraries-only:
    disable: "Our unit of work uses SQLAlchemy sessions; orders/sessions replaces this rule."
  orders/sessions:
    rationale: "The application uses SQLAlchemy's ORM session and no other third-party package."
    on: application
    uses: hexagonal/allowed-packages
    with:
      packages: [sqlalchemy.orm]
`)
	_, found = activeViolations(t, root)
	want = []string{
		"arclint/hexagonal:core/performs-no-io orders/domain/storage.py",
		"arclint/hexagonal:core/performs-no-io orders/service_layer/http_from.py",
		"arclint/hexagonal:core/performs-no-io orders/service_layer/notify.py",
		"arclint/hexagonal:core/performs-no-io orders/service_layer/urllib_from.py",
		"arclint/hexagonal:domain/no-wire-format orders/domain/serialize.py",
	}
	if !slices.Equal(found, want) {
		t.Errorf("own rule listing sqlalchemy.orm: findings %v, want %v", found, want)
	}
}

const hexagonalOrderingDomain = `version: 1
project: Orders
contexts:
  ordering:
    definition: Taking and keeping customer orders.
    aggregates:
      Order:
        definition: A customer's request for goods, with its total.
        identity: OrderID
        repository: OrderRepository
        invariants:
          total-not-negative: An Order's total is never negative.
    value_objects:
      OrderID:
        definition: The identity of an Order.
`

// TestHexagonalAgreesWithDomainIsolation proves Aggregate Repositories
// declared in domain conform to the built-in domain rules and that the
// Pattern and domain_isolation both refuse shared imports from the core.
func TestHexagonalAgreesWithDomainIsolation(t *testing.T) {
	root := t.TempDir()
	if _, stderr, code := runBin(t, root, os.Environ(), "init", "--pattern", "hexagonal", "--domain", "domain.arclint.yaml"); code != 0 {
		t.Fatalf("init exit %d\nstderr: %s", code, stderr)
	}
	writeHexagonalService(t, root)
	write(t, root, "domain.arclint.yaml", hexagonalOrderingDomain)
	if code, found := activeViolations(t, root); code != 0 || len(found) != 0 {
		t.Fatalf("conforming recorded service: exit %d, findings %v", code, found)
	}

	write(t, root, "internal/domain/shared.go", `package domain

import _ "example.com/orders/internal/shared/log"
`)
	write(t, root, "internal/application/place_order.go", `package application

import (
	"context"

	"example.com/orders/internal/domain"
	_ "example.com/orders/internal/shared/log"
)

func PlaceOrder(ctx context.Context, repo domain.OrderRepository, id string, total int) error {
	o, err := domain.NewOrder(domain.OrderID(id), total)
	if err != nil {
		return err
	}
	return repo.Save(ctx, o)
}
`)
	_, found := activeViolations(t, root)
	want := []string{
		"arclint/hexagonal:application/through-ports internal/application/place_order.go",
		"arclint/hexagonal:domain/depends-on-nothing-outside internal/domain/shared.go",
		"domain_isolation/model-imports-nothing-outside-itself internal/domain/shared.go",
	}
	if !slices.Equal(found, want) {
		t.Errorf("findings =\n  %s\nwant\n  %s", strings.Join(found, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestHexagonalContextZonesIsolateEachHexagon(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/shop\n\ngo 1.22\n")
	ruleset := `runtime: [go]
domain:
  file: domain.arclint.yaml
extends:
  - pattern: arclint/hexagonal@0.1.0-beta.2
    bind:
      domain: "internal/*/domain/**"
      application: "internal/*/app/**"
      inbound: "internal/*/inbound/**"
      outbound: "internal/*/outbound/**"
      shared: "internal/shared/**"
      composition: "cmd/**"
`
	write(t, root, "rules.arclint.yaml", ruleset)
	write(t, root, "domain.arclint.yaml", `version: 1
project: Shop
contexts:
  ordering:
    definition: Taking and keeping customer orders.
    value_objects:
      OrderID:
        definition: The identity of an Order.
  billing:
    definition: Charging customers for what they ordered.
    value_objects:
      InvoiceID:
        definition: The identity of an Invoice.
`)
	write(t, root, "internal/shared/log/log.go", "package log\n\nfunc Info(string) {}\n")
	write(t, root, "internal/ordering/domain/order.go", "package domain\n\ntype OrderID string\n")
	write(t, root, "internal/ordering/outbound/store.go", `package outbound

import "example.com/shop/internal/shared/log"

func Record() { log.Info("saved") }
`)
	write(t, root, "internal/billing/domain/invoice.go", "package domain\n\ntype InvoiceID string\n")
	write(t, root, "internal/billing/app/charge.go", `package app

import "example.com/shop/internal/billing/domain"

func Charge(id domain.InvoiceID) domain.InvoiceID { return id }
`)
	write(t, root, "internal/ordering/app/place.go", `package app

import (
	billing "example.com/shop/internal/billing/app"
	"example.com/shop/internal/ordering/domain"
)

func Place(id domain.OrderID) { _ = billing.Charge("i1") }
`)
	if code, found := activeViolations(t, root); code != 0 || len(found) != 0 {
		t.Fatalf("bindings alone: exit %d, findings %v", code, found)
	}

	write(t, root, "rules.arclint.yaml", ruleset+`zones:
  ordering:
    paths:
      - "internal/ordering/domain/**"
      - "internal/ordering/app/**"
    description: "The ordering context's core."
  billing:
    paths:
      - "internal/billing/domain/**"
      - "internal/billing/app/**"
    description: "The billing context's core."
`)
	_, found := activeViolations(t, root)
	if want := []string{"bounded_context/isolated internal/ordering/app/place.go"}; !slices.Equal(found, want) {
		t.Errorf("context Zones: findings %v, want %v", found, want)
	}
}
