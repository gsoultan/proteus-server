// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

import { Box, Code, SimpleGrid, Stack, Text, Title, UnstyledButton } from "@mantine/core";

import type { FieldSpec } from "@/gen/proteus/v1/template_pb.js";
import { KINDS, TYPE_NAMES, flattenFields } from "@/model/layout";
import { useBuilder } from "@/store/builder";

export function Palette({ fields }: { fields: FieldSpec[] }) {
  const addBlock = useBuilder((s) => s.addBlock);
  const addFieldBlock = useBuilder((s) => s.addFieldBlock);
  const flat = flattenFields(fields);
  return (
    <Stack gap={14} p="12px" component="aside" aria-label="Blocks and fields" style={{ height: "100%", overflowY: "auto" }}>
      <Box>
        <Title order={2} fz={12} fw={700} c="dimmed" tt="uppercase" style={{ letterSpacing: "0.04em" }} mb={8}>
          Blocks
        </Title>
        <SimpleGrid cols={2} spacing={6}>
          {KINDS.map((k) => (
            <UnstyledButton
              key={k.kind}
              onClick={() => addBlock(k.kind)}
              title={`Add ${k.name.toLowerCase()} after the selected block`}
              style={{ minHeight: 44, padding: "6px 8px", borderRadius: 8, border: "1px solid var(--mantine-color-gray-2)", background: "var(--mantine-color-gray-0)", fontSize: 12.5, display: "flex", flexDirection: "column", gap: 1 }}
            >
              <Text fz={12.5} fw={600} lh={1.2}>
                {k.name}
              </Text>
              <Text fz={11} c="dimmed" lh={1.2}>
                {k.hint}
              </Text>
            </UnstyledButton>
          ))}
        </SimpleGrid>
      </Box>
      <Box>
        <Title order={2} fz={12} fw={700} c="dimmed" tt="uppercase" style={{ letterSpacing: "0.04em" }} mb={8}>
          Data fields
        </Title>
        <Stack gap={2}>
          {flat.length === 0 && (
            <Text fz={12} c="dimmed">
              No fields yet. Add them in the Data tab or infer them from the sample data.
            </Text>
          )}
          {flat.map((f) => {
            const insertable = f.depth === 0 && f.type !== 8 && f.type !== 9; // not list, not object
            const path = f.path.replace(/\[\]$/, "");
            return (
              <UnstyledButton
                key={f.path}
                disabled={!insertable}
                onClick={() => insertable && addFieldBlock(path)}
                title={insertable ? `Insert ${path} as a field block` : undefined}
                style={{ display: "flex", alignItems: "center", gap: 6, minHeight: 28, padding: "0 6px", borderRadius: 4, fontSize: 12.5, cursor: insertable ? "pointer" : "default" }}
              >
                <Code fz={12} bg="transparent" style={{ flex: 1, paddingLeft: f.depth * 10, whiteSpace: "pre" }}>
                  {f.path}
                </Code>
                <Text fz={11} c="dimmed">
                  {TYPE_NAMES[f.type]}
                </Text>
              </UnstyledButton>
            );
          })}
        </Stack>
        <Text fz={12} c="dimmed" mt={8} lh={1.5}>
          Click a field to insert it after the selected block. In any text, type <Code fz={11}>{"{{.path}}"}</Code> to show a value.
        </Text>
      </Box>
    </Stack>
  );
}
