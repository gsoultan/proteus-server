// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// Pure helpers over the Layout model: block kinds, tree edits, field paths,
// and the light formatting the canvas uses for sample data. The exact
// result always comes from the server (ConvertTemplate); nothing here is
// used to produce a document.

import { clone, create, type DescMessage, type JsonObject, type JsonValue, type MessageShape } from "@bufbuild/protobuf";

import { PageSize, type PageOptions } from "@/gen/proteus/v1/conversion_pb.js";
import {
  AggregateFunc,
  Align,
  BlockSchema,
  ConditionOp,
  FieldFormat,
  FieldSpecSchema,
  FieldType,
  LayoutSchema,
  RegionSchema,
  Spacing,
  TemplateSchema,
  type Block,
  type Condition,
  type FieldSpec,
  type Layout,
  type Region,
  type Template,
} from "@/gen/proteus/v1/template_pb.js";

export type BlockKind = NonNullable<Block["kind"]["case"]>;

export interface KindInfo {
  kind: BlockKind;
  name: string;
  hint: string;
}

/** The palette, in display order. */
export const KINDS: KindInfo[] = [
  { kind: "heading", name: "Heading", hint: "Title text" },
  { kind: "text", name: "Text", hint: "With fields" },
  { kind: "field", name: "Field", hint: "One value" },
  { kind: "image", name: "Image", hint: "Logo, photo" },
  { kind: "table", name: "Table", hint: "Repeats rows" },
  { kind: "keyValue", name: "Key-value", hint: "Label: value" },
  { kind: "totals", name: "Totals", hint: "Sums, tax" },
  { kind: "columns", name: "Columns", hint: "2 to 4" },
  { kind: "signature", name: "Signature", hint: "Sign lines" },
  { kind: "qr", name: "QR code", hint: "From a field" },
  { kind: "divider", name: "Divider", hint: "Line" },
  { kind: "spacer", name: "Spacer", hint: "Gap" },
  { kind: "section", name: "Section", hint: "If or repeat" },
  { kind: "pageBreak", name: "Page break", hint: "New page" },
];

export const CATEGORIES = [
  { value: "invoice", label: "Invoice" },
  { value: "quotation", label: "Quotation" },
  { value: "receipt", label: "Receipt" },
  { value: "purchase_order", label: "Purchase order" },
  { value: "delivery_note", label: "Delivery note" },
  { value: "letter", label: "Letter" },
  { value: "certificate", label: "Certificate" },
  { value: "report", label: "Report" },
  { value: "", label: "Other" },
];

export const CATEGORY_GROUPS: Record<string, string> = {
  invoice: "Finance",
  receipt: "Finance",
  purchase_order: "Finance",
  quotation: "Sales",
  delivery_note: "Operations",
  letter: "Documents",
  certificate: "Documents",
  report: "Documents",
};

export function categoryLabel(c: string): string {
  return CATEGORIES.find((x) => x.value === c)?.label ?? (c || "Other");
}

export const LOCALES = [
  { value: "id-ID|IDR", label: "Indonesia (Rp 1.250.000, 2 Nov 2026)" },
  { value: "en-US|USD", label: "United States ($1,250.00, Nov 2, 2026)" },
  { value: "en-GB|GBP", label: "United Kingdom (£1,250.00, 2 Nov 2026)" },
  { value: "de-DE|EUR", label: "Germany (1.250,00 €, 2.11.2026)" },
  { value: "fr-FR|EUR", label: "France (1 250,00 €, 2 nov. 2026)" },
  { value: "ja-JP|JPY", label: "Japan (￥1,250, 2026/11/2)" },
  { value: "en-SG|SGD", label: "Singapore (S$1,250.00, 2 Nov 2026)" },
  { value: "en-AU|AUD", label: "Australia (A$1,250.00, 2 Nov 2026)" },
  { value: "ms-MY|MYR", label: "Malaysia (RM 1,250.00, 2 Nov 2026)" },
];

