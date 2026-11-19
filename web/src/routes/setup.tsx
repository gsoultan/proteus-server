// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// First-run wizard. The server prints a setup token to its log on first
// start; with it, the wizard picks where data is kept, a few basics, and
// creates the first admin key, which this browser then signs in with. Once
// setup is complete the server refuses the wizard, and the root route
// stops sending anyone here.

import { Alert, Anchor, Badge, Box, Button, Card, Checkbox, Code, CopyButton, Group, Loader, NumberInput, PasswordInput, Radio, Stack, Stepper, Switch, Text, TextInput, Title } from "@mantine/core";
import { IconCheck, IconCopy, IconDatabase, IconServer } from "@tabler/icons-react";
import { useQueryClient } from "@tanstack/react-query";
import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { errorMessage, proteus, useProteus, useSettings } from "@/api/client";
import { keys, useSetupStatus } from "@/api/queries";
import { Logo } from "@/components/Shell";
import { Storage, type TestDatabaseResponse } from "@/gen/proteus/v1/setup_pb.js";

export const Route = createFileRoute("/setup")({
  beforeLoad: async ({ context }) => {
    const { baseUrl, apiKey } = useSettings.getState();
    const status = await context.queryClient
      .fetchQuery({ queryKey: keys.setupStatus(baseUrl), queryFn: () => proteus(baseUrl, apiKey).setup.getSetupStatus({}), staleTime: 0 })
      .catch(() => null);
    if (status && !status.needsSetup) {
      throw redirect({ to: "/" });
    }
  },
  component: SetupPage,
});

const STEPS = ["Setup token", "Storage", "Basics", "Admin key"];

