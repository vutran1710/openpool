"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Lock } from "lucide-react";

export default function LoginPage() {
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const router = useRouter();

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError("");

    const res = await fetch("/api/auth", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password }),
    });

    if (res.ok) {
      router.push("/");
      router.refresh();
    } else {
      setError("Invalid password");
      setLoading(false);
    }
  }

  return (
    <div className="noise relative flex min-h-screen items-center justify-center">
      <div className="gradient-mesh absolute inset-0" />
      <div className="relative w-full max-w-sm px-6">
        <div className="mb-8 text-center">
          <Lock className="mx-auto mb-4 h-6 w-6 text-[var(--violet-400)]" strokeWidth={1.5} />
          <h1 className="mb-2 text-2xl font-bold">
            <span className="text-[var(--pink)]">♥</span>{" "}
            <span className="text-[var(--text)]">dating</span>
            <span className="text-[var(--text-dim)]">.dev</span>
          </h1>
          <p className="text-xs uppercase tracking-widest text-[var(--text-dim)]">
            Private project — login required
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="Password"
            className="w-full rounded-lg border border-[var(--border)] bg-[var(--bg-surface)] px-4 py-3 text-sm text-[var(--text)] outline-none transition-colors placeholder:text-[var(--text-dim)] focus:border-[var(--violet-500)]"
            autoFocus
          />

          {error && (
            <p className="text-center text-xs text-[var(--pink)]">{error}</p>
          )}

          <button
            type="submit"
            disabled={loading || !password}
            className="w-full rounded-lg bg-[var(--pink)] px-4 py-3 text-sm font-semibold text-[var(--bg)] transition-all hover:shadow-[0_0_20px_var(--pink-dim)] disabled:opacity-40"
          >
            {loading ? "..." : "Enter"}
          </button>
        </form>
      </div>
    </div>
  );
}
