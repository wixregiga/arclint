import type { TenantID } from "../tenant/index.js";

/** The name a person is addressed by. */
export class FullName {
  private constructor(readonly firstName: string, readonly lastName: string) {}

  /** The one door: invariant both-names-present. */
  static of(firstName: string, lastName: string): FullName {
    const first = firstName.trim();
    const last = lastName.trim();
    if (first === "" || last === "") {
      throw new Error("full name needs a first and a last name");
    }
    return new FullName(first, last);
  }

  toString(): string {
    return `${this.firstName} ${this.lastName}`;
  }
}

const ADDRESS = /^[^\s@]+@[^\s@]+\.[^\s@]+$/u;

/** An address mail can be delivered to. */
export class EmailAddress {
  private constructor(readonly address: string) {}

  /** The one door: invariant well-formed. */
  static from(text: string): EmailAddress {
    const trimmed = text.trim();
    if (!ADDRESS.test(trimmed)) {
      throw new Error("email address is malformed");
    }
    return new EmailAddress(trimmed);
  }

  toString(): string {
    return this.address;
  }
}

/** The human behind a user account: their tenant, name, and how to reach them. */
export class Person {
  private constructor(readonly tenantId: TenantID, readonly name: FullName, readonly emailAddress: EmailAddress) {}

  static of(tenantId: TenantID, name: FullName, emailAddress: EmailAddress): Person {
    return new Person(tenantId, name, emailAddress);
  }

  withName(name: FullName): Person {
    return new Person(this.tenantId, name, this.emailAddress);
  }
}
