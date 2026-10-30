import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

// Studio builds to web/dist with relative asset URLs, so the same build
// serves from the map server's root and from a nested host path. The
// parent embeds dist into the Go binary; nothing here runs a Node service.
export default defineConfig({
  base: "./",
  plugins: [react()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
    sourcemap: false,
  },
  test: {
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
    environment: "node",
  },
});
