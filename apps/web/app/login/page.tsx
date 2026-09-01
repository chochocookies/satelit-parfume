"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Eye, EyeOff, LogIn } from "lucide-react";

import { apiClient, ApiError } from "@/lib/api-client";
import { useCustomerAuthStore } from "@/stores/customer-auth-store";

export default function LoginPage() {
  const router = useRouter();
  const setSession = useCustomerAuthStore((state) => state.setSession);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const { subject, tokens } = await apiClient.customerLogin(email, password);
      setSession(tokens.access_token, tokens.refresh_token, subject);
      router.push("/");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal masuk. Coba lagi.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="mx-auto flex min-h-[70vh] max-w-sm flex-col justify-center px-6 py-12">
      <div className="animate-fade-in-up rounded-2xl border border-line bg-surface p-6 shadow-xl">
        <div className="flex items-center gap-2">
          <LogIn className="h-5 w-5 text-accent" aria-hidden="true" />
          <h1 className="font-display text-2xl text-ink">Masuk</h1>
        </div>
        <p className="mt-1 text-sm text-ink-muted">Masuk ke akun Satelit Parfume Anda.</p>

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
              className="rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted transition focus:border-accent focus:outline-none"
              placeholder="nama@email.com"
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <label htmlFor="password" className="text-sm text-ink-muted">
              Kata Sandi
            </label>
            <div className="relative">
              <input
                id="password"
                type={showPassword ? "text" : "password"}
                required
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="w-full rounded-full border border-line bg-background px-4 py-2 pr-11 text-sm text-ink placeholder:text-ink-muted transition focus:border-accent focus:outline-none"
                placeholder="••••••••"
              />
              <button
                type="button"
                onClick={() => setShowPassword((v) => !v)}
                aria-label={showPassword ? "Sembunyikan kata sandi" : "Lihat kata sandi"}
                className="absolute inset-y-0 right-3 flex items-center text-ink-muted transition hover:text-ink"
              >
                {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
            </div>
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

        <p className="mt-6 text-center text-sm text-ink-muted">
          Belum punya akun?{" "}
          <Link href="/register" className="text-accent transition hover:opacity-80">
            Daftar
          </Link>
        </p>
        <p className="mt-2 text-center text-xs text-ink-muted">
          Staf toko?{" "}
          <Link href="/admin/login" className="text-accent transition hover:opacity-80">
            Masuk di sini
          </Link>
        </p>
      </div>
    </div>
  );
}
