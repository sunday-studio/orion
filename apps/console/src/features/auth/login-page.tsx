import { Server } from "lucide-react";
import type { FormEvent } from "react";
import { useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { authLogin } from "@/lib/custom-instance";
import { Input } from "@/components/ui/input";
import { InputWithButton } from "@/components/ui/input-with-button";

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
      navigate("/incidents", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to sign in.");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <main className="max-w-sm mx-auto">
      <form
        onSubmit={submitLogin}
        className="flex h-screen flex-col items-center justify-center gap-4"
      >
        <div className="border border-neutral-200 rounded-full w-12 h-12 bg-white flex items-center justify-center outline-4 outline-neutral-100">
          <Server className="w-4 h-4 text-neutral-500" />
        </div>
        <p>Monitor your own systems and servers</p>

        <Input
          className="w-full bg-white"
          placeholder="Username"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          required
        />
        <InputWithButton
          placeholder="Password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          buttonLabel={isSubmitting ? "Signing in" : "Enter"}
          buttonAriaLabel="Enter"
          buttonType="submit"
          disabled={isSubmitting}
          required
        />
        {sessionExpired && !error && (
          <p className="text-sm text-amber-700">Your session expired. Sign in again.</p>
        )}
        {error && <p className="text-sm text-rose-700">{error}</p>}
      </form>
    </main>
  );
}
