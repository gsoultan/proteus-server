// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

import { Code, ConnectError, createClient, type Client, type Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { create } from "zustand";
import { persist } from "zustand/middleware";

import { ConversionService } from "@/gen/proteus/v1/conversion_pb.js";
import { TemplateDataErrorSchema, TemplateService, type DataProblem } from "@/gen/proteus/v1/template_pb.js";

/** Connection settings, kept in localStorage on this device. */
export interface Settings {
  /** Empty means the origin the app was loaded from. */
  baseUrl: string;
  apiKey: string;
  setSettings(s: Partial<Settings>): void;
}

export const useSettings = create<Settings>()(
  persist(
    (set) => ({
      baseUrl: "",
      apiKey: "",
      setSettings: (s) => set(s),
    }),
    { name: "proteus.settings", partialize: (s) => ({ baseUrl: s.baseUrl, apiKey: s.apiKey }) },
  ),
);

export interface Proteus {
  conversion: Client<typeof ConversionService>;
  templates: Client<typeof TemplateService>;
}

function apiKeyInterceptor(apiKey: string): Interceptor {
  return (next) => (req) => {
    if (apiKey) {
      req.header.set("Authorization", `Bearer ${apiKey}`);
    }
    return next(req);
  };
}

const cache = new Map<string, Proteus>();

/** Clients for the current settings; one per (url, key). */
export function proteus(baseUrl: string, apiKey: string): Proteus {
  const url = baseUrl || window.location.origin;
  const k = `${url}\n${apiKey}`;
  let p = cache.get(k);
  if (!p) {
    const transport = createConnectTransport({
      baseUrl: url,
      useBinaryFormat: true,
      interceptors: [apiKeyInterceptor(apiKey)],
    });
    p = { conversion: createClient(ConversionService, transport), templates: createClient(TemplateService, transport) };
    cache.set(k, p);
  }
  return p;
}

export function useProteus(): Proteus {
  const baseUrl = useSettings((s) => s.baseUrl);
  const apiKey = useSettings((s) => s.apiKey);
  return proteus(baseUrl, apiKey);
}

/** Field problems attached to a template render error, by data path. */
export function dataProblems(err: unknown): DataProblem[] {
  const cerr = ConnectError.from(err);
  if (cerr.code !== Code.InvalidArgument) {
    return [];
  }
  return cerr.findDetails(TemplateDataErrorSchema).flatMap((d) => d.problems);
}

/** A short human message for any error. */
export function errorMessage(err: unknown): string {
  const cerr = ConnectError.from(err);
  return cerr.rawMessage || cerr.message;
}

export function isUnauthenticated(err: unknown): boolean {
  return ConnectError.from(err).code === Code.Unauthenticated;
}

/** Saves bytes as a file download. */
export function download(bytes: Uint8Array, fileName: string, mimeType: string): void {
  const blob = new Blob([bytes as BlobPart], { type: mimeType });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = fileName;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 10_000);
}
