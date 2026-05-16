import { Link, type LinkProps } from "react-router-dom";

import { cn } from "@/lib/utils";

type DataTableLinkProps = LinkProps & {
  truncate?: boolean;
};

export const DataTableLink = ({ className, truncate = false, ...props }: DataTableLinkProps) => {
  return (
    <Link
      className={cn(
        "hover:text-emerald-950 hover:underline",
        truncate && "block truncate",
        className,
      )}
      {...props}
    />
  );
};
