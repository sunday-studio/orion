-- A Service is one named thing that ties monitors, an app's logs and measured resources together,
-- so an incident can show all three. See docs/plans/service-correlation-plan.md.
--
-- Each of apps, monitors and node_app_resources links to at most one service through a nullable
-- service_id (a join table is deferred until a monitor must serve several services). There is no
-- foreign key: the link is cleared by Core when a service is deleted, and a stale value can never
-- block a delete, a node report or a monitor change. Unlinking only clears service_id; logs,
-- monitor reports and incidents are keyed by their own ids and are never touched.
CREATE TABLE IF NOT EXISTS services (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  slug TEXT NOT NULL,
  environment TEXT NOT NULL DEFAULT '',
  node_id TEXT NOT NULL DEFAULT '',
  runbook_url TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_services_slug ON services(slug);

ALTER TABLE apps ADD COLUMN service_id TEXT;
ALTER TABLE monitors ADD COLUMN service_id TEXT;
ALTER TABLE node_app_resources ADD COLUMN service_id TEXT;

CREATE INDEX IF NOT EXISTS idx_apps_service_id ON apps(service_id);
CREATE INDEX IF NOT EXISTS idx_monitors_service_id ON monitors(service_id);
CREATE INDEX IF NOT EXISTS idx_node_app_resources_service_id ON node_app_resources(service_id);
