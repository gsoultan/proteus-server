// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import type { MessageInitShape } from "@bufbuild/protobuf";

import { type Template, type TemplateSchema } from "@/gen/proteus/v1/template_pb.js";

import { useProteus, useSettings } from "./client";

export const keys = {
  formats: (url: string, key: string) => ["formats", url, key] as const,
  templates: (category: string) => ["templates", category] as const,
  template: (id: string) => ["template", id] as const,
  starters: ["starters"] as const,
};

/** The server's formats double as the connection check. */
export function useConnection() {
  const api = useProteus();
  const { baseUrl, apiKey } = useSettings();
  return useQuery({
    queryKey: keys.formats(baseUrl, apiKey),
    queryFn: () => api.conversion.listFormats({}),
    retry: false,
    staleTime: 60_000,
  });
}

export function useTemplates(category: string) {
  const api = useProteus();
  return useQuery({
    queryKey: keys.templates(category),
    queryFn: async () => {
      const out: Template[] = [];
      let pageToken = "";
      do {
        const res = await api.templates.listTemplates({ category, pageSize: 100, pageToken });
        out.push(...res.templates);
        pageToken = res.nextPageToken;
      } while (pageToken);
      return out;
    },
  });
}

export function useTemplate(id: string) {
  const api = useProteus();
  return useQuery({
    queryKey: keys.template(id),
    queryFn: async () => (await api.templates.getTemplate({ id })).template!,
    staleTime: Infinity,
  });
}

export function useStarters() {
  const api = useProteus();
  return useQuery({
    queryKey: keys.starters,
    queryFn: async () => (await api.templates.listStarterTemplates({})).starters,
    staleTime: Infinity,
  });
}

export function useCreateTemplate() {
  const api = useProteus();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (req: { template: MessageInitShape<typeof TemplateSchema>; starterId?: string }) => {
      const res = await api.templates.createTemplate({ template: req.template, starterId: req.starterId ?? "" });
      return res.template!;
    },
    onSuccess: (t) => {
      qc.setQueryData(keys.template(t.id), t);
      void qc.invalidateQueries({ queryKey: ["templates"] });
    },
  });
}

export function useUpdateTemplate() {
  const api = useProteus();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (template: Template) => (await api.templates.updateTemplate({ template })).template!,
    onSuccess: (t) => {
      qc.setQueryData(keys.template(t.id), t);
      void qc.invalidateQueries({ queryKey: ["templates"] });
    },
  });
}

export function useDeleteTemplate() {
  const api = useProteus();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.templates.deleteTemplate({ id }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["templates"] }),
  });
}

export function useConvertToCode() {
  const api = useProteus();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (req: { id: string; revision: bigint }) => (await api.templates.convertToCode(req)).template!,
    onSuccess: (t) => {
      qc.setQueryData(keys.template(t.id), t);
      void qc.invalidateQueries({ queryKey: ["templates"] });
    },
  });
}
