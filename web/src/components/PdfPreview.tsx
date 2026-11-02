// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// The exact preview: after a pause in editing, the unsaved template is sent
// to ConvertTemplate and the PDF is drawn page by page with pdf.js, which
// parses in its own web worker.

import { Alert, Box, Button, Group, Loader, Stack, Text } from "@mantine/core";
import { useDebouncedValue } from "@mantine/hooks";
import { IconAlertTriangle, IconDownload } from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState } from "react";
import { toJsonString } from "@bufbuild/protobuf";

import { dataProblems, download, errorMessage, useProteus } from "@/api/client";
import { OutputFormat } from "@/gen/proteus/v1/conversion_pb.js";
import { TemplateSchema, type Template } from "@/gen/proteus/v1/template_pb.js";

export function PdfPreview({ template, enabled }: { template: Template; enabled: boolean }) {
  const api = useProteus();
  // Debounce on the message itself: edits produce new objects.
  const [debounced] = useDebouncedValue(template, 700);
  // Query keys are hashed with JSON.stringify, which cannot take the
  // message's bigint fields, so key on the serialized template.
  const key = useMemo(() => toJsonString(TemplateSchema, debounced), [debounced]);
  const q = useQuery({
    queryKey: ["preview", key],
    queryFn: () => api.templates.convertTemplate({ template: debounced, outputFormat: OutputFormat.PDF }),
    enabled,
    staleTime: Infinity,
    retry: false,
  });
  const stale = debounced !== template || q.isFetching;
  return (
    <Stack gap={8} style={{ height: "100%" }}>
      <Group justify="space-between" px={4}>
        <Group gap={8}>
          <Text fz={12} fw={600} c="dimmed">
            PDF preview
          </Text>
          {stale && <Loader size={12} />}
        </Group>
        {q.data && (
          <Button size="compact-xs" variant="subtle" leftSection={<IconDownload size={12} />} onClick={() => download(q.data.content, q.data.fileName || "document.pdf", q.data.mimeType)}>
            Download
          </Button>
        )}
      </Group>
      {q.error && <PreviewError err={q.error} />}
      {q.data && q.data.warnings.length > 0 && (
        <Alert color="yellow" icon={<IconAlertTriangle size={14} />} p="xs" fz={12}>
          {q.data.warnings.join(" · ")}
        </Alert>
      )}
      {q.data && <PdfPages bytes={q.data.content} />}
      {!q.data && !q.error && enabled && (
        <Text fz={12} c="dimmed" ta="center" mt="xl">
          Rendering the first preview…
        </Text>
      )}
    </Stack>
  );
}

function PreviewError({ err }: { err: unknown }) {
  const problems = dataProblems(err);
  return (
    <Alert color="red" icon={<IconAlertTriangle size={14} />} p="xs" title={problems.length ? "The sample data does not fit the fields" : "Cannot render"} fz={12}>
      {problems.length ? (
        <ul style={{ margin: 0, paddingLeft: 16 }}>
          {problems.map((p, i) => (
            <li key={i}>
              <code>{p.path}</code> {p.message}
            </li>
          ))}
        </ul>
      ) : (
        errorMessage(err)
      )}
    </Alert>
  );
}

type PdfJs = typeof import("pdfjs-dist");
let pdfjsPromise: Promise<PdfJs> | undefined;

function loadPdfJs(): Promise<PdfJs> {
  if (!pdfjsPromise) {
    pdfjsPromise = import("pdfjs-dist").then((m) => {
      m.GlobalWorkerOptions.workerSrc = new URL("pdfjs-dist/build/pdf.worker.min.mjs", import.meta.url).toString();
      return m;
    });
  }
  return pdfjsPromise;
}

export function PdfPages({ bytes, scale = 1.1 }: { bytes: Uint8Array; scale?: number }) {
  const [pages, setPages] = useState<string[]>([]);
  const [error, setError] = useState<string>();
  const gen = useRef(0);
  useEffect(() => {
    const my = ++gen.current;
    let cancelled = false;
    (async () => {
      try {
        const pdfjs = await loadPdfJs();
        const task = pdfjs.getDocument({ data: bytes.slice() });
        const doc = await task.promise;
        const urls: string[] = [];
        for (let i = 1; i <= doc.numPages; i++) {
          if (cancelled) return;
          const page = await doc.getPage(i);
          const viewport = page.getViewport({ scale: scale * (window.devicePixelRatio || 1) });
          const canvas = document.createElement("canvas");
          canvas.width = viewport.width;
          canvas.height = viewport.height;
          const ctx = canvas.getContext("2d")!;
          await page.render({ canvas, canvasContext: ctx, viewport }).promise;
          urls.push(canvas.toDataURL("image/png"));
        }
        if (!cancelled && gen.current === my) {
          setPages(urls);
          setError(undefined);
        }
        await task.destroy();
      } catch (e) {
        if (!cancelled) setError(String(e));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [bytes, scale]);
  if (error) {
    return (
      <Alert color="red" p="xs" fz={12}>
        {error}
      </Alert>
    );
  }
  return (
    <Box style={{ overflow: "auto", flex: 1 }}>
      <Stack gap={12} align="center" py={4}>
        {pages.map((src, i) => (
          <img key={i} src={src} alt={`Page ${i + 1}`} style={{ width: "100%", maxWidth: 595 * scale, boxShadow: "0 2px 10px rgba(0,0,0,0.15)", background: "#fff" }} />
        ))}
        {pages.length > 0 && (
          <Text fz={11} c="dimmed">
            {pages.length} page{pages.length === 1 ? "" : "s"}
          </Text>
        )}
      </Stack>
    </Box>
  );
}
