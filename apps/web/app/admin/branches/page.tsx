"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Pencil, Plus, Trash2, Users } from "lucide-react";

import { apiClient, ApiError, type Branch, type BranchUpsertRequest } from "@/lib/api-client";

const EMPTY_FORM: BranchUpsertRequest = {
  name: "",
  code: "",
  address: "",
  city: "",
  province: "",
  postal_code: "",
  latitude: null,
  longitude: null,
  phone: "",
  whatsapp: "",
  opening_time: "",
  closing_time: "",
};

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

export default function AdminBranchesPage() {
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState<Branch | null>(null);
  const [form, setForm] = useState<BranchUpsertRequest | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [staffBranch, setStaffBranch] = useState<Branch | null>(null);

  const branchesQuery = useQuery({
    queryKey: ["branches"],
    queryFn: () => apiClient.listBranches(),
  });

  function invalidate() {
    queryClient.invalidateQueries({ queryKey: ["branches"] });
  }

  const createMutation = useMutation({
    mutationFn: (req: BranchUpsertRequest) => apiClient.adminCreateBranch(req),
    onSuccess: () => {
      invalidate();
      closeForm();
    },
    onError: (err) => setFormError(err instanceof ApiError ? err.message : "Gagal menyimpan cabang."),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, req }: { id: string; req: BranchUpsertRequest }) => apiClient.adminUpdateBranch(id, req),
    onSuccess: () => {
      invalidate();
      closeForm();
    },
    onError: (err) => setFormError(err instanceof ApiError ? err.message : "Gagal menyimpan cabang."),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiClient.adminDeleteBranch(id),
    onSuccess: () => invalidate(),
  });

  function openCreate() {
    setEditing(null);
    setForm({ ...EMPTY_FORM });
    setFormError(null);
  }

  function openEdit(branch: Branch) {
    setEditing(branch);
    setForm({
      name: branch.name,
      code: branch.code,
      address: branch.address ?? "",
      city: branch.city ?? "",
      province: branch.province ?? "",
      postal_code: branch.postal_code ?? "",
      latitude: branch.latitude ?? null,
      longitude: branch.longitude ?? null,
      phone: branch.phone ?? "",
      whatsapp: branch.whatsapp ?? "",
      opening_time: branch.opening_time ?? "",
      closing_time: branch.closing_time ?? "",
    });
    setFormError(null);
  }

  function closeForm() {
    setEditing(null);
    setForm(null);
    setFormError(null);
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form) return;
    setFormError(null);
    if (editing) {
      updateMutation.mutate({ id: editing.id, req: form });
    } else {
      createMutation.mutate(form);
    }
  }

  const saving = createMutation.isPending || updateMutation.isPending;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="font-display text-2xl text-ink">Cabang</h1>
          <p className="mt-1 text-sm text-ink-muted">Kelola cabang toko dan staf yang bertugas di sana.</p>
        </div>
        <button
          onClick={openCreate}
          className="flex items-center gap-2 rounded-full bg-accent px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90"
        >
          <Plus className="h-4 w-4" />
          Tambah Cabang
        </button>
      </div>

      {branchesQuery.isLoading && <p className="text-sm text-ink-muted">Memuat cabang...</p>}
      {branchesQuery.isError && <p className="text-sm text-red-400">Gagal memuat cabang.</p>}

      {branchesQuery.data && (
        <div className="overflow-x-auto rounded-2xl border border-line bg-surface">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-line text-ink-muted">
                <th className="px-4 py-3 font-medium">Nama</th>
                <th className="px-4 py-3 font-medium">Kode</th>
                <th className="px-4 py-3 font-medium">Kota</th>
                <th className="px-4 py-3 font-medium">Telepon</th>
                <th className="px-4 py-3 font-medium">Status</th>
                <th className="px-4 py-3 font-medium"></th>
              </tr>
            </thead>
            <tbody>
              {branchesQuery.data.map((branch) => (
                <tr key={branch.id} className="border-b border-line last:border-0">
                  <td className="px-4 py-3 text-ink">{branch.name}</td>
                  <td className="px-4 py-3 text-ink-muted">{branch.code}</td>
                  <td className="px-4 py-3 text-ink-muted">{branch.city ?? "-"}</td>
                  <td className="px-4 py-3 text-ink-muted">{branch.phone ?? "-"}</td>
                  <td className="px-4 py-3 text-ink-muted">{branch.status}</td>
                  <td className="px-4 py-3">
                    <div className="flex justify-end gap-3">
                      <button onClick={() => setStaffBranch(branch)} className="flex items-center gap-1 text-accent transition hover:opacity-80">
                        <Users className="h-3.5 w-3.5" />
                        Staf
                      </button>
                      <button onClick={() => openEdit(branch)} className="flex items-center gap-1 text-accent transition hover:opacity-80">
                        <Pencil className="h-3.5 w-3.5" />
                        Ubah
                      </button>
                      <button
                        onClick={() => {
                          if (confirm(`Nonaktifkan cabang "${branch.name}"?`)) {
                            deleteMutation.mutate(branch.id);
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
            </tbody>
          </table>
        </div>
      )}

      {form && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="max-h-[90vh] w-full max-w-lg overflow-y-auto animate-scale-in rounded-2xl border border-line bg-surface p-6 shadow-xl">
            <h2 className="font-display text-lg text-ink">{editing ? "Ubah Cabang" : "Tambah Cabang"}</h2>

            <form onSubmit={handleSubmit} className="mt-4 flex flex-col gap-3">
              <div className="grid grid-cols-2 gap-3">
                <Field label="Nama">
                  <input required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} className={inputClass} />
                </Field>
                <Field label="Kode">
                  <input required value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })} className={inputClass} />
                </Field>
              </div>

              <Field label="Alamat">
                <input value={form.address} onChange={(e) => setForm({ ...form, address: e.target.value })} className={inputClass} />
              </Field>

              <div className="grid grid-cols-3 gap-3">
                <Field label="Kota">
                  <input value={form.city} onChange={(e) => setForm({ ...form, city: e.target.value })} className={inputClass} />
                </Field>
                <Field label="Provinsi">
                  <input value={form.province} onChange={(e) => setForm({ ...form, province: e.target.value })} className={inputClass} />
                </Field>
                <Field label="Kode Pos">
                  <input value={form.postal_code} onChange={(e) => setForm({ ...form, postal_code: e.target.value })} className={inputClass} />
                </Field>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <Field label="Telepon">
                  <input value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} className={inputClass} />
                </Field>
                <Field label="WhatsApp">
                  <input value={form.whatsapp} onChange={(e) => setForm({ ...form, whatsapp: e.target.value })} className={inputClass} />
                </Field>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <Field label="Jam Buka (HH:MM)">
                  <input
                    value={form.opening_time}
                    onChange={(e) => setForm({ ...form, opening_time: e.target.value })}
                    placeholder="09:00"
                    className={inputClass}
                  />
                </Field>
                <Field label="Jam Tutup (HH:MM)">
                  <input
                    value={form.closing_time}
                    onChange={(e) => setForm({ ...form, closing_time: e.target.value })}
                    placeholder="21:00"
                    className={inputClass}
                  />
                </Field>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <Field label="Latitude">
                  <input
                    type="number"
                    step="any"
                    value={form.latitude ?? ""}
                    onChange={(e) => setForm({ ...form, latitude: e.target.value ? Number(e.target.value) : null })}
                    className={inputClass}
                  />
                </Field>
                <Field label="Longitude">
                  <input
                    type="number"
                    step="any"
                    value={form.longitude ?? ""}
                    onChange={(e) => setForm({ ...form, longitude: e.target.value ? Number(e.target.value) : null })}
                    className={inputClass}
                  />
                </Field>
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

      {staffBranch && <BranchStaffModal branch={staffBranch} onClose={() => setStaffBranch(null)} />}
    </div>
  );
}

