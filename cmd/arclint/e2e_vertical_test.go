package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func initVertical(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	stdout, stderr, code := runBin(t, root, os.Environ(), "init", "--pattern", "vertical")
	if code != 0 {
		t.Fatalf("init --pattern vertical exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	return root
}

func writeCompliantVertical(t *testing.T, root string) {
	t.Helper()
	write(t, root, "go.mod", "module fixture\n\ngo 1.22\n")
	write(t, root, "internal/order/application/create_order.go", `package application

import "context"

type CreateOrderCommand struct{}

func CreateOrder(ctx context.Context, cmd CreateOrderCommand) error {
	return nil
}
`)
	write(t, root, "internal/order/application/order_repository.go", `package application

import "context"

type OrderRepository interface {
	Find(ctx context.Context, id string) error
}
`)
	write(t, root, "internal/order/application/ports.go", `package application

type Port interface {
	Name() string
}
`)
	write(t, root, "internal/order/application/dto.go", `package application

type OrderDTO struct {
	ID string
}
`)
	write(t, root, "internal/order/application/errors.go", `package application

type AppError struct{}
`)
	for _, concern := range []string{
		"logging", "tracing", "metrics", "config", "security",
		"errors", "cache", "db", "validation", "events",
	} {
		write(t, root, "internal/shared/"+concern+"/doc.go", "package "+concern+"\n")
	}
}

func activeVerticalIDs(t *testing.T, root string) (code int, ids []string, messages []string, stdout string) {
	t.Helper()
	out, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
	var diagnostics []diagnosticDoc
	if err := json.Unmarshal([]byte(out), &diagnostics); err != nil {
		t.Fatalf("json: %v\nstdout: %s\nstderr: %s", err, out, stderr)
	}
	for _, d := range diagnostics {
		if d.Kind == "violation" && d.Status == "active" {
			ids = append(ids, d.RuleID)
			messages = append(messages, d.Message)
		}
	}
	return code, ids, messages, out
}

func TestVerticalInitInstallsExtensions(t *testing.T) {
	root := initVertical(t)
	want := []string{
		"vertical_forbid_imports.ts",
		"vertical_repository_context.ts",
		"vertical_repository_location.ts",
		"vertical_shared_concerns.ts",
		"vertical_usecase.ts",
	}
	for _, name := range want {
		if _, err := os.Stat(filepath.Join(root, ".arclint", "extensions", name)); err != nil {
			t.Errorf("missing installed extension %s: %v", name, err)
		}
	}
}

func TestVerticalCompliantTree(t *testing.T) {
	root := initVertical(t)
	writeCompliantVertical(t, root)
	code, ids, _, stdout := activeVerticalIDs(t, root)
	if code != 0 || len(ids) != 0 {
		t.Errorf("compliant tree: exit %d ids %v\n%s", code, ids, stdout)
	}
}

func TestVerticalDomainNoContext(t *testing.T) {
	root := initVertical(t)
	writeCompliantVertical(t, root)
	write(t, root, "internal/order/domain/model.go", "package domain\n\nimport \"context\"\n\nfunc unused(_ context.Context) {}\n")
	code, ids, _, stdout := activeVerticalIDs(t, root)
	if code != 1 || len(ids) != 1 || ids[0] != "vertical:domain/no-context" {
		t.Errorf("exit %d ids %v, want only vertical:domain/no-context\n%s", code, ids, stdout)
	}
}

func TestVerticalDomainNoIO(t *testing.T) {
	root := initVertical(t)
	writeCompliantVertical(t, root)
	write(t, root, "internal/order/domain/model.go", "package domain\n\nimport \"os\"\n")
	code, ids, _, stdout := activeVerticalIDs(t, root)
	if code != 1 || len(ids) != 1 || ids[0] != "vertical:domain/no-io" {
		t.Errorf("exit %d ids %v, want only vertical:domain/no-io\n%s", code, ids, stdout)
	}
}

func TestVerticalRepositoryOutsideApplication(t *testing.T) {
	root := initVertical(t)
	writeCompliantVertical(t, root)
	write(t, root, "pkg/customer_repository.go", `package pkg

type CustomerRepository interface {
	Find(id string) error
}
`)
	code, ids, _, stdout := activeVerticalIDs(t, root)
	if code != 1 || len(ids) != 1 || ids[0] != "vertical:repositories/application-only" {
		t.Errorf("exit %d ids %v, want only vertical:repositories/application-only\n%s", code, ids, stdout)
	}
}

func TestVerticalRepositoryContextRequired(t *testing.T) {
	root := initVertical(t)
	writeCompliantVertical(t, root)
	write(t, root, "internal/order/application/order_repository.go", `package application

type OrderRepository interface {
	Find(id string) error
}
`)
	code, ids, messages, stdout := activeVerticalIDs(t, root)
	if code != 1 || len(ids) != 1 || ids[0] != "vertical:application/repository-context" {
		t.Errorf("exit %d ids %v, want only vertical:application/repository-context\n%s", code, ids, stdout)
	}
	if len(messages) != 1 || !strings.Contains(messages[0], `Repository method "OrderRepository.Find" must take ctx context.Context as its first parameter`) {
		t.Errorf("message = %v", messages)
	}
}

func TestVerticalUseCaseWrongFile(t *testing.T) {
	root := initVertical(t)
	writeCompliantVertical(t, root)
	os.Remove(filepath.Join(root, "internal/order/application/create_order.go"))
	write(t, root, "internal/order/application/handler.go", `package application

import "context"

type CreateOrderCommand struct{}

func CreateOrder(ctx context.Context, cmd CreateOrderCommand) error {
	return nil
}
`)
	code, ids, messages, stdout := activeVerticalIDs(t, root)
	if code != 1 || len(ids) != 1 || ids[0] != "vertical:application/usecase-contract" {
		t.Errorf("exit %d ids %v, want only vertical:application/usecase-contract\n%s", code, ids, stdout)
	}
	if len(messages) != 1 || messages[0] != `Use case "CreateOrder" must be declared in "create_order.go"` {
		t.Errorf("messages = %v", messages)
	}
}

func TestVerticalUseCaseBadSignature(t *testing.T) {
	root := initVertical(t)
	writeCompliantVertical(t, root)
	write(t, root, "internal/order/application/create_order.go", `package application

func CreateOrder() error { return nil }
`)
	code, ids, messages, stdout := activeVerticalIDs(t, root)
	if code != 1 || len(ids) != 1 || ids[0] != "vertical:application/usecase-contract" {
		t.Errorf("exit %d ids %v, want only vertical:application/usecase-contract\n%s", code, ids, stdout)
	}
	if len(messages) != 1 || messages[0] != `Use case "CreateOrder" must have signature CreateOrder(ctx context.Context, cmd CreateOrderCommand) error` {
		t.Errorf("messages = %v", messages)
	}
}

func TestVerticalSharedUnknownConcern(t *testing.T) {
	root := initVertical(t)
	writeCompliantVertical(t, root)
	write(t, root, "internal/shared/utils/x.go", "package utils\n")
	code, ids, _, stdout := activeVerticalIDs(t, root)
	if code != 1 || len(ids) != 1 || ids[0] != "vertical:shared/concerns" {
		t.Errorf("exit %d ids %v, want only vertical:shared/concerns\n%s", code, ids, stdout)
	}
}

func TestVerticalSharedDirectFile(t *testing.T) {
	root := initVertical(t)
	writeCompliantVertical(t, root)
	write(t, root, "internal/shared/x.go", "package shared\n")
	code, ids, _, stdout := activeVerticalIDs(t, root)
	if code != 1 || len(ids) != 1 || ids[0] != "vertical:shared/concerns" {
		t.Errorf("exit %d ids %v, want only vertical:shared/concerns\n%s", code, ids, stdout)
	}
}

func TestVerticalMissingExtensionFailsLoudly(t *testing.T) {
	root := initVertical(t)
	writeCompliantVertical(t, root)
	if err := os.Remove(filepath.Join(root, ".arclint", "extensions", "vertical_usecase.ts")); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runBin(t, root, os.Environ(), "check")
	if code != 2 {
		t.Fatalf("missing extension must be configuration error, exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, `no extension registers rule "vertical/usecase"`) &&
		!strings.Contains(stdout, `no extension registers rule "vertical/usecase"`) {
		t.Errorf("missing uses must name the unregistered type\nstdout: %s\nstderr: %s", stdout, stderr)
	}
}
