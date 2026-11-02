// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// The editing canvas: a page drawn by the client from the layout, with the
// sample data filled in (or field placeholders), selectable blocks and a
// watermark overlay. Approximate by design; the PDF preview is exact.

import { Box, Text } from "@mantine/core";
import type { CSSProperties, ReactNode } from "react";

import { WatermarkPlacement, type Watermark } from "@/gen/proteus/v1/conversion_pb.js";
import { AggregateFunc, Align, FieldFormat, Spacing, type Block, type Region, type Template } from "@/gen/proteus/v1/template_pb.js";
import { conditionHolds, fillText, fmtOf, formatValue, lookup, pageMm, type Fmt } from "@/model/layout";
import { useBuilder } from "@/store/builder";

const MM = 3.2; // px per mm at 100%

export function Canvas({ template }: { template: Template }) {
  const selectedId = useBuilder((s) => s.selectedId);
  const select = useBuilder((s) => s.select);
  const showData = useBuilder((s) => s.showData);
  const layout = template.layout;
  const theme = layout?.theme;
  const page = template.options?.page;
  const { w, h } = pageMm(page);
  const margin = page?.marginMm ?? 15;
  const headerMm = layout?.header?.heightMm || 25;
  const footerMm = layout?.footer?.heightMm || 15;
  const fmt = fmtOf(template);
  const data = showData ? (template.sampleData ?? {}) : undefined;
  const ctx: Ctx = { data, fmt, selectedId, select, showData, accent: theme?.accentColor || "#3b5bdb" };
  const hasHeader = (layout?.header?.blocks.length ?? 0) > 0;
  const hasFooter = (layout?.footer?.blocks.length ?? 0) > 0;

  return (
    <Box
      style={{
        position: "relative",
        width: w * MM,
        minHeight: h * MM,
        maxWidth: "100%",
        background: "#fff",
        boxShadow: "0 4px 18px rgba(0,0,0,0.14)",
        boxSizing: "border-box",
        padding: `${margin * MM}px`,
        display: "flex",
        flexDirection: "column",
        overflow: "hidden",
        fontFamily: theme?.fontFamily || "Inter, Helvetica, Arial, sans-serif",
        fontSize: (theme?.fontSizePt || 10) * 1.333 * 0.95,
        color: theme?.textColor || "#212529",
        lineHeight: 1.4,
        ["--accent" as string]: ctx.accent,
      }}
      onClick={() => select("")}
    >
      <WatermarkOverlay wm={page?.watermark} data={data} fmt={fmt} />
      <Band label="Header · every page" />
      <Box style={{ minHeight: hasHeader ? headerMm * MM : 0, position: "relative", zIndex: 1 }}>
        <RegionView region={layout?.header} ctx={ctx} empty="Header: add a block from the palette and it repeats on every page." />
      </Box>
      <Band label="Body" />
      <Box style={{ flex: 1, position: "relative", zIndex: 1 }}>
        <RegionView region={layout?.body} ctx={ctx} empty="Empty page. Pick a block on the left to start." />
      </Box>
      <Band label="Footer · every page" />
      <Box style={{ minHeight: hasFooter ? footerMm * MM : 0, position: "relative", zIndex: 1 }}>
        <RegionView region={layout?.footer} ctx={ctx} empty="Footer: optional." />
      </Box>
    </Box>
  );
}

interface Ctx {
  data: unknown;
  fmt: Fmt;
  selectedId: string;
  select(id: string): void;
  showData: boolean;
  accent: string;
}

function Band({ label }: { label: string }) {
  return (
    <Text fz={9} fw={700} c="gray.6" style={{ letterSpacing: "0.06em", textTransform: "uppercase", position: "relative", zIndex: 1, margin: "4px 0 4px" }}>
      {label}
    </Text>
  );
}

function RegionView({ region, ctx, empty }: { region: Region | undefined; ctx: Ctx; empty?: string }) {
  if (!region || region.blocks.length === 0) {
    return empty ? (
      <Text fz={11} c="dimmed" fs="italic" p={6}>
        {empty}
      </Text>
    ) : null;
  }
  return (
    <>
      {region.blocks.map((b) => (
        <BlockView key={b.id} block={b} ctx={ctx} />
      ))}
    </>
  );
}

const SPACE: Record<number, number> = { [Spacing.UNSPECIFIED]: 5, [Spacing.NONE]: 0, [Spacing.SMALL]: 2, [Spacing.MEDIUM]: 5, [Spacing.LARGE]: 10 };
const ALIGN: Record<number, CSSProperties["textAlign"]> = { [Align.LEFT]: "left", [Align.CENTER]: "center", [Align.RIGHT]: "right" };

