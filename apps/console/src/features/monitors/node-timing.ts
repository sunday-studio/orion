/** Check frequencies and failure counts offered for Node monitors (Core allows 10 s to 24 h, 1 to 20 failures). */
export const INTERVALS = [30, 60, 300, 900, 1800, 3600].map((value) => ({
  value: String(value),
  label:
    value >= 3600 ? "Every hour" : value >= 60 ? `Every ${value / 60} min` : `Every ${value} s`,
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

export const everyLabel = (value: number) =>
  value >= 3600
    ? `Every ${value / 3600} h`
    : value >= 60
      ? `Every ${Math.round(value / 60)} min`
      : `Every ${value} s`;
