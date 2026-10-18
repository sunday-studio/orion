import { type FormEvent, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { authLogin } from "@/api/client";
import { Button, Callout, DotMatrixText, Input } from "@/design-system";
import "./login.css";

const HEADLINE = ["Know before", "your users do"] as const;

const CAPABILITIES = [
  "Know the moment a server, site or job stops responding",
  "Get told by webhook or email, and see every delivery",
  "Publish a status page and ship logs from your apps",
] as const;

const safeReturnPath = (from: string | null) =>
  from && /^\/(?![/\\])/.test(from) && !from.startsWith("/login") ? from : "/";

/** Sign in: dark hero on the left, credentials form on the right (canvas "login"). */
export function LoginPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const sessionExpired = searchParams.get("session") === "expired";

  const submitLogin = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError("");
    setIsSubmitting(true);
    try {
      await authLogin(username, password);
      navigate(safeReturnPath(searchParams.get("from")), { replace: true });
    } catch (err) {
      const status = (err as { status?: number }).status;
      setError(
        status === 401 || status === 400
          ? "Username or password is incorrect. Check them and try again."
          : err instanceof Error
            ? err.message
            : "Unable to sign in.",
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <main className="relative min-h-screen overflow-hidden bg-page">
      <div className="lg-bg" aria-hidden>
        <div className="lg-glow" />
        <div className="lg-dots" />
        <div className="lg-sweep" />
      </div>
      <div
        className="absolute inset-y-0 left-0 hidden w-1/2 flex-col justify-between text-white lg:flex"
        style={{ padding: "clamp(32px, 3.75vw, 72px) clamp(40px, 4.4vw, 88px)" }}
      >
        <div className="flex items-center gap-2.5">
          <span className="flex size-[22px] items-center justify-center rounded-[7px] bg-[#80ffaa] text-[#142a23]">
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.6"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              <circle cx="12" cy="12" r="3" />
              <circle cx="12" cy="12" r="9" strokeDasharray="2 4" />
            </svg>
          </span>
          <b className="text-base font-semibold">Orion</b>
        </div>
        <div className="flex flex-col" style={{ gap: "clamp(16px, 1.25vw, 24px)" }}>
          <div className="text-xs font-medium text-[#80ffaa]">Self-hosted monitoring</div>
          <div style={{ width: "clamp(474px, 37vw, 760px)", maxWidth: "100%" }}>
            <DotMatrixText lines={HEADLINE} columns={79} fluid />
          </div>
          <p
            className="max-w-[420px] text-sm opacity-75"
            style={{ fontSize: "clamp(14px, 1.1vw, 18px)" }}
          >
            Servers, monitors, incidents and status pages in one self-hosted console.
          </p>
          <ul
            className="m-0 flex list-none flex-col gap-2.5 p-0 text-[#d7efe3]"
            style={{ fontSize: "clamp(13px, 1vw, 17px)" }}
          >
            {CAPABILITIES.map((line) => (
              <li key={line} className="flex items-center gap-2.5">
                <span aria-hidden className="shrink-0 text-[#80ffaa]">
                  ✓
                </span>
                {line}
              </li>
            ))}
          </ul>
        </div>
        <span className="text-xs opacity-60">Self-hosted</span>
      </div>
      <div className="relative flex min-h-screen items-center justify-center lg:absolute lg:inset-y-0 lg:right-0 lg:w-1/2">
        <form className="flex w-[340px] max-w-full flex-col gap-3.5" onSubmit={submitLogin}>
          <h1 className="text-2xl font-medium tracking-[-0.025em]">Sign in</h1>
          <p className="-mt-1.5 text-sm text-mid">
            Use the admin credentials set on your Core host.
          </p>
          {sessionExpired && !error && (
            <Callout tone="warn">Your session expired. Sign in again.</Callout>
          )}
          <label className="flex flex-col gap-1.5 text-[13px] font-medium">
            Username
            <Input
              className="font-normal"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              autoComplete="username"
              required
            />
          </label>
          <label className="flex flex-col gap-1.5 text-[13px] font-medium">
            Password
            <Input
              className="font-normal"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
              required
            />
          </label>
          {error && <Callout tone="down">{error}</Callout>}
          <Button className="h-10 w-full" type="submit" disabled={isSubmitting}>
            {isSubmitting ? "Signing in" : "Sign in"}
          </Button>
          <span className="text-xs text-quiet">
            Forgot the password? Reset ORION_ADMIN_PASSWORD on the Core host.
          </span>
        </form>
      </div>
    </main>
  );
}
