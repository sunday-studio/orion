import { type FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Activity, Eye, EyeOff, LoaderCircle, LockKeyhole, Server, ShieldCheck } from "lucide-react";

import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { InputWithButton } from "../components/ui/input-with-button";
import { authLogin } from "../lib/custom-instance";

export function LoginPage() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      await authLogin(username, password);
      navigate("/", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="min-h-screen bg-background text-foreground">
      <div className="grid min-h-screen w-full lg:grid-cols-[minmax(0,1fr)_440px]">
        <section className="hidden border-r border-border bg-muted/30 px-10 py-8 lg:flex lg:flex-col">
          <div className="flex items-center gap-3">
            <div className="flex size-10 items-center justify-center rounded-md border border-border bg-background shadow-xs">
              <ShieldCheck className="size-5 text-primary" aria-hidden="true" />
            </div>
            <span className="text-sm font-semibold tracking-normal">Orion Console</span>
          </div>

          <div className="flex flex-1 items-center">
            <div className="max-w-xl">
              <p className="mb-4 text-sm font-medium text-muted-foreground">Self-hosted monitoring</p>
              <h1 className="mb-5 text-5xl font-semibold leading-tight tracking-normal text-foreground">
                Keep every agent and service in sight.
              </h1>
              <p className="max-w-lg text-base leading-7 text-muted-foreground">
                Sign in to inspect reports, monitor health, and review incident candidates from a
                single operational console.
              </p>

              <div className="mt-8 grid max-w-lg gap-3 sm:grid-cols-3">
                <div className="rounded-lg border border-border bg-background p-4 shadow-xs">
                  <Server className="mb-3 size-5 text-primary" aria-hidden="true" />
                  <p className="text-sm font-medium">Agents</p>
                  <p className="mt-1 text-xs leading-5 text-muted-foreground">Registered hosts and system reports</p>
                </div>
                <div className="rounded-lg border border-border bg-background p-4 shadow-xs">
                  <Activity className="mb-3 size-5 text-primary" aria-hidden="true" />
                  <p className="text-sm font-medium">Health</p>
                  <p className="mt-1 text-xs leading-5 text-muted-foreground">Monitor state and uptime history</p>
                </div>
                <div className="rounded-lg border border-border bg-background p-4 shadow-xs">
                  <LockKeyhole className="mb-3 size-5 text-primary" aria-hidden="true" />
                  <p className="text-sm font-medium">Access</p>
                  <p className="mt-1 text-xs leading-5 text-muted-foreground">Admin console credentials</p>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section className="flex min-h-screen items-center justify-center px-4 py-10">
          <div className="w-full max-w-sm rounded-lg border border-border bg-card p-6 text-card-foreground shadow-sm">
          <div className="mb-6">
            <div className="mb-3 flex size-10 items-center justify-center rounded-md border border-border lg:hidden">
              <ShieldCheck className="size-5" aria-hidden="true" />
            </div>
            <h2 className="text-2xl font-semibold tracking-normal">Sign in</h2>
            <p className="mt-2 text-sm text-muted-foreground">
              Use your Orion administrator credentials.
            </p>
          </div>

          <form className="space-y-4" onSubmit={handleSubmit}>
            <div className="space-y-2">
              <label className="text-sm font-medium" htmlFor="username">
                Username
              </label>
              <Input
                id="username"
                type="text"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                required
                autoComplete="username"
                placeholder="admin"
              />
            </div>

            <div className="space-y-2">
              <label className="text-sm font-medium" htmlFor="password">
                Password
              </label>
              <InputWithButton
                id="password"
                type={showPassword ? "text" : "password"}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                required
                autoComplete="current-password"
                placeholder="Password"
                buttonAriaLabel={showPassword ? "Hide password" : "Show password"}
                buttonLabel={
                  showPassword ? (
                    <EyeOff className="size-4" aria-hidden="true" />
                  ) : (
                    <Eye className="size-4" aria-hidden="true" />
                  )
                }
                onButtonClick={() => setShowPassword((value) => !value)}
                className="min-w-0"
                buttonClassName="border-l border-border bg-secondary text-secondary-foreground hover:bg-secondary/80"
              />
            </div>

            {error && (
              <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {error}
              </p>
            )}

            <Button className="w-full" type="submit" disabled={loading}>
              {loading && <LoaderCircle className="size-4 animate-spin" aria-hidden="true" />}
              {loading ? "Signing in..." : "Sign in"}
            </Button>
          </form>
          </div>
        </section>
      </div>
    </main>
  );
}
