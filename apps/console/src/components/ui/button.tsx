import type * as React from "react";
import { Button as DsButton, buttonStyles } from "@/design-system";

/**
 * Legacy shadcn-style API kept so existing screens keep compiling; renders the design-system
 * button. New code should import from "@/design-system".
 */
type LegacyVariant = "default" | "destructive" | "outline" | "secondary" | "ghost" | "link";
type LegacySize = "default" | "sm" | "lg" | "icon";

const variantMap = {
  default: "primary",
  destructive: "destructive",
  outline: "secondary",
  secondary: "secondary",
  ghost: "ghost",
  link: "link",
} as const;

const sizeMap = { default: "md", sm: "sm", lg: "md", icon: "icon" } as const;

type ButtonProps = Omit<React.ComponentProps<"button">, "color"> & {
  variant?: LegacyVariant | null;
  size?: LegacySize | null;
};

function Button({ variant, size, ...props }: ButtonProps) {
  return (
    <DsButton
      variant={variantMap[variant ?? "default"]}
      size={sizeMap[size ?? "default"]}
      {...props}
    />
  );
}

function buttonVariants({
  variant,
  size,
}: { variant?: LegacyVariant | null; size?: LegacySize | null } = {}) {
  return buttonStyles({
    variant: variantMap[variant ?? "default"],
    size: sizeMap[size ?? "default"],
  });
}

export { Button, buttonVariants };
