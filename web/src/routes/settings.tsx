// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// Settings for admins: every server setting (environment values shown but
// locked) and the API keys. Saving settings restarts the server's services
// in place; requests in flight finish first.

import { Alert, Badge, Box, Button, Card, Code, CopyButton, Group, Loader, Modal, PasswordInput, Select, Stack, Switch, Table, Tabs, Text, TextInput, Title, Tooltip } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconCheck, IconCopy, IconKey, IconLock, IconPlus, IconSettings, IconTrash } from "@tabler/icons-react";
import { useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useMemo, useState } from "react";

import { errorMessage, isPermissionDenied, isUnauthenticated, proteus, useSettings } from "@/api/client";
import { keys, useApiKeys, useCreateApiKey, useDeleteApiKey, useServerSettings, useUpdateSettings } from "@/api/queries";
import { Header } from "@/components/Shell";
import { ApiKey_Role, Setting_Kind, Setting_Source, Storage, type ApiKey, type Setting } from "@/gen/proteus/v1/setup_pb.js";
import { changedValues, checkValue, groupSettings, initialValue, locked, sourceLabel } from "@/model/settings";

export const Route = createFileRoute("/settings")({
  component: SettingsPage,
});

function SettingsPage() {
  const [tab, setTab] = useState<string | null>("server");
  return (
    <>
      <Header crumbs={[{ label: "Templates", to: "/" }, { label: "Settings" }]} />
      <Box component="main" maw={1000} w="100%" mx="auto" px={20} py={28} style={{ display: "flex", flexDirection: "column", gap: 18 }}>
        <Box>
          <Title order={1} fz={24}>
            Settings
          </Title>
          <Text c="dimmed" lh={1.5}>
            How this Proteus server behaves, and who may use it.
          </Text>
        </Box>
        <Tabs value={tab} onChange={setTab}>
          <Tabs.List>
            <Tabs.Tab value="server" leftSection={<IconSettings size={14} />}>
              Server
            </Tabs.Tab>
            <Tabs.Tab value="keys" leftSection={<IconKey size={14} />}>
              API keys
            </Tabs.Tab>
          </Tabs.List>
          <Tabs.Panel value="server" pt="lg">
            <ServerSettings />
          </Tabs.Panel>
          <Tabs.Panel value="keys" pt="lg">
            <ApiKeys />
          </Tabs.Panel>
        </Tabs>
      </Box>
    </>
  );
}

function AccessError({ error }: { error: unknown }) {
  if (isPermissionDenied(error)) {
    return (
      <Alert color="yellow" title="Admin key needed">
        This page needs an API key with the admin role. Use the key button at the top right to switch keys.
      </Alert>
    );
  }
  return (
    <Alert color="red" title={isUnauthenticated(error) ? "Set your API key" : "Cannot load settings"}>
      {isUnauthenticated(error) ? "Use the key button at the top right to enter an admin API key." : errorMessage(error)}
    </Alert>
  );
}

/** Polls until the server answers again after restarting its services. */
async function waitForServer(): Promise<void> {
  const { baseUrl } = useSettings.getState();
  await new Promise((r) => setTimeout(r, 500));
  for (let i = 0; i < 60; i++) {
    try {
      await proteus(baseUrl, "").setup.getSetupStatus({});
      return;
    } catch {
      await new Promise((r) => setTimeout(r, 1000));
    }
  }
}

