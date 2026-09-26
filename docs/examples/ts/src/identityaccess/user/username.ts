/** The name a user signs in with, unique within a tenant. */
export class Username {
  private constructor(private readonly value: string) {}

  /** The one door: invariant length-within-bounds, 3 to 250 characters. */
  static from(text: string): Username {
    const trimmed = text.trim();
    const length = [...trimmed].length;
    if (length < 3 || length > 250) {
      throw new Error("username must be 3 to 250 characters");
    }
    return new Username(trimmed);
  }

  equals(other: Username): boolean {
    return this.value === other.value;
  }

  toString(): string {
    return this.value;
  }
}
