import { Server } from "lucide-react";
import { useState } from "react";

import { InputWithButton } from "../../components/ui/input-with-button";

export function LoginPage() {
  const [password, setPassword] = useState("");

  return (
    <main className="">
      <div className="flex flex-col items-center justify-center h-screen gap-4">
        <div className="border border-neutral-200 rounded-full w-12 h-12 bg-white flex items-center justify-center outline-4 outline-neutral-100">
          <Server className="w-4 h-4 text-neutral-500" />
        </div>
        <p>Monitor your own systems and services</p>

        <InputWithButton
          placeholder="Password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          buttonLabel="Enter"
          buttonAriaLabel="Enter"
          buttonType="submit"
        />
      </div>
    </main>
  );
}
