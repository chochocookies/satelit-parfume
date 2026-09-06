"use client";

import { useState } from "react";
import Link from "next/link";
import { Star } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiClient, ApiError } from "@/lib/api-client";
import { useCustomerAuthStore } from "@/stores/customer-auth-store";
import { cn } from "@/lib/utils";

function StarRow({ value, size = "h-4 w-4" }: { value: number; size?: string }) {
  return (
    <div className="flex gap-0.5" aria-hidden="true">
      {[1, 2, 3, 4, 5].map((n) => (
        <Star key={n} className={cn(size, n <= Math.round(value) ? "fill-accent text-accent" : "text-line")} />
      ))}
    </div>
  );
}

// Sits at the bottom of the product detail page. Eligibility (a
// COMPLETED order containing this product) is enforced server-side —
// this shows the form to any logged-in customer and just surfaces the
// server's message if they turn out not to be eligible, rather than
// trying to duplicate that check here.
export function ProductReviews({ slug }: { slug: string }) {
  const queryClient = useQueryClient();
  const customerAccessToken = useCustomerAuthStore((s) => s.accessToken);
  const customerHydrated = useCustomerAuthStore((s) => s.hasHydrated);
  const isCustomer = customerHydrated && !!customerAccessToken;

  const [showForm, setShowForm] = useState(false);
  const [rating, setRating] = useState(0);
  const [comment, setComment] = useState("");

  const reviewsQuery = useQuery({
    queryKey: ["reviews", slug],
    queryFn: () => apiClient.getProductReviews(slug),
  });

  const submitMutation = useMutation({
    mutationFn: () => apiClient.submitReview(slug, rating, comment),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["reviews", slug] });
      setShowForm(false);
      setRating(0);
      setComment("");
    },
  });

  const summary = reviewsQuery.data?.summary;
  const list = reviewsQuery.data?.reviews ?? [];

  return (
    <section className="mt-16 border-t border-line pt-10">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="font-display text-xl text-ink">Ulasan</h2>
        {summary && summary.count > 0 && (
          <div className="flex items-center gap-2">
            <StarRow value={summary.average} />
            <span className="text-sm text-ink-muted">
              {summary.average.toFixed(1)} ({summary.count} ulasan)
            </span>
          </div>
        )}
      </div>

      {isCustomer && !showForm && (
        <button onClick={() => setShowForm(true)} className="mt-4 text-sm text-accent hover:underline">
          Tulis ulasan
        </button>
      )}
      {!isCustomer && customerHydrated && (
        <p className="mt-4 text-sm text-ink-muted">
          <Link href="/login" className="text-accent hover:underline">
            Masuk
          </Link>{" "}
          untuk menulis ulasan — khusus pembeli yang pesanannya sudah selesai.
        </p>
      )}

      {showForm && (
        <div className="mt-4 space-y-3 rounded-2xl border border-line bg-surface p-4">
          <div className="flex items-center gap-1">
            {[1, 2, 3, 4, 5].map((n) => (
              <button key={n} type="button" onClick={() => setRating(n)} aria-label={`${n} bintang`}>
                <Star className={cn("h-6 w-6", n <= rating ? "fill-accent text-accent" : "text-line")} />
              </button>
            ))}
          </div>
          <textarea
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            placeholder="Ceritakan pengalamanmu dengan produk ini (opsional)"
            rows={3}
            className="w-full rounded-xl border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none"
          />
          <div className="flex items-center gap-3">
            <button
              onClick={() => submitMutation.mutate()}
              disabled={rating === 0 || submitMutation.isPending}
              className="rounded-full bg-accent px-4 py-2 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
            >
              {submitMutation.isPending ? "Mengirim…" : "Kirim Ulasan"}
            </button>
            <button onClick={() => setShowForm(false)} className="text-sm text-ink-muted">
              Batal
            </button>
          </div>
          {submitMutation.isError && (
            <p className="text-xs text-red-400">
              {submitMutation.error instanceof ApiError ? submitMutation.error.message : "Gagal mengirim ulasan."}
            </p>
          )}
        </div>
      )}

      <div className="mt-6 space-y-5">
        {reviewsQuery.isLoading && <p className="text-sm text-ink-muted">Memuat ulasan…</p>}
        {!reviewsQuery.isLoading && list.length === 0 && (
          <p className="text-sm text-ink-muted">Belum ada ulasan untuk produk ini.</p>
        )}
        {list.map((r) => (
          <div key={r.id} className="border-b border-line pb-4 last:border-0">
            <div className="flex items-center justify-between">
              <span className="text-sm text-ink">{r.customer_name}</span>
              <StarRow value={r.rating} size="h-3.5 w-3.5" />
            </div>
            {r.comment && <p className="mt-1.5 text-sm text-ink-muted">{r.comment}</p>}
          </div>
        ))}
      </div>
    </section>
  );
}
