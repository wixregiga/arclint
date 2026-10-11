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

// writeHexagonalService writes a Go service laid out the way
// arclint/hexagonal suggests: a domain, a use case behind a port, a
// driven adapter implementing the port, a driving HTTP handler, shared
// errors, and a main that wires them. The driven adapter uses a
// third-party library, which adapters may do; a test in the application
// wires the driven adapter, which tests may do.
func writeHexagonalService(t *testing.T, root string) {
	t.Helper()
	write(t, root, "go.mod", "module example.com/orders\n\ngo 1.22\n\nrequire github.com/google/uuid v1.6.0\n")
	write(t, root, "internal/adapters/driven/memory/ids.go", `package memory

import "github.com/google/uuid"

func NewID() string { return uuid.NewString() }
`)
	write(t, root, "internal/shared/errs/errs.go", `package errs

import "errors"

var ErrNotFound = errors.New("not found")
`)
	write(t, root, "internal/domain/order.go", `package domain

import (
	"fmt"

	"example.com/orders/internal/shared/errs"
)

type Order struct {
	ID    string
	Total int
}

func NewOrder(id string, total int) (Order, error) {
	if total < 0 {
		return Order{}, fmt.Errorf("order %s: %w", id, errs.ErrNotFound)
	}
	return Order{ID: id, Total: total}, nil
}
`)
	write(t, root, "internal/application/place_order.go", `package application

import (
	"context"

	"example.com/orders/internal/domain"
)

type OrderRepository interface {
	Save(ctx context.Context, o domain.Order) error
}

func PlaceOrder(ctx context.Context, repo OrderRepository, id string, total int) error {
	o, err := domain.NewOrder(id, total)
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

	"example.com/orders/internal/adapters/driven/memory"
	"example.com/orders/internal/application"
)

func TestPlaceOrder(t *testing.T) {
	if err := application.PlaceOrder(context.Background(), memory.New(), "o1", 3); err != nil {
		t.Fatal(err)
	}
}
`)
	write(t, root, "internal/adapters/driven/memory/orders.go", `package memory

import (
	"context"
	"sync"

	"example.com/orders/internal/domain"
)

type Orders struct {
	mu     sync.Mutex
	orders map[string]domain.Order
}

func New() *Orders { return &Orders{orders: map[string]domain.Order{}} }

func (s *Orders) Save(_ context.Context, o domain.Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[o.ID] = o
	return nil
}
`)
	write(t, root, "internal/adapters/driving/http/orders.go", `package http

import (
	"net/http"

	"example.com/orders/internal/application"
)

func PlaceOrder(repo application.OrderRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := application.PlaceOrder(r.Context(), repo, r.URL.Query().Get("id"), 1); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	}
}
`)
	write(t, root, "cmd/orders/main.go", `package main

import (
	"log"
	"net/http"

	"example.com/orders/internal/adapters/driven/memory"
	handlers "example.com/orders/internal/adapters/driving/http"
)

func main() {
	http.Handle("/orders", handlers.PlaceOrder(memory.New()))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
`)
}

// TestHexagonalInitExtendsThePattern proves init --pattern hexagonal
// writes an adopting ruleset that binds all six zones and lists the
// seven rules the pattern distributes.
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
		"extends:\n  - pattern: arclint/hexagonal@0.1.0\n",
		"      domain:", "      application:", "      driving:",
		"      driven:", "      shared:", "      composition:",
	} {
		if !strings.Contains(string(ruleset), want) {
			t.Errorf("init --pattern hexagonal ruleset misses %q:\n%s", want, ruleset)
		}
	}
	stdout, stderr, code = runBin(t, root, os.Environ(), "rules")
	if code != 0 {
		t.Fatalf("rules exit %d\nstderr: %s", code, stderr)
	}
	if lines := strings.Split(strings.TrimSpace(stdout), "\n"); len(lines) != 7 {
		t.Errorf("rules listed = %d, want the 7 the pattern distributes\n%s", len(lines), stdout)
	}
}

// TestHexagonalGoServiceConformsThenEachBoundaryFires checks a
// conforming Go service, then adds one forbidden import per boundary
// and expects exactly those findings.
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

import _ "example.com/orders/internal/adapters/driven/memory"
`)
	write(t, root, "internal/domain/uuid.go", `package domain

import _ "github.com/google/uuid"
`)
	write(t, root, "internal/application/leak.go", `package application

import _ "example.com/orders/internal/adapters/driving/http"
`)
	write(t, root, "internal/adapters/driving/http/leak.go", `package http

import _ "example.com/orders/internal/adapters/driven/memory"
`)
	write(t, root, "internal/adapters/driven/memory/leak.go", `package memory

import _ "example.com/orders/internal/adapters/driving/http"
`)
	write(t, root, "internal/shared/errs/leak.go", `package errs