function BlockView({ block, ctx, scopeData }: { block: Block; ctx: Ctx; scopeData?: unknown }) {
  const data = scopeData ?? ctx.data;
  const hidden = ctx.showData && !conditionHolds(block.showIf, data);
  const on = ctx.selectedId === block.id;
  const st = block.style;
  const style: CSSProperties = {
    position: "relative",
    marginBottom: SPACE[st?.spaceBelow ?? 0] ?? 5,
    textAlign: ALIGN[st?.align ?? 0],
    fontSize: st?.fontSizePt ? st.fontSizePt * 1.333 * 0.95 : undefined,
    fontWeight: st?.bold ? 700 : undefined,
    color: st?.color || undefined,
    background: st?.background || undefined,
    opacity: hidden ? 0.35 : 1,
    outline: on ? "2px solid #3b5bdb" : "1px dashed transparent",
    outlineOffset: 2,
    borderRadius: 4,
    cursor: "pointer",
    padding: 2,
  };
  return (
    <div
      style={style}
      data-block={block.id}
      onClick={(e) => {
        e.stopPropagation();
        ctx.select(block.id);
      }}
      onMouseEnter={(e) => {
        if (!on) (e.currentTarget as HTMLElement).style.outline = "1px dashed #adb5bd";
      }}
      onMouseLeave={(e) => {
        if (!on) (e.currentTarget as HTMLElement).style.outline = "1px dashed transparent";
      }}
    >
      {hidden && block.showIf?.path && (
        <Text fz={9} c="indigo.7" style={{ position: "absolute", right: 2, top: -8, background: "#fff", padding: "0 4px" }}>
          hidden: {block.showIf.path}
        </Text>
      )}
      <Kind block={block} ctx={ctx} data={data} />
    </div>
  );
}

