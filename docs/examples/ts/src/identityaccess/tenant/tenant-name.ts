/** The name a subscribing organisation registers under. */
export class TenantName {
  private constructor(private readonly value: string) {}

  // ANCHOR: value-door
  /** The one door: invariant length-within-bounds, the trimmed name is 1 to 100 characters. */
  static from(text: string): TenantName {
    const trimmed = text.trim();
    if (trimmed === "") {
      throw new Error("tenant name must not be blank");
    }
    if ([...trimmed].length > 100) {
      throw new Error("tenant name must be at most 100 characters");
    }
    return new TenantName(trimmed);
  }
  // ANCHOR_END: value-door

  equals(other: TenantName): boolean {
    return this.value === other.value;
  }

  toString(): string {
    return this.value;
  }
}
