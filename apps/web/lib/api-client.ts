const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

type ApiEnvelope<T> = {
  success: boolean;
  message: string;
  data: T;
};

// Minimal typed fetch wrapper. Every backend response follows the
// { success, message, data } envelope from the API spec (section 65),
// so callers get `data` back directly instead of re-unwrapping it
// at every call site.
async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });

  const body = (await res.json()) as ApiEnvelope<T>;

  if (!res.ok || !body.success) {
    throw new ApiError(body.message ?? `Request to ${path} failed`, res.status);
  }

  return body.data;
}

// Builds a query string, dropping any key whose value is empty/undefined
// so callers can pass a full filter object without manually deciding
// which fields to include.
function toQuery(params: Record<string, string | number | undefined>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") {
      search.set(key, String(value));
    }
  }
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

export type HealthStatus = {
  api: "ok";
  database: "ok" | "unreachable";
  redis: "ok" | "unreachable";
};

export type Brand = { id: string; name: string; slug: string };
export type Category = { id: string; name: string; slug: string };

export type ProductListItem = {
  id: string;
  name: string;
  slug: string;
  status: string;
  is_featured: boolean;
  is_bestseller: boolean;
  brand?: Brand;
  category?: Category;
  price_from: number;
  primary_image_url?: string;
  branch_stock?: number;
};

export type ProductVariant = {
  id: string;
  name: string;
  sku?: string;
  barcode?: string;
  size?: string;
  base_price: number;
  status: string;
};

export type ProductImage = {
  id: string;
  image_url: string;
  alt_text?: string;
  sort_order: number;
  is_primary: boolean;
};

export type BranchAvailability = {
  branch_slug: string;
  branch_name: string;
  available_stock: number;
  price: number;
};

export type ProductDetail = {
  id: string;
  name: string;
  slug: string;
  sku?: string;
  barcode?: string;
  description?: string;
  short_description?: string;
  size?: string;
  gender?: string;
  fragrance_family?: string;
  status: string;
  is_featured: boolean;
  is_bestseller: boolean;
  brand?: Brand;
  category?: Category;
  variants: ProductVariant[];
  images: ProductImage[];
  availability?: BranchAvailability;
  created_at: string;
  updated_at: string;
};

