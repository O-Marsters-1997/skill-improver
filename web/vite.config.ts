import path from "node:path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  build: {
    // The build output is embedded by internal/server/server.go via `//go:embed web`,
    // replacing the hand-written static files that used to live there.
    outDir: "../internal/server/web",
    // true would delete the committed not-built.html fallback the Go embed depends on.
    emptyOutDir: false,
    cssCodeSplit: false,
    assetsDir: "",
    rolldownOptions: {
      output: {
        codeSplitting: false,
        entryFileNames: "app.js",
        assetFileNames: "app[extname]",
      },
    },
  },
  server: {
    proxy: {
      "/api": { target: "http://127.0.0.1:8420", changeOrigin: true },
    },
  },
});
