import { Plus } from "lucide-react";
import { Link } from "react-router-dom";
import { Button, PageHeader } from "@/design-system";
import { NodeList } from "@/features/nodes/components/node-list";
import { useGetNodeSummary } from "@/orion-sdk";
import { pluralize } from "@/utils/plural";

export const NodesPage = () => {
  const summary = useGetNodeSummary();
  const total = summary.data?.total;
  return (
    <div className="w-full pb-8">
      <PageHeader
        title="Servers"
        description={!total ? undefined : `${pluralize(total, "node")} connected to Core.`}
        actions={
          <Link to="/servers/new">
            <Button size="sm">
              <Plus /> Add server
            </Button>
          </Link>
        }
      />
      <NodeList />
    </div>
  );
};
