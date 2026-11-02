// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// The builder's document: the template being edited, selection, inspector
// state and an undo history. The template is a plain protobuf message; every
// edit replaces it with a new one through `edit`, which also records history.

import { clone, create } from "@bufbuild/protobuf";
import { create as createStore } from "zustand";

import { OptionsSchema, PageOptionsSchema, type Options } from "@/gen/proteus/v1/conversion_pb.js";
import { BlockSchema, TemplateSchema, type Block, type Layout, type Template } from "@/gen/proteus/v1/template_pb.js";
import { emptyLayout, findBlock, insertBlock, insertInto, moveBlock, newBlock, regionOf, removeBlock, updateBlock, type BlockKind } from "@/model/layout";

export type Tab = "block" | "page" | "watermark" | "data";
export type Mode = "visual" | "code";

export interface BuilderState {
  template: Template | undefined;
  /** The revision last saved, to know when there are unsaved edits. */
  savedRevision: bigint;
  dirty: boolean;
  selectedId: string;
  tab: Tab;
  mode: Mode;
  showData: boolean;
  past: Template[];
  future: Template[];

  load(t: Template): void;
  edit(change: (t: Template) => Template, options?: { history?: boolean }): void;
  editLayout(change: (l: Layout) => Layout): void;
  editOptions(change: (o: Options) => Options): void;
  select(id: string, tab?: Tab): void;
  setTab(tab: Tab): void;
  setMode(mode: Mode): void;
  toggleData(): void;
  undo(): void;
  redo(): void;
  markSaved(t: Template): void;

  addBlock(kind: BlockKind): string;
  addFieldBlock(path: string): string;
  addInto(containerId: string, regionIndex: number, kind: BlockKind): void;
  updateSelected(change: (b: Block) => Block): void;
  removeSelected(): void;
  moveSelected(dir: -1 | 1): void;
}

const MAX_HISTORY = 100;

function layoutOf(t: Template): Layout {
  return t.layout ?? emptyLayout();
}

export const useBuilder = createStore<BuilderState>()((set, get) => ({
  template: undefined,
  savedRevision: 0n,
  dirty: false,
  selectedId: "",
  tab: "block",
  mode: "visual",
  showData: true,
  past: [],
  future: [],

  load: (t) =>
    set({ template: clone(TemplateSchema, t), savedRevision: t.revision, dirty: false, past: [], future: [], selectedId: "", mode: t.layout ? "visual" : "code" }),

  edit: (change, options) => {
    const { template, past } = get();
    if (!template) {
      return;
    }
    const next = change(template);
    const history = options?.history ?? true;
    set({
      template: next,
      dirty: true,
      past: history ? [...past.slice(-MAX_HISTORY + 1), template] : past,
      future: history ? [] : get().future,
    });
  },

  editLayout: (change) => get().edit((t) => create(TemplateSchema, { ...t, layout: change(layoutOf(t)) })),

  editOptions: (change) =>
    get().edit((t) => create(TemplateSchema, { ...t, options: change(t.options ?? create(OptionsSchema, { page: create(PageOptionsSchema, {}) })) })),

  select: (id, tab) => set({ selectedId: id, tab: tab ?? (id ? "block" : get().tab) }),
  setTab: (tab) => set({ tab }),
  setMode: (mode) => set({ mode }),
  toggleData: () => set((s) => ({ showData: !s.showData })),

  undo: () => {
    const { past, template, future } = get();
    const prev = past[past.length - 1];
    if (!prev || !template) {
      return;
    }
    set({ template: prev, past: past.slice(0, -1), future: [template, ...future], dirty: true });
  },
  redo: () => {
    const { past, template, future } = get();
    const next = future[0];
    if (!next || !template) {
      return;
    }
    set({ template: next, past: [...past, template], future: future.slice(1), dirty: true });
  },

  markSaved: (t) => {
    const cur = get().template;
    set({
      template: cur ? create(TemplateSchema, { ...cur, id: t.id, revision: t.revision, content: t.content, updateTime: t.updateTime }) : clone(TemplateSchema, t),
      savedRevision: t.revision,
      dirty: false,
    });
  },

  addBlock: (kind) => {
    const { template, selectedId } = get();
    const layout = layoutOf(template ?? create(TemplateSchema, {}));
    const block = newBlock(kind, layout);
    const region = selectedId ? (regionOf(layout, selectedId) ?? "body") : "body";
    get().editLayout((l) => insertBlock(l, block, selectedId, region));
    set({ selectedId: block.id, tab: "block" });
    return block.id;
  },

  addFieldBlock: (path) => {
    const id = get().addBlock("field");
    get().updateSelected((b) => create(BlockSchema, { ...b, kind: { case: "field", value: { path, label: "" } } as never }));
    return id;
  },

  addInto: (containerId, regionIndex, kind) => {
    const layout = layoutOf(get().template ?? create(TemplateSchema, {}));
    const block = newBlock(kind, layout);
    get().editLayout((l) => insertInto(l, containerId, regionIndex, block));
    set({ selectedId: block.id, tab: "block" });
  },

  updateSelected: (change) => {
    const { selectedId } = get();
    if (!selectedId) {
      return;
    }
    get().editLayout((l) => updateBlock(l, selectedId, change));
  },

  removeSelected: () => {
    const { selectedId } = get();
    if (!selectedId) {
      return;
    }
    get().editLayout((l) => removeBlock(l, selectedId));
    set({ selectedId: "" });
  },

  moveSelected: (dir) => {
    const { selectedId, template } = get();
    if (!selectedId || !template || !findBlock(layoutOf(template), selectedId)) {
      return;
    }
    get().editLayout((l) => moveBlock(l, selectedId, dir));
  },
}));

/** The selected block, if any. */
export function selectedBlock(s: BuilderState): Block | undefined {
  return s.template?.layout && s.selectedId ? findBlock(s.template.layout, s.selectedId) : undefined;
}
