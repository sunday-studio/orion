import { Hashvatar } from "hashvatar/react";

interface RadialAvatarProps {
  seed?: string;
  size?: number;
  className?: string;
}

export function RadialAvatar({
  seed = Math.random().toString(),
  size = 24,
  className = "",
}: RadialAvatarProps) {
  return <Hashvatar hash={seed} mode="dither" animated size={size} className={className} />;
}
