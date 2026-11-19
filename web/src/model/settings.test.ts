// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

import { create, type MessageInitShape } from "@bufbuild/protobuf";
import { describe, expect, test } from "bun:test";

import { SettingSchema, Setting_Kind, Setting_Source, type Setting } from "@/gen/proteus/v1/setup_pb.js";

import { changedValues, checkValue, groupSettings, locked } from "./settings";

function setting(name: string, kind: Setting_Kind, value: string, def: string, extra: MessageInitShape<typeof SettingSchema> = {}): Setting {
  return create(SettingSchema, { name, kind, value, defaultValue: def, editable: true, group: "Limits", source: Setting_Source.DEFAULT, ...extra });
}

describe("settings model", () => {
  const all = [
    setting("PROTEUS_TIMEOUT", Setting_Kind.DURATION, "5s", "30s", { source: Setting_Source.SAVED }),
    setting("PROTEUS_WORKERS", Setting_Kind.INT, "3", "0", { source: Setting_Source.ENVIRONMENT, group: "Conversion" }),
    setting("PROTEUS_ALLOWED_HOSTS", Setting_Kind.LIST, "a.com,b.com", "", { group: "Remote resources", source: Setting_Source.SAVED }),
    setting("PROTEUS_WEBHOOK_SECRET", Setting_Kind.SECRET, "", "", { isSet: true, group: "Jobs" }),
    setting("PROTEUS_ADDR", Setting_Kind.STRING, ":8080", ":8080", { editable: false, group: "Server" }),
  ];

  test("groups editable settings and keeps startup ones apart", () => {
    const g = groupSettings(all);
    expect(g.editable.map((x) => x.name)).toEqual(["Limits", "Conversion", "Remote resources", "Jobs"]);
    expect(g.startup.map((s) => s.name)).toEqual(["PROTEUS_ADDR"]);
  });

  test("environment values are locked", () => {
    expect(locked(all[1]!)).toBe(true);
    expect(locked(all[0]!)).toBe(false);
  });

  test("only real changes are sent; back to default clears", () => {
    expect(changedValues(all, {})).toEqual({});
    expect(changedValues(all, { PROTEUS_ALLOWED_HOSTS: " a.com , b.com " })).toEqual({});
    expect(changedValues(all, { PROTEUS_TIMEOUT: "30s" })).toEqual({ PROTEUS_TIMEOUT: "" });
    expect(changedValues(all, { PROTEUS_TIMEOUT: "10s", PROTEUS_WORKERS: "8" })).toEqual({ PROTEUS_TIMEOUT: "10s" });
  });

  test("secrets are sent only when typed or cleared", () => {
    expect(changedValues(all, { PROTEUS_WEBHOOK_SECRET: "" })).toEqual({});
    expect(changedValues(all, { PROTEUS_WEBHOOK_SECRET: "x".repeat(32) })).toEqual({ PROTEUS_WEBHOOK_SECRET: "x".repeat(32) });
    expect(changedValues(all, {}, new Set(["PROTEUS_WEBHOOK_SECRET"]))).toEqual({ PROTEUS_WEBHOOK_SECRET: "" });
  });

  test("checks values", () => {
    expect(checkValue(Setting_Kind.DURATION, "1h30m")).toBeNull();
    expect(checkValue(Setting_Kind.DURATION, "soon")).not.toBeNull();
    expect(checkValue(Setting_Kind.DURATION, "0s")).not.toBeNull();
    expect(checkValue(Setting_Kind.BYTES, "20MiB")).toBeNull();
    expect(checkValue(Setting_Kind.BYTES, "big")).not.toBeNull();
    expect(checkValue(Setting_Kind.INT, "-1")).not.toBeNull();
    expect(checkValue(Setting_Kind.FLOAT, "2.5")).toBeNull();
    expect(checkValue(Setting_Kind.INT, "")).toBeNull();
  });
});