function SetupPage() {
  const api = useProteus();
  const status = useSetupStatus();
  const setSettings = useSettings((s) => s.setSettings);
  const qc = useQueryClient();
  const navigate = useNavigate();

  const [step, setStep] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const [token, setToken] = useState("");
  const [storage, setStorage] = useState<"postgres" | "memory">("postgres");
  const [dbUrl, setDbUrl] = useState("");
  const [dbSchema, setDbSchema] = useState("public");
  const [migrate, setMigrate] = useState(true);
  const [dbTest, setDbTest] = useState<TestDatabaseResponse | null>(null);
  const [allowRemote, setAllowRemote] = useState(false);
  const [allowedHosts, setAllowedHosts] = useState("");
  const [rateLimit, setRateLimit] = useState<number | string>(10);
  const [keyName, setKeyName] = useState("Admin");
  const [secret, setSecret] = useState("");
  const [saved, setSaved] = useState(false);
  const [ready, setReady] = useState(false);

  const fromEnv = status.data?.storageFromEnvironment ?? false;
  const database = { url: dbUrl.trim(), schema: dbSchema.trim(), autoMigrate: migrate };

  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError("");
    try {
      await fn();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  const checkToken = () =>
    run(async () => {
      await api.setup.checkSetupToken({ setupToken: token.trim() });
      setStep(1);
    });

  const testDb = () =>
    run(async () => {
      setDbTest(await api.setup.testDatabase({ setupToken: token.trim(), database }));
    });

  const finish = () =>
    run(async () => {
      const settings: Record<string, string> = { PROTEUS_ALLOW_REMOTE: String(allowRemote), PROTEUS_RATE_LIMIT: String(rateLimit === "" ? 10 : rateLimit) };
      if (allowRemote && allowedHosts.trim()) {
        settings.PROTEUS_ALLOWED_HOSTS = allowedHosts.trim();
      }
      const res = await api.setup.completeSetup({
        setupToken: token.trim(),
        storage: fromEnv ? Storage.UNSPECIFIED : storage === "postgres" ? Storage.POSTGRES : Storage.MEMORY,
        database: !fromEnv && storage === "postgres" ? database : undefined,
        settings,
        adminKeyName: keyName.trim(),
      });
      setSecret(res.secret);
      setSettings({ apiKey: res.secret });
      // The server restarts its services to apply the setup; wait for it.
      void waitForServer().then(() => {
        setReady(true);
        void qc.invalidateQueries();
      });
    });

  const waitForServer = async () => {
    for (let i = 0; i < 60; i++) {
      await new Promise((r) => setTimeout(r, 1000));
      try {
        const s = await proteus(useSettings.getState().baseUrl, "").setup.getSetupStatus({});
        if (!s.needsSetup) {
          return;
        }
      } catch {
        // Restarting; try again.
      }
    }
  };

  const storageOk = fromEnv || storage === "memory" || (dbUrl.trim() !== "" && dbTest?.ok === true);

  return (
    <Box component="main" maw={680} w="100%" mx="auto" px={20} py={40} style={{ display: "flex", flexDirection: "column", gap: 24 }}>
      <Group justify="space-between">
        <Logo />
        {status.data?.version && <Badge variant="default">{status.data.version}</Badge>}
      </Group>
      <Box>
        <Title order={1} fz={24}>
          Set up Proteus
        </Title>
        <Text c="dimmed" lh={1.5}>
          A few choices and this server is ready. Everything here can be changed later in Settings.
        </Text>
      </Box>
      {status.isPending && <Loader />}
      {status.isError && (
        <Alert color="red" title="Cannot reach the server">
          {errorMessage(status.error)}
        </Alert>
      )}
      {status.data && (
        <Card withBorder radius="lg" p="xl">
          <Stepper active={secret ? 4 : step} onStepClick={(i) => !secret && i < step && setStep(i)} size="sm" allowNextStepsSelect={false}>
            <Stepper.Step label={STEPS[0]}>
              <Stack mt="lg">
                <Text fz={14} lh={1.55}>
                  To prove you run this server, enter the setup token it printed to its log when it started. Look for the line with <Code>setup_token</Code> (for Docker: <Code>docker logs</Code>), or set <Code>PROTEUS_SETUP_TOKEN</Code> yourself.
                </Text>
                <PasswordInput label="Setup token" value={token} onChange={(e) => setToken(e.currentTarget.value)} autoFocus data-autofocus onKeyDown={(e) => e.key === "Enter" && token.trim() && void checkToken()} />
                <Group justify="flex-end">
                  <Button onClick={() => void checkToken()} loading={busy} disabled={!token.trim()}>
                    Continue
                  </Button>
                </Group>
              </Stack>
            </Stepper.Step>

            <Stepper.Step label={STEPS[1]}>
              <Stack mt="lg">
                {fromEnv ? (
                  <Alert color="blue" icon={<IconDatabase size={18} />} title="PostgreSQL from the environment">
                    The database is set by PROTEUS_DATABASE_URL, so templates, jobs, settings and keys are kept there.
                  </Alert>
                ) : (
                  <Radio.Group value={storage} onChange={(v) => setStorage(v as "postgres" | "memory")} label="Where should Proteus keep templates, jobs and settings?">
                    <Stack mt="xs" gap="xs">
                      <Radio.Card value="postgres" p="md" radius="md">
                        <Group wrap="nowrap" align="flex-start">
                          <Radio.Indicator />
                          <Box>
                            <Group gap={6}>
                              <IconDatabase size={16} />
                              <Text fw={600}>PostgreSQL</Text>
                              <Badge size="xs" variant="light">recommended</Badge>
                            </Group>
                            <Text fz={13} c="dimmed">
                              Keeps everything across restarts and lets several Proteus servers share one queue.
                            </Text>
                          </Box>
                        </Group>
                      </Radio.Card>
                      <Radio.Card value="memory" p="md" radius="md">
                        <Group wrap="nowrap" align="flex-start">
                          <Radio.Indicator />
                          <Box>
                            <Group gap={6}>
                              <IconServer size={16} />
                              <Text fw={600}>This server only</Text>
                            </Group>
                            <Text fz={13} c="dimmed">
                              Templates and jobs live in memory and are lost when the server restarts; settings and keys are kept in the data folder. Good for trying Proteus out.
                            </Text>
                          </Box>
                        </Group>
                      </Radio.Card>
                    </Stack>
                  </Radio.Group>
                )}
                {!fromEnv && storage === "postgres" && (
                  <Stack gap="sm">
                    <PasswordInput
                      label="Connection URL"
                      placeholder="postgres://user:password@host:5432/proteus?sslmode=require"
                      description="Stored in the server's data folder, readable only by the server."
                      value={dbUrl}
                      onChange={(e) => {
                        setDbUrl(e.currentTarget.value);
                        setDbTest(null);
                      }}
                    />
                    <Group grow align="flex-start">
                      <TextInput label="Schema" value={dbSchema} onChange={(e) => { setDbSchema(e.currentTarget.value); setDbTest(null); }} />
                      <Checkbox mt={32} label="Create the tables for me" checked={migrate} onChange={(e) => setMigrate(e.currentTarget.checked)} />
                    </Group>
                    <Group>
                      <Button variant="default" onClick={() => void testDb()} loading={busy} disabled={!dbUrl.trim()}>
                        Test connection
                      </Button>
                      {dbTest?.ok && (
                        <Text c="green.8" fz={14}>
                          <IconCheck size={14} style={{ verticalAlign: -2 }} /> Connected to {dbTest.serverVersion}
                        </Text>
                      )}
                    </Group>
                    {dbTest && !dbTest.ok && (
                      <Alert color="red" title="Cannot connect">
                        {dbTest.error}
                      </Alert>
                    )}
                  </Stack>
                )}
                <Group justify="space-between">
                  <Button variant="default" onClick={() => setStep(0)}>
                    Back
                  </Button>
                  <Button onClick={() => setStep(2)} disabled={!storageOk}>
                    Continue
                  </Button>
                </Group>
              </Stack>
            </Stepper.Step>

            <Stepper.Step label={STEPS[2]}>
              <Stack mt="lg">
                <NumberInput label="Requests per second per key" description="0 turns rate limiting off." min={0} value={rateLimit} onChange={setRateLimit} w={260} />
                <Switch
                  label="Allow remote images, styles and fonts"
                  description="Lets documents that ask for it load files over http(s). Private and local addresses are always refused."
                  checked={allowRemote}
                  onChange={(e) => setAllowRemote(e.currentTarget.checked)}
                />
                {allowRemote && (
                  <TextInput label="Allowed hosts" description="Comma separated; empty allows any public host. A leading dot allows subdomains, e.g. .example.com." value={allowedHosts} onChange={(e) => setAllowedHosts(e.currentTarget.value)} />
                )}
                <Text fz={13} c="dimmed">
                  Limits, timeouts and the other settings keep sensible defaults; change them any time in Settings.
                </Text>
                <Group justify="space-between">
                  <Button variant="default" onClick={() => setStep(1)}>
                    Back
                  </Button>
                  <Button onClick={() => setStep(3)}>Continue</Button>
                </Group>
              </Stack>
            </Stepper.Step>

            <Stepper.Step label={STEPS[3]}>
              <Stack mt="lg">
                <Text fz={14} lh={1.55}>
                  Proteus creates an admin API key. This browser signs in with it, and you can use it to create keys for your apps in Settings.
                </Text>
                <TextInput label="Key name" value={keyName} onChange={(e) => setKeyName(e.currentTarget.value)} w={320} />
                <Group justify="space-between">
                  <Button variant="default" onClick={() => setStep(2)}>
                    Back
                  </Button>
                  <Button onClick={() => void finish()} loading={busy} disabled={!keyName.trim()}>
                    Finish setup
                  </Button>
                </Group>
              </Stack>
            </Stepper.Step>

            <Stepper.Completed>
              <Stack mt="lg">
                <Alert color="green" title="Proteus is set up">
                  Copy the admin key now and keep it somewhere safe. The server keeps only a fingerprint of it, so it cannot be shown again.
                </Alert>
                <Group wrap="nowrap" gap="xs">
                  <Code block style={{ flex: 1, wordBreak: "break-all" }} fz={13}>
                    {secret}
                  </Code>
                  <CopyButton value={secret}>
                    {({ copied, copy }) => (
                      <Button variant="default" leftSection={copied ? <IconCheck size={16} /> : <IconCopy size={16} />} onClick={copy}>
                        {copied ? "Copied" : "Copy"}
                      </Button>
                    )}
                  </CopyButton>
                </Group>
                <Checkbox label="I saved the key" checked={saved} onChange={(e) => setSaved(e.currentTarget.checked)} />
                <Group justify="space-between">
                  <Text fz={13} c="dimmed">
                    {ready ? "The server is ready." : "Applying the setup…"}
                  </Text>
                  <Button disabled={!saved || !ready} loading={!ready} onClick={() => void navigate({ to: "/" })}>
                    Open Proteus
                  </Button>
                </Group>
                <Text fz={13} c="dimmed">
                  Next: <Anchor onClick={() => saved && ready && void navigate({ to: "/settings" })}>create keys for your apps</Anchor> in Settings.
                </Text>
              </Stack>
            </Stepper.Completed>
          </Stepper>
          {error && (
            <Alert color="red" mt="md">
              {error}
            </Alert>
          )}
        </Card>
      )}
    </Box>
  );
}
