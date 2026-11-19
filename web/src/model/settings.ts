// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// Pure helpers for the settings page and the setup wizard: grouping,
// change tracking and quick client-side checks. The server validates
// everything again; these only give feedback before saving.

import { Setting_Kind, Setting_Source, type Setting } from "@/gen/proteus/v1/setup_pb.js";

export interface SettingGroup {
  name: string;
  settings: Setting[];
}

/** Groups settings in server order, editable groups first. */
export function groupSettings(settings: Setting[]): { editable: SettingGroup[]; startup: Setting[] } {
  const editable: SettingGroup[] = [];
  const startup: Setting[] = [];
  for (const s of settings) {
    if (!s.editable) {
      startup.push(s);
      continue;
    }
    let g = editable.find((x) => x.name === s.group);
    if (!g) {
      g = { name: s.group, settings: [] };
      editable.push(g);
    }
    g.settings.push(s);
  }
  return { editable, startup };
}

/** The text a setting's input starts with. Secrets start empty. */
export function initialValue(s: Setting): string {
  return s.kind === Setting_Kind.SECRET ? "" : s.value;
}

/** Whether the environment fixes this setting, so the UI cannot change it. */
export function locked(s: Setting): boolean {
  return !s.editable || s.source === Setting_Source.ENVIRONMENT;
}

/**
 * The values to send: settings whose draft differs from what is in effect.
 * A setting set back to its default is sent as "" so the saved value goes.
 * Secrets are sent only when typed into or explicitly cleared.
 */
export function changedValues(settings: Setting[], draft: Record<string, string>, clearedSecrets: Set<string> = new Set()): Record<string, string> {
  const out: Record<string, string> = {};
  for (const s of settings) {
    if (locked(s)) {
      continue;
    }
    if (s.kind === Setting_Kind.SECRET) {
      const v = (draft[s.name] ?? "").trim();
      if (v) {
        out[s.name] = v;
      } else if (clearedSecrets.has(s.name)) {
        out[s.name] = "";
      }
      continue;
    }
    const v = (draft[s.name] ?? initialValue(s)).trim();
    if (normalize(s.kind, v) === normalize(s.kind, s.value)) {
      continue;
    }
    out[s.name] = normalize(s.kind, v) === normalize(s.kind, s.defaultValue) ? "" : v;
  }
  return out;
}

function normalize(kind: Setting_Kind, v: string): string {
  v = v.trim();
  if (kind === Setting_Kind.LIST) {
    return v.split(",").map((x) => x.trim()).filter(Boolean).join(",");
  }
  if (kind === Setting_Kind.BOOL) {
    return String(["1", "t", "true", "y", "yes", "on"].includes(v.toLowerCase()));
  }
  return v;
}

const DURATION = /^(\d+(\.\d+)?(ns|us|µs|ms|s|m|h))+$/;
const BYTES = /^\d+\s*(k|m|g|kib|mib|gib)?$/i;

/** A short problem with the text, or null when it looks fine. */
export function checkValue(kind: Setting_Kind, v: string): string | null {
  v = v.trim();
  if (!v) {
    return null;
  }
  switch (kind) {
    case Setting_Kind.INT:
      return /^\d+$/.test(v) ? null : "Whole number, 0 or more";
    case Setting_Kind.FLOAT:
      return /^\d+(\.\d+)?$/.test(v) ? null : "Number, 0 or more";
    case Setting_Kind.DURATION:
      return DURATION.test(v) && !/^0+(\.0+)?[a-zµ]+$/.test(v) ? null : "Duration like 30s, 5m or 1h30m";
    case Setting_Kind.BYTES:
      return BYTES.test(v) && !/^0+\s*[a-z]*$/i.test(v) ? null : "Size like 20MiB or 512KiB";
    case Setting_Kind.SECRET:
      return null;
    default:
      return null;
  }
}

/** Human label for where a value came from. */
export function sourceLabel(s: Setting): string {
  switch (s.source) {
    case Setting_Source.ENVIRONMENT:
      return "environment";
    case Setting_Source.BOOTSTRAP:
      return "setup";
    case Setting_Source.SAVED:
      return "saved";
    default:
      return "default";
  }
}
