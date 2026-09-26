/** Names a role within a tenant, unique per tenant. */
export class RoleName {
  private constructor(private readonly value: string) {}

  /** The one door: invariant length-within-bounds, 1 to 250 characters. */
  static from(text: string): RoleName {
    const trimmed = text.trim();
    const length = [...trimmed].length;
    if (length < 1 || length > 250) {
      throw new Error("role name must be 1 to 250 characters");
    }
    return new RoleName(trimmed);
  }

  equals(other: RoleName): boolean {
    return this.value === other.value;
  }

  toString(): string {
    return this.value;
  }
}
