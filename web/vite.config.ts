// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

import { writeFileSync } from "node:fs";
import { fileURLToPath, URL } from "node:url";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import { VitePWA } from "vite-plugin-pwa";

// The app is served by proteus-server itself, so the API is same-origin in
// production. In development Vite proxies the Connect routes to a local
// server (PROTEUS_URL, default http://localhost:8080).
export default defineConfig({
  plugins: [
    tanstackRouter({ target: "react", autoCodeSplitting: true }),
    react(),
    {
      // dist/.gitkeep is committed so `go:embed all:dist` compiles without a
      // build; a build empties dist/, so put it back.
      name: "keep-dist",
      closeBundle: () => writeFileSync(fileURLToPath(new URL("./dist/.gitkeep", import.meta.url)), ""),
    },
    VitePWA({
      registerType: "autoUpdate",
      includeAssets: ["icon.svg"],
      manifest: {
        name: "Proteus template builder",
        short_name: "Proteus",
        description: "Build invoices, quotations, letters and other documents as templates and generate PDFs.",
        theme_color: "#3b5bdb",
        background_color: "#f8f9fa",
        display: "standalone",
        start_url: "/",
        icons: [{ src: "/icon.svg", sizes: "any", type: "image/svg+xml", purpose: "any" }],
      },
      workbox: {
        // The API is never cached: documents must always come from the server.
        navigateFallbackDenylist: [/^\/proteus\.v1\./, /^\/grpc\./],
        globPatterns: ["**/*.{js,css,html,svg,woff2}"],
        maximumFileSizeToCacheInBytes: 4 * 1024 * 1024,
      },
    }),
  ],
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  worker: { format: "es" },
  build: {
    target: "es2022",
    sourcemap: false,
  },
  server: {
    port: 5173,
    proxy: {
      "/proteus.v1.ConversionService": { target: process.env.PROTEUS_URL ?? "http://localhost:8080", changeOrigin: true },
      "/proteus.v1.TemplateService": { target: process.env.PROTEUS_URL ?? "http://localhost:8080", changeOrigin: true },
    },
  },
});
