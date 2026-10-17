import { FieldPanel, RaisedCard, StepRows } from "@/design-system";
import type { ApiEnrollmentResponse } from "@/orion-sdk";
import { clockWithSeconds } from "./enrollment";

/**
 * Live progress from the enrollment. Core sees the token being accepted and the agent
 * registering; the installer's download and doctor steps never reach Core, so they are not listed.
 */
export const AddServerProgress = ({ enrollment }: { enrollment: ApiEnrollmentResponse }) => {
  const registered = Boolean(enrollment.agent_id);
  const platform = [enrollment.agent_os, enrollment.agent_arch].filter(Boolean).join("/");
  return (
    <FieldPanel title="Progress" aside="Live from Core">
      <RaisedCard>
        <StepRows
          rows={[
            {
              tone: "done",
              title: "Command started",
              detail: `Token accepted, ${clockWithSeconds(enrollment.claimed_at)}`,
            },
            registered
              ? {
                  tone: "done",
                  title: `Registered as ${enrollment.agent_name ?? "the new server"}`,
                  detail: [
                    platform,
                    enrollment.agent_version && `agent ${enrollment.agent_version}`,
                  ]
                    .filter(Boolean)
                    .join(" · "),
                }
              : {
                  tone: "pending",
                  title: "Registering the agent",
                  detail: "Usually a few seconds",
                },
            {
              tone: registered ? "pending" : "idle",
              title: "Waiting for the first report",
              detail: "Usually within 30 seconds",
            },
          ]}
        />
      </RaisedCard>
    </FieldPanel>
  );
};
