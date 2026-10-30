/** Check frequencies and failure counts offered for Node monitors (Core allows 10 s to 24 h, 1 to 20 failures). */
export const INTERVALS = [
  30, 60, 120, 300, 600, 900, 1800, 3600, 7200, 14400, 21600, 43200, 86400,
].map((value) => ({
  value: String(value),
  label: everyLabel(value),
}));

export const FAIL_AFTER = [1, 2, 3, 5].map((value) => ({
  value: String(value),
  label: value === 1 ? "1 failed check" : `${value} failed checks`,
}));

/** Keeps a saved value that is not one of the presets selectable. */
export const withCurrent = (
  options: { value: string; label: string }[],
  current: string | number | undefined,
  label: (value: number) => string,
) =>
  current === undefined || options.some((o) => o.value === String(current))
    ? options
    : [...options, { value: String(current), label: label(Number(current)) }].sort(
        (a, b) => Number(a.value) - Number(b.value),
      );

export function everyLabel(value: number) {
  if (value === 3600) return "Every hour";
  if (value >= 3600) return `Every ${Number((value / 3600).toFixed(1))} hours`;
  if (value >= 60) return `Every ${Math.round(value / 60)} min`;
  return `Every ${value} s`;
}
