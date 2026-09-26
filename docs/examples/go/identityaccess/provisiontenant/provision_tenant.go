// Package provisiontenant is the ProvisionTenant use case: one request,
// one flow, one transaction. It translates the request, delegates the
// process to the TenantProvisioningService, and publishes what happened.
// It holds no business rule.
package provisiontenant

import (
	"context"
	"fmt"

	"example.com/saasovation/identityaccess/event"
	"example.com/saasovation/identityaccess/provisioning"
	"example.com/saasovation/identityaccess/tenant"
	"example.com/saasovation/identityaccess/user"
)

// EventPublisher is the use case's outbound port for domain events.
type EventPublisher interface {
	Publish(ctx context.Context, events []event.Event) error
}

// UnitOfWork is the use case's outbound port for transaction control; the
// function it runs either commits whole or rolls back whole.
type UnitOfWork interface {
	Run(ctx context.Context, work func(ctx context.Context) error) error
}

// Outcome is what the use case reports back: the identities created and
// the one-time password to deliver to the administrator.
type Outcome struct {
	TenantID        tenant.TenantID
	AdministratorID user.UserID
	InitialPassword user.Password
}

// ProvisionTenant is the use case.
type ProvisionTenant struct {
	service   *provisioning.TenantProvisioningService
	publisher EventPublisher
	unit      UnitOfWork
}

// NewProvisionTenant wires the use case to its collaborators.
func NewProvisionTenant(service *provisioning.TenantProvisioningService, publisher EventPublisher, unit UnitOfWork) *ProvisionTenant {
	return &ProvisionTenant{service: service, publisher: publisher, unit: unit}
}

// ANCHOR: application-service

// Execute runs the use case flow.
func (uc *ProvisionTenant) Execute(ctx context.Context, req ProvisionTenantRequest) (Outcome, error) {
	name, err := tenant.NewTenantName(req.TenantName)
	if err != nil {
		return Outcome{}, fmt.Errorf("provision tenant: %w", err)
	}
	fullName, err := user.NewFullName(req.AdministratorFirstName, req.AdministratorLastName)
	if err != nil {
		return Outcome{}, fmt.Errorf("provision tenant: %w", err)
	}
	email, err := user.NewEmailAddress(req.AdministratorEmailAddress)
	if err != nil {
		return Outcome{}, fmt.Errorf("provision tenant: %w", err)
	}

	var provisioned provisioning.Provisioned
	err = uc.unit.Run(ctx, func(ctx context.Context) error {
		var err error
		provisioned, err = uc.service.ProvisionTenant(ctx, name, req.TenantDescription, provisioning.Administrator{Name: fullName, Email: email})
		return err
	})
	if err != nil {
		return Outcome{}, err
	}
	if err := uc.publisher.Publish(ctx, provisioned.Events); err != nil {
		return Outcome{}, fmt.Errorf("provision tenant %s: publish events: %w", provisioned.TenantID, err)
	}
	return Outcome{TenantID: provisioned.TenantID, AdministratorID: provisioned.AdministratorID, InitialPassword: provisioned.InitialPassword}, nil
}

// ANCHOR_END: application-service