export const FONTS = [
  "Inter, Helvetica, Arial, sans-serif",
  "Helvetica, Arial, sans-serif",
  "Georgia, 'Times New Roman', serif",
  "'Liberation Serif', 'Times New Roman', serif",
  "'DejaVu Sans', Verdana, sans-serif",
  "'Courier New', monospace",
];

export const ACCENTS = ["#3b5bdb", "#0c8599", "#2f9e44", "#d9480f", "#c92a2a", "#5f3dc4", "#343a40", "#b08968"];

/**
 * A copy of a message (or a fresh one) with some fields changed. Spreading
 * a message into create() does not type-check because $typeName becomes
 * optional, so every "edit a few fields" goes through here.
 */
export function patch<Desc extends DescMessage>(schema: Desc, base: MessageShape<Desc> | undefined, change: Partial<MessageShape<Desc>>): MessageShape<Desc> {
  const out = base ? clone(schema, base) : create(schema);
  Object.assign(out, change);
  return out;
}

let counter = 0;

/** A short id unique within a layout. */
export function newId(kind: string, layout?: Layout): string {
  const used = new Set(layout ? allBlocks(layout).map((b) => b.id) : []);
  for (;;) {
    counter += 1;
    const id = `${kind}_${counter.toString(36)}`;
    if (!used.has(id)) {
      return id;
    }
  }
}

/** A new block of a kind with sensible content. */
export function newBlock(kind: BlockKind, layout?: Layout): Block {
  const id = newId(kind, layout);
  switch (kind) {
    case "heading":
      return create(BlockSchema, { id, kind: { case: "heading", value: { text: "Heading", level: 2 } } });
    case "text":
      return create(BlockSchema, { id, kind: { case: "text", value: { text: "Text. Type {{.field}} to insert a value." } } });
    case "field":
      return create(BlockSchema, { id, kind: { case: "field", value: { path: "", label: "" } } });
    case "image":
      return create(BlockSchema, {
        id,
        kind: { case: "image", value: { source: { case: "asset", value: "assets/logo.png" }, widthMm: 30, alt: "" } },
      });
    case "table":
      return create(BlockSchema, {
        id,
        kind: {
          case: "table",
          value: {
            list: "items",
            numbered: false,
            striped: true,
            emptyText: "No items",
            columns: [
              { header: "Description", path: "description", format: FieldFormat.TEXT, widthPct: 60 },
              { header: "Qty", path: "qty", format: FieldFormat.INTEGER, widthPct: 15 },
              { header: "Amount", path: "amount", format: FieldFormat.MONEY, widthPct: 25 },
            ],
          },
        },
      });
    case "keyValue":
      return create(BlockSchema, {
        id,
        kind: { case: "keyValue", value: { rows: [{ label: "Number", value: "number" }, { label: "Date", value: "issued", format: FieldFormat.DATE }] } },
      });
    case "totals":
      return create(BlockSchema, {
        id,
        kind: {
          case: "totals",
          value: {
            rows: [
              { label: "Subtotal", value: { case: "aggregate", value: { func: AggregateFunc.SUM, list: "items", column: "amount" } } },
              { label: "Total", value: { case: "path", value: "total" }, emphasized: true },
            ],
          },
        },
      });
    case "columns":
      return create(BlockSchema, { id, kind: { case: "columns", value: { columns: [{ blocks: [] }, { blocks: [] }] } } });
    case "signature":
      return create(BlockSchema, { id, kind: { case: "signature", value: { lines: [{ label: "Signed" }, { label: "Received by" }] } } });
    case "qr":
      return create(BlockSchema, { id, kind: { case: "qr", value: { value: "", sizeMm: 22 } } });
    case "divider":
      return create(BlockSchema, { id, kind: { case: "divider", value: {} } });
    case "spacer":
      return create(BlockSchema, { id, kind: { case: "spacer", value: { heightMm: 8 } } });
    case "section":
      return create(BlockSchema, { id, kind: { case: "section", value: { repeat: "", content: { blocks: [] } } } });
    case "pageBreak":
      return create(BlockSchema, { id, kind: { case: "pageBreak", value: {} } });
  }
}