import _ "example.com/orders/internal/domain"
`)
	code, found := activeViolations(t, root)
	if code != 1 {
		t.Errorf("check exit = %d, want 1", code)
	}
	want := []string{
		"arclint/hexagonal:application/through-ports internal/application/leak.go",
		"arclint/hexagonal:domain/depends-on-nothing-outside internal/domain/leak.go",
		"arclint/hexagonal:domain/standard-library-only internal/domain/uuid.go",
		"arclint/hexagonal:driven/implements-ports internal/adapters/driven/memory/leak.go",
		"arclint/hexagonal:driving/calls-the-application internal/adapters/driving/http/leak.go",
		"arclint/hexagonal:shared/product-free internal/shared/errs/leak.go",
	}
	if !slices.Equal(found, want) {
		t.Errorf("findings =\n  %s\nwant\n  %s", strings.Join(found, "\n  "), strings.Join(want, "\n  "))
	}
}

// TestHexagonalEffectCoreAndFileRoutes checks the TypeScript side: a
// plain TypeScript domain, a Context.Service port in the application, a Layer
// adapter on a SQL driver, a ManagedRuntime in composition, and route
// files a file-based router loads, which reach the application through
// that runtime, written with relative imports and a tsconfig "@/*"
// alias. Any package in the domain (even effect), technology in the
// application (effect/http, a SQL driver), and a route that grabs an
// adapter directly, by either spelling, are the only findings.
func TestHexagonalEffectCoreAndFileRoutes(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"name": "orders", "private": true, "type": "module",
  "dependencies": {"effect": "4.0.2", "@effect/sql-pg": "4.0.2"}}
`)
	write(t, root, "rules.arclint.yaml", `runtime: [ts]
extends:
  - pattern: arclint/hexagonal@0.1.0
    bind:
      domain: src/domain/**
      application: src/application/**
      driving: src/routes/**
      driven: src/adapters/**
      shared: src/shared/**
      composition: src/app/**
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
	write(t, root, "src/routes/orders/leak.ts", `import { PgOrderRepository } from "../../adapters/pg_order_repository.ts"

export const layer = PgOrderRepository
`)
	write(t, root, "src/routes/orders/leak_alias.ts", `import { PgOrderRepository } from "@/adapters/pg_order_repository"

export const layer = PgOrderRepository
`)
	_, found := activeViolations(t, root)
	want := []string{
		"arclint/hexagonal:application/core-libraries-only src/application/fetch_rates.ts",
		"arclint/hexagonal:application/core-libraries-only src/application/orders_db.ts",
		"arclint/hexagonal:domain/standard-library-only src/domain/schema.ts",
		"arclint/hexagonal:domain/standard-library-only src/domain/sql.ts",
		"arclint/hexagonal:driving/calls-the-application src/routes/orders/leak.ts",
		"arclint/hexagonal:driving/calls-the-application src/routes/orders/leak_alias.ts",
	}
	if !slices.Equal(found, want) {
		t.Errorf("findings =\n  %s\nwant\n  %s", strings.Join(found, "\n  "), strings.Join(want, "\n  "))
	}
}

// TestHexagonalPythonMatchesDottedPathsAsWritten checks a Python
// service: the application may use no third-party package, a dotted
// module is judged as written, and a project that allows one writes its
// own imports rule under its own id after disabling the Pattern's.
func TestHexagonalPythonMatchesDottedPathsAsWritten(t *testing.T) {
	root := t.TempDir()
	write(t, root, "pyproject.toml", `[project]
name = "orders"
dependencies = ["sqlalchemy>=2"]
`)
	ruleset := `runtime: [py]
extends:
  - pattern: arclint/hexagonal@0.1.0
    bind:
      domain: orders/domain/**
      application: orders/service_layer/**
      driving: orders/entrypoints/**
      driven: orders/adapters/**
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
	write(t, root, "orders/service_layer/__init__.py", "")
	write(t, root, "orders/service_layer/unit_of_work.py", `from sqlalchemy.orm import Session

from orders.domain.model import Order


def place(session: Session, order: Order) -> None:
    session.add(order)
`)
	_, found := activeViolations(t, root)
	want := []string{"arclint/hexagonal:application/core-libraries-only orders/service_layer/unit_of_work.py"}
	if !slices.Equal(found, want) {
		t.Fatalf("findings = %v, want %v", found, want)
	}

	write(t, root, "rules.arclint.yaml", ruleset+`rules:
  arclint/hexagonal:application/core-libraries-only:
    disable: "Our unit of work uses SQLAlchemy sessions; orders/sessions replaces this rule."
  orders/sessions:
    rationale: "The application may use SQLAlchemy's ORM session and nothing else outside."
    on: application
    imports:
      external: [sqlalchemy.orm]
`)
	if code, found := activeViolations(t, root); code != 0 || len(found) != 0 {
		t.Errorf("own rule listing sqlalchemy.orm: exit %d, findings %v", code, found)
	}
}
