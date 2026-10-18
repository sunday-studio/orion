import { useMemo } from "react";

/** 5x7 dot-matrix glyphs, one string per row (1 = lit dot). */
const GLYPHS: Record<string, readonly string[]> = {
  A: ["01110", "10001", "10001", "11111", "10001", "10001", "10001"],
  B: ["11110", "10001", "10001", "11110", "10001", "10001", "11110"],
  C: ["01110", "10001", "10000", "10000", "10000", "10001", "01110"],
  D: ["11110", "10001", "10001", "10001", "10001", "10001", "11110"],
  E: ["11111", "10000", "10000", "11110", "10000", "10000", "11111"],
  F: ["11111", "10000", "10000", "11110", "10000", "10000", "10000"],
  G: ["01110", "10001", "10000", "10111", "10001", "10001", "01111"],
  H: ["10001", "10001", "10001", "11111", "10001", "10001", "10001"],
  I: ["01110", "00100", "00100", "00100", "00100", "00100", "01110"],
  J: ["00111", "00010", "00010", "00010", "00010", "10010", "01100"],
  K: ["10001", "10010", "10100", "11000", "10100", "10010", "10001"],
  L: ["10000", "10000", "10000", "10000", "10000", "10000", "11111"],
  M: ["10001", "11011", "10101", "10101", "10001", "10001", "10001"],
  N: ["10001", "11001", "10101", "10011", "10001", "10001", "10001"],
  O: ["01110", "10001", "10001", "10001", "10001", "10001", "01110"],
  P: ["11110", "10001", "10001", "11110", "10000", "10000", "10000"],
  Q: ["01110", "10001", "10001", "10001", "10101", "10010", "01101"],
  R: ["11110", "10001", "10001", "11110", "10100", "10010", "10001"],
  S: ["01111", "10000", "10000", "01110", "00001", "00001", "11110"],
  T: ["11111", "00100", "00100", "00100", "00100", "00100", "00100"],
  U: ["10001", "10001", "10001", "10001", "10001", "10001", "01110"],
  V: ["10001", "10001", "10001", "10001", "10001", "01010", "00100"],
  W: ["10001", "10001", "10001", "10101", "10101", "11011", "10001"],
  X: ["10001", "10001", "01010", "00100", "01010", "10001", "10001"],
  Y: ["10001", "10001", "01010", "00100", "00100", "00100", "00100"],
  Z: ["11111", "00001", "00010", "00100", "01000", "10000", "11111"],
  " ": ["00000", "00000", "00000", "00000", "00000", "00000", "00000"],
};

const PITCH = 6;
const GLYPH_COLS = 5;
const GLYPH_ROWS = 7;

/** Headline drawn as a grid of dots, each line 7 rows tall with 2 empty rows between lines. */
export const DotMatrixText = ({
  lines,
  columns,
  lit = "#80ffaa",
  dim = "rgba(128,255,170,.10)",
  fluid = false,
}: {
  lines: readonly string[];
  /** Grid width in dots; defaults to the longest line. */
  columns?: number;
  lit?: string;
  dim?: string;
  /** Fill the parent width instead of using the grid's pixel size. */
  fluid?: boolean;
}) => {
  const label = lines.join(" ");
  const grid = useMemo(() => {
    const widest = Math.max(...lines.map((line) => line.length * (GLYPH_COLS + 1)));
    const cols = columns ?? widest;
    const rows = lines.length * GLYPH_ROWS + (lines.length - 1) * 2;
    const cells: { x: number; y: number; on: boolean }[] = [];
    for (let row = 0; row < rows; row += 1) {
      const lineIndex = Math.floor(row / (GLYPH_ROWS + 2));
      const glyphRow = row - lineIndex * (GLYPH_ROWS + 2);
      const text = lines[lineIndex]?.toUpperCase() ?? "";
      for (let col = 0; col < cols; col += 1) {
        const charIndex = Math.floor(col / (GLYPH_COLS + 1));
        const glyphCol = col % (GLYPH_COLS + 1);
        const glyph = GLYPHS[text[charIndex] ?? " "] ?? GLYPHS[" "];
        const on =
          glyphRow < GLYPH_ROWS && glyphCol < GLYPH_COLS && glyph[glyphRow]?.[glyphCol] === "1";
        cells.push({ x: col * PITCH + PITCH / 2, y: row * PITCH + PITCH / 2, on });
      }
    }
    return { cells, width: cols * PITCH, height: rows * PITCH };
  }, [lines, columns]);

  return (
    <svg
      role="img"
      aria-label={label}
      width={fluid ? "100%" : grid.width}
      height={fluid ? undefined : grid.height}
      viewBox={`0 0 ${grid.width} ${grid.height}`}
      className="block max-w-full"
    >
      {grid.cells.map((cell) => (
        <circle
          key={`${cell.x}-${cell.y}`}
          cx={cell.x}
          cy={cell.y}
          r={2}
          fill={cell.on ? lit : dim}
        />
      ))}
    </svg>
  );
};
