// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// The right-hand inspector: Block (the selected block), Page (size, theme),
// Watermark and Data (fields and sample JSON).

import {
  ActionIcon,
  Badge,
  Button,
  Checkbox,
  ColorInput,
  Group,
  NumberInput,
  SegmentedControl,
  Select,
  SimpleGrid,
  Slider,
  Stack,
  Switch,
  Tabs,
  Text,
  TextInput,
  Textarea,
  Tooltip,
} from "@mantine/core";
import { IconArrowDown, IconArrowUp, IconPlus, IconTrash } from "@tabler/icons-react";
import { create, type JsonObject } from "@bufbuild/protobuf";

import { PageOptionsSchema, PageSize, WatermarkLayer, WatermarkPages, WatermarkPlacement, WatermarkSchema, type Watermark } from "@/gen/proteus/v1/conversion_pb.js";
import {
  AggregateFunc,
  Align,
  BlockSchema,
  BlockStyleSchema,
  ConditionOp,
  ConditionSchema,
  FieldFormat,
  FieldSpecSchema,
  FieldType,
  LayoutSchema,
  RegionSchema,
  Spacing,
  TemplateSchema,
  ThemeSchema,
  type Block,
  type Column,
  type FieldSpec,
  type KeyValueRow,
  type Template,
  type TotalRow,
} from "@/gen/proteus/v1/template_pb.js";
import {
  ACCENTS,
  ALIGNS,
  FONTS,
  FORMATS,
  LOCALES,
  OPS,
  PAGE_SIZES,
  SPACINGS,
  TYPE_NAMES,
  blockTitle,
  fieldPaths,
  patch,
  inferFields,
  kindName,
  listFields,
} from "@/model/layout";
import { selectedBlock, useBuilder, type Tab } from "@/store/builder";

import { JsonEditor } from "./FillForm";