export function kindName(kind: BlockKind | undefined): string {
  return KINDS.find((k) => k.kind === kind)?.name ?? "Block";
}

/** Human title of a block for the inspector. */
export function blockTitle(b: Block): string {
  const k = b.kind;
  switch (k.case) {
    case "heading":
      return k.value.text || "Heading";
    case "text":
      return firstLine(k.value.text) || "Text";
    case "field":
      return k.value.label || k.value.path || "Field";
    case "table":
      return `Table of ${k.value.list || "…"}`;
    case "section":
      return k.value.repeat ? `Repeat for ${k.value.repeat}` : "Section";
    case "image":
      return k.value.source.case === "asset" ? k.value.source.value : `Image from ${k.value.source.value ?? "…"}`;
    default:
      return kindName(k.case);
  }
}

function firstLine(s: string): string {
  const line = s.split("\n")[0] ?? "";
  return line.length > 40 ? line.slice(0, 38) + "…" : line;
}

// --- tree edits (all return new objects; the inputs are not changed) ---

export type RegionName = "header" | "body" | "footer";

/** The nested regions a block owns (columns, section content). */
export function childRegions(b: Block): Region[] {
  if (b.kind.case === "columns") {
    return b.kind.value.columns;
  }
  if (b.kind.case === "section" && b.kind.value.content) {
    return [b.kind.value.content];
  }
  return [];
}

export function allBlocks(layout: Layout): Block[] {
  const out: Block[] = [];
  const walk = (r: Region | undefined) => {
    for (const b of r?.blocks ?? []) {
      out.push(b);
      for (const c of childRegions(b)) {
        walk(c);
      }
    }
  };
  walk(layout.header);
  walk(layout.body);
  walk(layout.footer);
  return out;
}

export function findBlock(layout: Layout, id: string): Block | undefined {
  return allBlocks(layout).find((b) => b.id === id);
}

/** Which top region holds the block. */
export function regionOf(layout: Layout, id: string): RegionName | undefined {
  for (const name of ["header", "body", "footer"] as const) {
    const r = layout[name];
    if (r && regionHas(r, id)) {
      return name;
    }
  }
  return undefined;
}

function regionHas(r: Region, id: string): boolean {
  return r.blocks.some((b) => b.id === id || childRegions(b).some((c) => regionHas(c, id)));
}

function mapRegion(r: Region | undefined, f: (blocks: Block[]) => Block[]): Region | undefined {
  if (!r) {
    return r;
  }
  const blocks = f(r.blocks).map((b) => {
    if (b.kind.case === "columns") {
      return create(BlockSchema, { ...b, kind: { case: "columns", value: { ...b.kind.value, columns: b.kind.value.columns.map((c) => mapRegion(c, f)!) } } });
    }
    if (b.kind.case === "section" && b.kind.value.content) {
      return create(BlockSchema, { ...b, kind: { case: "section", value: { ...b.kind.value, content: mapRegion(b.kind.value.content, f) } } });
    }
    return b;
  });
  return create(RegionSchema, { ...r, blocks });
}

function mapLayout(layout: Layout, f: (blocks: Block[]) => Block[]): Layout {
  return create(LayoutSchema, {
    ...layout,
    header: mapRegion(layout.header, f),
    body: mapRegion(layout.body, f),
    footer: mapRegion(layout.footer, f),
  });
}

/** Replaces the block with the given id everywhere it appears. */
export function updateBlock(layout: Layout, id: string, change: (b: Block) => Block): Layout {
  return mapLayout(layout, (blocks) => blocks.map((b) => (b.id === id ? change(b) : b)));
}

export function removeBlock(layout: Layout, id: string): Layout {
  return mapLayout(layout, (blocks) => blocks.filter((b) => b.id !== id));
}

/**
 * Inserts a block after the block `afterId`, or at the end of `region` when
 * afterId is empty or not found.
 */
