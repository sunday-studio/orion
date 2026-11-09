const FILL: Record<string, string> = {
  up: "fill-[#2a9f69]",
  degraded: "fill-[#f59e0b]",
  down: "fill-[#e5484d]",
};

const BAR_WIDTH = 3;
const BAR_GAP = 1;
const HEIGHT = 16;
const MAX_BARS = 24;

/**
 * "Last 24 checks": one thin bar per check, oldest left (canvas `.ub`).
 *
 * Drawn as one SVG, not one element per bar. Separate 3px elements are each snapped to whole device
 * pixels, so at fractional zoom or display scaling some bars come out wider and some gaps wider. In an
 * SVG every bar shares one coordinate system and is anti-aliased the same way, so widths and spacing
 * stay identical at any scale.
 */
export const CheckStrip = ({ checks }: { checks?: readonly string[] | null }) => {
  if (!checks || checks.length === 0) return <span className="text-faint">-</span>;
  const bars = checks.slice(-MAX_BARS);
  const width = bars.length * (BAR_WIDTH + BAR_GAP) - BAR_GAP;
  return (
    <svg
      aria-hidden
      width={width}
      height={HEIGHT}
      viewBox={`0 0 ${width} ${HEIGHT}`}
      className="block shrink-0"
    >
      {bars.map((check, index) => (
        <rect
          key={index}
          x={index * (BAR_WIDTH + BAR_GAP)}
          width={BAR_WIDTH}
          height={HEIGHT}
          rx={1}
          className={FILL[check] ?? "fill-[#d4d4d8]"}
        >
          <title>{check}</title>
        </rect>
      ))}
    </svg>
  );
};
