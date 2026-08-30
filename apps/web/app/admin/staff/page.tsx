"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Pencil, Plus } from "lucide-react";

import { apiClient, ApiError, type StaffAccount } from "@/lib/api-client";
import { useAuthStore } from "@/stores/auth-store";

// Matches migration 000002_auth's seeded roles table exactly — this list
// isn't fetched from the backend since there's no GET /roles endpoint
// (roles are fixed infrastructure, not admin-editable data).
const ALL_ROLES = ["SUPER_ADMIN", "ADMIN", "BRANCH_MANAGER", "CASHIER", "INVENTORY_STAFF"];

const inputClass =
  "rounded-full border border-line bg-background px-4 py-2 text-sm text-ink placeholder:text-ink-muted focus:border-accent focus:outline-none";

type FormState = {
  name: string;
  email: string;
  password: string;
  status: string;
  roles: string[];
};

const EMPTY_FORM: FormState = { name: "", email: "", password: "", status: "active", roles: [] };

export default function AdminStaffPage() {
  const queryClient = useQueryClient();
  const currentUserId = useAuthStore((state) => state.subject?.id);
  const [editing, setEditing] = useState<StaffAccount | null>(null);
  const [form, setForm] = useState<FormState | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  const staffQuery = useQuery({
    queryKey: ["admin", "users"],
    queryFn: () => apiClient.adminListStaff(),
  });

  function invalidate() {
    queryClient.invalidateQueries({ queryKey: ["admin", "users"] });
  }

  const createMutation = useMutation({
    mutationFn: () =>
      apiClient.adminCreateStaff({ name: form!.name, email: form!.email, password: form!.password, roles: form!.roles }),
    onSuccess: () => {
      invalidate();
      closeForm();
    },
    onError: (err) => setFormError(err instanceof ApiError ? err.message : "Gagal membuat akun staf."),
  });

  const updateMutation = useMutation({
    mutationFn: () =>
      apiClient.adminUpdateStaff(editing!.id, { name: form!.name, status: form!.status, roles: form!.roles }),
    onSuccess: () => {
      invalidate();
      closeForm();
    },
    onError: (err) => setFormError(err instanceof ApiError ? err.message : "Gagal menyimpan akun staf."),
  });

  function openCreate() {
    setEditing(null);
    setForm({ ...EMPTY_FORM });
    setFormError(null);
  }

  function openEdit(account: StaffAccount) {
    setEditing(account);
    setForm({ name: account.name, email: account.email, password: "", status: account.status, roles: [...account.roles] });
    setFormError(null);
  }

  function closeForm() {
    setEditing(null);
    setForm(null);
    setFormError(null);
  }

  function toggleRole(role: string) {
    if (!form) return;
    setForm({
      ...form,
      roles: form.roles.includes(role) ? form.roles.filter((r) => r !== role) : [...form.roles, role],
    });
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form || form.roles.length === 0) {
      setFormError("Pilih minimal satu peran.");
      return;
    }
    setFormError(null);
    if (editing) {
      updateMutation.mutate();
    } else {
      createMutation.mutate();
    }
  }

  const saving = createMutation.isPending || updateMutation.isPending;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="font-display text-2xl text-ink">Staf</h1>
          <p className="mt-1 text-sm text-ink-muted">Kelola akun staf dan perannya.</p>
        </div>
        <button
          onClick={openCreate}
          className="flex items-center gap-2 rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90"
        >
          <Plus className="h-4 w-4" />
          Tambah Staf
        </button>
      </div>

      {staffQuery.isLoading && <p className="text-sm text-ink-muted">Memuat staf...</p>}
      {staffQuery.isError && <p className="text-sm text-red-400">Gagal memuat staf.</p>}

      {staffQuery.data && (
        <div className="overflow-x-auto rounded-2xl border border-line bg-surface">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-line text-ink-muted">
                <th className="px-4 py-3 font-medium">Nama</th>
                <th className="px-4 py-3 font-medium">Email</th>
                <th className="px-4 py-3 font-medium">Peran</th>
                <th className="px-4 py-3 font-medium">Status</th>
                <th className="px-4 py-3 font-medium"></th>
              </tr>
            </thead>
            <tbody>
              {staffQuery.data.map((account) => (
                <tr key={account.id} className="border-b border-line last:border-0">
                  <td className="px-4 py-3 text-ink">
                    {account.name}
                    {account.id === currentUserId && <span className="ml-2 text-xs text-ink-muted">(Anda)</span>}
                  </td>
                  <td className="px-4 py-3 text-ink-muted">{account.email}</td>
                  <td className="px-4 py-3 text-ink-muted">{account.roles.join(", ")}</td>
                  <td className="px-4 py-3 text-ink-muted">{account.status === "active" ? "Aktif" : "Nonaktif"}</td>
                  <td className="px-4 py-3">
                    <div className="flex justify-end">
                      <button onClick={() => openEdit(account)} className="flex items-center gap-1 text-accent transition hover:opacity-80">
                        <Pencil className="h-3.5 w-3.5" />
                        Ubah
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {form && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="max-h-[90vh] w-full max-w-md overflow-y-auto animate-scale-in rounded-2xl border border-line bg-surface p-6 shadow-xl">
            <h2 className="font-display text-lg text-ink">{editing ? "Ubah Staf" : "Tambah Staf"}</h2>

            <form onSubmit={handleSubmit} className="mt-4 flex flex-col gap-3">
              <div className="flex flex-col gap-1.5">
                <label className="text-sm text-ink-muted">Nama</label>
                <input required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} className={inputClass} />
              </div>

              <div className="flex flex-col gap-1.5">
                <label className="text-sm text-ink-muted">Email</label>
                <input
                  required
                  type="email"
                  disabled={!!editing}
                  value={form.email}
                  onChange={(e) => setForm({ ...form, email: e.target.value })}
                  className={`${inputClass} disabled:opacity-50`}
                />
              </div>

              {!editing && (
                <div className="flex flex-col gap-1.5">
                  <label className="text-sm text-ink-muted">Kata Sandi</label>
                  <input
                    required
                    type="password"
                    minLength={8}
                    value={form.password}
                    onChange={(e) => setForm({ ...form, password: e.target.value })}
                    className={inputClass}
                  />
                </div>
              )}

              {editing && (
                <div className="flex flex-col gap-1.5">
                  <label className="text-sm text-ink-muted">Status</label>
                  <select value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value })} className={inputClass}>
                    <option value="active">Aktif</option>
                    <option value="suspended">Nonaktif</option>
                  </select>
                </div>
              )}

              <div className="flex flex-col gap-1.5">
                <label className="text-sm text-ink-muted">Peran</label>
                <div className="flex flex-wrap gap-3">
                  {ALL_ROLES.map((role) => (
                    <label key={role} className="flex items-center gap-2 text-sm text-ink">
                      <input type="checkbox" checked={form.roles.includes(role)} onChange={() => toggleRole(role)} />
                      {role}
                    </label>
                  ))}
                </div>
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
