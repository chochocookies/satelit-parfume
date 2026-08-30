"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { UserPlus } from "lucide-react";

import { apiClient, ApiError } from "@/lib/api-client";
import { useCustomerAuthStore } from "@/stores/customer-auth-store";

export default function RegisterPage() {
  const router = useRouter();
  const setSession = useCustomerAuthStore((state) => state.setSession);
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const { subject, tokens } = await apiClient.customerRegister(name, email, phone, password);
      setSession(tokens.access_token, tokens.refresh_token, subject);
      router.push("/");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Gagal mendaftar. Coba lagi.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="mx-auto flex min-h-[70vh] max-w-sm flex-col justify-center px-6 py-12">
      <div className="animate-fade-in-up rounded-2xl border border-line bg-surface p-6 shadow-xl">
        <div className="flex items-center gap-2">
          <UserPlus className="h-5 w-5 text-accent" aria-hidden="true" />
          <h1 className="font-display text-2xl text-ink">Daftar</h1>
        </div>
        <p className="mt-1 text-sm text-ink-muted">Buat akun untuk belanja lebih mudah di Satelit Parfume.</p>

        <form onSubmit={handleSubmit} className="mt-6 flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <label htmlFor="name" className="text-sm text-ink-muted">
              Nama Lengkap
            </label>
            <input
              id="name"
              required
              minLength={2}
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted transition focus:border-accent focus:outline-none"
              placeholder="Nama Anda"
            />
          </div>

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
            <label htmlFor="phone" className="text-sm text-ink-muted">
              No. WhatsApp (opsional)
            </label>
            <input
              id="phone"
              type="tel"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              className="rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted transition focus:border-accent focus:outline-none"
              placeholder="08xxxxxxxxxx"
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <label htmlFor="password" className="text-sm text-ink-muted">
              Kata Sandi
            </label>
            <input
              id="password"
              type="password"
              required
              minLength={8}
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted transition focus:border-accent focus:outline-none"
              placeholder="Minimal 8 karakter"
            />
          </div>

          {error && <p className="text-sm text-red-400">{error}</p>}

          <button
            type="submit"
            disabled={submitting}
            className="mt-2 rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
          >
            {submitting ? "Memproses..." : "Daftar"}
          </button>
        </form>

        <p className="mt-6 text-center text-sm text-ink-muted">
          Sudah punya akun?{" "}
          <Link href="/login" className="text-accent transition hover:opacity-80">
            Masuk
          </Link>
        </p>
      </div>
    </div>
  );
}
