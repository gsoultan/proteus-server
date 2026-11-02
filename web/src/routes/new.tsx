// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// New template: pick a starter (or blank), a locale and paper, and open it
// in the builder. The server copies the starter; the choices here are
// merged over it.

import { Alert, Badge, Box, Button, Group, Loader, Select, Stack, Text, TextInput, Title, UnstyledButton } from "@mantine/core";
import { IconSearch } from "@tabler/icons-react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { create } from "@bufbuild/protobuf";

import { errorMessage } from "@/api/client";
import { useCreateTemplate, useStarters } from "@/api/queries";
import { Header } from "@/components/Shell";
import { PageSize } from "@/gen/proteus/v1/conversion_pb.js";
import { TemplateSchema, type Template } from "@/gen/proteus/v1/template_pb.js";
import { ACCENTS, CATEGORY_GROUPS, LOCALES, categoryLabel, emptyLayout, fieldPaths } from "@/model/layout";

export const Route = createFileRoute("/new")({
  component: NewTemplatePage,
});

const GROUPS = ["All", "Finance", "Sales", "Operations", "Documents"];

function NewTemplatePage() {
  const starters = useStarters();
  const create_ = useCreateTemplate();
  const navigate = useNavigate();
  const [group, setGroup] = useState("All");
  const [search, setSearch] = useState("");
  const [sel, setSel] = useState<string>("invoice");
  const [locale, setLocale] = useState<string | null>(null);
  const [paper, setPaper] = useState<string | null>(null);
  const [name, setName] = useState("");

  const all = starters.data ?? [];
  const shown = all.filter((s) => (group === "All" || s.starterId === "blank" || CATEGORY_GROUPS[s.category] === group) && (!search || s.name.toLowerCase().includes(search.toLowerCase()) || s.description.toLowerCase().includes(search.toLowerCase())));
  const chosen = all.find((s) => s.starterId === sel) ?? all[0];

  const start = () => {
    if (!chosen) return;
    const [loc, cur] = (locale ?? `${chosen.layout?.theme?.locale ?? "en-US"}|${chosen.layout?.theme?.currency ?? "USD"}`).split("|");
    const t = create(TemplateSchema, {
      name: name.trim() || chosen.name,
      layout: { theme: { locale: loc, currency: cur } },
      options: paper ? { page: { size: Number(paper) as PageSize } } : undefined,
    });
    create_.mutate(
      { template: t, starterId: chosen.starterId },
      {
        onSuccess: (created) => void navigate({ to: "/t/$id", params: { id: created.id } }),
      },
    );
  };

  const startBlank = () => {
    create_.mutate(
      { template: create(TemplateSchema, { name: name.trim() || "Untitled", layout: emptyLayout(), format: 1, sampleData: {} }) },
      { onSuccess: (created) => void navigate({ to: "/t/$id", params: { id: created.id } }) },
    );
  };

  return (
    <>
      <Header crumbs={[{ label: "Templates", to: "/" }, { label: "New" }]} />
      <Box component="main" maw={1240} w="100%" mx="auto" px={20} py={28} style={{ display: "flex", flexDirection: "column", gap: 18, flex: 1 }}>
        <Group align="flex-end" wrap="wrap" gap={16}>
          <Box style={{ flex: "1 1 320px" }}>
            <Title order={1} fz={24}>
              New template
            </Title>
            <Text c="dimmed" lh={1.5}>
              Start from a starter and change anything, or start blank. Every starter is an ordinary visual layout with sample data and a field list.
            </Text>
          </Box>
          <TextInput placeholder="Search starters" leftSection={<IconSearch size={14} />} value={search} onChange={(e) => setSearch(e.currentTarget.value)} w={280} />
        </Group>
        <Group gap={6} role="tablist" aria-label="Document type">
          {GROUPS.map((g) => (
            <Button key={g} role="tab" aria-selected={group === g} size="xs" radius="xl" variant={group === g ? "light" : "default"} onClick={() => setGroup(g)}>
              {g}
            </Button>
          ))}
        </Group>
        {starters.isPending && <Loader />}
        {starters.error && <Alert color="red">{errorMessage(starters.error)}</Alert>}
        <Box style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))", gap: 16 }}>
          {shown.map((s, i) => (
            <StarterCard key={s.starterId} starter={s} color={ACCENTS[i % ACCENTS.length]!} selected={sel === s.starterId} onPick={() => setSel(s.starterId)} />
          ))}
        </Box>
        {chosen && (
          <Box component="section" aria-label="Create" pos="sticky" bottom={0} bg="white" p="14px 16px" style={{ border: "1px solid var(--mantine-color-gray-3)", borderRadius: 12, boxShadow: "0 -4px 16px rgba(0,0,0,0.06)" }}>
            <Group align="flex-end" wrap="wrap" gap={12}>
              <Stack gap={2} style={{ flex: "1 1 260px", minWidth: 0 }}>
                <Text fw={700}>Start from: {chosen.name}</Text>
                <Text fz={12.5} c="dimmed" lineClamp={1}>
                  Fields: {fieldPaths(chosen.fields).join(", ") || "none yet"}
                </Text>
              </Stack>
              <TextInput label="Name" placeholder={chosen.name} value={name} onChange={(e) => setName(e.currentTarget.value)} w={220} />
              <Select label="Locale" data={LOCALES} value={locale ?? `${chosen.layout?.theme?.locale ?? "en-US"}|${chosen.layout?.theme?.currency ?? "USD"}`} onChange={setLocale} w={300} allowDeselect={false} />
              <Select label="Paper" data={[{ value: String(PageSize.A4), label: "A4" }, { value: String(PageSize.LETTER), label: "Letter" }, { value: String(PageSize.A5), label: "A5" }]} value={paper ?? String(chosen.options?.page?.size || PageSize.A4)} onChange={setPaper} w={110} allowDeselect={false} />
              <Button size="md" loading={create_.isPending} onClick={chosen.starterId === "blank" ? startBlank : start}>
                Open in builder
              </Button>
            </Group>
            {create_.error && (
              <Text c="red" fz={13} mt={8}>
                {errorMessage(create_.error)}
              </Text>
            )}
          </Box>
        )}
      </Box>
    </>
  );
}