export function insertBlock(layout: Layout, block: Block, afterId: string, region: RegionName = "body"): Layout {
  let placed = false;
  let out = afterId
    ? mapLayout(layout, (blocks) => {
        const i = blocks.findIndex((b) => b.id === afterId);
        if (i < 0 || placed) {
          return blocks;
        }
        placed = true;
        return [...blocks.slice(0, i + 1), block, ...blocks.slice(i + 1)];
      })
    : layout;
  if (!placed) {
    const r = out[region] ?? create(RegionSchema, {});
    out = create(LayoutSchema, { ...out, [region]: create(RegionSchema, { ...r, blocks: [...r.blocks, block] }) });
  }
  return out;
}

/** Moves a block one step up or down among its siblings. */
export function moveBlock(layout: Layout, id: string, dir: -1 | 1): Layout {
  return mapLayout(layout, (blocks) => {
    const i = blocks.findIndex((b) => b.id === id);
    const j = i + dir;
    if (i < 0 || j < 0 || j >= blocks.length) {
      return blocks;
    }
    const out = blocks.slice();
    [out[i], out[j]] = [out[j]!, out[i]!];
    return out;
  });
}

/** Adds a block to one of a container block's regions. */
export function insertInto(layout: Layout, containerId: string, regionIndex: number, block: Block): Layout {
  return updateBlock(layout, containerId, (b) => {
    if (b.kind.case === "columns") {
      const columns = b.kind.value.columns.map((c, i) => (i === regionIndex ? create(RegionSchema, { ...c, blocks: [...c.blocks, block] }) : c));
      return create(BlockSchema, { ...b, kind: { case: "columns", value: { ...b.kind.value, columns } } });
    }
    if (b.kind.case === "section") {
      const content = b.kind.value.content ?? create(RegionSchema, {});
      return create(BlockSchema, {
        ...b,
        kind: { case: "section", value: { ...b.kind.value, content: create(RegionSchema, { ...content, blocks: [...content.blocks, block] }) } },
      });
    }
    return b;
  });
}

/** An empty layout with the default theme. */
export function emptyLayout(locale = "en-US", currency = "USD"): Layout {
  return create(LayoutSchema, {
    theme: { locale, currency, accentColor: "#3b5bdb" },
    header: { blocks: [], heightMm: 25 },
    body: { blocks: [] },
    footer: { blocks: [], heightMm: 15 },
  });
}

// --- fields ---

export interface FlatField {
  path: string;
  type: FieldType;
  required: boolean;
  depth: number;
  spec: FieldSpec;
}

/** Fields flattened with dotted paths; list item fields are indented. */
export function flattenFields(fields: FieldSpec[], prefix = "", depth = 0): FlatField[] {
  const out: FlatField[] = [];
  for (const f of fields) {
    const path = prefix + f.path;
    out.push({ path: f.type === FieldType.LIST ? path + "[]" : path, type: f.type, required: f.required, depth, spec: f });
    if (f.type === FieldType.OBJECT) {
      out.push(...flattenFields(f.itemFields, path + ".", depth + 1));
    } else if (f.type === FieldType.LIST) {
      out.push(...flattenFields(f.itemFields, "  .", depth + 1).map((x) => ({ ...x, path: x.path })));
    }
  }
  return out;
}

/** Top-level paths usable in blocks (objects expand to dotted paths). */
export function fieldPaths(fields: FieldSpec[], prefix = ""): string[] {
  const out: string[] = [];
  for (const f of fields) {
    const p = prefix + f.path;
    if (f.type === FieldType.OBJECT) {
      out.push(...fieldPaths(f.itemFields, p + "."));
    } else {
      out.push(p);
    }
  }
  return out;
}

export function listFields(fields: FieldSpec[]): FieldSpec[] {
  return fields.filter((f) => f.type === FieldType.LIST);
}

