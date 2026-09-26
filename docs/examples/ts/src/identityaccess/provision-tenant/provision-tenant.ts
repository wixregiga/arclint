import type { DomainEvent } from "../event/index.js";
import type { TenantProvisioningService } from "../provisioning/index.js";
import { TenantName, type TenantID } from "../tenant/index.js";
import { EmailAddress, FullName, type Password, type UserID } from "../user/index.js";

/** What the outside world supplies, as plain text. */
export interface ProvisionTenantRequest {
  readonly tenantName: string;
  readonly tenantDescription: string;
  readonly administratorFirstName: string;
  readonly administratorLastName: string;
  readonly administratorEmailAddress: string;
}

/** The use case's outbound port for domain events. */
export interface EventPublisher {
  publish(events: DomainEvent[]): Promise<void>;
}

/** The use case's outbound port for transaction control. */
export interface UnitOfWork {
  run<T>(work: () => Promise<T>): Promise<T>;
}

export interface Outcome {
  readonly tenantId: TenantID;
  readonly administratorId: UserID;
  readonly initialPassword: Password;
}

/**
 * The ProvisionTenant use case: one request, one flow, one transaction.
 * It translates the request, delegates the process to the domain
 * service, and publishes what happened. It holds no business rule.
 */
export class ProvisionTenant {
  constructor(
    private readonly service: TenantProvisioningService,
    private readonly publisher: EventPublisher,
    private readonly unit: UnitOfWork,
  ) {}

  // ANCHOR: application-service
  async execute(request: ProvisionTenantRequest): Promise<Outcome> {
    const name = TenantName.from(request.tenantName);
    const fullName = FullName.of(request.administratorFirstName, request.administratorLastName);
    const email = EmailAddress.from(request.administratorEmailAddress);

    const provisioned = await this.unit.run(() => this.service.provisionTenant(name, request.tenantDescription, { name: fullName, email }));
    await this.publisher.publish(provisioned.events);
    return { tenantId: provisioned.tenantId, administratorId: provisioned.administratorId, initialPassword: provisioned.initialPassword };
  }
  // ANCHOR_END: application-service
}
