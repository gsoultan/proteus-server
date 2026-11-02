// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

import { ActionIcon, Anchor, type ActionIconProps, Badge, Box, Button, Code, Group, Modal, PasswordInput, Stack, Text, TextInput, Tooltip } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { IconKey } from "@tabler/icons-react";
import { Link } from "@tanstack/react-router";
import { useState, type ReactNode } from "react";

import { useSettings } from "@/api/client";
import { useConnection } from "@/api/queries";

export function Logo() {
  return (
    <Group gap={10} wrap="nowrap">
      <svg width="28" height="28" viewBox="0 0 28 28" aria-hidden="true">
        <rect width="28" height="28" rx="7" fill="#3b5bdb" />
        <path d="M9 7h7a5 5 0 0 1 0 10h-3v4H9z" fill="none" stroke="#fff" strokeWidth="2.2" strokeLinejoin="round" />
      </svg>
      <Text fw={700} fz={17}>
        Proteus
      </Text>
    </Group>
  );
}

export function ConnectionBadge() {
  const q = useConnection();
  if (q.isPending) {
    return <Badge variant="light" color="gray">Connecting…</Badge>;
  }
  if (q.isError) {
    return (
      <Tooltip label={String(q.error)}>
        <Badge variant="light" color="red">Not connected</Badge>
      </Tooltip>
    );
  }
  return (
    <Badge variant="light" color="green" leftSection={<Box w={8} h={8} bg="green.7" style={{ borderRadius: 999 }} />}>
      Connected
    </Badge>
  );
}

export function SettingsButton() {
  const { baseUrl, apiKey, setSettings } = useSettings();
  const [opened, { open, close }] = useDisclosure(false);
  const [url, setUrl] = useState(baseUrl);
  const [key, setKey] = useState(apiKey);
  const label = apiKey ? `key: ${apiKey.split(":")[0]?.slice(0, 12) ?? "set"}` : "no key";
  return (
    <>
      <Tooltip label="Server and API key">
        <Button variant="default" size="compact-sm" leftSection={<IconKey size={14} />} onClick={() => { setUrl(baseUrl); setKey(apiKey); open(); }}>
          <Code fz={12} bg="transparent">{label}</Code>
        </Button>
      </Tooltip>
      <Modal opened={opened} onClose={close} title="Connection" centered>
        <Stack>
          <TextInput label="Server URL" description="Leave empty when this app is served by proteus-server itself." placeholder={window.location.origin} value={url} onChange={(e) => setUrl(e.currentTarget.value)} />
          <PasswordInput label="API key" description="Sent as a bearer token; kept only in this browser." value={key} onChange={(e) => setKey(e.currentTarget.value)} />
          <Group justify="flex-end">
            <Button variant="default" onClick={close}>Cancel</Button>
            <Button onClick={() => { setSettings({ baseUrl: url.trim().replace(/\/$/, ""), apiKey: key.trim() }); close(); }}>Save</Button>
          </Group>
        </Stack>
      </Modal>
    </>
  );
}

export function Header({ crumbs, children }: { crumbs?: { label: string; to?: string }[]; children?: ReactNode }) {
  return (
    <Box component="header" h={60} px={20} bg="white" style={{ borderBottom: "1px solid var(--mantine-color-gray-3)", display: "flex", alignItems: "center", gap: 16 }}>
      <Logo />
      {crumbs && (
        <Group component="nav" aria-label="Breadcrumb" gap={6} fz={13} c="dimmed" wrap="nowrap">
          {crumbs.map((c, i) => (
            <Group key={i} gap={6} wrap="nowrap">
              {i > 0 && <span aria-hidden="true">/</span>}
              {c.to ? (
                <Anchor component={Link} to={c.to} fz={13}>
                  {c.label}
                </Anchor>
              ) : (
                <Text fz={13} span>{c.label}</Text>
              )}
            </Group>
          ))}
        </Group>
      )}
      <Box style={{ flex: 1 }} />
      {children}
      <ConnectionBadge />
      <SettingsButton />
    </Box>
  );
}

export function IconButton({ label, children, ...rest }: { label: string; children: ReactNode; onClick?: () => void; disabled?: boolean } & ActionIconProps) {
  return (
    <Tooltip label={label}>
      <ActionIcon variant="default" size={40} aria-label={label} {...rest}>
        {children}
      </ActionIcon>
    </Tooltip>
  );
}
