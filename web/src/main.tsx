// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

import "@mantine/core/styles.css";
import "@mantine/notifications/styles.css";

import { MantineProvider, createTheme } from "@mantine/core";
import { Notifications } from "@mantine/notifications";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createRouter } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { registerSW } from "virtual:pwa-register";

import { routeTree } from "./routeTree.gen";

const theme = createTheme({
  primaryColor: "indigo",
  fontFamily: "Inter, system-ui, sans-serif",
  fontFamilyMonospace: "'JetBrains Mono', ui-monospace, monospace",
  defaultRadius: "md",
});

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 10_000, refetchOnWindowFocus: false } },
});

const router = createRouter({ routeTree, context: { queryClient }, defaultPreload: "intent" });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

registerSW({ immediate: true });

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <MantineProvider theme={theme}>
      <Notifications position="bottom-right" />
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </MantineProvider>
  </StrictMode>,
);
