import { Button, EmptyState } from "@/design-system";
import { Component, type ErrorInfo, type ReactNode } from "react";

type Props = { children: ReactNode };
type State = { error: Error | null };

const isChunkLoadError = (error: Error) =>
  /Failed to fetch dynamically imported module|Importing a module script failed|Loading chunk/i.test(
    error.message,
  );

export class RouteErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Console route error", error, info.componentStack);
  }

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;

    const stale = isChunkLoadError(error);
    return (
      <EmptyState
        tone="error"
        title={stale ? "Console was updated" : "This page failed to load"}
        description={
          stale
            ? "A newer version is available. Reload to continue."
            : "Reload the page. If it keeps failing, check that Core is reachable."
        }
        action={
          <Button size="sm" variant="secondary" onClick={() => window.location.reload()}>
            Reload
          </Button>
        }
      />
    );
  }
}
