type SparklineProps = {
  /** Oldest first. Fewer than two points renders nothing. */
  values: readonly number[];
  color?: string;
  width?: number;
  height?: number;
  className?: string;
};

/** Tiny trend line (canvas `.spk`): 1.6px stroke, no axes. */
export const Sparkline = ({
  values,
  color = "currentColor",
  width = 80,
  height = 16,
  className,
}: SparklineProps) => {
  if (values.length < 2) return null;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  const step = (width - 2) / (values.length - 1);
  const path = values
    .map((value, index) => {
      const x = 1 + index * step;
      const y = 1 + (height - 2) * (1 - (value - min) / range);
      return `${index === 0 ? "M" : "L"}${x.toFixed(1)} ${y.toFixed(1)}`;
    })
    .join(" ");
  return (
    <svg
      aria-hidden
      className={className}
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
    >
      <path d={path} fill="none" stroke={color} strokeWidth="1.6" strokeLinejoin="round" />
    </svg>
  );
};
