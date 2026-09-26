import type { DomainEvent } from "../../event/index.js";
import type { EventPublisher } from "../../provision-tenant/index.js";

/** Appends published events to an in-memory log that tests read back. */
export class InMemoryEventLog implements EventPublisher {
  private readonly log: DomainEvent[] = [];

  async publish(events: DomainEvent[]): Promise<void> {
    this.log.push(...events);
  }

  published(): readonly DomainEvent[] {
    return [...this.log];
  }
}
