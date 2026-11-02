// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// The builder: name and category, Visual/Code mode, undo/redo, save, the
// palette, the canvas, the exact PDF preview and the inspector.

import { Alert, Badge, Box, Button, Code, Group, Loader, ScrollArea, SegmentedControl, Select, Stack, Text, TextInput, Textarea } from "@mantine/core";
import { useDisclosure, useHotkeys } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconArrowBackUp, IconArrowForwardUp, IconDeviceFloppy, IconFileTypePdf } from "@tabler/icons-react";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { create } from "@bufbuild/protobuf";

import { errorMessage } from "@/api/client";
import { useConvertToCode, useTemplate, useUpdateTemplate } from "@/api/queries";
import { Canvas } from "@/components/Canvas";
import { FillForm } from "@/components/FillForm";
import { Inspector } from "@/components/Inspector";
import { Palette } from "@/components/Palette";
import { PdfPreview } from "@/components/PdfPreview";
import { Header, IconButton } from "@/components/Shell";
import { TemplateSchema } from "@/gen/proteus/v1/template_pb.js";
import { CATEGORIES, pageSummary } from "@/model/layout";
import { useBuilder } from "@/store/builder";

export const Route = createFileRoute("/t/$id")({
  component: BuilderPage,
});

function BuilderPage() {
  const { id } = Route.useParams();
  const q = useTemplate(id);
  const load = useBuilder((s) => s.load);
  const template = useBuilder((s) => s.template);
  useEffect(() => {
    if (q.data && (!template || template.id !== q.data.id)) {
      load(q.data);
    }
  }, [q.data, template, load]);
  if (q.isPending || !template || template.id !== id) {
    return (
      <>
        <Header crumbs={[{ label: "Templates", to: "/" }, { label: "Builder" }]} />
        <Box p="xl" style={{ display: "flex", justifyContent: "center" }}>
          {q.error ? <Alert color="red">{errorMessage(q.error)}</Alert> : <Loader />}
        </Box>
      </>
    );
  }
  return <Builder />;
}