function Kind({ block, ctx, data }: { block: Block; ctx: Ctx; data: unknown }) {
  const k = block.kind;
  const fill = (s: string) => (ctx.showData ? fillText(s, data, ctx.fmt) : s);
  const val = (path: string, format: FieldFormat) => {
    if (!ctx.showData) {
      return `{{${path}}}`;
    }
    const v = lookup(data, path);
    return v === undefined ? "" : formatValue(v, format, ctx.fmt);
  };
  switch (k.case) {
    case "heading": {
      const sizes = [2.2, 1.5, 1.15];
      return (
        <div style={{ color: "var(--accent)", fontWeight: 700, lineHeight: 1.2, fontSize: `${sizes[(k.value.level || 1) - 1] ?? 1.15}em`, letterSpacing: k.value.level <= 1 ? "0.02em" : undefined }}>
          {fill(k.value.text) || <Placeholder>Heading</Placeholder>}
        </div>
      );
    }
    case "text":
      return (
        <div>
          {fill(k.value.text)
            .split(/\n\n+/)
            .map((p, i) => (
              <p key={i} style={{ margin: "0 0 0.6em", whiteSpace: "pre-wrap" }}>
                {p}
              </p>
            ))}
        </div>
      );
    case "field": {
      const v = val(k.value.path, k.value.format);
      return (
        <div>
          {k.value.label && <Label>{fill(k.value.label)}</Label>}
          <span style={{ fontWeight: 600 }}>
            {k.value.prefix}
            {v || (k.value.path ? "" : <Placeholder>Pick a field</Placeholder>)}
            {k.value.suffix}
          </span>
        </div>
      );
    }
    case "image": {
      const w = (k.value.widthMm || 30) * MM;
      const hgt = k.value.heightMm ? k.value.heightMm * MM : w * 0.6;
      return (
        <div style={{ width: w, height: hgt, borderRadius: 6, background: "var(--accent)", color: "#fff", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 11, fontWeight: 700 }}>
          {k.value.source.case === "asset" ? k.value.source.value.split("/").pop() : `{{${k.value.source.value ?? "image"}}}`}
        </div>
      );
    }
    case "table": {
      const t = k.value;
      const rows = ctx.showData ? ((lookup(data, t.list) as unknown[] | undefined) ?? []) : [undefined];
      const widths = t.columns.map((c) => (c.widthPct ? `${c.widthPct}%` : "auto"));
      if (ctx.showData && rows.length === 0) {
        return <Text fz="inherit" c="dimmed" fs="italic">{t.emptyText || "No items"}</Text>;
      }
      return (
        <table style={{ width: "100%", borderCollapse: "collapse" }}>
          <thead>
            <tr>
              {t.numbered && <th style={th(ctx.accent, "center", "8%")}>#</th>}
              {t.columns.map((c, i) => (
                <th key={i} style={th(ctx.accent, numeric(c.format) ? "right" : "left", widths[i])}>
                  {c.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row, ri) => (
              <tr key={ri} style={{ background: t.striped && ri % 2 === 1 ? "#f8f9fa" : undefined }}>
                {t.numbered && <td style={td("center")}>{ri + 1}</td>}
                {t.columns.map((c, i) => (
                  <td key={i} style={td(numeric(c.format) ? "right" : "left")}>
                    {ctx.showData ? formatValue(lookup(row, c.path), c.format, ctx.fmt) : `{{.${c.path}}}`}
                  </td>
                ))}
              </tr>
            ))}
            {!ctx.showData && (
              <tr>
                <td colSpan={t.columns.length + (t.numbered ? 1 : 0)} style={{ padding: "3px 6px", fontSize: 10, color: "#364fc7" }}>
                  Repeats for each entry in {t.list || "…"}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      );
    }
    case "keyValue":
      return (
        <div style={{ display: "grid", gridTemplateColumns: "auto 1fr", gap: "2px 10px" }}>
          {k.value.rows.map((r, i) => (
            <KV key={i} label={fill(r.label)} value={val(r.value, r.format)} />
          ))}
        </div>
      );
    case "totals":
      return (
        <div style={{ marginLeft: "auto", width: "45%" }}>
          {k.value.rows.map((r, i) => {
            let v = "";
            if (r.value.case === "path") {
              v = val(r.value.value, r.format || FieldFormat.MONEY);
            } else if (r.value.case === "aggregate") {
              const a = r.value.value;
              const list = ctx.showData ? ((lookup(data, a.list) as unknown[] | undefined) ?? []) : [];
              const nums = list.map((it) => Number(lookup(it, a.column))).filter((n) => Number.isFinite(n));
              const agg = a.func === AggregateFunc.COUNT ? list.length : a.func === AggregateFunc.AVERAGE ? (nums.length ? nums.reduce((x, y) => x + y, 0) / nums.length : 0) : nums.reduce((x, y) => x + y, 0);
              v = ctx.showData ? formatValue(agg, r.format || (a.func === AggregateFunc.COUNT ? FieldFormat.INTEGER : FieldFormat.MONEY), ctx.fmt) : `{{${a.func === AggregateFunc.COUNT ? "count" : a.func === AggregateFunc.AVERAGE ? "avg" : "sum"} .${a.list} "${a.column}"}}`;
            }
            return (
              <div key={i} style={{ display: "flex", justifyContent: "space-between", padding: "2px 0", fontWeight: r.emphasized ? 700 : undefined, fontSize: r.emphasized ? "1.2em" : undefined, borderTop: r.emphasized ? "1px solid #ced4da" : undefined, marginTop: r.emphasized ? 2 : undefined }}>
                <span style={{ color: r.emphasized ? undefined : "#495057" }}>{fill(r.label)}</span>
                <span>{v}</span>
              </div>
            );
          })}
        </div>
      );
    case "columns": {
      const widths = k.value.widths;
      return (
        <div style={{ display: "flex", gap: 8 * MM * 0.6 }}>
          {k.value.columns.map((col, i) => (
            <div key={i} style={{ flex: `${widths[i] ?? 1} 1 0`, minWidth: 0 }}>
              <RegionView region={col} ctx={ctx} empty="Empty column" />
            </div>
          ))}
        </div>
      );
    }
    case "signature":
      return (
        <div style={{ display: "flex", gap: 12 * MM * 0.6, marginTop: 10 * MM * 0.6 }}>
          {k.value.lines.map((l, i) => (
            <div key={i} style={{ flex: "1 1 0" }}>
              <div style={{ height: 34, borderBottom: "1px solid #495057" }} />
              <div style={{ paddingTop: 3, fontSize: "0.85em", color: "#495057" }}>{fill(l.label)}</div>
            </div>
          ))}
        </div>
      );
    case "qr": {
      const s = (k.value.sizeMm || 22) * MM;
      return (
        <div style={{ width: s, height: s, border: "2px solid #212529", display: "grid", placeItems: "center", fontSize: 9, textAlign: "center", padding: 4, boxSizing: "border-box" }}>
          QR
          <br />
          {k.value.value || "field"}
        </div>
      );
    }
    case "divider":
      return <hr style={{ border: 0, borderTop: "1px solid var(--accent)", margin: 0 }} />;
    case "spacer":
      return <div style={{ height: (k.value.heightMm || 8) * MM, background: "repeating-linear-gradient(135deg, transparent 0 6px, #f1f3f5 6px 8px)" }} />;
    case "section": {
      const s = k.value;
      const items = ctx.showData && s.repeat ? ((lookup(data, s.repeat) as unknown[] | undefined) ?? []) : [data];
      return (
        <div style={{ borderLeft: "2px solid #dbe4ff", paddingLeft: 6 }}>
          {s.repeat && <Text fz={9} c="indigo.7">Repeats for each entry in {s.repeat}{s.pagePerItem ? ", one per page" : ""}</Text>}
          {(items.length ? items : [undefined]).slice(0, 3).map((it, i) => (
            <div key={i}>
              {s.content?.blocks.map((b) => (
                <BlockView key={b.id} block={b} ctx={ctx} scopeData={s.repeat ? it : data} />
              ))}
            </div>
          ))}
          {items.length > 3 && <Text fz={9} c="dimmed">… and {items.length - 3} more</Text>}
        </div>
      );
    }
    case "pageBreak":
      return <div style={{ borderTop: "2px dashed #adb5bd", textAlign: "center", fontSize: 9, color: "#868e96" }}>page break</div>;
    default:
      return <Placeholder>Unknown block</Placeholder>;
  }
}

function numeric(f: FieldFormat): boolean {
  return f === FieldFormat.NUMBER || f === FieldFormat.INTEGER || f === FieldFormat.MONEY || f === FieldFormat.PERCENT;
}

function th(accent: string, align: CSSProperties["textAlign"], width?: string): CSSProperties {
  return { background: accent, color: "#fff", textAlign: align, padding: "4px 6px", fontSize: "0.9em", fontWeight: 600, width };
}

function td(align: CSSProperties["textAlign"]): CSSProperties {
  return { padding: "4px 6px", borderBottom: "1px solid #dee2e6", verticalAlign: "top", textAlign: align };
}

function KV({ label, value }: { label: string; value: string }) {
  return (
    <>
      <span style={{ color: "#495057" }}>{label}</span>
      <span style={{ fontWeight: 600 }}>{value}</span>
    </>
  );
}

function Label({ children }: { children: ReactNode }) {
  return <span style={{ display: "block", fontSize: "0.75em", fontWeight: 700, color: "#495057", textTransform: "uppercase", letterSpacing: "0.06em" }}>{children}</span>;
}

function Placeholder({ children }: { children: ReactNode }) {
  return <span style={{ color: "#adb5bd", fontStyle: "italic" }}>{children}</span>;
}

function WatermarkOverlay({ wm, data, fmt }: { wm: Watermark | undefined; data: unknown; fmt: Fmt }) {
  if (!wm || wm.mark.case !== "text") {
    return null;
  }
  const text = data === undefined ? wm.mark.value.replace(/\{\{[^}]*\}\}/g, "").trim() || "WATERMARK" : fillText(wm.mark.value, data, fmt).trim();
  if (!text) {
    return null;
  }
  const color = wm.color || "#444444";
  const opacity = wm.opacity || 0.15;
  const angle = wm.angleDeg ?? 30;
  const size = (wm.fontSizePt || 72) * 1.1;
  const base: CSSProperties = { color, opacity, fontWeight: 800, letterSpacing: "0.06em", whiteSpace: "nowrap", transform: `rotate(${-angle}deg)`, fontFamily: wm.fontFamily || undefined };
  if (wm.placement === WatermarkPlacement.TILED) {
    return (
      <div aria-hidden="true" style={{ position: "absolute", inset: -40, display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gridAutoRows: 150, alignItems: "center", justifyItems: "center", pointerEvents: "none", zIndex: 0 }}>
        {Array.from({ length: 18 }, (_, i) => (
          <span key={i} style={{ ...base, fontSize: size * 0.4 }}>
            {text}
          </span>
        ))}
      </div>
    );
  }
  return (
    <div aria-hidden="true" style={{ position: "absolute", inset: 0, display: "flex", alignItems: "center", justifyContent: "center", pointerEvents: "none", zIndex: 0 }}>
      <span style={{ ...base, fontSize: size }}>{text}</span>
    </div>
  );
}
