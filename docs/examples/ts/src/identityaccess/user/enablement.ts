/** Whether a user may sign in, and optionally during which period. */
export class Enablement {
  private constructor(
    private readonly enabled: boolean,
    private readonly startDate: Date | undefined,
    private readonly endDate: Date | undefined,
  ) {}

  static indefinite(): Enablement {
    return new Enablement(true, undefined, undefined);
  }

  /** The one door: invariant window-is-whole, a period has both ends and does not end before it starts. */
  static of(enabled: boolean, startDate?: Date, endDate?: Date): Enablement {
    if ((startDate === undefined) !== (endDate === undefined)) {
      throw new Error("enablement window needs both a start and an end");
    }
    if (startDate !== undefined && endDate !== undefined && endDate < startDate) {
      throw new Error("enablement window must not end before it starts");
    }
    return new Enablement(enabled, startDate, endDate);
  }

  isEnabled(): boolean {
    return this.enabled;
  }

  /** Side-effect-free: the flag is on and the instant lies inside the period, if any. */
  isEnablementEnabled(at: Date): boolean {
    if (!this.enabled) {
      return false;
    }
    if (this.startDate === undefined || this.endDate === undefined) {
      return true;
    }
    return at >= this.startDate && at <= this.endDate;
  }
}