export const TYPE_NAMES: Record<FieldType, string> = {
  [FieldType.UNSPECIFIED]: "text",
  [FieldType.TEXT]: "text",
  [FieldType.NUMBER]: "number",
  [FieldType.MONEY]: "money",
  [FieldType.DATE]: "date",
  [FieldType.BOOL]: "bool",
  [FieldType.CHOICE]: "choice",
  [FieldType.IMAGE]: "image",
  [FieldType.LIST]: "list",
  [FieldType.OBJECT]: "object",
};

const ISO_DATE = /^\d{4}-\d{2}-\d{2}(T.*)?$/;

/** Guesses a field list from sample data. */
export function inferFields(data: JsonObject | undefined, depth = 0): FieldSpec[] {
  const out: FieldSpec[] = [];
  for (const [path, v] of Object.entries(data ?? {})) {
    if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(path)) {
      continue;
    }
    out.push(inferField(path, v, depth));
  }
  return out;
}

function inferField(path: string, v: JsonValue, depth: number): FieldSpec {
  const spec = create(FieldSpecSchema, { path, type: FieldType.TEXT });
  if (typeof v === "number") {
    spec.type = /price|amount|total|tax|discount|subtotal|fee|cost/i.test(path) ? FieldType.MONEY : FieldType.NUMBER;
  } else if (typeof v === "boolean") {
    spec.type = FieldType.BOOL;
  } else if (typeof v === "string" && ISO_DATE.test(v)) {
    spec.type = FieldType.DATE;
  } else if (Array.isArray(v)) {
    spec.type = FieldType.LIST;
    const first = v.find((x) => x && typeof x === "object" && !Array.isArray(x)) as JsonObject | undefined;
    if (first && depth < 2) {
      spec.itemFields = inferFields(first, depth + 1);
    }
  } else if (v && typeof v === "object") {
    spec.type = FieldType.OBJECT;
    if (depth < 2) {
      spec.itemFields = inferFields(v as JsonObject, depth + 1);
    }
  }
  return spec;
}

// --- formatting for the canvas ---

export interface Fmt {
  locale: string;
  currency: string;
}

export function fmtOf(t: Template | undefined): Fmt {
  return { locale: t?.layout?.theme?.locale || "en-US", currency: t?.layout?.theme?.currency || "USD" };
}

/** Currencies the server prints without decimals. */
const ZERO_DECIMAL = new Set(["IDR", "JPY", "KRW", "VND", "CLP", "HUF"]);

export function formatMoney(v: unknown, f: Fmt): string {
  const n = typeof v === "number" ? v : Number(v);
  if (!Number.isFinite(n)) {
    return String(v ?? "");
  }
  try {
    const whole = ZERO_DECIMAL.has(f.currency);
    return new Intl.NumberFormat(f.locale, {
      style: "currency",
      currency: f.currency,
      currencyDisplay: "narrowSymbol",
      minimumFractionDigits: whole ? 0 : 2,
      maximumFractionDigits: whole ? 0 : 2,
    }).format(n);
  } catch {
    return n.toFixed(2);
  }
}

export function formatNumber(v: unknown, f: Fmt, decimals?: number): string {
  const n = typeof v === "number" ? v : Number(v);
  if (!Number.isFinite(n)) {
    return String(v ?? "");
  }
  const opts: Intl.NumberFormatOptions = decimals === undefined ? { maximumFractionDigits: 2 } : { minimumFractionDigits: decimals, maximumFractionDigits: decimals };
  return new Intl.NumberFormat(f.locale, opts).format(n);
}

export function formatPercent(v: unknown, f: Fmt): string {
  const n = typeof v === "number" ? v : Number(v);
  if (!Number.isFinite(n)) {
    return String(v ?? "");
  }
  return new Intl.NumberFormat(f.locale, { style: "percent", maximumFractionDigits: 2 }).format(n);
}

export function formatDate(v: unknown, f: Fmt): string {
  if (typeof v !== "string" || !ISO_DATE.test(v)) {
    return String(v ?? "");
  }
  const d = new Date(v.length === 10 ? v + "T00:00:00" : v);
  if (Number.isNaN(d.getTime())) {
    return v;
  }
  return new Intl.DateTimeFormat(f.locale, { year: "numeric", month: "short", day: "numeric" }).format(d);
}

