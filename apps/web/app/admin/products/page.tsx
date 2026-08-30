"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Pencil, Plus, Trash2 } from "lucide-react";

import { apiClient, ApiError, type ProductListItem, type ProductUpsertRequest } from "@/lib/api-client";
import { formatRupiah } from "@/lib/format";

const EMPTY_FORM: ProductUpsertRequest = {
  name: "",
  category: "",
  brand: "",
  sku: "",
  barcode: "",
  description: "",
  short_description: "",
  size: "",
  gender: "",
  fragrance_family: "",
  status: "active",
  is_featured: false,
  is_bestseller: false,
  price: 0,
  image_url: "",
};

export default function AdminProductsPage() {
  const queryClient = useQueryClient();
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<ProductUpsertRequest | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  const categoriesQuery = useQuery({
    queryKey: ["categories"],
    queryFn: () => apiClient.listCategories(),
  });

  const productsQuery = useQuery({
    queryKey: ["admin", "products", { search, status, page }],
    queryFn: () => apiClient.adminListProducts({ search: search || undefined, status: status || undefined, page, limit: 20 }),
  });

  function invalidate() {
    queryClient.invalidateQueries({ queryKey: ["admin", "products"] });
  }

  const createMutation = useMutation({
    mutationFn: (req: ProductUpsertRequest) => apiClient.adminCreateProduct(req),
    onSuccess: () => {
      invalidate();
      closeForm();
    },
    onError: (err) => setFormError(err instanceof ApiError ? err.message : "Gagal menyimpan produk."),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, req }: { id: string; req: ProductUpsertRequest }) => apiClient.adminUpdateProduct(id, req),
    onSuccess: () => {
      invalidate();
      closeForm();
    },
    onError: (err) => setFormError(err instanceof ApiError ? err.message : "Gagal menyimpan produk."),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiClient.adminDeleteProduct(id),
    onSuccess: () => invalidate(),
  });

  function openCreate() {
    setEditingId(null);
    setForm({ ...EMPTY_FORM });
    setFormError(null);
  }

  function openEdit(item: ProductListItem) {
    setEditingId(item.id);
    setForm({
      name: item.name,
      category: item.category?.name ?? "",
      brand: item.brand?.name ?? "",
      sku: "",
      barcode: "",
      description: "",
      short_description: "",
      size: "",
      gender: "",
      fragrance_family: "",
      status: (item.status as ProductUpsertRequest["status"]) ?? "active",
      is_featured: item.is_featured,
      is_bestseller: item.is_bestseller,
      price: item.price_from,
      image_url: item.primary_image_url ?? "",
    });
    setFormError(null);

    // The list row doesn't carry every field (sku/barcode/description/
    // etc. aren't part of ListItem — see products.ListItem), so once the
    // form opens, fetch the full detail and fill in the rest.
    apiClient.getProduct(item.slug).then((detail) => {
      setForm({
        name: detail.name,
        category: detail.category?.name ?? "",
        brand: detail.brand?.name ?? "",
        sku: detail.sku ?? "",
        barcode: detail.barcode ?? "",
        description: detail.description ?? "",
        short_description: detail.short_description ?? "",
        size: detail.size ?? "",
        gender: detail.gender ?? "",
        fragrance_family: detail.fragrance_family ?? "",
        status: (detail.status as ProductUpsertRequest["status"]) ?? "active",
        is_featured: detail.is_featured,
        is_bestseller: detail.is_bestseller,
        price: detail.variants.find((v) => v.name === "Default")?.base_price ?? detail.variants[0]?.base_price ?? 0,
        image_url: detail.images.find((i) => i.is_primary)?.image_url ?? detail.images[0]?.image_url ?? "",
      });
    });
  }

  function closeForm() {
    setEditingId(null);
    setForm(null);
    setFormError(null);
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form) return;
    setFormError(null);
    if (editingId) {
      updateMutation.mutate({ id: editingId, req: form });
    } else {
      createMutation.mutate(form);
    }
  }

  const saving = createMutation.isPending || updateMutation.isPending;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="font-display text-2xl text-ink">Produk</h1>
          <p className="mt-1 text-sm text-ink-muted">Kelola katalog produk di semua cabang.</p>
        </div>
        <button
          onClick={openCreate}
          className="flex items-center gap-2 rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90"
        >
          <Plus className="h-4 w-4" />
          Tambah Produk
        </button>
      </div>

      <div className="flex flex-wrap gap-3">
        <input
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setPage(1);
          }}
          placeholder="Cari nama, SKU, brand..."
          className="min-w-[200px] flex-1 rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none"
        />
        <select
          value={status}
          onChange={(e) => {
            setStatus(e.target.value);
            setPage(1);
          }}
          className="rounded-full border border-line bg-background px-4 py-2 text-sm text-ink focus:border-accent focus:outline-none"
        >
          <option value="">Semua Status</option>
          <option value="active">Aktif</option>
          <option value="draft">Draf</option>
          <option value="archived">Diarsipkan</option>
        </select>
      </div>

      {productsQuery.isLoading && <p className="text-sm text-ink-muted">Memuat produk...</p>}
      {productsQuery.isError && <p className="text-sm text-red-400">Gagal memuat produk.</p>}

      {productsQuery.data && (
        <div className="overflow-x-auto rounded-2xl border border-line bg-surface">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-line text-ink-muted">
                <th className="px-4 py-3 font-medium">Nama</th>
                <th className="px-4 py-3 font-medium">Brand</th>
                <th className="px-4 py-3 font-medium">Harga</th>
                <th className="px-4 py-3 font-medium">Status</th>
                <th className="px-4 py-3 font-medium"></th>
              </tr>
            </thead>
            <tbody>
              {productsQuery.data.items.map((item) => (
                <tr key={item.id} className="border-b border-line last:border-0">
                  <td className="px-4 py-3 text-ink">{item.name}</td>
                  <td className="px-4 py-3 text-ink-muted">{item.brand?.name ?? "-"}</td>
                  <td className="px-4 py-3 text-ink">{formatRupiah(item.price_from)}</td>
                  <td className="px-4 py-3 text-ink-muted">{item.status}</td>
                  <td className="px-4 py-3">
                    <div className="flex justify-end gap-2">
                      <button onClick={() => openEdit(item)} className="flex items-center gap-1 text-accent transition hover:opacity-80">
                        <Pencil className="h-3.5 w-3.5" />
                        Ubah
                      </button>
                      <button
                        onClick={() => {
                          if (confirm(`Hapus produk "${item.name}"?`)) {
                            deleteMutation.mutate(item.id);
                          }
                        }}
                        className="flex items-center gap-1 text-red-400 transition hover:opacity-80"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                        Hapus
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
              {productsQuery.data.items.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-4 py-8 text-center text-ink-muted">
                    Tidak ada produk yang cocok.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {productsQuery.data && productsQuery.data.total_pages > 1 && (
        <div className="flex items-center justify-center gap-4 text-sm">
          <button
            disabled={page <= 1}
            onClick={() => setPage((p) => p - 1)}
            className="rounded-full border border-line px-4 py-2 text-ink disabled:opacity-40"
          >
            Sebelumnya
          </button>
          <span className="text-ink-muted">
            Halaman {productsQuery.data.page} dari {productsQuery.data.total_pages}
          </span>
          <button
            disabled={page >= productsQuery.data.total_pages}
            onClick={() => setPage((p) => p + 1)}
            className="rounded-full border border-line px-4 py-2 text-ink disabled:opacity-40"
          >
            Berikutnya
          </button>
        </div>
      )}

      {form && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="max-h-[90vh] w-full max-w-lg overflow-y-auto animate-scale-in rounded-2xl border border-line bg-surface p-6 shadow-xl">
            <h2 className="font-display text-lg text-ink">{editingId ? "Ubah Produk" : "Tambah Produk"}</h2>

            <form onSubmit={handleSubmit} className="mt-4 flex flex-col gap-3">
              <Field label="Nama">
                <input
                  required
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  className={inputClass}
                />
              </Field>

              <div className="grid grid-cols-2 gap-3">
                <Field label="Kategori">
                  <input
                    list="category-options"
                    value={form.category}
                    onChange={(e) => setForm({ ...form, category: e.target.value })}
                    className={inputClass}
                  />
                  <datalist id="category-options">
                    {categoriesQuery.data?.map((c) => <option key={c.id} value={c.name} />)}
                  </datalist>
                </Field>
                <Field label="Brand">
                  <input value={form.brand} onChange={(e) => setForm({ ...form, brand: e.target.value })} className={inputClass} />
                </Field>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <Field label="SKU">
                  <input value={form.sku} onChange={(e) => setForm({ ...form, sku: e.target.value })} className={inputClass} />
                </Field>
                <Field label="Barcode">
                  <input value={form.barcode} onChange={(e) => setForm({ ...form, barcode: e.target.value })} className={inputClass} />
                </Field>
              </div>

              <Field label="Harga (Rp)">
                <input
                  required
                  type="number"
                  min={1}
                  value={form.price || ""}
                  onChange={(e) => setForm({ ...form, price: Number(e.target.value) })}
                  className={inputClass}
                />
              </Field>

              <Field label="Deskripsi Singkat">
                <input
                  value={form.short_description}
                  onChange={(e) => setForm({ ...form, short_description: e.target.value })}
                  className={inputClass}
                />
              </Field>

              <Field label="Deskripsi">
                <textarea
                  value={form.description}
                  onChange={(e) => setForm({ ...form, description: e.target.value })}
                  rows={3}
                  className={`${inputClass} rounded-2xl`}
                />
              </Field>

              <div className="grid grid-cols-3 gap-3">
                <Field label="Ukuran">
                  <input value={form.size} onChange={(e) => setForm({ ...form, size: e.target.value })} className={inputClass} />
                </Field>
                <Field label="Gender">
                  <select value={form.gender} onChange={(e) => setForm({ ...form, gender: e.target.value })} className={inputClass}>
                    <option value="">-</option>
                    <option value="men">Pria</option>
                    <option value="women">Wanita</option>
                    <option value="unisex">Unisex</option>
                  </select>
                </Field>
                <Field label="Status">
                  <select
                    value={form.status}
                    onChange={(e) => setForm({ ...form, status: e.target.value as ProductUpsertRequest["status"] })}
                    className={inputClass}
                  >
                    <option value="active">Aktif</option>
                    <option value="draft">Draf</option>
                    <option value="archived">Diarsipkan</option>
                  </select>
                </Field>
              </div>

              <Field label="Keluarga Aroma">
                <input
                  value={form.fragrance_family}
                  onChange={(e) => setForm({ ...form, fragrance_family: e.target.value })}
                  className={inputClass}
                />
              </Field>

              <Field label="URL Gambar Utama">
                <input
                  value={form.image_url}
                  onChange={(e) => setForm({ ...form, image_url: e.target.value })}
                  className={inputClass}
                  placeholder="https://..."
                />
              </Field>

              <div className="flex gap-4">
                <label className="flex items-center gap-2 text-sm text-ink">
                  <input
                    type="checkbox"
                    checked={form.is_featured}
                    onChange={(e) => setForm({ ...form, is_featured: e.target.checked })}
                  />
                  Unggulan
                </label>
                <label className="flex items-center gap-2 text-sm text-ink">
                  <input
                    type="checkbox"
                    checked={form.is_bestseller}
                    onChange={(e) => setForm({ ...form, is_bestseller: e.target.checked })}
                  />
                  Terlaris
                </label>
              </div>

              {formError && <p className="text-sm text-red-400">{formError}</p>}

              <div className="mt-2 flex justify-end gap-3">
                <button type="button" onClick={closeForm} className="rounded-full border border-line px-4 py-2 text-sm text-ink">
                  Batal
                </button>
                <button
                  type="submit"
                  disabled={saving}
                  className="rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
                >
                  {saving ? "Menyimpan..." : "Simpan"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

const inputClass =
  "rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none";

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-sm text-ink-muted">{label}</label>
      {children}
    </div>
  );
}
