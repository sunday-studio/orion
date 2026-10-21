import { FieldPanel, RaisedCard, StepRows } from "@/design-system";
import type { ApiEnrollmentResponse } from "@/orion-sdk";

/**
 * The installer's steps are a static list (D-028): Core never sees the download or doctor
 * results, so only the last row is live.
 */
export const AddServerProgress = ({ enrollment }: { enrollment: ApiEnrollmentResponse }) => {
  const name = enrollment.node_name;
  return (
    <FieldPanel title="The command will:" aside="Static list">
      <RaisedCard>
        <StepRows
          rows={[
            { tone: "idle", title: "Download and verify the Node" },
            { tone: "idle", title: "Register this server with Core" },
            { tone: "idle", title: "Run orion-node doctor" },
            {
              tone: "pending",
              title: name ? `Waiting for ${name} to report` : "Waiting for the server",
              detail: "Usually within 30 seconds",
            },
          ]}
        />
      </RaisedCard>
    </FieldPanel>
  );
};
