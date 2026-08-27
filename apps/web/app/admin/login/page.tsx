"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

import { apiClient, ApiError } from "@/lib/api-client";
import { useAuthStore } from "@/stores/auth-store";

export default function AdminLoginPage() {
  const router = useRouter();
  const setSession = useAuthStore((state) => state.setSession);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const { subject, tokens } = await apiClient.staffLogin(email, password);
      if (!subject.roles?.some((role) => role === "SUPER_ADMIN" || role === "ADMIN")) {
        setError("Akun ini tidak punya akses ke dashboard admin.");
        return;
      }
      setSession(tokens.access_token, tokens.refresh_token, subject);
      router.replace("/admin");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal masuk. Coba lagi.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <div className="w-full max-w-sm rounded-2xl border border-line bg-surface p-6 shadow-xl">
        <h1 className="font-display text-2xl text-ink">Admin Satelit Parfume</h1>
        <p className="mt-1 text-sm text-ink-muted">Masuk dengan akun staf untuk mengelola toko.</p>

        <form onSubmit={handleSubmit} className="mt-6 flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <label htmlFor="email" className="text-sm text-ink-muted">
              Email
            </label>
            <input
              id="email"
              type="email"
              required
              autoComplete="username"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none"
              placeholder="nama@satelitparfume.id"
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <label htmlFor="password" className="text-sm text-ink-muted">
              Kata sandi
            </label>
            <input
              id="password"
              type="password"
              required
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none"
              placeholder="••••••••"
            />
          </div>

          {error && <p className="text-sm text-red-400">{error}</p>}

          <button
            type="submit"
            disabled={submitting}
            className="mt-2 rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
          >
            {submitting ? "Memproses..." : "Masuk"}
          </button>
        </form>
      </div>
    </div>
  );
}
