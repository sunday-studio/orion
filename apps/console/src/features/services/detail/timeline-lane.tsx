import { TIME_FORMAT, formatDate } from "@/utils/date";
import { type Line, hasData, linePath, niceMax } from "../timeline-model";

export const W = 900;
export const LEFT = 40;
export const LANE_H = 64;
export const GAP = 26;
// Theme tokens, so the lines stay readable on the dark card. The violet has no token and works on both.
export const LANE_COLORS = [
  "var(--color-ink)",
  "var(--color-dot-up)",
  "#8b8bf5",
  "var(--color-dot-warn)",
  "var(--color-dot-idle)",
];

export type LaneProps = {
  title: string;
  top: number;
  lines: Line[];
  bars?: boolean;
  format: (value: number) => string;
  from: number;
  to: number;
  /** What to say when this lane has nothing to draw. */
  empty: string;
  max?: number;
  /** Times to mark in red, for example checks that failed. */
  marks?: number[];
  /** Drawn dashed against its own scale on the right, for a second unit in the same lane. */
  secondary?: Line[];
  secondaryFormat?: (value: number) => string;
  /** Colour of bars. Red by default, which suits errors; counters pass a neutral one. */
  barColor?: string;
  /** Leave the line names out of the heading when a legend beside the chart names them. */
  hideLabels?: boolean;
};

export const Lane = ({
  title,
  top,
  lines,
  bars,
  format,
  from,
  to,
  empty,
  max,
  marks = [],
  secondary = [],
  secondaryFormat = String,
  barColor = "var(--color-dot-down)",
  hideLabels = false,
}: LaneProps) => {
  const x = (t: number) => LEFT + ((t - from) / Math.max(to - from, 1)) * (W - LEFT);
  const peak = niceMax(max ?? Math.max(0, ...lines.flatMap((l) => l.points.map((p) => p.v))));
  const y = (v: number) => top + LANE_H - (Math.min(v, peak) / peak) * LANE_H;
  const peak2 = niceMax(Math.max(0, ...secondary.flatMap((l) => l.points.map((p) => p.v))));
  const y2 = (v: number) => top + LANE_H - (Math.min(v, peak2) / peak2) * LANE_H;
  const drawn = [...lines, ...secondary];
  const labels =
    !bars && !hideLabels && drawn.length > 0
      ? [...new Set(drawn.map((l) => l.label))].join(", ")
      : "";
  const heading = [title, labels].filter(Boolean).join(" · ");
  return (
    <g>
      {heading && (
        <text x={LEFT} y={top - 8} className="fill-quiet" fontSize="11">
          {heading}
        </text>
      )}
      <line x1={LEFT} x2={W} y1={top + LANE_H} y2={top + LANE_H} className="stroke-hairline" />
      {marks.map((t) => (
        <rect
          key={t}
          x={x(t) - 1.5}
          y={top + LANE_H - 12}
          width={3}
          height={12}
          fill="var(--color-dot-down)"
        >
          <title>{`${formatDate(new Date(t).toISOString(), TIME_FORMAT)}: checks failed`}</title>
        </rect>
      ))}
      {hasData(drawn) ? (
        <>
          <line
            x1={LEFT}
            x2={W}
            y1={top}
            y2={top}
            className="stroke-hairline"
            strokeDasharray="3 4"
          />
          <text x={LEFT - 6} y={top + 4} textAnchor="end" className="fill-faint" fontSize="10">
            {format(peak)}
          </text>
          <text
            x={LEFT - 6}
            y={top + LANE_H + 3}
            textAnchor="end"
            className="fill-faint"
            fontSize="10"
          >
            0
          </text>
          {secondary.length > 0 && (
            <text x={W - 4} y={top + 4} textAnchor="end" className="fill-faint" fontSize="10">
              {secondaryFormat(peak2)}
            </text>
          )}
          {secondary.map((line, index) => (
            <path
              key={line.id}
              d={linePath(line.points, x, y2)}
              fill="none"
              stroke={LANE_COLORS[(index + lines.length) % LANE_COLORS.length]}
              strokeWidth="2"
              strokeLinejoin="round"
              strokeDasharray="5 3"
            />
          ))}
          {lines.map((line, index) =>
            bars ? (
              <g key={line.id} fill={barColor} opacity=".85">
                {line.points.map((p) => (
                  <rect
                    key={p.t}
                    x={x(p.t) - 2}
                    y={y(p.v)}
                    width={4}
                    height={Math.max(top + LANE_H - y(p.v), p.v > 0 ? 1 : 0)}
                  >
                    <title>{`${formatDate(new Date(p.t).toISOString(), TIME_FORMAT)}: ${format(p.v)}`}</title>
                  </rect>
                ))}
              </g>
            ) : (
              <g key={line.id}>
                {line.points.length === 1 && (
                  <circle
                    cx={x(line.points[0]?.t ?? from)}
                    cy={y(line.points[0]?.v ?? 0)}
                    r={3}
                    fill={LANE_COLORS[index % LANE_COLORS.length]}
                  />
                )}
                <path
                  d={linePath(line.points, x, y)}
                  fill="none"
                  stroke={LANE_COLORS[index % LANE_COLORS.length]}
                  strokeWidth="2"
                  strokeLinejoin="round"
                />
              </g>
            ),
          )}
        </>
      ) : (
        <text x={LEFT + 8} y={top + LANE_H / 2 + 4} className="fill-faint" fontSize="12">
          {empty}
        </text>
      )}
    </g>
  );
};
