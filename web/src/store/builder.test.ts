// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

import { beforeEach, describe, expect, test } from "bun:test";
import { create } from "@bufbuild/protobuf";

import { TemplateSchema } from "@/gen/proteus/v1/template_pb.js";
import { allBlocks, emptyLayout } from "@/model/layout";

import { selectedBlock, useBuilder } from "./builder";

describe("builder store", () => {
  beforeEach(() => {
    useBuilder.getState().load(create(TemplateSchema, { id: "t1", name: "T", revision: 3n, layout: emptyLayout() }));
  });

  test("adds blocks after the selection and keeps undo history", () => {
    const s = useBuilder.getState;
    const h = s().addBlock("heading");
    expect(s().selectedId).toBe(h);
    expect(s().dirty).toBe(true);
    const t = s().addBlock("text");
    expect(s().template!.layout!.body!.blocks.map((b) => b.id)).toEqual([h, t]);
    s().select(h);
    const f = s().addFieldBlock("client.name");
    expect(s().template!.layout!.body!.blocks.map((b) => b.id)).toEqual([h, f, t]);
    const sel = selectedBlock(s());
    expect(sel?.kind.case === "field" && sel.kind.value.path).toBe("client.name");

    s().undo();
    s().undo();
    expect(s().template!.layout!.body!.blocks.map((b) => b.id)).toEqual([h, t]);
    s().redo();
    s().redo();
    expect(allBlocks(s().template!.layout!).length).toBe(3);

    s().select(t);
    s().moveSelected(-1);
    expect(s().template!.layout!.body!.blocks.map((b) => b.id)).toEqual([h, t, f]);
    s().removeSelected();
    expect(s().selectedId).toBe("");
    expect(allBlocks(s().template!.layout!).map((b) => b.id)).toEqual([h, f]);
  });

  test("markSaved clears dirty and keeps edits", () => {
    const s = useBuilder.getState;
    s().addBlock("divider");
    s().markSaved(create(TemplateSchema, { id: "t1", revision: 4n, content: new TextEncoder().encode("<html>") }));
    expect(s().dirty).toBe(false);
    expect(s().template!.revision).toBe(4n);
    expect(allBlocks(s().template!.layout!).length).toBe(1);
    expect(s().template!.content.length).toBeGreaterThan(0);
  });

  test("mode follows the template kind", () => {
    const s = useBuilder.getState;
    expect(s().mode).toBe("visual");
    s().load(create(TemplateSchema, { id: "c", content: new TextEncoder().encode("x") }));
    expect(s().mode).toBe("code");
    expect(s().past).toEqual([]);
  });
});
