import { type FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Eye, EyeOff, LoaderCircle, ShieldCheck } from "lucide-react";

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
    <main className="min-h-[calc(100vh-4rem)] bg-background text-foreground">
      <div className="mx-auto grid min-h-[calc(100vh-4rem)] w-full max-w-6xl items-center gap-10 px-4 py-10 md:grid-cols-[1fr_420px]">
        <section className="hidden max-w-2xl md:block">
          <div className="mb-5 flex size-12 items-center justify-center rounded-lg border border-border bg-card shadow-xs">
            <ShieldCheck className="size-6 text-primary" aria-hidden="true" />
          </div>
          <h1 className="mb-4 text-4xl font-semibold tracking-normal text-foreground">
            Orion Console
          </h1>
          <p className="max-w-xl text-base leading-7 text-muted-foreground">
            Monitor agents, inspect service health, and keep operational state close to the systems
            that report it.
          </p>
        </section>

        <section className="w-full rounded-lg border border-border bg-card p-6 text-card-foreground shadow-sm">
          <div className="mb-6">
            <div className="mb-3 flex size-10 items-center justify-center rounded-md border border-border md:hidden">
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
        </section>
      </div>
    </main>
  );
}
