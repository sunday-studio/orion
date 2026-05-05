import * as React from "react";

import { cn } from "../../lib/utils";
import { Button } from "./button";
import { Input } from "./input";

type InputWithButtonProps = Omit<
  React.ComponentProps<typeof Input>,
  "className"
> & {
  buttonLabel: React.ReactNode;
  buttonAriaLabel?: string;
  buttonType?: React.ComponentProps<typeof Button>["type"];
  onButtonClick?: React.ComponentProps<typeof Button>["onClick"];
  className?: string;
  inputClassName?: string;
  buttonClassName?: string;
};

function InputWithButton({
  buttonLabel,
  buttonAriaLabel,
  buttonType = "button",
  onButtonClick,
  className,
  inputClassName,
  buttonClassName,
  ...inputProps
}: InputWithButtonProps) {
  return (
    <div
      className={cn(
        "p-1 ring-input ring-neutral-200 bg-white focus-within:border-ring focus-within:ring-ring/50 flex h-10 min-w-64 items-center overflow-hidden rounded-full border border-neutral-300 shadow-xs transition-[color,box-shadow] focus-within:ring-[3px]",
        className,
      )}
    >
      <Input
        className={cn(
          "h-full flex-1 border-0 shadow-none focus-visible:ring-0",
          inputClassName,
        )}
        {...inputProps}
      />
      <Button
        type={buttonType}
        aria-label={buttonAriaLabel}
        onClick={onButtonClick}
        className={cn("h-full border-y-0 border-r-0 px-3 ", buttonClassName)}
      >
        {buttonLabel}
      </Button>
    </div>
  );
}

export { InputWithButton };
