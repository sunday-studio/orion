import { Search } from "lucide-react";
import { parseAsString, useQueryStates } from "nuqs";
import { useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Button,
  Callout,
  EmptyState,
  GridRow,
  GridTable,
  Input,
  PageHeader,
  Segmented,
  Select,
  Skeleton,
} from "@/design-system";
import { useListServiceSuggestions, useListServices } from "@/orion-sdk";
import { ServiceErrorRate } from "./components/service-error-rate";
import { ServiceFormDialog } from "./components/service-form-dialog";
import { SuggestionsDialog } from "./components/suggestions-dialog";
import { HealthBadge, HealthDot } from "./components/service-health";
import {
  type HealthFilter,
  environmentsOf,
  filterServices,
  healthCounts,
  healthFilterFromParam,
  plural,
  serviceHref,
} from "./services.domain";

const COLUMNS = "18px minmax(0,1.4fr) 90px 70px 100px 130px";
const HEALTH_SEGMENTS: HealthFilter[] = ["all", "down", "degraded", "up"];

export const ServicesPage = () => {
  const navigate = useNavigate();
  const query = useListServices();
  const suggestions = useListServiceSuggestions();
  const suggestionCount = suggestions.data?.suggestions?.length ?? 0;
  const [creating, setCreating] = useState(false);
  const [reviewing, setReviewing] = useState(false);
  const [{ q, health, env }, setQuery] = useQueryStates({
    q: parseAsString.withDefault(""),
    health: parseAsString.withDefault("all"),
    env: parseAsString.withDefault("all"),
  });
  const services = useMemo(() => query.data?.services ?? [], [query.data]);
  const counts = healthCounts(services);
  const environments = environmentsOf(services);
  const visible = filterServices(services, {
    query: q,
    health: healthFilterFromParam(health),
    environment: env,
  });
  const filtersActive = Boolean(q.trim()) || health !== "all" || env !== "all";
  const reset = () => void setQuery({ q: "", health: "all", env: "all" });

  return (
    <div className="w-full pb-8">
      <PageHeader
        title="Services"
        description={
          services.length > 0
            ? `${plural(services.length, "service")} grouping your checks, logs and resources.`
            : "Nothing grouped yet."
        }
        actions={
          <Button size="sm" onClick={() => setCreating(true)}>
            New service
          </Button>
        }
      />
      {services.length > 0 && suggestionCount > 0 && (
        <Callout tone="warn" className="mt-2">
          Orion found {plural(suggestionCount, "group")} you could turn into services.{" "}
          <button type="button" className="underline" onClick={() => setReviewing(true)}>
            Review suggestions
          </button>
        </Callout>
      )}
      {query.isPending ? (
        <Skeleton className="mt-3 h-40 w-full rounded-xl" />
      ) : query.isError ? (
        <EmptyState
          tone="error"
          title="Unable to load services"
          description="Core may be unreachable or need updating."
          action={
            <Button size="sm" variant="secondary" onClick={() => void query.refetch()}>
              Retry
            </Button>
          }
        />
      ) : services.length === 0 ? (
        <EmptyState
          className="mt-3 min-h-[207px] rounded-xl bg-card"
          title="No services yet"
          description={`A service puts one thing you run in one place: its checks, its logs, and its CPU and memory.${
            suggestionCount > 0
              ? ` Orion found ${plural(suggestionCount, "group")} in what you already have.`
              : ""
          }`}
          action={
            <div className="flex gap-2">
              {suggestionCount > 0 && (
                <Button size="sm" onClick={() => setReviewing(true)}>
                  Review suggestions
                </Button>
              )}
              <Button size="sm" variant="secondary" onClick={() => setCreating(true)}>
                New service
              </Button>
            </div>
          }
        />
      ) : (
        <div className="mt-3 flex flex-col gap-3">
          <div className="flex flex-wrap items-center gap-3">
            <div className="relative w-[260px]">
              <Search
                aria-hidden
                className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-faint"
              />
              <Input
                aria-label="Search services"
                placeholder="Search services"
                className="pl-9"
                value={q}
                onChange={(event) => void setQuery({ q: event.target.value })}
              />
            </div>
            <Segmented
              role="group"
              aria-label="Service health"
              options={HEALTH_SEGMENTS.map((value) => ({
                value,
                label: `${value.charAt(0).toUpperCase()}${value.slice(1)} ${counts[value]}`,
              }))}
              value={healthFilterFromParam(health)}
              onValueChange={(next) => void setQuery({ health: next })}
            />
            <span className="flex-1" />
            {environments.length > 0 && (
              <Select
                aria-label="Environment"
                label="Environment"
                className="min-w-[120px]"
                options={[
                  { value: "all", label: "Any" },
                  ...environments.map((value) => ({ value, label: value })),
                ]}
                value={env}
                onValueChange={(next) => void setQuery({ env: next })}
              />
            )}
          </div>
          {visible.length === 0 ? (
            <EmptyState
              title="No services match"
              description="Clear the search or filters to see every service."
              action={
                filtersActive ? (
                  <Button size="sm" variant="secondary" onClick={reset}>
                    Clear filters
                  </Button>
                ) : undefined
              }
            />
          ) : (
            <div className="overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
              <GridTable>
                <GridRow columns={COLUMNS} header>
                  <span />
                  <span>Name</span>
                  <span>Monitors</span>
                  <span>Apps</span>
                  <span>Errors/min</span>
                  <span>Status</span>
                </GridRow>
                {visible.map((service) => {
                  const id = service.id ?? "";
                  const open = service.open_incident_count ?? 0;
                  return (
                    <GridRow
                      key={id}
                      flipId={id}
                      columns={COLUMNS}
                      className="cursor-pointer"
                      onClick={() => navigate(serviceHref(id))}
                    >
                      <HealthDot health={service.health} />
                      <span className="flex min-w-0 items-center gap-1.5">
                        <Link
                          to={serviceHref(id)}
                          className="truncate font-medium outline-none hover:underline focus-visible:underline"
                          onClick={(event) => event.stopPropagation()}
                        >
                          {service.name ?? id}
                        </Link>
                        {service.environment && (
                          <span className="text-faint">{service.environment}</span>
                        )}
                      </span>
                      <span className="tabular-nums text-quiet">{service.monitor_count ?? 0}</span>
                      <span className="tabular-nums text-quiet">{service.app_count ?? 0}</span>
                      <span>
                        <ServiceErrorRate
                          serviceId={id}
                          appCount={service.app_count ?? 0}
                          health={service.health}
                        />
                      </span>
                      <span className="flex items-center gap-2">
                        <HealthBadge health={service.health} />
                        {open > 0 && (
                          <span className="text-xs text-(--tone-down-fg)">{open} open</span>
                        )}
                      </span>
                    </GridRow>
                  );
                })}
              </GridTable>
            </div>
          )}
        </div>
      )}
      <ServiceFormDialog
        open={creating}
        onOpenChange={setCreating}
        onCreated={(id) => navigate(serviceHref(id))}
      />
      <SuggestionsDialog open={reviewing} onOpenChange={setReviewing} />
    </div>
  );
};
