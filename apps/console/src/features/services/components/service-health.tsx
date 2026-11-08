import { Dot, StatusBadge } from "@/design-system";
import { type ServiceHealth, healthLabel, serviceHealth } from "../services.domain";

const COLOR: Record<ServiceHealth, string> = {
  up: "var(--color-dot-up)",
  degraded: "var(--color-dot-warn)",
  down: "var(--color-dot-down)",
  unknown: "var(--color-dot-idle)",
};

export const HealthDot = ({ health }: { health?: string }) => (
  <Dot color={COLOR[serviceHealth(health)]} />
);

export const HealthBadge = ({ health }: { health?: string }) => {
  const value = serviceHealth(health);
  return <StatusBadge status={value}>{healthLabel(value)}</StatusBadge>;
};
