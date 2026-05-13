import { useMemo } from "react";

interface RadialAvatarProps {
  seed?: string;
  size?: "xs" | "sm" | "md" | "lg" | "xl";
  className?: string;
}

const sizeMap = {
  xs: "size-6",
  sm: "size-8",
  md: "size-12",
  lg: "size-18",
  xl: "size-24",
};

const colorCombinations = [
  { from: "from-pink-400", via: "via-fuchsia-500", to: "to-purple-700" },
  { from: "from-rose-300", via: "via-pink-500", to: "to-fuchsia-800" },
  { from: "from-fuchsia-400", via: "via-purple-500", to: "to-indigo-700" },

  { from: "from-sky-200", via: "via-blue-400", to: "to-indigo-900" },
  { from: "from-cyan-300", via: "via-blue-500", to: "to-blue-800" },
  { from: "from-teal-300", via: "via-cyan-500", to: "to-blue-700" },

  { from: "from-amber-300", via: "via-orange-500", to: "to-red-700" },
  { from: "from-yellow-300", via: "via-amber-500", to: "to-orange-700" },
  { from: "from-orange-300", via: "via-red-500", to: "to-rose-800" },

  { from: "from-emerald-300", via: "via-green-500", to: "to-teal-800" },
  { from: "from-lime-300", via: "via-green-500", to: "to-emerald-700" },
  { from: "from-green-300", via: "via-emerald-500", to: "to-cyan-700" },

  { from: "from-white", via: "via-gray-400", to: "to-zinc-900" },
  { from: "from-slate-200", via: "via-slate-500", to: "to-slate-900" },

  { from: "from-violet-300", via: "via-purple-500", to: "to-pink-700" },
  { from: "from-indigo-300", via: "via-blue-500", to: "to-cyan-700" },
];

const radialPositions = [
  "bg-radial",
  "bg-radial-[at_50%_75%]",
  "bg-radial-[at_25%_25%]",
  "bg-radial-[at_75%_25%]",
  "bg-radial-[at_50%_25%]",
  "bg-radial-[at_25%_50%]",
];

function hashString(str: string): number {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i);
    hash = (hash << 5) - hash + char;
    hash = hash & hash;
  }
  return Math.abs(hash);
}

export function RadialAvatar({
  seed = Math.random().toString(),
  size = "lg",
  className = "",
}: RadialAvatarProps) {
  const gradientClasses = useMemo(() => {
    const hash = hashString(seed);

    // Pick colors based on seed
    const colorIndex = hash % colorCombinations.length;
    const colors = colorCombinations[colorIndex];

    // Pick position based on seed
    const positionIndex = Math.floor(hash / colorCombinations.length) % radialPositions.length;
    const position = radialPositions[positionIndex];

    // Sometimes add from/to percentages for variety
    const usePercentages = hash % 3 === 0;
    const fromPercent = usePercentages ? "from-40%" : "";
    const toPercent = usePercentages ? "to-90%" : "";

    return `${position} ${colors.from} ${fromPercent} ${colors.via} ${colors.to} ${toPercent}`;
  }, [seed]);

  const sizeClass = sizeMap[size];

  return (
    <div
      className={`${sizeClass} rounded-full ${gradientClasses} ${className}`}
      aria-hidden="true"
    />
  );
}
