import { Plus } from "lucide-react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Button,
  EmptyState,
  GridRow,
  GridTable,
  PageHeader,
  Skeleton,
  StatusBadge,
} from "@/design-system";
import { useListStatusPages } from "@/orion-sdk";
import { CreateStatusPageDialog } from "./dialogs/create-status-page-dialog";
import { stateLabel, stateTone } from "./status-format";

const COLUMNS = "minmax(0,1.2fr) minmax(0,1.2fr) 100px 100px 100px 64px";

export const StatusPagesPage = () => {
  const navigate = useNavigate();
  const [creating, setCreating] = useState(false);
  const query = useListStatusPages();
  const pages = query.data?.pages ?? [];

  return (
    <div className="w-full pb-8">
      <PageHeader
        title="Status pages"
        description={pages.length > 0 ? "Public pages your users can check." : undefined}
        actions={
          <Button size="sm" onClick={() => setCreating(true)}>
            <Plus /> New page
          </Button>
        }
      />
      <div className="mt-3">
        {query.isPending ? (
          <Skeleton className="h-40 w-full rounded-xl" />
        ) : query.isError ? (
          <EmptyState
            tone="error"
            title="Unable to load status pages"
            action={
              <Button size="sm" variant="secondary" onClick={() => void query.refetch()}>
                Retry
              </Button>
            }
          />
        ) : pages.length === 0 ? (
          <div className="flex min-h-[207px] flex-col items-center justify-center gap-1 rounded-xl border border-hairline bg-card px-4 text-center shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
            <b className="text-[15px] font-medium">No status pages yet</b>
            <p className="text-[13px] text-mid">
              Publish a page so your users can check what is up.
            </p>
            <Button size="sm" className="mt-3" onClick={() => setCreating(true)}>
              Create your first page
            </Button>
          </div>
        ) : (
          <div className="overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
            <GridTable>
              <GridRow columns={COLUMNS} header>
                <span>Page</span>
                <span>Address</span>
                <span>State</span>
                <span>Components</span>
                <span>Subscribers</span>
                <span />
              </GridRow>
              {pages.map((page) => (
                <GridRow
                  key={page.id}
                  flipId={page.id}
                  columns={COLUMNS}
                  className="cursor-pointer"
                  onClick={() => navigate(`/status-pages/${page.id}`)}
                >
                  <b className="font-medium">{page.title}</b>
                  <span className="text-faint">{page.public_url}</span>
                  <span>
                    <StatusBadge
                      status={stateTone(page.state)}
                      shape={page.state === "draft" ? "hollow" : undefined}
                    >
                      {stateLabel(page.state)}
                    </StatusBadge>
                  </span>
                  <span className="text-faint">{page.component_count ?? 0}</span>
                  <span className="text-faint">{page.subscriber_count ?? 0}</span>
                  <span className="!overflow-visible">
                    <Button size="sm" variant="secondary">
                      Open
                    </Button>
                  </span>
                </GridRow>
              ))}
            </GridTable>
          </div>
        )}
      </div>
      <CreateStatusPageDialog open={creating} onOpenChange={setCreating} />
    </div>
  );
};
