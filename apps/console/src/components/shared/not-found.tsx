import { Link } from "react-router-dom";
import { EmptyState } from "@/components/shared/empty-state";

export const NotFound = () => (
  <EmptyState
    title="Page not found"
    description="The page you are looking for does not exist or has moved."
    action={
      <Link className="text-sm font-medium underline" to="/incidents">
        Back to incidents
      </Link>
    }
  />
);