function BranchStaffModal({ branch, onClose }: { branch: Branch; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [selectedUserId, setSelectedUserId] = useState("");

  const staffQuery = useQuery({
    queryKey: ["admin", "branch-staff", branch.id],
    queryFn: () => apiClient.adminListBranchStaff(branch.id),
  });

  const allStaffQuery = useQuery({
    queryKey: ["admin", "users"],
    queryFn: () => apiClient.adminListStaff(),
  });

  function invalidate() {
    queryClient.invalidateQueries({ queryKey: ["admin", "branch-staff", branch.id] });
  }

  const assignMutation = useMutation({
    mutationFn: (userId: string) => apiClient.adminAssignBranchStaff(branch.id, userId),
    onSuccess: () => {
      invalidate();
      setSelectedUserId("");
    },
  });

  const unassignMutation = useMutation({
    mutationFn: (userId: string) => apiClient.adminUnassignBranchStaff(branch.id, userId),
    onSuccess: () => invalidate(),
  });

  const assignedIds = new Set(staffQuery.data?.map((s) => s.user_id));
  const assignableStaff = allStaffQuery.data?.filter((s) => !assignedIds.has(s.id)) ?? [];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="max-h-[90vh] w-full max-w-md overflow-y-auto animate-scale-in rounded-2xl border border-line bg-surface p-6 shadow-xl">
        <h2 className="font-display text-lg text-ink">Staf — {branch.name}</h2>

        {staffQuery.isLoading && <p className="mt-4 text-sm text-ink-muted">Memuat...</p>}

        {staffQuery.data && (
          <div className="mt-4 flex flex-col divide-y divide-line">
            {staffQuery.data.map((member) => (
              <div key={member.user_id} className="flex items-center justify-between py-2 text-sm">
                <div>
                  <p className="text-ink">{member.name}</p>
                  <p className="text-xs text-ink-muted">{member.roles?.join(", ") ?? "-"}</p>
                </div>
                <button
                  onClick={() => unassignMutation.mutate(member.user_id)}
                  className="text-red-400 hover:opacity-80"
                >
                  Lepas
                </button>
              </div>
            ))}
            {staffQuery.data.length === 0 && <p className="py-3 text-sm text-ink-muted">Belum ada staf di cabang ini.</p>}
          </div>
        )}

        <div className="mt-4 flex gap-2">
          <select
            value={selectedUserId}
            onChange={(e) => setSelectedUserId(e.target.value)}
            className="flex-1 rounded-full border border-line bg-background px-4 py-2 text-sm text-ink focus:border-accent focus:outline-none"
          >
            <option value="">Pilih staf untuk ditambahkan...</option>
            {assignableStaff.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name} ({s.roles.join(", ")})
              </option>
            ))}
          </select>
          <button
            disabled={!selectedUserId || assignMutation.isPending}
            onClick={() => selectedUserId && assignMutation.mutate(selectedUserId)}
            className="rounded-full bg-accent px-4 py-2 text-sm font-medium text-background transition hover:opacity-90 disabled:opacity-40"
          >
            Tambah
          </button>
        </div>

        <div className="mt-6 flex justify-end">
          <button onClick={onClose} className="rounded-full border border-line px-4 py-2 text-sm text-ink">
            Tutup
          </button>
        </div>
      </div>
    </div>
  );
}
