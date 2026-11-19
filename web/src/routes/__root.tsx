// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

import { Box } from "@mantine/core";
import type { QueryClient } from "@tanstack/react-query";
import { Outlet, createRootRouteWithContext, redirect } from "@tanstack/react-router";

import { proteus, useSettings } from "@/api/client";
import { keys } from "@/api/queries";

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  // A server that has not been set up sends every page to the wizard. When
  // the status cannot be read (offline, another server), pages load as usual
  // and show their own errors.
  beforeLoad: async ({ context, location }) => {
    if (location.pathname === "/setup") {
      return;
    }
    const { baseUrl, apiKey } = useSettings.getState();
    const status = await context.queryClient
      .fetchQuery({ queryKey: keys.setupStatus(baseUrl), queryFn: () => proteus(baseUrl, apiKey).setup.getSetupStatus({}), staleTime: 30_000 })
      .catch(() => null);
    if (status?.needsSetup) {
      throw redirect({ to: "/setup" });
    }
  },
  component: () => (
    <Box mih="100vh" bg="gray.0" style={{ display: "flex", flexDirection: "column" }}>
      <Outlet />
    </Box>
  ),
});