export function formatValue(v: unknown, format: FieldFormat, f: Fmt): string {
  if (v === undefined || v === null) {
    return "";
  }
  switch (format) {
    case FieldFormat.MONEY:
      return formatMoney(v, f);
    case FieldFormat.NUMBER:
      return formatNumber(v, f);
    case FieldFormat.INTEGER:
      return formatNumber(v, f, 0);
    case FieldFormat.DATE:
      return formatDate(v, f);
    case FieldFormat.PERCENT:
      return formatPercent(v, f);
    default:
      return typeof v === "object" ? JSON.stringify(v) : String(v);
  }
}

/** Follows a dotted path through sample data. */
export function lookup(data: unknown, path: string): unknown {
  let cur: unknown = data;
  for (const part of path.split(".")) {
    if (!part) {
      continue;
    }
    if (cur && typeof cur === "object" && !Array.isArray(cur)) {
      cur = (cur as Record<string, unknown>)[part];
    } else {
      return undefined;
    }
  }
  return cur;
}

const ACTION = /\{\{-?\s*([^}]*?)\s*-?\}\}/g;

/**
 * Fills the simple template actions the canvas can show: {{.a.b}},
 * {{money .x}}, {{fdate .x}}, {{percent .x}}, {{fnumber .x N}},
 * {{default "x" .y}} and {{if …}}…{{end}} chains. Anything else is shown as
 * written; the PDF preview is exact.
 */
export function fillText(text: string, data: unknown, f: Fmt): string {
  // Resolve if / else if / else / end chains first, innermost first, by
  // evaluating each condition's truthiness.
  let out = text;
  for (let i = 0; i < 8; i++) {
    const next = out.replace(/\{\{if ([^}]*)\}\}((?:(?!\{\{if )[\s\S])*?)\{\{end\}\}/g, (_m, cond: string, body: string) => {
      const parts = body.split(/\{\{else(?: if ([^}]*))?\}\}/);
      // parts alternates: text, cond|undefined, text, cond|undefined, text…
      let c: string | undefined = cond;
      for (let j = 0; j < parts.length; j += 2) {
        const clause = parts[j] ?? "";
        if (c === undefined || truthy(c, data)) {
          return clause;
        }
        c = parts[j + 1];
      }
      return "";
    });
    if (next === out) {
      break;
    }
    out = next;
  }
  return out.replace(ACTION, (m, body: string) => {
    const parts = body.split(/\s+/);
    const head = parts[0] ?? "";
    if (head.startsWith(".") || head.startsWith("$.")) {
      const v = lookup(data, head.replace(/^\$?\./, ""));
      return v === undefined ? m : formatValue(v, FieldFormat.TEXT, f);
    }
    const arg = parts[1] ?? "";
    const value = arg.startsWith(".") || arg.startsWith("$.") ? lookup(data, arg.replace(/^\$?\./, "")) : undefined;
    switch (head) {
      case "money":
        return value === undefined ? m : formatMoney(value, f);
      case "fdate":
        return value === undefined ? m : formatDate(value, f);
      case "percent":
        return value === undefined ? m : formatPercent(value, f);
      case "fnumber":
        return value === undefined ? m : formatNumber(value, f, parts[2] ? Number(parts[2]) : undefined);
      case "default": {
        const fallback = arg.replace(/^"|"$/g, "");
        const v = lookup(data, (parts[2] ?? "").replace(/^\$?\./, ""));
        return v === undefined || v === "" ? fallback : String(v);
      }
      default:
        return m;
    }
  });
}

function truthy(cond: string, data: unknown): boolean {
  const eq = cond.match(/^eq\s+\.([\w.]+)\s+"([^"]*)"$/);
  if (eq) {
    return String(lookup(data, eq[1]!) ?? "") === eq[2];
  }
  const path = cond.match(/^\.([\w.]+)$/);
  if (path) {
    const v = lookup(data, path[1]!);
    return !(v === undefined || v === null || v === "" || v === false || v === 0 || (Array.isArray(v) && v.length === 0));
  }
  return true;
}