export type ProductListResult = {
  items: ProductListItem[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
};

export type ProductListParams = {
  search?: string;
  category?: string;
  brand?: string;
  gender?: string;
  branch?: string;
  sort?: "newest" | "price_asc" | "price_desc" | "";
  page?: number;
  limit?: number;
};

export type Branch = {
  id: string;
  name: string;
  code: string;
  slug: string;
  address?: string;
  city?: string;
  province?: string;
  postal_code?: string;
  latitude?: number;
  longitude?: number;
  phone?: string;
  whatsapp?: string;
  opening_time?: string;
  closing_time?: string;
  status: string;
  distance_km?: number;
  created_at: string;
  updated_at: string;
};

export type CartItem = {
  id: string;
  product_variant_id: string;
  product_name: string;
  product_slug: string;
  variant_name: string;
  branch_id: string;
  quantity: number;
  unit_price: number;
  line_total: number;
  available_stock: number;
};

export type Cart = {
  id: string;
  branch_id?: string;
  items: CartItem[];
  item_count: number;
  subtotal: number;
  session_token?: string;
  updated_at: string;
};

// Every cart call needs the guest token attached, when there is one — a
// logged-in customer would use an Authorization header instead, added
// automatically once a real access token exists to send (no login UI
// yet — see stores/cart-store.ts).
function cartHeaders(cartToken: string | null): Record<string, string> {
  return cartToken ? { "X-Cart-Token": cartToken } : {};
}

export type OrderItem = {
  id: string;
  product_variant_id?: string;
  product_name: string;
  variant_name: string;
  sku?: string;
  unit_price: number;
  quantity: number;
  subtotal: number;
};

export type StatusEvent = {
  status: string;
  note?: string;
  created_at: string;
};

export type Order = {
  id: string;
  order_number: string;
  branch_id: string;
  order_type: "pickup" | "delivery";
  status: string;
  subtotal: number;
  total: number;
  guest_name?: string;
  guest_phone?: string;
  recipient_name?: string;
  recipient_phone?: string;
  address_line?: string;
  city?: string;
  notes?: string;
  expires_at?: string;
  // Phase 10 additions — see migration 000008_pos's column comments.
  payment_method?: string;
  cashier_shift_id?: string;
  items: OrderItem[];
  status_history: StatusEvent[];
  created_at: string;
};

export type CheckoutRequest = {
  order_type: "pickup" | "delivery";
  guest_name?: string;
  guest_phone?: string;
  recipient_name?: string;
  recipient_phone?: string;
  address_line?: string;
  city?: string;
  province?: string;
  postal_code?: string;
  notes?: string;
};

// ── Admin (Phase 9) ──────────────────────────────────────────────────
//
// Everything below backs the staff-only /admin section. adminRequest is
// request<T>'s authenticated sibling: it attaches the staff access
// token from useAuthStore and, on a 401, tries exactly one refresh
// before giving up and clearing the session — the plain access token's
// short lifetime (15 minutes by the root README's env defaults) means
// any real admin session will outlive it at least once.

export type TokenPair = {
  access_token: string;
  access_token_expires_at: string;
  refresh_token: string;
  refresh_token_expires_at: string;
};

export type AdminSubject = {
  id: string;
  type: string;
  name: string;
  email: string;
  roles: string[];
};

export type AuthResponse = {
  subject: AdminSubject;
  tokens: TokenPair;
};

function authHeaders(token: string | null): Record<string, string> {
  return token ? { Authorization: `Bearer ${token}` } : {};
}

async function tryRefresh(): Promise<string | null> {
  // Imported lazily inside the function body (not at module scope) so
  // api-client.ts — used by every page, logged in or not — never has a
  // hard import-time dependency on the auth store.
  const { useAuthStore } = await import("@/stores/auth-store");
  const { refreshToken, subject } = useAuthStore.getState();
  if (!refreshToken || !subject) {
    return null;
  }
  try {
    const tokens = await request<TokenPair>("/api/v1/auth/refresh", {
      method: "POST",
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    useAuthStore.getState().setSession(tokens.access_token, tokens.refresh_token, subject);
    return tokens.access_token;
  } catch {
    useAuthStore.getState().clear();
    return null;
  }
}

async function adminRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const { useAuthStore } = await import("@/stores/auth-store");
  const { accessToken } = useAuthStore.getState();

  try {
    return await request<T>(path, {
      ...init,
      headers: { ...authHeaders(accessToken), ...init?.headers },
    });
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      const refreshed = await tryRefresh();
      if (refreshed) {
        return request<T>(path, {
          ...init,
          headers: { ...authHeaders(refreshed), ...init?.headers },
        });
      }
    }
    throw err;
  }
}

export type StaffAccount = {
  id: string;
  name: string;
  email: string;
  status: string;
  roles: string[];
  created_at: string;
  updated_at: string;
};

export type CreateStaffRequest = {
  name: string;
  email: string;
  password: string;
  roles: string[];
};

export type UpdateStaffRequest = {
  name: string;
  status: string;
  roles: string[];
};

export type BranchStaffMember = {
  user_id: string;
  name: string;
  email: string;
  roles?: string[];
};

export type AdminProductListParams = {
  search?: string;
  category?: string;
  brand?: string;
  gender?: string;
  status?: string;
  sort?: string;
  page?: number;
  limit?: number;
};

export type ProductUpsertRequest = {
  name: string;
  category?: string;
  brand?: string;
  sku?: string;
  barcode?: string;
  description?: string;
  short_description?: string;
  size?: string;
  gender?: string;
  fragrance_family?: string;
  status?: "active" | "draft" | "archived" | "";
  is_featured: boolean;
  is_bestseller: boolean;
  price: number;
  image_url?: string;
};

export type AdminOrderListParams = {
  status?: string;
  branch_id?: string;
  search?: string;
  page?: number;
  limit?: number;
};

export type AdminOrderListResult = {
  items: Order[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
};

export type BranchUpsertRequest = {
  name: string;
  code: string;
  address?: string;
  city?: string;
  province?: string;
  postal_code?: string;
  latitude?: number | null;
  longitude?: number | null;
  phone?: string;
  whatsapp?: string;
  opening_time?: string;
  closing_time?: string;
};

export type DashboardStats = {
  revenue: number;
  total_orders: number;
  order_counts: Record<string, number>;
  total_products: number;
  total_branches: number;
  total_staff: number;
  low_stock_count: number;
};

// ── POS (Phase 10) ───────────────────────────────────────────────────
//
// A POS sale reuses the same cart mechanism a guest customer's browser
// session already uses (see cartHeaders above) — the POS screen just
// manages its own cart token in memory rather than localStorage (see
// app/pos/page.tsx), scoped to whichever branch the cashier is working.

export type Shift = {
  id: string;
  branch_id: string;
  user_id: string;
  opening_balance: number;
  closing_balance?: number;
  expected_balance?: number;
  discrepancy?: number;
  status: "open" | "closed";
  notes?: string;
  opened_at: string;
  closed_at?: string;
  created_at: string;
  updated_at: string;
};

export type OpenShiftRequest = {
  opening_balance: number;
  notes?: string;
};

export type CloseShiftRequest = {
  closing_balance: number;
  notes?: string;
};

export type Payment = {
  id: string;
  order_id: string;
  provider: string;
  payment_method: string;
  reference: string;
  amount: number;
  status: "PENDING" | "PAID" | "FAILED";
  qr_string?: string;
  va_number?: string;
  payment_url?: string;
  created_at: string;
  updated_at: string;
};

// ── Advanced inventory (Phase 11) ───────────────────────────────────

export type InventoryItem = {
  id: string;
  branch_id: string;
  product_variant_id: string;
  product_name: string;
  product_slug: string;
  variant_name: string;
  stock_quantity: number;
  reserved_quantity: number;
  available_stock: number;
  minimum_stock: number;
  price?: number;
  status: string;
  updated_at: string;
};

export type SetStockRequest = {
  stock_quantity: number;
  minimum_stock: number;
  price?: number | null;
};

export type ReceiveRequest = {
  quantity: number;
  note?: string;
};

export type AdjustRequest = {
  quantity_change: number;
  note?: string;
};

export type StockMovement = {
  id: string;
  branch_id: string;
  product_variant_id: string;
  product_name: string;
  variant_name: string;
  quantity_change: number;
  reason: string;
  reference_type?: string;
  reference_id?: string;
  note?: string;
  actor_user_id?: string;
  actor_name?: string;
  created_at: string;
};

export type TransferItem = {
  product_variant_id: string;
  product_name?: string;
  variant_name?: string;
  quantity: number;
};

export type StockTransfer = {
  id: string;
  from_branch_id: string;
  to_branch_id: string;
  status: "pending" | "completed" | "cancelled";
  requested_by: string;
  completed_by?: string;
  notes?: string;
  items: TransferItem[];
  completed_at?: string;
  created_at: string;
  updated_at: string;
};

export type CreateTransferRequest = {
  to_branch_id: string;
  items: TransferItem[];
  notes?: string;
};

export type OpnameItem = {
  id: string;
  product_variant_id: string;
  product_name: string;
  variant_name: string;
  system_quantity: number;
  counted_quantity?: number;
};

export type StockOpname = {
  id: string;
  branch_id: string;
  status: "open" | "completed";
  started_by: string;
  completed_by?: string;
  notes?: string;
  items: OpnameItem[];
  completed_at?: string;
  created_at: string;
  updated_at: string;
};

export const apiClient = {
  getHealth: () => request<HealthStatus>("/api/v1/health"),

  listProducts: (params: ProductListParams = {}) =>
    request<ProductListResult>(`/api/v1/products${toQuery(params)}`),

  getProduct: (slug: string, branch?: string) =>
    request<ProductDetail>(`/api/v1/products/${encodeURIComponent(slug)}${toQuery({ branch })}`),

  listCategories: () => request<Category[]>("/api/v1/categories"),

  listBranches: (coords?: { lat: number; lng: number }) =>
    request<Branch[]>(`/api/v1/branches${toQuery({ lat: coords?.lat, lng: coords?.lng })}`),

  getCart: (cartToken: string | null) => request<Cart>("/api/v1/cart", { headers: cartHeaders(cartToken) }),

  addCartItem: (
    cartToken: string | null,
    item: { product_variant_id: string; branch_id: string; quantity: number },
  ) =>
    request<Cart>("/api/v1/cart/items", {
      method: "POST",
      headers: cartHeaders(cartToken),
      body: JSON.stringify(item),
    }),

  updateCartItem: (cartToken: string | null, itemId: string, quantity: number) =>
    request<Cart>(`/api/v1/cart/items/${itemId}`, {
      method: "PUT",
      headers: cartHeaders(cartToken),
      body: JSON.stringify({ quantity }),
    }),

  removeCartItem: (cartToken: string | null, itemId: string) =>
    request<Cart>(`/api/v1/cart/items/${itemId}`, {
      method: "DELETE",
      headers: cartHeaders(cartToken),
    }),

  clearCart: (cartToken: string | null) =>
    request<null>("/api/v1/cart", { method: "DELETE", headers: cartHeaders(cartToken) }),

  checkout: (cartToken: string | null, req: CheckoutRequest) =>
    request<Order>("/api/v1/orders", {
      method: "POST",
      headers: cartHeaders(cartToken),
      body: JSON.stringify(req),
    }),

  // Auth
  staffLogin: (email: string, password: string) =>
    request<AuthResponse>("/api/v1/auth/staff/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),

  me: (accessToken: string) => request<AdminSubject>("/api/v1/auth/me", { headers: authHeaders(accessToken) }),

  logout: (refreshToken: string) =>
    request<null>("/api/v1/auth/logout", {
      method: "POST",
      body: JSON.stringify({ refresh_token: refreshToken }),
    }),

  // Admin: products — the manual create/edit/delete path Phase 3 left
  // for this phase (see internal/products' Phase 3 note).
  adminListProducts: (params: AdminProductListParams = {}) =>
    adminRequest<ProductListResult>(`/api/v1/admin/products${toQuery(params)}`),

  adminCreateProduct: (req: ProductUpsertRequest) =>
    adminRequest<ProductDetail>("/api/v1/admin/products", { method: "POST", body: JSON.stringify(req) }),

  adminUpdateProduct: (id: string, req: ProductUpsertRequest) =>
    adminRequest<ProductDetail>(`/api/v1/admin/products/${id}`, { method: "PUT", body: JSON.stringify(req) }),

  adminDeleteProduct: (id: string) => adminRequest<null>(`/api/v1/admin/products/${id}`, { method: "DELETE" }),

  // Admin: orders — cross-branch view; see internal/orders'
  // AdminListFilter doc comment for how this differs from a branch's
  // own order list.
  adminListOrders: (params: AdminOrderListParams = {}) =>
    adminRequest<AdminOrderListResult>(`/api/v1/admin/orders${toQuery(params)}`),

  adminGetOrder: (id: string) => adminRequest<Order>(`/api/v1/admin/orders/${id}`),

  adminUpdateOrderStatus: (id: string, status: string, note?: string) =>
    adminRequest<Order>(`/api/v1/admin/orders/${id}/status`, {
      method: "PUT",
      body: JSON.stringify({ status, note }),
    }),

  // Admin: branches (CRUD already existed from Phase 4 — this is its
  // first frontend) and per-branch staff assignment.
  adminCreateBranch: (req: BranchUpsertRequest) =>
    adminRequest<Branch>("/api/v1/admin/branches", { method: "POST", body: JSON.stringify(req) }),

  adminUpdateBranch: (id: string, req: BranchUpsertRequest) =>
    adminRequest<Branch>(`/api/v1/admin/branches/${id}`, { method: "PUT", body: JSON.stringify(req) }),

  adminDeleteBranch: (id: string) => adminRequest<null>(`/api/v1/admin/branches/${id}`, { method: "DELETE" }),

  adminListBranchStaff: (branchId: string) =>
    adminRequest<BranchStaffMember[]>(`/api/v1/admin/branches/${branchId}/staff`),

  adminAssignBranchStaff: (branchId: string, userId: string) =>
    adminRequest<null>(`/api/v1/admin/branches/${branchId}/staff`, {
      method: "POST",
      body: JSON.stringify({ user_id: userId }),
    }),

  adminUnassignBranchStaff: (branchId: string, userId: string) =>
    adminRequest<null>(`/api/v1/admin/branches/${branchId}/staff/${userId}`, { method: "DELETE" }),

  // Admin: staff accounts — the "create staff user" endpoint
  // internal/users' repository doc comment (pre-Phase-9) flagged as
  // missing.
  adminListStaff: () => adminRequest<StaffAccount[]>("/api/v1/admin/users"),

  adminGetStaff: (id: string) => adminRequest<StaffAccount>(`/api/v1/admin/users/${id}`),

  adminCreateStaff: (req: CreateStaffRequest) =>
    adminRequest<StaffAccount>("/api/v1/admin/users", { method: "POST", body: JSON.stringify(req) }),

  adminUpdateStaff: (id: string, req: UpdateStaffRequest) =>
    adminRequest<StaffAccount>(`/api/v1/admin/users/${id}`, { method: "PUT", body: JSON.stringify(req) }),

  // Admin: dashboard overview
  adminDashboardStats: () => adminRequest<DashboardStats>("/api/v1/admin/dashboard/stats"),

  // POS (Phase 10)
  posOpenShift: (branchId: string, req: OpenShiftRequest) =>
    adminRequest<Shift>(`/api/v1/admin/branches/${branchId}/shifts`, { method: "POST", body: JSON.stringify(req) }),

  posCurrentShift: (branchId: string) => adminRequest<Shift | null>(`/api/v1/admin/branches/${branchId}/shifts/current`),

  posCloseShift: (branchId: string, shiftId: string, req: CloseShiftRequest) =>
    adminRequest<Shift>(`/api/v1/admin/branches/${branchId}/shifts/${shiftId}/close`, {
      method: "PUT",
      body: JSON.stringify(req),
    }),

  posShiftHistory: (branchId: string) => adminRequest<Shift[]>(`/api/v1/admin/branches/${branchId}/shifts`),

  // posCheckout carries both the cashier's own Bearer token (adminRequest)
  // and the in-progress sale's cart token (cartHeaders) — the backend
  // needs the first to find the cashier's open shift, and the second to
  // know which cart to check out. See internal/orders' POSCheckout doc
  // comment.
  posCheckout: (branchId: string, cartToken: string | null, req: CheckoutRequest) =>
    adminRequest<Order>(`/api/v1/admin/branches/${branchId}/pos/checkout`, {
      method: "POST",
      headers: cartHeaders(cartToken),
      body: JSON.stringify(req),
    }),

  // posConfirmCash reuses the same branch-scoped status-update endpoint
  // Phase 7 built for exactly this ("a cashier confirming cash at the
  // counter") — payment_method is Phase 10's one addition to it.
  posConfirmCash: (branchId: string, orderId: string, note?: string) =>
    adminRequest<Order>(`/api/v1/admin/branches/${branchId}/orders/${orderId}/status`, {
      method: "PUT",
      body: JSON.stringify({ status: "PAID", note: note ?? "Cash payment confirmed by cashier", payment_method: "cash" }),
    }),

  posPay: (branchId: string, orderId: string, paymentMethod: string) =>
    adminRequest<Payment>(`/api/v1/admin/branches/${branchId}/orders/${orderId}/pay`, {
      method: "POST",
      body: JSON.stringify({ payment_method: paymentMethod }),
    }),

  posGetOrder: (branchId: string, orderId: string) =>
    adminRequest<Order>(`/api/v1/admin/branches/${branchId}/orders/${orderId}`),

  // ── Advanced inventory (Phase 11) ──────────────────────────────────

  adminListInventory: (branchId: string) => adminRequest<InventoryItem[]>(`/api/v1/admin/branches/${branchId}/inventory`),

  adminSetStock: (branchId: string, variantId: string, req: SetStockRequest) =>
    adminRequest<InventoryItem>(`/api/v1/admin/branches/${branchId}/inventory/${variantId}`, {
      method: "PUT",
      body: JSON.stringify(req),
    }),

  adminReceiveStock: (branchId: string, variantId: string, req: ReceiveRequest) =>
    adminRequest<StockMovement>(`/api/v1/admin/branches/${branchId}/inventory/${variantId}/receive`, {
      method: "POST",
      body: JSON.stringify(req),
    }),

  adminAdjustStock: (branchId: string, variantId: string, req: AdjustRequest) =>
    adminRequest<StockMovement>(`/api/v1/admin/branches/${branchId}/inventory/${variantId}/adjust`, {
      method: "POST",
      body: JSON.stringify(req),
    }),

  adminStockMovements: (branchId: string, variantId: string) =>
    adminRequest<StockMovement[]>(`/api/v1/admin/branches/${branchId}/inventory/${variantId}/movements`),

  adminCreateTransfer: (branchId: string, req: CreateTransferRequest) =>
    adminRequest<StockTransfer>(`/api/v1/admin/branches/${branchId}/transfers`, {
      method: "POST",
      body: JSON.stringify(req),
    }),

  adminListTransfers: (branchId: string) => adminRequest<StockTransfer[]>(`/api/v1/admin/branches/${branchId}/transfers`),

  adminCompleteTransfer: (branchId: string, transferId: string) =>
    adminRequest<StockTransfer>(`/api/v1/admin/branches/${branchId}/transfers/${transferId}/complete`, { method: "PUT" }),

  adminCancelTransfer: (branchId: string, transferId: string) =>
    adminRequest<StockTransfer>(`/api/v1/admin/branches/${branchId}/transfers/${transferId}/cancel`, { method: "PUT" }),

  adminStartOpname: (branchId: string) =>
    adminRequest<StockOpname>(`/api/v1/admin/branches/${branchId}/opnames`, { method: "POST" }),

  adminCurrentOpname: (branchId: string) => adminRequest<StockOpname | null>(`/api/v1/admin/branches/${branchId}/opnames/current`),

  adminListOpnames: (branchId: string) => adminRequest<StockOpname[]>(`/api/v1/admin/branches/${branchId}/opnames`),

  adminCountOpnameItem: (branchId: string, opnameId: string, itemId: string, countedQuantity: number) =>
    adminRequest<OpnameItem>(`/api/v1/admin/branches/${branchId}/opnames/${opnameId}/items/${itemId}`, {
      method: "PUT",
      body: JSON.stringify({ counted_quantity: countedQuantity }),
    }),

  adminCompleteOpname: (branchId: string, opnameId: string, notes?: string) =>
    adminRequest<StockOpname>(`/api/v1/admin/branches/${branchId}/opnames/${opnameId}/complete`, {
      method: "PUT",
      body: JSON.stringify({ notes: notes ?? "" }),
    }),
};
