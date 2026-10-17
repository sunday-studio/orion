import { FieldPanel, RaisedCard, StepRows } from "@/design-system";
import type { ApiAgentResponse } from "@/orion-sdk";

/**
 * Live progress for a server that has registered but not reported yet.
 * Installer steps before registration (token accepted, download verified) need the enrollment
 * progress API; only what Core can report today is listed.
 */
export const AddServerProgress = ({ agent }: { agent: ApiAgentResponse }) => (
  <FieldPanel title="Progress" aside="Live from the installer">
    <RaisedCard>
      <StepRows
        rows={[
          {
            tone: "done",
            title: `Registered as ${agent.name ?? "the new server"}`,
            detail: [agent.os, agent.arch].filter(Boolean).join("/") || undefined,
          },
          {
            tone: "pending",
            title: "Waiting for the first report",
            detail: "Usually within 30 seconds",
          },
        ]}
      />
    </RaisedCard>
  </FieldPanel>
);
