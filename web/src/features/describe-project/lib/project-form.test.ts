import { describe, expect, it } from "vitest";
import { projectRequestFrom } from "./project-form";

describe("projectRequestFrom", () => {
  it("sends only what differs, trimmed, and nothing when nothing does", () => {
    const recorded = { name: "shop", description: "A shop." };
    expect(projectRequestFrom(recorded, { name: "shop", description: "A shop." })).toBeNull();
    expect(projectRequestFrom(recorded, { name: " boutique ", description: "A shop." })).toEqual({ name: "boutique" });
    expect(projectRequestFrom(recorded, { name: "shop", description: "A corner shop." })).toEqual({ description: "A corner shop." });
    expect(projectRequestFrom(recorded, { name: "boutique", description: "" })).toEqual({ name: "boutique", description: "" });
  });

  it("names a project that has no recording yet", () => {
    expect(projectRequestFrom({ name: "", description: "" }, { name: "shop", description: "" })).toEqual({ name: "shop" });
  });
});
