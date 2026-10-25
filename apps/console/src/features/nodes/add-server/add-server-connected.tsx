import { Link } from "react-router-dom";
import { Button, FieldPanel, PageHeader, RaisedCard, StatusBadge } from "@/design-system";
import type { ApiEnrollmentResponse } from "@/orion-sdk";
import { clockWithSeconds } from "./enrollment";

const Row = ({
  title,
  detail,
  action,
}: {
  title: string;
  detail: string;
  action?: React.ReactNode;
}) => (
  <div className="flex items-center gap-3 border-b border-line-soft px-3.5 py-[9px] text-[13px] last:border-b-0">
    <b className="w-60 shrink-0 font-medium">{title}</b>
    <span className="min-w-0 flex-1 truncate text-xs text-faint">{detail}</span>
    {action}
  </div>
);

export const AddServerConnected = ({ enrollment }: { enrollment: ApiEnrollmentResponse }) => {
  const name = enrollment.node_name ?? "The server";
  const platform = [enrollment.node_os, enrollment.node_arch].filter(Boolean).join("/");
  return (
    <>
      <PageHeader
        title={`${name} is connected`}
        description="The Node registered and sent its first report."
      />
      <div className="mt-3 flex flex-col gap-3">
        <FieldPanel title="Server" aside="Just registered">
          <RaisedCard className="flex items-center gap-3 px-4 py-3 text-[13px]">
            <span aria-hidden className="size-2 shrink-0 rounded-full bg-[#2a9f69]" />
            <b className="font-medium">{name}</b>
            <span className="ml-auto truncate text-xs text-faint">
              {[
                platform,
                enrollment.node_version && `node ${enrollment.node_version}`,
                enrollment.first_report_at &&
                  `first report ${clockWithSeconds(enrollment.first_report_at)}`,
              ]
                .filter(Boolean)
                .join(" · ")}
            </span>
            <StatusBadge status="up">Reporting</StatusBadge>
          </RaisedCard>
        </FieldPanel>
        <FieldPanel title="Next" aside="Optional">
          <RaisedCard>
            <Row
              title="Add default monitors"
              detail="Disk, memory, CPU and the Node itself"
              action={
                <Button
                  size="sm"
                  disabled
                  title="Default monitors need a Core endpoint that does not exist yet"
                >
                  Add
                </Button>
              }
            />
            <Row
              title="Create a custom monitor"
              detail="HTTP, TCP or a process on this server"
              action={
                <Link to="/monitors?new=1">
                  <Button size="sm" variant="secondary">
                    Create
                  </Button>
                </Link>
              }
            />
          </RaisedCard>
        </FieldPanel>
        <div className="flex justify-center">
          <Link to={enrollment.node_id ? `/servers/${enrollment.node_id}` : "/servers"}>
            <Button variant="secondary">Go to server</Button>
          </Link>
        </div>
      </div>
    </>
  );
};
