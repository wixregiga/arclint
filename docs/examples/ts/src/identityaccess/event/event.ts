/** Something that happened in the identityaccess domain that experts care about. */
export interface DomainEvent {
  readonly occurredOn: Date;
}

/**
 * Collects the events an aggregate raised during one operation. A root
 * holds one; the use case drains it after the aggregate is saved.
 */
export class EventRecorder {
  private pending: DomainEvent[] = [];

  raise(event: DomainEvent): void {
    this.pending.push(event);
  }

  /** Returns and forgets every recorded event, in raising order. */
  drain(): DomainEvent[] {
    const out = this.pending;
    this.pending = [];
    return out;
  }
}
