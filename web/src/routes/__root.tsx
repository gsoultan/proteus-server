// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

import { Box } from "@mantine/core";
import type { QueryClient } from "@tanstack/react-query";
import { Outlet, createRootRouteWithContext } from "@tanstack/react-router";

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: () => (
    <Box mih="100vh" bg="gray.0" style={{ display: "flex", flexDirection: "column" }}>
      <Outlet />
    </Box>
  ),
});