function ServerSettings() {
  const q = useServerSettings();
  const update = useUpdateSettings();
  const qc = useQueryClient();
  const [draft, setDraft] = useState<Record<string, string>>({});
  const [cleared, setCleared] = useState<Set<string>>(new Set());
  const [applying, setApplying] = useState(false);

  const settings = q.data?.settings ?? [];
  const groups = useMemo(() => groupSettings(settings), [settings]);
  const changes = useMemo(() => changedValues(settings, draft, cleared), [settings, draft, cleared]);
  const problems = settings.filter((s) => !locked(s) && checkValue(s.kind, draft[s.name] ?? initialValue(s)));
  const dirty = Object.keys(changes).length > 0;

  if (q.isPending) {
    return <Loader />;
  }
  if (q.isError) {
    return <AccessError error={q.error} />;
  }

  const reset = () => {
    setDraft({});
    setCleared(new Set());
  };

  const save = () => {
    update.mutate(
      { values: changes, revision: q.data.revision },
      {
        onSuccess: async (res) => {
          reset();
          if (res.restarting) {
            setApplying(true);
            await waitForServer();
            setApplying(false);
          }
          notifications.show({ color: "green", message: res.restarting ? "Settings saved and applied." : "Nothing changed." });
          void qc.invalidateQueries({ queryKey: keys.settings });
        },
        onError: (e) => notifications.show({ color: "red", title: "Not saved", message: errorMessage(e), autoClose: false }),
      },
    );
  };

  return (
    <Stack gap="lg">
      <Group gap="xs">
        <Text fz={13} c="dimmed">
          Storage:
        </Text>
        <Badge variant="light">{q.data.storage === Storage.POSTGRES ? "PostgreSQL" : "This server only (memory)"}</Badge>
        <Text fz={13} c="dimmed">
          Data folder <Code>{q.data.dataDir}</Code>
        </Text>
      </Group>
      {groups.editable.map((g) => (
        <Card key={g.name} withBorder radius="md" padding="lg" component="section" aria-label={g.name}>
          <Title order={2} fz={16} mb="sm">
            {g.name}
          </Title>
          <Stack gap="md">
            {g.settings.map((s) => (
              <SettingInput
                key={s.name}
                setting={s}
                value={draft[s.name] ?? initialValue(s)}
                cleared={cleared.has(s.name)}
                onChange={(v) => setDraft((d) => ({ ...d, [s.name]: v }))}
                onClear={() => setCleared((c) => new Set(c).add(s.name))}
              />
            ))}
          </Stack>
        </Card>
      ))}
      <Card withBorder radius="md" padding="lg" component="section" aria-label="Set at startup">
        <Title order={2} fz={16}>
          Set at startup
        </Title>
        <Text fz={13} c="dimmed" mb="sm">
          These are read before settings can be loaded, so they come from environment variables (or, for the database, from setup).
        </Text>
        <Table fz={13} verticalSpacing={6}>
          <Table.Tbody>
            {groups.startup.map((s) => (
              <Table.Tr key={s.name}>
                <Table.Td w="40%">{s.label}</Table.Td>
                <Table.Td>
                  {s.kind === Setting_Kind.SECRET ? <Text fz={13} c="dimmed">{s.isSet ? "set" : "not set"}</Text> : <Code>{s.value || "—"}</Code>}
                </Table.Td>
                <Table.Td>
                  <Code fz={11}>{s.name}</Code>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      </Card>
      <Box pos="sticky" bottom={0} py="sm" bg="gray.0" style={{ zIndex: 2, borderTop: "1px solid var(--mantine-color-gray-3)" }}>
        <Group justify="space-between">
          <Text fz={13} c="dimmed">
            {applying ? "Applying: the server is restarting its services…" : dirty ? `${Object.keys(changes).length} unsaved change(s). Saving restarts the server's services; requests in flight finish first.` : "No unsaved changes."}
          </Text>
          <Group gap="xs">
            <Button variant="default" onClick={reset} disabled={!dirty || update.isPending}>
              Discard
            </Button>
            <Button onClick={save} disabled={!dirty || problems.length > 0} loading={update.isPending || applying}>
              Save and apply
            </Button>
          </Group>
        </Group>
      </Box>
    </Stack>
  );
}

function SettingInput({ setting: s, value, cleared, onChange, onClear }: { setting: Setting; value: string; cleared: boolean; onChange(v: string): void; onClear(): void }) {
  const isLocked = locked(s);
  const problem = isLocked ? null : checkValue(s.kind, value);
  const label = (
    <Group gap={6} component="span">
      <span>{s.label}</span>
      {s.source === Setting_Source.ENVIRONMENT ? (
        <Tooltip label={`Set by ${s.name}; change it there`}>
          <Badge size="xs" variant="light" color="gray" leftSection={<IconLock size={10} />}>
            environment
          </Badge>
        </Tooltip>
      ) : (
        s.source === Setting_Source.SAVED && (
          <Badge size="xs" variant="light">
            {sourceLabel(s)}
          </Badge>
        )
      )}
    </Group>
  );
  const description = (
    <>
      {s.help && <>{s.help} </>}
      <Code fz={10} bg="transparent" c="dimmed" px={0}>
        {s.name}
      </Code>
    </>
  );
  if (s.kind === Setting_Kind.BOOL) {
    return <Switch label={label} description={description} checked={["1", "t", "true", "y", "yes", "on"].includes(value.toLowerCase())} onChange={(e) => onChange(String(e.currentTarget.checked))} disabled={isLocked} />;
  }
  if (s.kind === Setting_Kind.SECRET) {
    return (
      <Group align="flex-end" gap="xs" wrap="nowrap">
        <PasswordInput
          style={{ flex: 1 }}
          label={label}
          description={description}
          placeholder={cleared ? "Will be removed" : s.isSet ? "Set; type to replace" : "Not set"}
          value={value}
          onChange={(e) => onChange(e.currentTarget.value)}
          disabled={isLocked}
          autoComplete="new-password"
        />
        {s.isSet && !isLocked && (
          <Button variant="default" onClick={onClear} disabled={cleared}>
            Remove
          </Button>
        )}
      </Group>
    );
  }
  return (
    <TextInput
      label={label}
      description={description}
      placeholder={s.defaultValue ? `Default: ${s.defaultValue}` : "Empty"}
      value={value}
      onChange={(e) => onChange(e.currentTarget.value)}
      disabled={isLocked}
      error={problem}
      maw={s.kind === Setting_Kind.LIST || s.kind === Setting_Kind.STRING ? undefined : 320}
    />
  );
}

function ApiKeys() {
  const q = useApiKeys();
  const del = useDeleteApiKey();
  const create = useCreateApiKey();
  const [opened, { open, close }] = useDisclosure(false);
  const [name, setName] = useState("");
  const [role, setRole] = useState<string>(String(ApiKey_Role.CLIENT));
  const [created, setCreated] = useState<{ key: ApiKey; secret: string } | null>(null);

  if (q.isPending) {
    return <Loader />;
  }
  if (q.isError) {
    return <AccessError error={q.error} />;
  }

  const submit = () =>
    create.mutate(
      { name: name.trim(), role: Number(role) as ApiKey_Role },
      {
        onSuccess: (res) => {
          close();
          setName("");
          setCreated({ key: res.key!, secret: res.secret });
        },
      },
    );

  return (
    <Stack>
      <Group justify="space-between" align="flex-end">
        <Text c="dimmed" fz={14} maw={600}>
          Apps send a key as a bearer token. Client keys convert documents and use templates; admin keys can also change settings and keys. Each key sees only its own templates and jobs.
        </Text>
        <Button leftSection={<IconPlus size={16} />} onClick={open}>
          New key
        </Button>
      </Group>
      <Card withBorder radius="md" p={0}>
        <Table verticalSpacing="sm" horizontalSpacing="md" highlightOnHover>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Name</Table.Th>
              <Table.Th>Role</Table.Th>
              <Table.Th>Starts with</Table.Th>
              <Table.Th>Created</Table.Th>
              <Table.Th />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {q.data.keys.map((k) => (
              <Table.Tr key={k.id}>
                <Table.Td>
                  <Group gap={6}>
                    <Text fw={600} fz={14}>
                      {k.name}
                    </Text>
                    {k.id === q.data.callerId && (
                      <Badge size="xs" variant="light" color="green">
                        you
                      </Badge>
                    )}
                    {k.fromEnvironment && (
                      <Tooltip label="From PROTEUS_API_KEYS; change it there">
                        <Badge size="xs" variant="light" color="gray" leftSection={<IconLock size={10} />}>
                          environment
                        </Badge>
                      </Tooltip>
                    )}
                  </Group>
                  <Code fz={11} bg="transparent" c="dimmed" px={0}>
                    {k.id}
                  </Code>
                </Table.Td>
                <Table.Td>
                  <Badge variant="light" color={k.role === ApiKey_Role.ADMIN ? "indigo" : "gray"}>
                    {k.role === ApiKey_Role.ADMIN ? "admin" : "client"}
                  </Badge>
                </Table.Td>
                <Table.Td>{k.prefix ? <Code>{k.prefix}…</Code> : <Text c="dimmed">—</Text>}</Table.Td>
                <Table.Td fz={13}>{k.createTime ? new Date(Number(k.createTime.seconds) * 1000).toLocaleString() : "—"}</Table.Td>
                <Table.Td ta="right">
                  {!k.fromEnvironment && k.id !== q.data.callerId && (
                    <Button
                      size="compact-sm"
                      variant="subtle"
                      color="red"
                      leftSection={<IconTrash size={14} />}
                      loading={del.isPending && del.variables === k.id}
                      onClick={() => {
                        if (window.confirm(`Revoke "${k.name}"? Apps using it stop working at once.`)) {
                          del.mutate(k.id, { onError: (e) => notifications.show({ color: "red", message: errorMessage(e) }) });
                        }
                      }}
                    >
                      Revoke
                    </Button>
                  )}
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      </Card>

      <Modal opened={opened} onClose={close} title="New API key" centered>
        <Stack>
          <TextInput label="Name" description="Who or what uses it, e.g. Billing app." value={name} onChange={(e) => setName(e.currentTarget.value)} data-autofocus onKeyDown={(e) => e.key === "Enter" && name.trim() && submit()} />
          <Select
            label="Role"
            value={role}
            onChange={(v) => setRole(v ?? String(ApiKey_Role.CLIENT))}
            data={[
              { value: String(ApiKey_Role.CLIENT), label: "Client: conversions and templates" },
              { value: String(ApiKey_Role.ADMIN), label: "Admin: also settings and keys" },
            ]}
            allowDeselect={false}
          />
          {create.isError && <Alert color="red">{errorMessage(create.error)}</Alert>}
          <Group justify="flex-end">
            <Button variant="default" onClick={close}>
              Cancel
            </Button>
            <Button onClick={submit} loading={create.isPending} disabled={!name.trim()}>
              Create key
            </Button>
          </Group>
        </Stack>
      </Modal>

      <Modal opened={created !== null} onClose={() => setCreated(null)} title={`Key for ${created?.key.name ?? ""}`} centered size="lg">
        <Stack>
          <Alert color="yellow">Copy the key now. The server keeps only a fingerprint of it, so it cannot be shown again.</Alert>
          <Group wrap="nowrap" gap="xs">
            <Code block style={{ flex: 1, wordBreak: "break-all" }} fz={13}>
              {created?.secret}
            </Code>
            <CopyButton value={created?.secret ?? ""}>
              {({ copied, copy }) => (
                <Button variant="default" leftSection={copied ? <IconCheck size={16} /> : <IconCopy size={16} />} onClick={copy}>
                  {copied ? "Copied" : "Copy"}
                </Button>
              )}
            </CopyButton>
          </Group>
          <Group justify="flex-end">
            <Button onClick={() => setCreated(null)}>Done</Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
}