/** Whether a show_if condition holds for the sample data. */
export function conditionHolds(c: Condition | undefined, data: unknown): boolean {
  if (!c || !c.path) {
    return true;
  }
  const v = lookup(data, c.path);
  const empty = v === undefined || v === null || v === "" || (Array.isArray(v) && v.length === 0);
  switch (c.op) {
    case ConditionOp.IS_EMPTY:
      return empty;
    case ConditionOp.EQUALS:
      return String(v ?? "") === c.value;
    case ConditionOp.NOT_EQUALS:
      return String(v ?? "") !== c.value;
    case ConditionOp.GREATER:
      return Number(v) > Number(c.value);
    case ConditionOp.LESS:
      return Number(v) < Number(c.value);
    default:
      return !empty;
  }
}

export const OPS = [
  { value: String(ConditionOp.IS_SET), label: "is set" },
  { value: String(ConditionOp.IS_EMPTY), label: "is empty" },
  { value: String(ConditionOp.EQUALS), label: "equals" },
  { value: String(ConditionOp.NOT_EQUALS), label: "is not" },
  { value: String(ConditionOp.GREATER), label: ">" },
  { value: String(ConditionOp.LESS), label: "<" },
];

export const FORMATS = [
  { value: String(FieldFormat.TEXT), label: "Text" },
  { value: String(FieldFormat.NUMBER), label: "Number" },
  { value: String(FieldFormat.INTEGER), label: "Integer" },
  { value: String(FieldFormat.MONEY), label: "Money" },
  { value: String(FieldFormat.DATE), label: "Date" },
  { value: String(FieldFormat.PERCENT), label: "Percent" },
];

export const ALIGNS = [
  { value: String(Align.UNSPECIFIED), label: "Default" },
  { value: String(Align.LEFT), label: "Left" },
  { value: String(Align.CENTER), label: "Center" },
  { value: String(Align.RIGHT), label: "Right" },
];

export const SPACINGS = [
  { value: String(Spacing.UNSPECIFIED), label: "Medium" },
  { value: String(Spacing.NONE), label: "None" },
  { value: String(Spacing.SMALL), label: "Small" },
  { value: String(Spacing.LARGE), label: "Large" },
];

export const PAGE_SIZES = [
  { value: String(PageSize.A4), label: "A4" },
  { value: String(PageSize.A5), label: "A5" },
  { value: String(PageSize.A3), label: "A3" },
  { value: String(PageSize.LETTER), label: "Letter" },
  { value: String(PageSize.LEGAL), label: "Legal" },
];

/** Page width and height in mm for the canvas. */
export function pageMm(page: PageOptions | undefined): { w: number; h: number } {
  const sizes: Record<number, [number, number]> = {
    [PageSize.A3]: [297, 420],
    [PageSize.A4]: [210, 297],
    [PageSize.A5]: [148, 210],
    [PageSize.LETTER]: [215.9, 279.4],
    [PageSize.LEGAL]: [215.9, 355.6],
  };
  const [w, h] = sizes[page?.size ?? PageSize.A4] ?? sizes[PageSize.A4]!;
  return page?.landscape ? { w: h, h: w } : { w, h };
}

export function pageSummary(page: PageOptions | undefined, layout: Layout | undefined): string {
  const size = PAGE_SIZES.find((p) => p.value === String(page?.size ?? PageSize.A4))?.label ?? "A4";
  const margin = page?.marginMm ?? 15;
  const repeats = (layout?.header?.blocks.length ?? 0) > 0 || (layout?.footer?.blocks.length ?? 0) > 0;
  return `${size} ${page?.landscape ? "landscape" : "portrait"} · ${margin} mm margins${repeats ? " · header and footer repeat on every page" : ""}`;
}

/** A deep copy of a template. */
export function cloneTemplate(t: Template): Template {
  return clone(TemplateSchema, t);
}