export function Inspector({ template }: { template: Template }) {
  const tab = useBuilder((s) => s.tab);
  const setTab = useBuilder((s) => s.setTab);
  return (
    <Tabs value={tab} onChange={(v) => setTab((v as Tab) ?? "block")} keepMounted={false} style={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <Tabs.List px={8} pt={6}>
        <Tabs.Tab value="block">Block</Tabs.Tab>
        <Tabs.Tab value="page">Page</Tabs.Tab>
        <Tabs.Tab value="watermark">Watermark</Tabs.Tab>
        <Tabs.Tab value="data">Data</Tabs.Tab>
      </Tabs.List>
      <div style={{ flex: 1, overflowY: "auto", padding: 14 }}>
        <Tabs.Panel value="block">
          <BlockPanel template={template} />
        </Tabs.Panel>
        <Tabs.Panel value="page">
          <PagePanel template={template} />
        </Tabs.Panel>
        <Tabs.Panel value="watermark">
          <WatermarkPanel template={template} />
        </Tabs.Panel>
        <Tabs.Panel value="data">
          <DataPanel template={template} />
        </Tabs.Panel>
      </div>
    </Tabs>
  );
}

// --- Block ---

function BlockPanel({ template }: { template: Template }) {
  const block = useBuilder(selectedBlock);
  const update = useBuilder((s) => s.updateSelected);
  const remove = useBuilder((s) => s.removeSelected);
  const move = useBuilder((s) => s.moveSelected);
  const addInto = useBuilder((s) => s.addInto);
  if (!block) {
    return (
      <Text fz={13} c="dimmed">
        Select a block on the page to edit it, or add one from the palette.
      </Text>
    );
  }
  const paths = fieldPaths(template.fields);
  const lists = listFields(template.fields).map((f) => f.path);
  const set = (change: (b: Block) => Block) => update(change);
  // Kind values may be plain objects (MessageInit) or messages; create() fills them in.
  const setKind = (_case: Block["kind"]["case"], value: unknown) => set((b) => create(BlockSchema, { ...b, kind: { case: _case, value } as never }));
  const k = block.kind;

  return (
    <Stack gap={12}>
      <Group gap={8} wrap="nowrap">
        <Text fw={700} style={{ flex: 1, minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
          {blockTitle(block)}
        </Text>
        <Badge variant="light">{kindName(k.case)}</Badge>
      </Group>
      <Group gap={6}>
        <Tooltip label="Move up">
          <ActionIcon variant="default" aria-label="Move up" onClick={() => move(-1)}>
            <IconArrowUp size={14} />
          </ActionIcon>
        </Tooltip>
        <Tooltip label="Move down">
          <ActionIcon variant="default" aria-label="Move down" onClick={() => move(1)}>
            <IconArrowDown size={14} />
          </ActionIcon>
        </Tooltip>
        <Tooltip label="Remove block">
          <ActionIcon variant="default" color="red" aria-label="Remove block" onClick={remove}>
            <IconTrash size={14} />
          </ActionIcon>
        </Tooltip>
      </Group>

      {k.case === "heading" && (
        <>
          <TextInput label="Text" value={k.value.text} onChange={(e) => setKind("heading", { ...k.value, text: e.currentTarget.value })} />
          <SegmentedControl fullWidth size="xs" data={[{ value: "1", label: "Title" }, { value: "2", label: "Heading" }, { value: "3", label: "Subheading" }]} value={String(k.value.level || 1)} onChange={(v) => setKind("heading", { ...k.value, level: Number(v) })} />
        </>
      )}
      {k.case === "text" && (
        <>
          <Textarea label="Content" autosize minRows={4} styles={{ input: { fontFamily: "var(--mantine-font-family-monospace)", fontSize: 12 } }} value={k.value.text} onChange={(e) => setKind("text", { text: e.currentTarget.value })} />
          <Text fz={12} c="dimmed">
            Type <code>{"{{.path}}"}</code> to insert a value, <code>{"{{money .total}}"}</code>, <code>{"{{fdate .issued}}"}</code> or <code>{"{{percent .rate}}"}</code> to format one. Blank lines separate paragraphs.
          </Text>
        </>
      )}
      {k.case === "field" && (
        <>
          <PathSelect label="Field" value={k.value.path} paths={paths} onChange={(path) => setKind("field", { ...k.value, path })} />
          <Select label="Format" data={FORMATS} value={String(k.value.format)} onChange={(v) => setKind("field", { ...k.value, format: Number(v) as FieldFormat })} />
          <TextInput label="Label" placeholder="Shown above the value" value={k.value.label} onChange={(e) => setKind("field", { ...k.value, label: e.currentTarget.value })} />
          <SimpleGrid cols={2}>
            <TextInput label="Prefix" value={k.value.prefix} onChange={(e) => setKind("field", { ...k.value, prefix: e.currentTarget.value })} />
            <TextInput label="Suffix" value={k.value.suffix} onChange={(e) => setKind("field", { ...k.value, suffix: e.currentTarget.value })} />
          </SimpleGrid>
        </>
      )}
      {k.case === "image" && (
        <>
          <SegmentedControl fullWidth size="xs" data={[{ value: "asset", label: "Uploaded asset" }, { value: "path", label: "From a field" }]} value={k.value.source.case ?? "asset"} onChange={(v) => setKind("image", { ...k.value, source: { case: v as "asset" | "path", value: v === "asset" ? "assets/logo.png" : "" } })} />
          {k.value.source.case === "path" ? (
            <PathSelect label="Field with the image URL" value={k.value.source.value} paths={paths} onChange={(p) => setKind("image", { ...k.value, source: { case: "path", value: p } })} />
          ) : (
            <TextInput label="Asset path" description="A template asset such as assets/logo.png" value={k.value.source.value ?? ""} onChange={(e) => setKind("image", { ...k.value, source: { case: "asset", value: e.currentTarget.value } })} />
          )}
          <SimpleGrid cols={2}>
            <NumberInput label="Width (mm)" min={0} max={300} value={k.value.widthMm || ""} onChange={(v) => setKind("image", { ...k.value, widthMm: Number(v) || 0 })} />
            <NumberInput label="Height (mm)" min={0} max={300} value={k.value.heightMm || ""} onChange={(v) => setKind("image", { ...k.value, heightMm: Number(v) || 0 })} />
          </SimpleGrid>
          <TextInput label="Alt text" value={k.value.alt} onChange={(e) => setKind("image", { ...k.value, alt: e.currentTarget.value })} />
        </>
      )}
      {k.case === "table" && (
        <>
          <Select label="Repeat for each" data={lists.length ? lists : [k.value.list || "items"]} value={k.value.list} searchable onChange={(v) => setKind("table", { ...k.value, list: v ?? "" })} />
          <Text fz={12} fw={600} c="dimmed">
            Columns
          </Text>
          {k.value.columns.map((c, i) => (
            <ColumnRow
              key={i}
              column={c}
              onChange={(col) => setKind("table", { ...k.value, columns: k.value.columns.map((x, j) => (j === i ? col : x)) })}
              onRemove={() => setKind("table", { ...k.value, columns: k.value.columns.filter((_, j) => j !== i) })}
            />
          ))}
          <Button variant="default" size="xs" leftSection={<IconPlus size={12} />} onClick={() => setKind("table", { ...k.value, columns: [...k.value.columns, { header: "Column", path: "", format: FieldFormat.TEXT }] as Column[] })}>
            Add column
          </Button>
          <Checkbox label="Number the rows" checked={k.value.numbered} onChange={(e) => setKind("table", { ...k.value, numbered: e.currentTarget.checked })} />
          <Checkbox label="Striped rows" checked={k.value.striped} onChange={(e) => setKind("table", { ...k.value, striped: e.currentTarget.checked })} />
          <Checkbox label="Repeat the header row on each page" checked={k.value.repeatHeader ?? true} onChange={(e) => setKind("table", { ...k.value, repeatHeader: e.currentTarget.checked })} />
          <TextInput label="When the list is empty, show" value={k.value.emptyText} onChange={(e) => setKind("table", { ...k.value, emptyText: e.currentTarget.value })} />
        </>
      )}
      {k.case === "keyValue" && (
        <>
          {k.value.rows.map((r, i) => (
            <KVRow key={i} row={r} paths={paths} onChange={(row) => setKind("keyValue", { rows: k.value.rows.map((x, j) => (j === i ? row : x)) })} onRemove={() => setKind("keyValue", { rows: k.value.rows.filter((_, j) => j !== i) })} />
          ))}
          <Button variant="default" size="xs" leftSection={<IconPlus size={12} />} onClick={() => setKind("keyValue", { rows: [...k.value.rows, { label: "Label", value: "", format: FieldFormat.TEXT }] as KeyValueRow[] })}>
            Add row
          </Button>
        </>
      )}
      {k.case === "totals" && (
        <>
          {k.value.rows.map((r, i) => (
            <TotalRowEditor key={i} row={r} paths={paths} lists={lists} onChange={(row) => setKind("totals", { rows: k.value.rows.map((x, j) => (j === i ? row : x)) })} onRemove={() => setKind("totals", { rows: k.value.rows.filter((_, j) => j !== i) })} />
          ))}
          <Button variant="default" size="xs" leftSection={<IconPlus size={12} />} onClick={() => setKind("totals", { rows: [...k.value.rows, { label: "Row", value: { case: "path", value: "" }, format: FieldFormat.MONEY, emphasized: false }] as TotalRow[] })}>
            Add row
          </Button>
        </>
      )}
      {k.case === "columns" && (
        <>
          <NumberInput label="Columns" min={2} max={4} value={k.value.columns.length} onChange={(v) => {
            const n = Math.max(2, Math.min(4, Number(v) || 2));
            const cols = k.value.columns.slice(0, n);
            while (cols.length < n) cols.push(create(RegionSchema, {}));
            setKind("columns", { ...k.value, columns: cols, widths: k.value.widths.slice(0, n) });
          }} />
          <TextInput label="Relative widths" description="Comma separated, such as 2,1" value={k.value.widths.join(",")} onChange={(e) => setKind("columns", { ...k.value, widths: e.currentTarget.value.split(",").map((x) => Number(x.trim())).filter((x) => x > 0) })} />
          <Text fz={12} fw={600} c="dimmed">
            Add to a column
          </Text>
          <SimpleGrid cols={Math.min(4, k.value.columns.length)}>
            {k.value.columns.map((_, i) => (
              <Select key={i} placeholder={`Column ${i + 1}`} size="xs" data={[{ value: "text", label: "Text" }, { value: "field", label: "Field" }, { value: "image", label: "Image" }, { value: "keyValue", label: "Key-value" }, { value: "heading", label: "Heading" }, { value: "qr", label: "QR" }, { value: "signature", label: "Signature" }]} value={null} onChange={(v) => v && addInto(block.id, i, v as never)} />
            ))}
          </SimpleGrid>
        </>
      )}
      {k.case === "signature" && (
        <>
          {k.value.lines.map((l, i) => (
            <Group key={i} gap={6} wrap="nowrap" align="flex-end">
              <TextInput style={{ flex: 1 }} label={i === 0 ? "Labels under the lines" : undefined} value={l.label} onChange={(e) => setKind("signature", { lines: k.value.lines.map((x, j) => (j === i ? { ...x, label: e.currentTarget.value } : x)) })} />
              <ActionIcon variant="subtle" color="red" aria-label="Remove line" onClick={() => setKind("signature", { lines: k.value.lines.filter((_, j) => j !== i) })}>
                <IconTrash size={14} />
              </ActionIcon>
            </Group>
          ))}
          <Button variant="default" size="xs" leftSection={<IconPlus size={12} />} disabled={k.value.lines.length >= 4} onClick={() => setKind("signature", { lines: [...k.value.lines, { label: "" }] })}>
            Add line
          </Button>
        </>
      )}
      {k.case === "qr" && (
        <>
          <PathSelect label="Field with the value to encode" value={k.value.value} paths={paths} onChange={(p) => setKind("qr", { ...k.value, value: p })} />
          <NumberInput label="Size (mm)" min={10} max={100} value={k.value.sizeMm || 22} onChange={(v) => setKind("qr", { ...k.value, sizeMm: Number(v) || 22 })} />
        </>
      )}
      {k.case === "spacer" && <NumberInput label="Height (mm)" min={1} max={200} value={k.value.heightMm || 8} onChange={(v) => setKind("spacer", { heightMm: Number(v) || 8 })} />}
      {k.case === "section" && (
        <>
          <Select label="Repeat for each" placeholder="Not repeated" clearable data={lists} value={k.value.repeat || null} onChange={(v) => setKind("section", { ...k.value, repeat: v ?? "" })} />
          <Checkbox label="One page per item" disabled={!k.value.repeat} checked={k.value.pagePerItem} onChange={(e) => setKind("section", { ...k.value, pagePerItem: e.currentTarget.checked })} />
          <Select label="Add a block inside" placeholder="Pick a kind" data={[{ value: "heading", label: "Heading" }, { value: "text", label: "Text" }, { value: "field", label: "Field" }, { value: "keyValue", label: "Key-value" }, { value: "table", label: "Table" }, { value: "image", label: "Image" }, { value: "divider", label: "Divider" }]} value={null} onChange={(v) => v && addInto(block.id, 0, v as never)} />
          <Text fz={12} c="dimmed">
            Inside a repeated section, paths refer to the item: <code>{"{{.heading}}"}</code>.
          </Text>
        </>
      )}

      <Text fz={12} fw={600} c="dimmed" mt={4}>
        Show only when
      </Text>
      <Group gap={6} wrap="nowrap" align="flex-end">
        <Select
          style={{ flex: 1 }}
          placeholder="Always"
          clearable
          searchable
          data={paths}
          value={block.showIf?.path || null}
          onChange={(v) => set((b) => create(BlockSchema, { ...b, showIf: v ? patch(ConditionSchema, b.showIf, { path: v, op: b.showIf?.op || ConditionOp.IS_SET }) : undefined }))}
        />
        {block.showIf?.path && (
          <Select w={100} data={OPS} value={String(block.showIf.op || ConditionOp.IS_SET)} onChange={(v) => set((b) => create(BlockSchema, { ...b, showIf: create(ConditionSchema, { ...b.showIf!, op: Number(v) as ConditionOp }) }))} />
        )}
      </Group>
      {block.showIf?.path && [ConditionOp.EQUALS, ConditionOp.NOT_EQUALS, ConditionOp.GREATER, ConditionOp.LESS].includes(block.showIf.op) && (
        <TextInput placeholder="value" value={block.showIf.value} onChange={(e) => set((b) => create(BlockSchema, { ...b, showIf: create(ConditionSchema, { ...b.showIf!, value: e.currentTarget.value }) }))} />
      )}

      <SimpleGrid cols={2}>
        <Select label="Align" data={ALIGNS} value={String(block.style?.align ?? Align.UNSPECIFIED)} onChange={(v) => set((b) => create(BlockSchema, { ...b, style: patch(BlockStyleSchema, b.style, { align: Number(v) as Align }) }))} />
        <Select label="Space below" data={SPACINGS} value={String(block.style?.spaceBelow ?? Spacing.UNSPECIFIED)} onChange={(v) => set((b) => create(BlockSchema, { ...b, style: patch(BlockStyleSchema, b.style, { spaceBelow: Number(v) as Spacing }) }))} />
        <NumberInput label="Font size (pt)" placeholder="theme" min={4} max={120} value={block.style?.fontSizePt || ""} onChange={(v) => set((b) => create(BlockSchema, { ...b, style: patch(BlockStyleSchema, b.style, { fontSizePt: Number(v) || 0 }) }))} />
        <Checkbox mt={28} label="Bold" checked={block.style?.bold ?? false} onChange={(e) => set((b) => create(BlockSchema, { ...b, style: patch(BlockStyleSchema, b.style, { bold: e.currentTarget.checked }) }))} />
      </SimpleGrid>
      <ColorInput label="Text colour" placeholder="theme" value={block.style?.color ?? ""} onChange={(v) => set((b) => create(BlockSchema, { ...b, style: patch(BlockStyleSchema, b.style, { color: v }) }))} />
    </Stack>
  );
}

function PathSelect({ label, value, paths, onChange }: { label: string; value: string; paths: string[]; onChange(v: string): void }) {
  const data = paths.includes(value) || !value ? paths : [value, ...paths];
  return <Select label={label} placeholder="Pick or type a path" searchable data={data} value={value || null} onChange={(v) => onChange(v ?? "")} nothingFoundMessage="Type a path and add it in the Data tab" />;
}

function ColumnRow({ column, onChange, onRemove }: { column: Column; onChange(c: Column): void; onRemove(): void }) {
  return (
    <Group gap={6} wrap="nowrap" align="flex-end">
      <TextInput size="xs" style={{ flex: 2 }} placeholder="Header" value={column.header} onChange={(e) => onChange({ ...column, header: e.currentTarget.value })} />
      <TextInput size="xs" style={{ flex: 2 }} placeholder="item field" styles={{ input: { fontFamily: "var(--mantine-font-family-monospace)" } }} value={column.path} onChange={(e) => onChange({ ...column, path: e.currentTarget.value })} />
      <Select size="xs" style={{ flex: 2 }} data={FORMATS} value={String(column.format)} onChange={(v) => onChange({ ...column, format: Number(v) as FieldFormat })} />
      <NumberInput size="xs" w={62} placeholder="%" min={0} max={100} value={column.widthPct || ""} onChange={(v) => onChange({ ...column, widthPct: Number(v) || 0 })} />
      <ActionIcon variant="subtle" color="red" aria-label="Remove column" onClick={onRemove}>
        <IconTrash size={14} />
      </ActionIcon>
    </Group>
  );
}

function KVRow({ row, paths, onChange, onRemove }: { row: KeyValueRow; paths: string[]; onChange(r: KeyValueRow): void; onRemove(): void }) {
  return (
    <Group gap={6} wrap="nowrap" align="flex-end">
      <TextInput size="xs" style={{ flex: 2 }} placeholder="Label" value={row.label} onChange={(e) => onChange({ ...row, label: e.currentTarget.value })} />
      <Select size="xs" style={{ flex: 3 }} searchable placeholder="field" data={paths.includes(row.value) || !row.value ? paths : [row.value, ...paths]} value={row.value || null} onChange={(v) => onChange({ ...row, value: v ?? "" })} />
      <Select size="xs" style={{ flex: 2 }} data={FORMATS} value={String(row.format)} onChange={(v) => onChange({ ...row, format: Number(v) as FieldFormat })} />
      <ActionIcon variant="subtle" color="red" aria-label="Remove row" onClick={onRemove}>
        <IconTrash size={14} />
      </ActionIcon>
    </Group>
  );
}

function TotalRowEditor({ row, paths, lists, onChange, onRemove }: { row: TotalRow; paths: string[]; lists: string[]; onChange(r: TotalRow): void; onRemove(): void }) {
  const agg = row.value.case === "aggregate" ? row.value.value : undefined;
  return (
    <Stack gap={4} p={6} style={{ border: "1px solid var(--mantine-color-gray-2)", borderRadius: 6 }}>
      <Group gap={6} wrap="nowrap" align="flex-end">
        <TextInput size="xs" style={{ flex: 1 }} placeholder="Label" value={row.label} onChange={(e) => onChange({ ...row, label: e.currentTarget.value })} />
        <SegmentedControl size="xs" data={[{ value: "path", label: "Field" }, { value: "aggregate", label: "Sum" }]} value={row.value.case ?? "path"} onChange={(v) => onChange({ ...row, value: v === "aggregate" ? { case: "aggregate", value: { func: AggregateFunc.SUM, list: lists[0] ?? "items", column: "amount" } as never } : { case: "path", value: "" } })} />
        <ActionIcon variant="subtle" color="red" aria-label="Remove row" onClick={onRemove}>
          <IconTrash size={14} />
        </ActionIcon>
      </Group>
      {agg ? (
        <Group gap={6} wrap="nowrap">
          <Select size="xs" w={90} data={[{ value: String(AggregateFunc.SUM), label: "sum" }, { value: String(AggregateFunc.COUNT), label: "count" }, { value: String(AggregateFunc.AVERAGE), label: "average" }]} value={String(agg.func)} onChange={(v) => onChange({ ...row, value: { case: "aggregate", value: { ...agg, func: Number(v) as AggregateFunc } } })} />
          <Select size="xs" style={{ flex: 1 }} data={lists.length ? lists : [agg.list]} value={agg.list} onChange={(v) => onChange({ ...row, value: { case: "aggregate", value: { ...agg, list: v ?? "" } } })} />
          <TextInput size="xs" style={{ flex: 1 }} placeholder="column" value={agg.column} onChange={(e) => onChange({ ...row, value: { case: "aggregate", value: { ...agg, column: e.currentTarget.value } } })} />
        </Group>
      ) : (
        <Select size="xs" searchable placeholder="field" data={paths} value={row.value.case === "path" && row.value.value ? row.value.value : null} onChange={(v) => onChange({ ...row, value: { case: "path", value: v ?? "" } })} />
      )}
      <Group gap={6}>
        <Select size="xs" style={{ flex: 1 }} data={FORMATS} value={String(row.format || FieldFormat.MONEY)} onChange={(v) => onChange({ ...row, format: Number(v) as FieldFormat })} />
        <Checkbox size="xs" label="Emphasized" checked={row.emphasized} onChange={(e) => onChange({ ...row, emphasized: e.currentTarget.checked })} />
      </Group>
    </Stack>
  );
}

// --- Page ---

function PagePanel({ template }: { template: Template }) {
  const editOptions = useBuilder((s) => s.editOptions);
  const editLayout = useBuilder((s) => s.editLayout);
  const page = template.options?.page;
  const theme = template.layout?.theme;
  const setPage = (change: Partial<NonNullable<typeof page>>) => editOptions((o) => ({ ...o, page: patch(PageOptionsSchema, o.page, { ...change }) }));
  const setTheme = (change: Partial<NonNullable<typeof theme>>) => editLayout((l) => create(LayoutSchema, { ...l, theme: patch(ThemeSchema, l.theme, { ...change }) }));
  const localeValue = `${theme?.locale || "en-US"}|${theme?.currency || "USD"}`;
  return (
    <Stack gap={12}>
      <SimpleGrid cols={2}>
        <Select label="Page size" data={PAGE_SIZES} value={String(page?.size || PageSize.A4)} onChange={(v) => setPage({ size: Number(v) as PageSize })} />
        <Select label="Orientation" data={[{ value: "p", label: "Portrait" }, { value: "l", label: "Landscape" }]} value={page?.landscape ? "l" : "p"} onChange={(v) => setPage({ landscape: v === "l" })} />
        <NumberInput label="Margins (mm)" min={0} max={80} value={page?.marginMm ?? 15} onChange={(v) => setPage({ marginMm: Number(v) || 0 })} />
        <Select label="Font" data={FONTS.map((f) => ({ value: f, label: f.split(",")[0]!.replace(/'/g, "") }))} value={FONTS.includes(theme?.fontFamily ?? "") ? theme?.fontFamily : FONTS[0]} onChange={(v) => setTheme({ fontFamily: v ?? "" })} />
        <NumberInput label="Base size (pt)" min={6} max={18} step={0.5} value={theme?.fontSizePt || 10} onChange={(v) => setTheme({ fontSizePt: Number(v) || 10 })} />
        <NumberInput label="Header height (mm)" min={5} max={100} value={template.layout?.header?.heightMm || 25} onChange={(v) => editLayout((l) => create(LayoutSchema, { ...l, header: patch(RegionSchema, l.header, { heightMm: Number(v) || 25 }) }))} />
        <NumberInput label="Footer height (mm)" min={5} max={100} value={template.layout?.footer?.heightMm || 15} onChange={(v) => editLayout((l) => create(LayoutSchema, { ...l, footer: patch(RegionSchema, l.footer, { heightMm: Number(v) || 15 }) }))} />
      </SimpleGrid>
      <Text fz={12} fw={600} c="dimmed">
        Accent colour
      </Text>
      <Group gap={8}>
        {ACCENTS.map((c) => (
          <ActionIcon key={c} radius="xl" size={36} aria-label={`Accent ${c}`} aria-pressed={(theme?.accentColor || "#3b5bdb") === c} style={{ background: c, border: (theme?.accentColor || "#3b5bdb") === c ? "3px solid #212529" : 0 }} onClick={() => setTheme({ accentColor: c })} />
        ))}
      </Group>
      <ColorInput label="Custom accent" value={theme?.accentColor ?? ""} onChange={(v) => setTheme({ accentColor: v })} />
      <Select label="Number and date format" data={LOCALES} value={LOCALES.some((l) => l.value === localeValue) ? localeValue : null} placeholder={localeValue} onChange={(v) => {
        if (!v) return;
        const [locale, currency] = v.split("|");
        setTheme({ locale, currency });
      }} />
    </Stack>
  );
}

// --- Watermark ---

function WatermarkPanel({ template }: { template: Template }) {
  const editOptions = useBuilder((s) => s.editOptions);
  const wm = template.options?.page?.watermark;
  const setWm = (change: Partial<Watermark> | undefined) =>
    editOptions((o) => ({
      ...o,
      page: patch(PageOptionsSchema, o.page, {
        watermark: change ? patch(WatermarkSchema, o.page?.watermark ?? create(WatermarkSchema, { mark: { case: "text", value: "DRAFT" } }), change) : undefined,
      }),
    }));
  const text = wm?.mark.case === "text" ? wm.mark.value : "";
  const cond = text.match(/^\{\{if eq \.(\w+) "([^"]*)"\}\}([^{]*)\{\{end\}\}$/);
  return (
    <Stack gap={12}>
      <Switch label="Watermark" checked={!!wm} onChange={(e) => setWm(e.currentTarget.checked ? { opacity: 0.15 } : undefined)} />
      {wm && (
        <>
          <SegmentedControl fullWidth size="xs" data={[{ value: "text", label: "Text" }, { value: "imageAsset", label: "Image" }]} value={wm.mark.case ?? "text"} onChange={(v) => setWm({ mark: v === "text" ? { case: "text", value: "DRAFT" } : { case: "imageAsset", value: "assets/logo.png" } })} />
          {wm.mark.case === "imageAsset" ? (
            <>
              <TextInput label="Image asset" description="PNG or JPEG uploaded with the template" value={wm.mark.value} onChange={(e) => setWm({ mark: { case: "imageAsset", value: e.currentTarget.value } })} />
              <NumberInput label="Width (mm)" min={10} max={300} value={wm.imageWidthMm || ""} placeholder="auto" onChange={(v) => setWm({ imageWidthMm: Number(v) || 0 })} />
            </>
          ) : (
            <>
              <TextInput label="Text" value={cond ? cond[3]! : text} onChange={(e) => setWm({ mark: { case: "text", value: cond ? `{{if eq .${cond[1]} "${cond[2]}"}}${e.currentTarget.value}{{end}}` : e.currentTarget.value } })} />
              <Text fz={12} fw={600} c="dimmed">
                Show only when
              </Text>
              <Group gap={6} wrap="nowrap">
                <Select style={{ flex: 1 }} placeholder="Always" clearable searchable data={fieldPaths(template.fields)} value={cond ? cond[1]! : null} onChange={(v) => setWm({ mark: { case: "text", value: v ? `{{if eq .${v} "${cond?.[2] ?? "draft"}"}}${cond ? cond[3]! : text || "DRAFT"}{{end}}` : cond ? cond[3]! : text } })} />
                <Text fz={12}>equals</Text>
                <TextInput w={100} disabled={!cond} value={cond?.[2] ?? ""} onChange={(e) => cond && setWm({ mark: { case: "text", value: `{{if eq .${cond[1]} "${e.currentTarget.value}"}}${cond[3]}{{end}}` } })} />
              </Group>
              {!cond && text.includes("{{") && (
                <Text fz={11} c="dimmed">
                  The text is a template; the condition editor shows simple cases only.
                </Text>
              )}
              <SimpleGrid cols={2}>
                <NumberInput label="Size (pt)" min={8} max={400} value={wm.fontSizePt || 72} onChange={(v) => setWm({ fontSizePt: Number(v) || 72 })} />
                <ColorInput label="Colour" value={wm.color || "#444444"} onChange={(v) => setWm({ color: v })} />
              </SimpleGrid>
            </>
          )}
          <Text fz={12} fw={600} c="dimmed">
            Placement
          </Text>
          <SegmentedControl fullWidth size="xs" data={[{ value: String(WatermarkPlacement.CENTER), label: "Centered" }, { value: String(WatermarkPlacement.TILED), label: "Tiled" }]} value={String(wm.placement || WatermarkPlacement.CENTER)} onChange={(v) => setWm({ placement: Number(v) as WatermarkPlacement })} />
          <Text fz={12} fw={600} c="dimmed">
            Opacity {Math.round((wm.opacity || 0.15) * 100)}%
          </Text>
          <Slider min={2} max={80} value={Math.round((wm.opacity || 0.15) * 100)} onChange={(v) => setWm({ opacity: v / 100 })} label={(v) => `${v}%`} />
          <NumberInput label="Angle (degrees, counter-clockwise)" min={-90} max={90} value={wm.angleDeg ?? 30} onChange={(v) => setWm({ angleDeg: Number(v) })} />
          <SimpleGrid cols={2}>
            <Select label="Layer" data={[{ value: String(WatermarkLayer.BEHIND), label: "Behind content" }, { value: String(WatermarkLayer.OVER), label: "Over content" }]} value={String(wm.layer || WatermarkLayer.BEHIND)} onChange={(v) => setWm({ layer: Number(v) as WatermarkLayer })} />
            <Select label="Pages" data={[{ value: String(WatermarkPages.ALL), label: "All pages" }, { value: String(WatermarkPages.FIRST), label: "First page" }, { value: String(WatermarkPages.ALL_BUT_FIRST), label: "All but first" }]} value={String(wm.pages || WatermarkPages.ALL)} onChange={(v) => setWm({ pages: Number(v) as WatermarkPages })} />
          </SimpleGrid>
          <Text fz={12} c="dimmed" lh={1.5}>
            Stamped on each PDF page by the converter. Use it for DRAFT, COPY, PAID, CONFIDENTIAL or a faint logo.
          </Text>
        </>
      )}
    </Stack>
  );
}

// --- Data ---

function DataPanel({ template }: { template: Template }) {
  const edit = useBuilder((s) => s.edit);
  const setFields = (fields: FieldSpec[]) => edit((t) => create(TemplateSchema, { ...t, fields }));
  const setSample = (sampleData: JsonObject) => edit((t) => create(TemplateSchema, { ...t, sampleData }), { history: false });
  return (
    <Stack gap={10}>
      <Text fz={13} c="dimmed" lh={1.5}>
        The data this template expects. It builds the fill-in form and checks API requests: missing or wrong fields are rejected with a list of problems.
      </Text>
      <FieldList fields={template.fields} onChange={setFields} depth={0} />
      <Group gap={8}>
        <Button size="xs" variant="default" leftSection={<IconPlus size={12} />} onClick={() => setFields([...template.fields, create(FieldSpecSchema, { path: `field_${template.fields.length + 1}`, type: FieldType.TEXT })])}>
          Add field
        </Button>
        <Button size="xs" variant="default" onClick={() => setFields(inferFields(template.sampleData))} disabled={!template.sampleData}>
          Infer from sample data
        </Button>
      </Group>
      <JsonEditor key={template.id + String(template.revision)} value={template.sampleData ?? {}} onChange={setSample} rows={10} />
    </Stack>
  );
}

const FIELD_TYPES = Object.entries(TYPE_NAMES)
  .filter(([k]) => Number(k) !== FieldType.UNSPECIFIED)
  .map(([k, v]) => ({ value: k, label: v }));

function FieldList({ fields, onChange, depth }: { fields: FieldSpec[]; onChange(f: FieldSpec[]): void; depth: number }) {
  const set = (i: number, change: Partial<FieldSpec>) => onChange(fields.map((f, j) => (j === i ? create(FieldSpecSchema, { ...f, ...change }) : f)));
  return (
    <Stack gap={6}>
      {fields.map((f, i) => (
        <Stack key={i} gap={4} p={6} style={{ border: "1px solid var(--mantine-color-gray-2)", borderRadius: 6, marginLeft: depth * 10 }}>
          <Group gap={6} wrap="nowrap" align="flex-end">
            <TextInput size="xs" style={{ flex: 2 }} placeholder="path" styles={{ input: { fontFamily: "var(--mantine-font-family-monospace)" } }} value={f.path} onChange={(e) => set(i, { path: e.currentTarget.value })} />
            <Select size="xs" style={{ flex: 2 }} data={FIELD_TYPES} value={String(f.type || FieldType.TEXT)} onChange={(v) => set(i, { type: Number(v) as FieldType })} />
            <Checkbox size="xs" label="req." checked={f.required} onChange={(e) => set(i, { required: e.currentTarget.checked })} />
            <ActionIcon variant="subtle" color="red" aria-label="Remove field" onClick={() => onChange(fields.filter((_, j) => j !== i))}>
              <IconTrash size={14} />
            </ActionIcon>
          </Group>
          <Group gap={6} wrap="nowrap">
            <TextInput size="xs" style={{ flex: 1 }} placeholder="Label" value={f.label} onChange={(e) => set(i, { label: e.currentTarget.value })} />
            {f.type === FieldType.CHOICE && <TextInput size="xs" style={{ flex: 1 }} placeholder="choices, comma separated" value={f.choices.join(", ")} onChange={(e) => set(i, { choices: e.currentTarget.value.split(",").map((s) => s.trim()).filter(Boolean) })} />}
          </Group>
          {(f.type === FieldType.LIST || f.type === FieldType.OBJECT) && depth < 2 && (
            <>
              <FieldList fields={f.itemFields} onChange={(itemFields) => set(i, { itemFields })} depth={depth + 1} />
              <Button size="compact-xs" variant="subtle" leftSection={<IconPlus size={10} />} onClick={() => set(i, { itemFields: [...f.itemFields, create(FieldSpecSchema, { path: `item_${f.itemFields.length + 1}`, type: FieldType.TEXT })] })}>
                Add item field
              </Button>
            </>
          )}
        </Stack>
      ))}
    </Stack>
  );
}
