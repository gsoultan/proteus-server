// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

import { describe, expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";

import { ConditionOp, FieldFormat, FieldType } from "@/gen/proteus/v1/template_pb.js";
import { BlockSchema } from "@/gen/proteus/v1/template_pb.js";

import {
  allBlocks,
  conditionHolds,
  emptyLayout,
  fieldPaths,
  fillText,
  flattenFields,
  formatValue,
  inferFields,
  insertBlock,
  insertInto,
  moveBlock,
  newBlock,
  regionOf,
  removeBlock,
  updateBlock,
} from "./layout";

describe("layout tree", () => {
  test("insert, update, move and remove blocks, including nested ones", () => {
    let l = emptyLayout();
    const a = newBlock("heading", l);
    const b = newBlock("text", l);
    l = insertBlock(l, a, "", "body");
    l = insertBlock(l, b, a.id);
    const cols = newBlock("columns", l);
    l = insertBlock(l, cols, "", "header");
    const inner = newBlock("field", l);
    l = insertInto(l, cols.id, 1, inner);
    expect(allBlocks(l).map((x) => x.id)).toEqual([cols.id, inner.id, a.id, b.id]);
    expect(regionOf(l, inner.id)).toBe("header");
    expect(regionOf(l, b.id)).toBe("body");

    l = updateBlock(l, inner.id, (x) => create(BlockSchema, { ...x, kind: { case: "field", value: { path: "client.name" } } as never }));
    const found = allBlocks(l).find((x) => x.id === inner.id)!;
    expect(found.kind.case === "field" && found.kind.value.path).toBe("client.name");

    l = moveBlock(l, b.id, -1);
    expect(l.body!.blocks.map((x) => x.id)).toEqual([b.id, a.id]);
    l = moveBlock(l, b.id, -1); // already first: no change
    expect(l.body!.blocks.map((x) => x.id)).toEqual([b.id, a.id]);

    l = removeBlock(l, inner.id);
    expect(allBlocks(l).map((x) => x.id)).toEqual([cols.id, b.id, a.id]);
  });

  test("ids are unique within a layout", () => {
    const l = emptyLayout();
    const ids = new Set(Array.from({ length: 50 }, () => newBlock("text", l).id));
    expect(ids.size).toBe(50);
  });
});

describe("fields", () => {
  test("infers types from sample data", () => {
    const fields = inferFields({
      number: "INV-1",
      issued: "2026-11-02",
      total: 100,
      qty: 3,
      paid: true,
      client: { name: "A", email: "a@example.com" },
      items: [{ description: "x", price: 5 }],
      "bad key": 1,
    });
    const byPath = Object.fromEntries(fields.map((f) => [f.path, f]));
    expect(byPath.number!.type).toBe(FieldType.TEXT);
    expect(byPath.issued!.type).toBe(FieldType.DATE);
    expect(byPath.total!.type).toBe(FieldType.MONEY);
    expect(byPath.qty!.type).toBe(FieldType.NUMBER);
    expect(byPath.paid!.type).toBe(FieldType.BOOL);
    expect(byPath.client!.type).toBe(FieldType.OBJECT);
    expect(byPath.client!.itemFields.map((f) => f.path)).toEqual(["name", "email"]);
    expect(byPath.items!.type).toBe(FieldType.LIST);
    expect(byPath.items!.itemFields.map((f) => f.type)).toEqual([FieldType.TEXT, FieldType.MONEY]);
    expect(byPath["bad key"]).toBeUndefined();

    expect(fieldPaths(fields)).toEqual(["number", "issued", "total", "qty", "paid", "client.name", "client.email", "items"]);
    const flat = flattenFields(fields);
    expect(flat.find((f) => f.path === "items[]")).toBeDefined();
    expect(flat.filter((f) => f.depth === 1).map((f) => f.path)).toEqual(["client.name", "client.email", "  .description", "  .price"]);
  });
});

describe("canvas formatting", () => {
  const id = { locale: "id-ID", currency: "IDR" };
  const us = { locale: "en-US", currency: "USD" };
  const data = { seller: { name: "Arunika" }, total: 1500000, issued: "2026-11-02", rate: 0.11, status: "paid", items: [] };

  test("formats values by kind", () => {
    expect(formatValue(1500000, FieldFormat.MONEY, id)).toMatch(/^Rp\s?1\.500\.000$/);
    expect(formatValue(1234.5, FieldFormat.MONEY, us)).toBe("$1,234.50");
    expect(formatValue(2, FieldFormat.INTEGER, us)).toBe("2");
    expect(formatValue(0.11, FieldFormat.PERCENT, us)).toBe("11%");
    expect(formatValue("2026-11-02", FieldFormat.DATE, us)).toBe("Nov 2, 2026");
    expect(formatValue("not a date", FieldFormat.DATE, us)).toBe("not a date");
  });

  test("fills simple template actions", () => {
    expect(fillText("Hello {{.seller.name}}, {{money .total}} due {{fdate .issued}} ({{percent .rate}})", data, id)).toMatch(
      /^Hello Arunika, Rp\s?1\.500\.000 due 2 Nov 2026 \(11%\)$/,
    );
    expect(fillText('{{if eq .status "paid"}}PAID{{else}}DUE{{end}}', data, us)).toBe("PAID");
    expect(fillText("{{if .items}}has items{{end}}-", data, us)).toBe("-");
    expect(fillText('{{if eq .status "draft"}}DRAFT{{else if eq .status "paid"}}PAID{{else}}DUE{{end}}', data, us)).toBe("PAID");
    expect(fillText('{{if eq .status "draft"}}DRAFT{{else if eq .status "void"}}VOID{{end}}', data, us)).toBe("");
    expect(fillText('{{default "Sincerely," .closing}}', data, us)).toBe("Sincerely,");
    // Unknown actions are left as written.
    expect(fillText("{{qr .url}} {{.missing}}", data, us)).toBe("{{qr .url}} {{.missing}}");
  });

  test("conditions", () => {
    expect(conditionHolds(undefined, data)).toBe(true);
    expect(conditionHolds({ path: "status", op: ConditionOp.EQUALS, value: "paid" } as never, data)).toBe(true);
    expect(conditionHolds({ path: "items", op: ConditionOp.IS_SET, value: "" } as never, data)).toBe(false);
    expect(conditionHolds({ path: "items", op: ConditionOp.IS_EMPTY, value: "" } as never, data)).toBe(true);
    expect(conditionHolds({ path: "total", op: ConditionOp.GREATER, value: "100" } as never, data)).toBe(true);
  });
});
