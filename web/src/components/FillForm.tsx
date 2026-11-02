// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// "Fill in and generate": a form built from the template's fields. The
// values are sent as data to ConvertTemplate and the PDF is downloaded;
// problems the server reports are shown next to the fields.

import { ActionIcon, Alert, Button, Checkbox, Group, Modal, NumberInput, Select, Stack, Text, TextInput, Textarea, Title } from "@mantine/core";
import { IconPlus, IconTrash } from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import type { JsonObject, JsonValue } from "@bufbuild/protobuf";

import { dataProblems, download, errorMessage, useProteus } from "@/api/client";
import { OutputFormat } from "@/gen/proteus/v1/conversion_pb.js";
import { FieldType, type FieldSpec, type Template } from "@/gen/proteus/v1/template_pb.js";

export function FillForm({ template, opened, onClose }: { template: Template; opened: boolean; onClose(): void }) {
  const api = useProteus();
  const [data, setData] = useState<JsonObject>(() => structuredClone(template.sampleData ?? {}));
  const [problems, setProblems] = useState<Map<string, string[]>>(new Map());
  const generate = useMutation({
    mutationFn: async () => {
      const req = template.id ? { id: template.id } : { template };
      return api.templates.convertTemplate({ ...req, data, outputFormat: OutputFormat.PDF });
    },
    onSuccess: (res) => {
      setProblems(new Map());
      download(res.content, res.fileName || "document.pdf", res.mimeType);
      onClose();
    },
    onError: (err) => {
      const map = new Map<string, string[]>();
      for (const p of dataProblems(err)) {
        map.set(p.path, [...(map.get(p.path) ?? []), p.message]);
      }
      setProblems(map);
    },
  });
  const generalError = generate.error && problems.size === 0 ? errorMessage(generate.error) : undefined;
  return (
    <Modal opened={opened} onClose={onClose} title={`Fill in: ${template.name || "template"}`} size="lg" centered>
      <Stack gap="sm">
        {template.fields.length === 0 && (
          <Text fz={13} c="dimmed">
            This template lists no fields, so the form is the sample data as JSON.
          </Text>
        )}
        {template.fields.length > 0 ? (
          <Fields specs={template.fields} value={data} onChange={setData} problems={problems} prefix="" />
        ) : (
          <JsonEditor value={data} onChange={setData} />
        )}
        {generalError && <Alert color="red">{generalError}</Alert>}
        {problems.size > 0 && <Alert color="red">Fix the highlighted fields and try again.</Alert>}
        <Group justify="flex-end">
          <Button variant="default" onClick={() => setData(structuredClone(template.sampleData ?? {}))}>
            Reset to sample
          </Button>
          <Button loading={generate.isPending} onClick={() => generate.mutate()}>
            Generate PDF
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
}

function Fields({ specs, value, onChange, problems, prefix }: { specs: FieldSpec[]; value: JsonObject; onChange(v: JsonObject): void; problems: Map<string, string[]>; prefix: string }) {
  const set = (k: string, v: JsonValue | undefined) => {
    const next = { ...value };
    if (v === undefined || v === "") {
      delete next[k];
    } else {
      next[k] = v;
    }
    onChange(next);
  };
  return (
    <Stack gap="xs">
      {specs.map((f) => {
        const path = prefix + f.path;
        const err = problems.get(path)?.join("; ");
        const label = f.label || f.path;
        const v = value[f.path];
        const common = { label, description: f.description || undefined, required: f.required, error: err };
        switch (f.type) {
          case FieldType.NUMBER:
          case FieldType.MONEY:
            return <NumberInput key={f.path} {...common} value={typeof v === "number" ? v : ""} onChange={(n) => set(f.path, typeof n === "number" ? n : n === "" ? undefined : Number(n))} thousandSeparator decimalScale={f.type === FieldType.MONEY ? 2 : undefined} />;
          case FieldType.DATE:
            return <TextInput key={f.path} {...common} type="date" value={typeof v === "string" ? v.slice(0, 10) : ""} onChange={(e) => set(f.path, e.currentTarget.value)} />;
          case FieldType.BOOL:
            return <Checkbox key={f.path} label={label} description={f.description || undefined} error={err} checked={v === true} onChange={(e) => set(f.path, e.currentTarget.checked)} />;
          case FieldType.CHOICE:
            return <Select key={f.path} {...common} data={f.choices} value={typeof v === "string" ? v : null} onChange={(s) => set(f.path, s ?? undefined)} clearable={!f.required} />;
          case FieldType.OBJECT:
            return (
              <Stack key={f.path} gap={6} pl="sm" style={{ borderLeft: "2px solid var(--mantine-color-gray-3)" }}>
                <Title order={4} fz={13}>
                  {label}
                </Title>
                <Fields specs={f.itemFields} value={asObject(v)} onChange={(o) => set(f.path, o)} problems={problems} prefix={path + "."} />
              </Stack>
            );
          case FieldType.LIST: {
            const items = Array.isArray(v) ? v : [];
            return (
              <Stack key={f.path} gap={6} pl="sm" style={{ borderLeft: "2px solid var(--mantine-color-gray-3)" }}>
                <Group justify="space-between">
                  <Title order={4} fz={13}>
                    {label} {f.required && <span style={{ color: "var(--mantine-color-red-6)" }}>*</span>}
                  </Title>
                  <Button size="compact-xs" variant="light" leftSection={<IconPlus size={12} />} onClick={() => set(f.path, [...items, {}])}>
                    Add
                  </Button>
                </Group>
                {err && !problems.has(path) ? null : problems.get(path) && <Text fz={12} c="red">{problems.get(path)!.join("; ")}</Text>}
                {items.map((it, i) => (
                  <Group key={i} align="flex-start" wrap="nowrap" gap={6}>
                    <div style={{ flex: 1 }}>
                      {f.itemFields.length > 0 ? (
                        <Fields specs={f.itemFields} value={asObject(it)} onChange={(o) => set(f.path, items.map((x, j) => (j === i ? o : x)))} problems={problems} prefix={`${path}[${i}].`} />
                      ) : (
                        <TextInput value={typeof it === "string" ? it : JSON.stringify(it)} onChange={(e) => set(f.path, items.map((x, j) => (j === i ? e.currentTarget.value : x)))} />
                      )}
                    </div>
                    <ActionIcon variant="subtle" color="red" aria-label="Remove item" onClick={() => set(f.path, items.filter((_, j) => j !== i))}>
                      <IconTrash size={14} />
                    </ActionIcon>
                  </Group>
                ))}
              </Stack>
            );
          }
          default: {
            const long = typeof v === "string" && (v.length > 80 || v.includes("\n"));
            return long || /body|notes|terms|summary|address|description/i.test(f.path) ? (
              <Textarea key={f.path} {...common} autosize minRows={2} value={typeof v === "string" ? v : v === undefined ? "" : String(v)} onChange={(e) => set(f.path, e.currentTarget.value)} />
            ) : (
              <TextInput key={f.path} {...common} value={typeof v === "string" ? v : v === undefined ? "" : String(v)} onChange={(e) => set(f.path, e.currentTarget.value)} />
            );
          }
        }
      })}
    </Stack>
  );
}

function asObject(v: JsonValue | undefined): JsonObject {
  return v && typeof v === "object" && !Array.isArray(v) ? v : {};
}

export function JsonEditor({ value, onChange, rows = 14 }: { value: JsonObject; onChange(v: JsonObject): void; rows?: number }) {
  const initial = useMemo(() => JSON.stringify(value, null, 2), [value]);
  const [text, setText] = useState(initial);
  const [err, setErr] = useState<string>();
  return (
    <Textarea
      label="Data (JSON)"
      value={text}
      error={err}
      minRows={rows}
      autosize
      styles={{ input: { fontFamily: "var(--mantine-font-family-monospace)", fontSize: 12 } }}
      onChange={(e) => {
        const t = e.currentTarget.value;
        setText(t);
        try {
          const parsed = JSON.parse(t) as JsonValue;
          if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
            throw new Error("must be an object");
          }
          setErr(undefined);
          onChange(parsed as JsonObject);
        } catch (e) {
          setErr(String((e as Error).message));
        }
      }}
    />
  );
}