function Builder() {
  const s = useBuilder();
  const template = s.template!;
  const update = useUpdateTemplate();
  const toCode = useConvertToCode();
  const [fillOpen, fill] = useDisclosure(false);
  const [preview, setPreview] = useState(true);
  const isVisual = !!template.layout;

  const save = () => {
    update.mutate(template, {
      onSuccess: (saved) => {
        s.markSaved(saved);
        notifications.show({ message: `Saved revision ${String(saved.revision)}`, color: "green", autoClose: 2000 });
      },
      onError: (e) => notifications.show({ color: "red", title: "Not saved", message: errorMessage(e), autoClose: 8000 }),
    });
  };
  useHotkeys([
    ["mod+s", (e) => { e.preventDefault(); save(); }],
    ["mod+z", (e) => { if (!(e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement)) { e.preventDefault(); s.undo(); } }],
    ["mod+shift+z", (e) => { if (!(e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement)) { e.preventDefault(); s.redo(); } }],
  ]);
  useEffect(() => {
    if (!s.dirty) return;
    const h = (e: BeforeUnloadEvent) => { e.preventDefault(); };
    window.addEventListener("beforeunload", h);
    return () => window.removeEventListener("beforeunload", h);
  }, [s.dirty]);

  const status = update.isPending ? "saving…" : s.dirty ? "unsaved changes" : "all changes saved";

  return (
    <>
      <Header crumbs={[{ label: "Templates", to: "/" }, { label: "Builder" }]} />
      <Box bg="white" px={20} py={10} style={{ borderBottom: "1px solid var(--mantine-color-gray-3)", display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
        <TextInput
          aria-label="Template name"
          variant="unstyled"
          size="lg"
          fw={700}
          style={{ flex: "1 1 220px", minWidth: 0 }}
          styles={{ input: { fontWeight: 700, fontSize: 19 } }}
          value={template.name}
          onChange={(e) => s.edit((t) => create(TemplateSchema, { ...t, name: e.currentTarget.value }), { history: false })}
        />
        <Select aria-label="Type" w={150} data={CATEGORIES} value={template.category} onChange={(v) => s.edit((t) => create(TemplateSchema, { ...t, category: v ?? "" }))} allowDeselect={false} />
        {isVisual && (
          <SegmentedControl size="xs" data={[{ value: "visual", label: "Visual" }, { value: "code", label: "Code" }]} value={s.mode} onChange={(v) => s.setMode(v as "visual" | "code")} />
        )}
        <IconButton label="Undo" onClick={s.undo} disabled={s.past.length === 0}>
          <IconArrowBackUp size={16} />
        </IconButton>
        <IconButton label="Redo" onClick={s.redo} disabled={s.future.length === 0}>
          <IconArrowForwardUp size={16} />
        </IconButton>
        <Text fz={12} c="dimmed">
          rev {String(template.revision)} · {status}
        </Text>
        <Button variant="default" leftSection={<IconFileTypePdf size={16} />} onClick={fill.open}>
          Fill in and generate
        </Button>
        <Button leftSection={<IconDeviceFloppy size={16} />} loading={update.isPending} onClick={save}>
          Save
        </Button>
      </Box>

      <Box style={{ display: "flex", flex: 1, minHeight: 0, alignItems: "stretch" }}>
        {isVisual && s.mode === "visual" && (
          <Box w={250} bg="white" style={{ borderRight: "1px solid var(--mantine-color-gray-3)", flexShrink: 0, height: "calc(100vh - 121px)" }}>
            <Palette fields={template.fields} />
          </Box>
        )}

        <Box component="main" bg="gray.2" style={{ flex: 1, minWidth: 0, height: "calc(100vh - 121px)", display: "flex", flexDirection: "column" }} px={20} py={12}>
          <Group gap={10} mb={10} wrap="wrap">
            {isVisual && s.mode === "visual" && (
              <Button size="compact-sm" variant="default" aria-pressed={s.showData} onClick={s.toggleData}>
                {s.showData ? "Showing sample data" : "Showing fields"}
              </Button>
            )}
            <Text fz={12} c="dimmed">
              {pageSummary(template.options?.page, template.layout)}
            </Text>
            <Box style={{ flex: 1 }} />
            <Button size="compact-sm" variant={preview ? "light" : "default"} onClick={() => setPreview((p) => !p)}>
              {preview ? "Hide PDF preview" : "Show PDF preview"}
            </Button>
          </Group>
          <Box style={{ display: "flex", gap: 16, flex: 1, minHeight: 0 }}>
            <ScrollArea style={{ flex: 1, minWidth: 0 }} type="auto">
              <Box style={{ display: "flex", justifyContent: "center", padding: "6px 0 20px" }}>
                {isVisual && s.mode === "visual" ? <Canvas template={template} /> : <CodeView />}
              </Box>
            </ScrollArea>
            {preview && (
              <Box w={420} style={{ flexShrink: 0, minHeight: 0, display: "flex", flexDirection: "column" }}>
                <PdfPreview template={template} enabled={preview} />
              </Box>
            )}
          </Box>
        </Box>

        {isVisual && s.mode === "visual" && (
          <Box component="aside" aria-label="Inspector" w={340} bg="white" style={{ borderLeft: "1px solid var(--mantine-color-gray-3)", flexShrink: 0, height: "calc(100vh - 121px)" }}>
            <Inspector template={template} />
          </Box>
        )}
      </Box>

      <FillForm key={fillOpen ? "open" : "closed"} template={template} opened={fillOpen} onClose={fill.close} />
    </>
  );

  function CodeView() {
    const code = new TextDecoder().decode(template.content);
    if (isVisual) {
      return (
        <Stack w="100%" maw={900} bg="white" p="md" style={{ border: "1px solid var(--mantine-color-gray-3)", borderRadius: 12 }}>
          <Alert color="yellow" fz={13}>
            Generated from the visual layout and read-only. <b>Convert to code</b> lets you edit the HTML directly, but the template then stays a code template.
          </Alert>
          <Code block fz={12} style={{ maxHeight: "60vh", overflow: "auto" }}>
            {code || "Save the template once to see the compiled HTML."}
          </Code>
          <Group>
            <Button
              variant="default"
              loading={toCode.isPending}
              disabled={s.dirty}
              title={s.dirty ? "Save first" : undefined}
              onClick={() => {
                if (!window.confirm("Convert to a code template? The visual layout is dropped; the compiled HTML stays editable.")) return;
                toCode.mutate(
                  { id: template.id, revision: template.revision },
                  {
                    onSuccess: (t) => { s.load(t); notifications.show({ message: "Now a code template", color: "green" }); },
                    onError: (e) => notifications.show({ color: "red", message: errorMessage(e) }),
                  },
                );
              }}
            >
              Convert to code
            </Button>
            {s.dirty && (
              <Text fz={12} c="dimmed">
                Save your changes first.
              </Text>
            )}
          </Group>
        </Stack>
      );
    }
    return (
      <Stack w="100%" maw={900} bg="white" p="md" style={{ border: "1px solid var(--mantine-color-gray-3)", borderRadius: 12 }}>
        <Group gap={8}>
          <Badge variant="light" color="gray">
            code template
          </Badge>
          <Text fz={12} c="dimmed">
            HTML or Markdown with Go template actions. The PDF preview on the right follows your edits.
          </Text>
        </Group>
        <Textarea
          aria-label="Template source"
          autosize
          minRows={24}
          styles={{ input: { fontFamily: "var(--mantine-font-family-monospace)", fontSize: 12, lineHeight: 1.5 } }}
          value={code}
          onChange={(e) => {
            const content = new TextEncoder().encode(e.currentTarget.value);
            s.edit((t) => create(TemplateSchema, { ...t, content }));
          }}
        />
      </Stack>
    );
  }
}
