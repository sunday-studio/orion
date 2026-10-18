import { Button, Callout, FieldPanel, PageHeader, RaisedCard, StepRows } from "@/design-system";

export const AddServerFailed = ({
  host,
  onStartOver,
  onCopy,
}: {
  host: string;
  onStartOver: () => void;
  onCopy: () => Promise<void>;
}) => (
  <>
    <PageHeader
      title="We have not seen the server yet"
      description="No agent registered in 5 minutes."
      actions={
        <Button size="sm" variant="secondary" onClick={onStartOver}>
          Start over
        </Button>
      }
    />
    <div className="mt-3 flex flex-col gap-3">
      <Callout tone="down" lead="Not connected.">
        The install command may not have finished, or the server cannot reach Core.
      </Callout>
      <FieldPanel title="Check these" aside="In order">
        <RaisedCard>
          <StepRows
            rows={[
              {
                tone: "idle",
                title: "Did the command finish without errors?",
                detail: "Re-run it and read the last lines",
              },
              {
                tone: "idle",
                title: `Can the server reach Core on port 443?`,
                detail: `curl -I https://${host}/healthz`,
              },
              {
                tone: "idle",
                title: "Is the registration token correct?",
                detail: "Settings, Security, Agent registration",
              },
              {
                tone: "idle",
                title: "Is the agent running?",
                detail: "systemctl status orion-agent",
              },
            ]}
          />
        </RaisedCard>
      </FieldPanel>
      <div className="flex justify-center gap-2">
        <Button onClick={onStartOver}>Check again</Button>
        <Button variant="secondary" onClick={() => void onCopy()}>
          Copy command
        </Button>
      </div>
    </div>
  </>
);
