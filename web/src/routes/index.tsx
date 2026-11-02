// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// The templates list: everything the key owns, filtered by category.

import { ActionIcon, Alert, Badge, Box, Button, Card, Group, Loader, Menu, SimpleGrid, Stack, Text, TextInput, Title } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconDotsVertical, IconPlus, IconSearch, IconTrash } from "@tabler/icons-react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { useState } from "react";

import { errorMessage, isUnauthenticated } from "@/api/client";
import { useDeleteTemplate, useTemplates } from "@/api/queries";
import { Header } from "@/components/Shell";
import { CATEGORIES, categoryLabel } from "@/model/layout";

export const Route = createFileRoute("/")({
  component: TemplatesPage,
});

function TemplatesPage() {
  const [category, setCategory] = useState("all");
  const [search, setSearch] = useState("");
  const q = useTemplates(category === "all" ? "" : category);
  const del = useDeleteTemplate();
  const list = (q.data ?? []).filter((t) => !search || t.name.toLowerCase().includes(search.toLowerCase()) || t.description.toLowerCase().includes(search.toLowerCase()));
  return (
    <>
      <Header crumbs={[{ label: "Templates" }]} />
      <Box component="main" maw={1240} w="100%" mx="auto" px={20} py={28} style={{ display: "flex", flexDirection: "column", gap: 18 }}>
        <Group align="flex-end" wrap="wrap" gap={16}>
          <Box style={{ flex: "1 1 320px" }}>
            <Title order={1} fz={24}>
              Templates
            </Title>
            <Text c="dimmed" lh={1.5}>
              Invoices, quotations, letters and any other document, filled from data and converted to PDF.
            </Text>
          </Box>
          <TextInput placeholder="Search" leftSection={<IconSearch size={14} />} value={search} onChange={(e) => setSearch(e.currentTarget.value)} w={260} />
          <Button component={Link} to="/new" leftSection={<IconPlus size={16} />}>
            New template
          </Button>
        </Group>
        <Group gap={6} role="tablist" aria-label="Category">
          {[{ value: "all", label: "All" }, ...CATEGORIES.filter((c) => c.value)].map((c) => (
            <Button key={c.value} role="tab" aria-selected={category === c.value} size="xs" radius="xl" variant={category === c.value ? "light" : "default"} onClick={() => setCategory(c.value)}>
              {c.label}
            </Button>
          ))}
        </Group>
        {q.isPending && <Loader />}
        {q.error && (
          <Alert color="red" title={isUnauthenticated(q.error) ? "Set your API key" : "Cannot load templates"}>
            {isUnauthenticated(q.error) ? "Use the key button at the top right to enter the API key for this server." : errorMessage(q.error)}
          </Alert>
        )}
        {q.data && list.length === 0 && (
          <Card withBorder radius="lg" p="xl">
            <Stack align="center" gap={8}>
              <Text fw={600}>No templates yet</Text>
              <Text c="dimmed" fz={13}>
                Start from a starter such as an invoice or a letter, or from a blank page.
              </Text>
              <Button component={Link} to="/new" leftSection={<IconPlus size={16} />}>
                New template
              </Button>
            </Stack>
          </Card>
        )}
        <SimpleGrid cols={{ base: 1, sm: 2, md: 3, lg: 4 }} spacing={16}>
          {list.map((t) => (
            <Card key={t.id} withBorder radius="md" padding="md" component="article" style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <Group justify="space-between" wrap="nowrap" align="flex-start">
                <Link to="/t/$id" params={{ id: t.id }} style={{ textDecoration: "none", color: "inherit", flex: 1, minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", fontWeight: 700 }}>
                  {t.name}
                </Link>
                <Menu position="bottom-end">
                  <Menu.Target>
                    <ActionIcon variant="subtle" color="gray" aria-label="Template actions">
                      <IconDotsVertical size={16} />
                    </ActionIcon>
                  </Menu.Target>
                  <Menu.Dropdown>
                    <Menu.Item renderRoot={(props) => <Link to="/t/$id" params={{ id: t.id }} {...props} />}>Open</Menu.Item>
                    <Menu.Item
                      color="red"
                      leftSection={<IconTrash size={14} />}
                      onClick={() => {
                        if (window.confirm(`Delete "${t.name}"? Jobs made from it are kept.`)) {
                          del.mutate(t.id, { onError: (e) => notifications.show({ color: "red", message: errorMessage(e) }) });
                        }
                      }}
                    >
                      Delete
                    </Menu.Item>
                  </Menu.Dropdown>
                </Menu>
              </Group>
              <Group gap={6}>
                <Badge size="sm" variant="light" color={t.layout ? "indigo" : "gray"}>
                  {t.layout ? "visual" : "code"}
                </Badge>
                <Badge size="sm" variant="default">
                  {categoryLabel(t.category)}
                </Badge>
              </Group>
              <Text fz={12.5} c="dimmed" lh={1.45} lineClamp={2}>
                {t.description || "No description"}
              </Text>
              <Text fz={11} c="dimmed" mt="auto">
                rev {String(t.revision)}
                {t.updateTime && ` · ${new Date(Number(t.updateTime.seconds) * 1000).toLocaleString()}`}
              </Text>
            </Card>
          ))}
        </SimpleGrid>
      </Box>
    </>
  );
}