function StarterCard({ starter: s, color, selected, onPick }: { starter: Template; color: string; selected: boolean; onPick(): void }) {
  const wm = s.options?.page?.watermark;
  // A templated text such as {{if …}}PAID{{else if …}}DRAFT{{end}} shows its first word.
  const wmText = wm?.mark.case === "text" ? (wm.mark.value.match(/\}\}([^{]+)\{\{/)?.[1] ?? wm.mark.value).replace(/\{\{[^}]*\}\}/g, "").trim() : "";
  return (
    <UnstyledButton
      onClick={onPick}
      aria-pressed={selected}
      style={{ display: "flex", flexDirection: "column", borderRadius: 10, background: "#fff", overflow: "hidden", border: selected ? "2px solid #3b5bdb" : "1px solid var(--mantine-color-gray-3)", boxShadow: selected ? "0 0 0 3px #dbe4ff" : undefined }}
    >
      <Box h={200} bg="gray.2" style={{ display: "flex", alignItems: "center", justifyContent: "center", position: "relative", overflow: "hidden" }}>
        <Box w={s.options?.page?.landscape ? 170 : 124} h={s.options?.page?.landscape ? 120 : 174} bg="white" p={12} style={{ boxShadow: "0 2px 8px rgba(0,0,0,0.14)", display: "flex", flexDirection: "column", gap: 5, position: "relative" }}>
          <span style={{ height: 14, width: 40, borderRadius: 2, background: color }} />
          <span style={{ height: 6, width: "70%", background: "#ced4da", borderRadius: 2 }} />
          <span style={{ height: 4, width: "50%", background: "#e9ecef", borderRadius: 2 }} />
          <span style={{ height: 36, width: "100%", borderRadius: 2, margin: "4px 0", background: color + "22", borderTop: `3px solid ${color}` }} />
          <span style={{ height: 4, width: "90%", background: "#e9ecef", borderRadius: 2 }} />
          <span style={{ height: 4, width: "80%", background: "#e9ecef", borderRadius: 2 }} />
          <span style={{ height: 4, width: "60%", background: "#e9ecef", borderRadius: 2 }} />
          {wmText && (
            <span style={{ position: "absolute", inset: 0, display: "flex", alignItems: "center", justifyContent: "center", fontSize: 20, fontWeight: 800, letterSpacing: "0.06em", color: "rgba(224,49,49,0.16)", transform: "rotate(-35deg)" }}>
              {wmText}
            </span>
          )}
        </Box>
      </Box>
      <Stack gap={4} p="12px 14px" align="stretch">
        <Group gap={6} wrap="nowrap">
          <Text fw={700} style={{ flex: 1 }}>
            {s.name}
          </Text>
          <Badge size="sm" variant="default">
            {s.starterId === "blank" ? "Any" : categoryLabel(s.category)}
          </Badge>
        </Group>
        <Text fz={12.5} c="dimmed" lh={1.45} ta="left">
          {s.description}
        </Text>
      </Stack>
    </UnstyledButton>
  );
}
