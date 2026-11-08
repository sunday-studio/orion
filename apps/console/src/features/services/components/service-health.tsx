import { Dot, StatusBadge } from "@/design-system";
import { type ServiceHealth, healthLabel, serviceHealth } from "../services.domain";

const COLOR: Record<ServiceHealth, string> = {
  up: "#2a9f69",
  degraded: "#f59e0b",
  down: "#e5484d",
  unknown: "#a1a1aa",
};

export const HealthDot = ({ health }: { health?: string }) => (
  <Dot color={COLOR[serviceHealth(health)]} />
);

export const HealthBadge = ({ health }: { health?: string }) => {
  const value = serviceHealth(health);
  return <StatusBadge status={value}>{healthLabel(value)}</StatusBadge>;
};
