# Satelit Parfume — Platform

Multi-branch perfume commerce & retail management platform for **Satelit
Parfume** (Instagram: [@satelit_parfume](https://www.instagram.com/satelit_parfume/),
Shopee: [satelit_parfume](https://shopee.co.id/satelit_parfume)).

> **Status: Phase 8 of 15 — Infra + Auth + Catalog + Branches + Shop +
> Cart + Orders + Payments.**
> Real online payment now exists behind a provider-agnostic interface,
> with a working Duitku adapter (QRIS + bank VAs) as its first
> implementation. Cash-at-counter (Phase 7) still works exactly as
> before — this adds a second path, it doesn't replace the first. See
> [Roadmap](#roadmap) for what's left.

---

## Overview

The full platform (per the project spec this repo was scaffolded from) is:
a public landing page, e-commerce site, installable PWA, Android/iOS apps,
multi-branch inventory, branch POS, admin + customer management, and
order/payment/promotion/membership systems, all backed by one PostgreSQL
database as the single source of truth. Shopee is a reference for the
current catalog, not the operational database.

Phase 1 delivered the foundation: a monorepo that runs, a Go API with a
real database + cache connection and a health check, a Next.js app with
Tailwind v4 configured and a live status page, and the Docker/env setup
everything else builds on. Phase 2 added working authentication: staff
accounts, customer accounts, role-based access, and JWT access + refresh
tokens with real rotation/revocation. Phase 3 added the product catalog:
schema, a real CSV import path, and public search/browse endpoints — and
used it to actually import your 23 verified products. Phase 4 added
branches and branch-scoped stock, plus authorization that actually
confines staff to the branch they're assigned to. Phase 5 turned all of
that into a real customer-facing site: landing page, shop, product
pages, a persistent branch selector. Phase 6 added the cart, with real
stock validation and one branch per cart. Phase 7 added checkout and
order management: real stock reservation, a validated status lifecycle,
and a working pickup-and-pay-at-counter flow end to end. Phase 8 adds
real online payment on top of that — see
[What Phase 8 adds](#what-phase-8-adds).

## Roadmap

| Phase | Scope |
|---|---|
| ~~1~~ | ~~Infra: monorepo, Docker Compose, DB/Redis connection, health check, basic pages~~ ✅ |
| ~~2~~ | ~~Auth: users, roles, JWT + refresh tokens~~ ✅ |
| ~~3~~ | ~~Catalog: products, categories, brands, images~~ ✅ |
| ~~4~~ | ~~Branches: branch CRUD, branch-scoped inventory~~ ✅ |
| ~~5~~ | ~~Customer shop: real landing page, branch selector, product detail, search~~ ✅ |
| ~~6~~ | ~~Cart~~ ✅ |
| ~~7~~ | ~~Orders & checkout~~ ✅ |
| ~~8~~ | ~~Payments: abstraction + QRIS/gateway + webhooks~~ ✅ |
| ~~9~~ | ~~Admin dashboard~~ ✅ |
| ~~10 (this repo)~~ | ~~POS~~ ✅ |
| 11 | Advanced inventory: stock movement, transfer, opname |
| 12 | Wishlist, reviews, membership, promotions |
| 13 | PWA |
| 14 | Capacitor (Android/iOS) |
| 15 | Production hardening: Nginx, SSL, backups, monitoring |

Each phase is meant to land as its own reviewable step — the app should
stay runnable after every one.

## What Phase 2 adds

- **Two separate account types**, on purpose: `users` (internal staff —
  admin, branch manager, cashier, inventory staff) with roles/permissions,
  and `customers` (shop accounts) with none — a customer's authorization
  is "is this their own order/cart/wishlist", not a role lookup. See the
  comment at the top of `migrations/000002_auth.up.sql` for the reasoning.
- **Password hashing** via bcrypt (`pkg/password`) — never plaintext,
  never a reversible scheme.
- **JWT access + refresh tokens** (`pkg/jwt`), both signed, with separate
  secrets. Access tokens are stateless (15 min default). Refresh tokens
  carry a `jti` that's tracked in the `refresh_tokens` table, so a refresh
  **rotates**: each one is valid for exactly one exchange, then it's
  revoked and a new pair is issued. A leaked, already-used refresh token
  fails loudly on its next use instead of quietly staying valid.
- **Redis-backed login rate limiting** — 5 failed attempts per
  (account type, email) locks further attempts out for 15 minutes. The
  first real use of the Redis connection Phase 1 only proved reachable.
- **`RequireAuth` / `RequireRole` middleware**, and one real protected
  route (`GET /api/v1/admin/ping`) proving the whole chain — register or
  log in as staff, hit it, get `200`; try it as a customer or with no
  token, get `403`/`401`. This is also the pattern Phase 3+ copies for
  every real admin-only endpoint.
- **Two Postgres-backed integration paths, and two pure-logic unit test
  files** (`pkg/jwt`, `pkg/password`) — see
  [Testing](#testing).

Deliberately **not** in Phase 2, to keep it focused: email verification,
and forgot/reset-password. Both need an email provider that isn't
configured anywhere in this project yet (no `SMTP_*`/`EMAIL_*` env vars
exist per the spec's own env list), so implementing the token mechanics
without real delivery would be half a feature. Worth adding once there's
an actual provider to send through.

## What Phase 3 adds

- **Schema**: `brands`, `categories`, `products`, `product_variants`,
  `product_images` (section 6/7). Every product currently gets exactly
  **one** `'Default'` variant carrying its real price — never a fake
  30ml/50ml/100ml lineup invented to look complete (section 7's own rule).
- **A real CSV import**, not a staging area for one: `POST
  /api/v1/admin/products/import` (staff-only) parses the exact schema
  `seeds/products_verified.csv` already uses, validates each row (name +
  a whole-Rupiah-integer price required), skips rows that are malformed
  or already imported, auto-creates categories/brands it hasn't seen
  before, and returns a per-row report — a batch with 3 bad rows out of
  65 still imports the other 62. Each row is its own transaction (product
  + variant + image together), so a failure never leaves a product with
  no variant behind, and one bad row can't roll back rows already
  committed earlier in the same file.
- **Public browsing**: `GET /api/v1/products` (search across
  name/SKU/barcode/brand/category, filter by category/brand/gender, sort,
  paginate — section 15/19) and `GET /api/v1/products/:slug` (full detail
  with variants + images). Deliberately **no stock/availability field**
  in either response yet — that's `branch_inventory`, which doesn't exist
  until Phase 4 (section 9); a fake "in stock" here would be exactly the
  kind of invented data this project has avoided everywhere else.
- **Slugs** (`pkg/slug`) are collision-safe: two products landing on the
  same base slug get `-2`, `-3`, etc., checked against the database at
  import time — not assumed unique from the name alone.

Deliberately **not** in Phase 3: manual single-product create/edit
endpoints, and a two-step "preview, then separately publish" import flow
(section 48/72 describe one). Both are admin-*UI* conveniences more than
catalog-*model* work — the real need for them shows up once Phase 9
builds the admin dashboard forms that would call them. Import already
reports full per-row errors before anything commits for that row, which
covers the "validate, then decide" spirit without a separate staging
table to build and maintain in the meantime.

## What Phase 4 adds

- **Branches** (section 8): full CRUD, admin-only. `GET /branches`
  accepts `?lat=&lng=` for nearest-first sorting (section 11) — a
  standard Haversine formula in SQL, no PostGIS needed for "which branch
  is closest".
- **`branch_staff`**, a many-to-many table, not a single `users.branch_id`
  column — section 5 lists it as its own entity, and it lets one person
  legitimately work more than one branch. This corrects Phase 2's own
  comment ("branch_id arrives via ALTER TABLE"), which assumed the
  simpler one-branch-per-user shape before this table's design was
  worked out — noting it here the same way the Flutter→Capacitor switch
  got called out earlier, rather than quietly diverging from what Phase
  2 said would happen.
- **`branch_inventory`**, with `available_stock` as a **generated
  column** (`stock_quantity - reserved_quantity`, computed by Postgres
  itself, not re-derived by every caller — section 9's own formula).
  Phase 4 gives you a direct "set the absolute quantity" endpoint;
  auditable stock *movements* (receive/adjust/transfer with a reason and
  an actor) are Phase 11's job (section 32) — this is deliberately not
  that yet.
- **Branch-scoped authorization**, not just role-based: `RequireBranchAccess`
  checks `branch_staff` fresh on every request. SUPER_ADMIN/ADMIN reach
  any branch; BRANCH_MANAGER/CASHIER/INVENTORY_STAFF only reach branches
  they're actually assigned to (section 9's rule: "branch users cannot
  access unauthorized branches") — checked against the database, not a
  JWT claim, so revoking someone's branch access takes effect on their
  very next request, not their next login.
- **`GET /products/:slug?branch=<slug>`** now resolves real stock and
  price for that branch (section 17: "Available at: ... ✓ 12 available")
  — closing the gap Phase 3's README flagged. Deliberately **not** wired
  into the product *list* endpoint yet, and deliberately not a persisted
  "selected branch" (cookie/session) — both are the actual customer-shop
  UX (branch selector, persistence, section 10) that Phase 5 builds; this
  just makes the data reachable by slug now that it exists.
- Closed a gap from Phase 3 in passing: `categories` was missing
  `deleted_at`, which section 83 lists it as needing alongside
  products/branches/users. No functional difference yet — there was no
  delete endpoint to use it — but the schema's right now instead of
  carrying the gap forward.

Deliberately **not** in Phase 4: a branch manager self-service staffing
endpoint (section 41 mentions BRANCH_MANAGER having some "staff" access;
for now, assigning/unassigning staff is SUPER_ADMIN/ADMIN-only — viewing
your own branch's roster works via `GET .../branches/:id/staff` for
BRANCH_MANAGER, but *changing* it is an org-structure decision left at
the admin level until there's a real admin UI screen driving it).

## What Phase 5 adds

The first pages an actual customer would see:

- **Landing page** (`/`) — hero, categories (from the real API), a
  "newest products" showcase, and a branch-locator teaser. Every section
  has a real empty state for a fresh install with no data yet ("Katalog
  masih kosong. Impor produk dulu — lihat README") rather than assuming
  the CSV import and a branch already exist.
- **Shop** (`/shop`) — search (debounced), category filter, sort, grid,
  pagination. Loading skeletons, an error state, and an empty state, per
  section 75/76.
- **Product detail** (`/product/[slug]`) — full detail, and (once a
  branch is selected) real stock + price for that branch, section 17's
  "Available at: ... ✓ 12 available".
- **Branch selector**, in the header, persisted to `localStorage`
  (section 10) via a small Zustand store — the first real use of
  `zustand`, declared as a dependency since Phase 1 but unused until now.
  Includes an opt-in "use my location" button (section 11) that re-sorts
  the list by distance using Phase 4's `?lat=&lng=` support; picking a
  branch is always a deliberate click, never automatic, per section 11's
  own rule.
- **Backend**: `GET /products` now also accepts `?branch=<slug>` (Phase 4
  only wired this into the single-product detail endpoint) — a single
  correlated subquery per product resolves that branch's stock without
  N+1 lookups, so the whole grid can show per-item availability in one
  query.

**On brand identity** (section 13): still genuinely unverified. This
phase tried to reach the real Instagram and Shopee pages directly (not
just image search, which returned an unrelated "Satellite" perfume brand
back in Phase 1) — Instagram blocks automated fetches outright, and
Shopee's page is a client-rendered shell with no usable content in a
plain fetch. So the landing page extends Phase 1's provisional palette
(warm dark neutrals, one muted gold accent, Fraunces/Manrope) rather than
inventing a new one — consistent, still explicitly not verified brand
colors. Getting real ones means either sharing exported brand assets
directly, or giving Claude a browsing-capable tool.

**No cart button anywhere** — not on the product card, not on the
product-detail page, even though section 16/17's own mockups show one.
Phase 6 doesn't exist yet, so there's nothing for it to add to; a button
that does nothing is worse than no button, same reasoning as the missing
wishlist heart icon and the missing header nav links (Cart, Wishlist,
Account). Cart arrived the next phase — see
[What Phase 6 adds](#what-phase-6-adds); Wishlist/Account are still
missing for the same reason, tracked there too.

Deliberately **not** in Phase 5: the fragrance finder (section 18) — a
whole guided-quiz recommendation feature with its own scoring engine, not
listed in the phase table's own Phase 5 scope; the customer account/order
history pages (need real orders to show, Phase 7); URL-syncing shop
filters both ways (reads the URL on load for shareable links, doesn't
write back as you type — a reasonable follow-up, not a correctness gap);
and server-side rendering the catalog for SEO (section 80) — every data
fetch here is client-side for consistency and to avoid a Docker-specific
server-vs-browser API URL split this environment can't verify against a
running container network.

## What Phase 6 adds

- **`carts`/`cart_items`** (section 20), with a database `CHECK`
  constraint doing real work: a cart belongs to *either* a logged-in
  customer *or* a guest session, enforced at the schema level, not just
  trusted from application code. A partial unique index guarantees one
  cart per customer the same way.
- **Guest carts work today** even though there's no customer-login UI
  yet: `auth.OptionalAuth` (new this phase) reads a Bearer token if one's
  there but never rejects the request for lacking one — the same cart
  endpoints serve both a logged-in customer (Phase 7+, once login exists
  in the frontend) and today's guest shopper (`X-Cart-Token` header) with
  no branching per caller type at the routing level.
- **Stock *validation*, not reservation** — adding or updating a cart
  item checks the combined quantity against live `available_stock` and
  rejects (`409 INSUFFICIENT_STOCK`) rather than silently capping it.
  Actual reservation — incrementing `reserved_quantity`, releasing it on
  payment failure/expiry (section 21) — needs a checkout to protect and
  is explicitly Phase 7/8 work; a cart can sit untouched for days; locking
  real inventory against it the whole time would be wrong.
- **One branch per cart** (section 20's own rule): the first item added
  locks the cart to that branch; adding an item from a different branch
  is rejected (`409 DIFFERENT_BRANCH`) rather than silently mixing
  branches in one order-to-be. Clearing the cart releases the lock.
- **Price snapshotting**: `cart_items.unit_price` is captured once, when
  a line is first added, and stays put even if more of the same item
  gets added later or the branch price changes — while `available_stock`
  is still resolved live on every read, so out-of-stock warnings stay
  accurate without re-pricing anything.
- **Frontend**: a cart icon + badge in the header (visible at every
  width, unlike the branch selector, which the mobile menu absorbs), a
  dropdown panel with quantity controls, and a real "Tambah ke Keranjang"
  button on the product page — gated on a branch being selected, since
  every cart item needs one. `zustand` now also holds the guest cart's
  token (mirroring the branch-selector's `localStorage` pattern from
  Phase 5); actual cart *contents* stay server state, fetched through
  TanStack Query, never duplicated into Zustand.
- One real bug caught and fixed while building this, not shipped:
  `AddItem`'s branch-locking logic updated the database but not the
  in-memory struct handed to the response builder, so a cart's first item
  would have come back reporting `branch_id: null` — the exact bug this
  project's "hand-trace instead of assuming" verification habit exists to
  catch. See [Verification notes](#verification-notes).

Deliberately **not** in Phase 6: a variant picker on the product page —
every real product still has exactly one variant, so there's nothing to
pick between yet; and cart merging when a guest logs in (a real feature,
genuinely deferred rather than forgotten, since there's no login UI for
it to trigger from yet either).

## What Phase 7 adds

- **`orders`/`order_items`/`order_status_histories`** (section 5), with
  the snapshot discipline section 84 asks for taken seriously: an
  order's branch, price, product name, and variant name are all copied
  in at checkout and never re-derived from the live catalog — change a
  product's price tomorrow and every past order still shows what was
  actually paid.
- **Stock is *reserved*, not just checked, at checkout** (section 21):
  `SELECT ... FOR UPDATE` row-locks each `branch_inventory` row before
  reserving it, so two customers racing for the last unit can't both
  succeed — the second transaction blocks until the first commits, then
  sees the real, current number. This is new: Phase 6's cart only ever
  *validated* against stock; nothing was held until now.
- **Guest checkout works today**, same reasoning as guest carts (Phase
  6): there's no login UI yet, so `order_type: pickup` with a name and
  phone is genuinely how every order gets placed right now. A guest
  tracks their order afterward via `POST /orders/lookup`
  (order number + phone — the same bar plenty of real delivery trackers
  use) rather than an account.
- **A validated status graph** (section 24), covering the full lifecycle
  — not just what Phase 7 itself can trigger. `PENDING_PAYMENT → PAID`
  is reachable today (a cashier confirming cash at the counter, section
  28's POS flow — no gateway involved, so it's not overstepping into
  Phase 8's territory), and it's what actually deducts stock
  (`stock_quantity` and `reserved_quantity` both drop together, so
  `available_stock` doesn't move — the unit left "available" the moment
  it was reserved, this just makes that permanent). The rest of the
  graph (`PROCESSING → PACKED → ...`) is real and enforced but has no
  staff UI driving it yet — that's Phase 9/10's job; the API is ready
  for it.
- **Lazy + swept expiry**: a `PENDING_PAYMENT` order past its 30-minute
  window gets expired (and its stock released) the moment anyone fetches
  it — customer, staff, doesn't matter. `POST
  /admin/orders/sweep-expired` catches the rest, the orders nobody
  happens to look at; nothing here needs a background worker or a Redis
  TTL listener, just a query and, ideally, a cron hitting that endpoint
  periodically (see [Trying checkout](#trying-checkout) for how).
- **Frontend**: pickup-only checkout, right in the cart panel — name +
  phone, no address form. Delivery needs real field-level validation
  (React Hook Form + Zod, still just declared dependencies — this is
  genuinely the first form that would justify pulling them in) and isn't
  built yet; rather than hand-roll a second, throwaway address form to
  claim delivery "works," pickup is what's real today.
- Two more things caught by hand-tracing instead of trusting the first
  draft, not shipped as bugs: `Cancel` was calling the repository's raw
  `GetByID` instead of the service's lazy-expiry-aware one, so
  cancelling an order that had actually already timed out would have
  labeled it `CANCELLED` instead of correctly recognizing it as
  `EXPIRED` first; and an early draft of that same fix duplicated a
  validation check `UpdateStatus` already performs, caught and removed
  before it became two copies of the same rule to keep in sync. See
  [Verification notes](#verification-notes).

Deliberately **not** in Phase 7: real payment (QRIS, virtual account,
e-wallet — Phase 8's own scope, and `PENDING_PAYMENT` exists specifically
to wait for it); a saved, reusable address book (section 38's "Preferred
Store"-style customer feature — delivery orders capture an address
inline as an order snapshot, not from a reusable record, matching
section 84's philosophy anyway); order status emails/SMS (no notification
provider configured, same reasoning email verification got deferred in
Phase 2); and real-time status push over WebSocket (section 25 says "when
possible" — `pkg/websocket` is still a stub; polling `GET /orders/:id`
covers the same need today without standing up a second transport).

## What Phase 8 adds

- **A real `Provider` interface** (`internal/payments/provider.go`)
  matching section 26's own naming almost exactly — `CreatePayment`,
  `VerifyPayment`, `HandleWebhook`, `RefundPayment` — so business logic
  never touches a specific gateway's request/response shapes directly. A
  second provider means a new adapter package; nothing in
  `internal/orders` or the checkout flow would need to change.
- **Duitku as the first implementation**, not an arbitrary pick: this is
  the same gateway a previous project of the author's (a Laravel site,
  momenikah.online) integrated — including debugging its signature
  handling and payment method codes the hard way. That real prior
  experience is why Duitku was chosen as the reference adapter over
  picking an unfamiliar gateway at random, and why `internal/payments/duitku`
  goes out of its way to keep Duitku's **three different MD5 signature
  formulas** — invoice creation, callback verification, and status
  check, each ordering the same four values differently — in three
  separate, clearly-labeled functions rather than one "generic" signer.
  Mixing those up is a documented, easy mistake with this exact gateway.
- **QRIS and bank VA support**, using the payment method codes verified
  from that same prior integration (`SP` for QRIS, `BR` for BRIVA) plus a
  few more sourced from Duitku's public docs but not personally
  exercised — `internal/payments/duitku/methods.go` marks the difference
  explicitly rather than presenting a guess with the same confidence as
  a confirmed value.
- **`payments`/`payment_transactions`** (section 5): a payment record per
  attempt, and a verbatim audit log of every webhook delivery received —
  valid or not (section 82: "important operations must be audited").
  Webhook **idempotency** (section 89's "duplicate payment webhook" must
  not double-process) needed no new mechanism: it falls straight out of
  `orders.UpdateStatus`'s guarded transition from Phase 7 — a repeat
  "paid" callback finds the order already `PAID`, the guarded update
  affects zero rows, and that's treated as success, not an error.
- **Signature verified before anything else happens** — a webhook with a
  bad signature is logged (for security monitoring) and rejected before
  `internal/orders` is ever called, never trusted enough to touch order
  state. Amounts are checked too: a callback claiming a different amount
  than the order's actual total is rejected as a mismatch, not silently
  accepted (section 52: "never trust payment status from the frontend"
  extends to a spoofed backend call, not just a client-supplied field).
- **`RefundPayment` returns "not implemented," honestly** — it's part of
  the `Provider` interface because section 26 names it explicitly, but
  Duitku's actual refund endpoint needed its own research this project
  hasn't done, and a guessed implementation that might move real money
  incorrectly is worse than a clear error. `REFUNDED` is still a valid
  order status from Phase 7 (for recording a refund handled some other
  way) — this is specifically about triggering one *through* Duitku.
- **Two new config values with a real reason**: `PUBLIC_API_URL` and
  `PUBLIC_WEB_URL`, because a container has no way to know its own
  externally-reachable address — needed to build the callback URL Duitku
  calls back to and the return URL it redirects the customer to.
  `DUITKU_BASE_URL` defaults to Duitku's sandbox, so a blank/missing
  value can never accidentally point at production.

Deliberately **not** in Phase 8: any frontend UI for online payment (a QR
code on screen, a VA number to copy) — today's *only* working checkout
path is Phase 7's pickup-and-pay-at-counter, which doesn't touch this
package at all, so there's no real flow yet for a payment UI to sit in.
The backend is complete and independently verifiable via `curl` (see
[Trying payments](#trying-payments)); the frontend piece arrives
naturally once delivery checkout (needing real prepayment, unlike
pickup) gives it something to attach to. Also not fixed here: Duitku's
classic API requires an email address, and Phase 7's guest checkout only
collects name and phone — `CreatePayment` fails clearly on a missing
email rather than fabricating one, but the actual fix (prompting for
email specifically when a customer picks an online method) is frontend
work that arrives with the payment UI itself.

## What Phase 9 adds

- **Manual product CRUD** (`POST/PUT/DELETE /admin/products`) — the gap
  Phase 3 named directly in its own code: CSV import creates products in
  bulk, but nothing let staff create or fix a single one by hand. This
  reuses import's exact resolution helpers (`ensureBrand`,
  `categoriesRepo.EnsureByName`, `uniqueSlug`, `nameExists`), so a product
  typed into the admin form and a product from a CSV row can never
  resolve the same brand or category name to two different rows. Same
  atomicity as import too: a product and its one `Default` variant are
  inserted together in a transaction, so a product can never exist with
  zero variants, even for a moment. `AdminUpdate` never touches the slug
  (matching `branches.Update`'s existing convention) — renaming a product
  shouldn't quietly break a link someone bookmarked. Delete is soft
  (`deleted_at` + `status = 'archived'`), same as branches: `order_items`
  snapshot their product name/SKU at checkout time (section 84) rather
  than reading live, but `product_variants` still carries a real foreign
  key back to `products`, so the row has to keep existing.
- **Cross-branch order view** (`GET /admin/orders`, `GET
  /admin/orders/:id`, `PUT /admin/orders/:id/status`) — Phase 7 gave
  branch staff `/admin/branches/:id/orders`, scoped to their own branch;
  nothing let SUPER_ADMIN/ADMIN see every order at once. The single-order
  routes reuse `Service.GetByID`/`Service.UpdateStatus` directly,
  unchanged — an admin route just skips the branch-ownership check a
  branch-scoped route makes, same validated transition graph either way.
  The list is new — filtered and paginated, unlike
  `ListForBranch`/`ListForCustomer`, since "every order on the platform"
  has no natural bound the way one branch's or one customer's does.
- **`internal/dashboard`**, a new small package for the overview page's
  numbers — total revenue, order counts by status, product/branch/staff
  counts, and a low-stock count. Every number is a live query against a
  table another package already owns; nothing is cached, precomputed, or
  fabricated. Revenue counts every order from `PAID` onward except
  `REFUNDED` — money that was collected and stayed collected. Low-stock
  uses `branch_inventory.minimum_stock`, a real column Phase 4 already
  shipped but nothing read yet: it only counts rows where a branch
  actually set a threshold above zero, since counting the
  still-default-zero rows too would flag nearly everything and tell
  staff nothing useful.
- **Staff account management** (`GET/POST /admin/users`, `GET/PUT
  /admin/users/:id`) — `internal/users`'s own repository doc comment said
  this plainly before today: *"a real admin-facing 'create staff user'
  endpoint is Phase 9 work."* Creating or editing an account that grants
  the `SUPER_ADMIN` role is restricted to a caller who already holds
  `SUPER_ADMIN` — without that check, any `ADMIN` (which `adminGroup`
  already lets manage staff) could mint themselves a `SUPER_ADMIN`
  account. A new `StaffAccount` response shape strips `PasswordHash` —
  the existing `User` struct has no JSON tags at all, because before this
  phase nothing ever serialized one directly over the API. `Update` has
  no email or password field on purpose (see below).
- **Frontend**: a full `/admin` section — staff login, an auth guard that
  re-confirms roles against `GET /auth/me` (not just whatever
  `localStorage` still remembers) rather than trusting the access token's
  own claims, a token store with one automatic refresh-and-retry on a 401
  before giving up, and pages for the dashboard overview, products,
  orders (list + detail + status update), branches (Phase 4's branch CRUD
  gets its first frontend here), and staff. `app/layout.tsx` needed one
  small change: it's a Server Component, so it can't call `usePathname()`
  itself to hide the shop's header/footer on `/admin/*` routes — a new
  `SiteChrome` client component does that instead, and renders
  byte-for-byte what `RootLayout` rendered inline before for every
  non-admin route. React Hook Form + Zod are still just declared
  dependencies — every admin form here has few enough fields and no
  cross-field validation that plain controlled inputs (the same pattern
  `cart-panel.tsx` already uses) were genuinely enough; the next form
  with real field-level validation needs is still the likely candidate.
- **One pre-existing bug fixed in passing**: `go test ./...` — runnable
  in full for the first time this phase, see
  [Verification notes](#verification-notes) — caught an off-by-one in
  `internal/products/import.go`'s `ParseCSV` from Phase 3: `rowNum`
  incremented *before* being recorded, so row 1 of a CSV was reported as
  row 2, row 2 as row 3, and so on, in every `"row N: ..."` error message.
  Fixed directly in the file this phase already touches, rather than left
  as a rediscovered problem for Phase 10.

Deliberately **not** in Phase 9: a few real gaps, named rather than
papered over. Nothing stops a `SUPER_ADMIN` from deactivating or
demoting the platform's last other `SUPER_ADMIN`, or from editing their
own account through `PUT /admin/users/:id` — a real safeguard for either
needs a "how many active SUPER_ADMINs remain" check this phase doesn't
add. Staff password reset and email verification aren't built — the same
reasoning Phase 2 gave for deferring customer email verification (no
notification provider configured) applies, plus resetting your own
password is a "prove it's really you" flow, meaningfully different from
an admin setting someone else's password on creation. A
`BRANCH_MANAGER`-scoped dashboard variant doesn't exist — that role
already has its own narrower, branch-scoped endpoints from Phases 4 and
7 (inventory, branch orders); a real dashboard *for* that scope is future
work, not a cut corner in this one, which is what the roadmap literally
calls "Admin dashboard." Fine-grained permission-code checks and general
admin-action audit logging are still open from Phase 8's own "Still
ahead" list — Phase 9 adds plenty of new admin-surface actions worth
auditing, but building the audit system itself is its own piece of work,
not a side effect of using it.

## What Phase 10 adds

- **Cashier shifts** (`internal/shifts`, new) — a till session: how much
  cash a cashier started with, and (once closed) how much was actually
  counted against what the system expected from cash sales recorded
  during that window. One open shift per cashier at a time is enforced
  by a partial unique index at the database level (migration
  000008_pos), not just application logic — a second open attempt hits a
  real constraint violation, translated into a normal `409
  SHIFT_ALREADY_OPEN` rather than a raw pg error leaking out.
- **POS checkout** (`POST /admin/branches/:id/pos/checkout`) reuses
  `orders.Service.Checkout` entirely rather than duplicating it — the POS
  screen manages its own cart through the exact same `X-Cart-Token`
  mechanism a guest customer's browser already uses (Phase 6), just
  scoped to whichever branch the cashier is working. The one new thing
  this handler does is find the caller's *own* open shift server-side
  (`shifts.GetOpenForUser`) and stamp its id onto the checkout request —
  a shift id is never accepted from the client at all
  (`CheckoutRequest.CashierShiftID` has `json:"-"`), so there's no way to
  attach a sale to someone else's shift just by knowing or guessing its
  UUID.
- **Cash vs. QRIS, and how a shift knows which sales were cash**: cash
  still never touches `internal/payments` at all — confirmed by that
  package's own doc comment, unchanged since Phase 8, which already
  says cash goes straight through orders' own PENDING_PAYMENT → PAID
  transition rather than through a provider. What's new is a
  `payment_method` column directly on `orders` (`'cash'` | `'qris'` |
  null), written as a small, best-effort step *after* the real status
  transition already committed — by the branch-scoped `UpdateStatus`
  handler when a cash confirm includes it, and by the Duitku webhook
  handler when an online/QRIS payment clears. That column is what a
  shift's close-time reconciliation query actually sums, and it's why
  `UpdateStatus` itself needed no changes to its guarded transition
  logic at all — every existing call site (Phases 7-9) is unaffected by
  a column that only ever gets written by two new, additive follow-up
  calls after the fact.
- **QRIS-at-counter** (`POST /admin/branches/:id/orders/:orderId/pay`) —
  `payments.Handler.Pay` (Phase 8) requires a logged-in *customer* who
  owns the order; a POS sale has neither, so this is a new sibling
  endpoint authorized by branch access instead of ownership, calling the
  exact same `Service.CreatePaymentForOrder`. The POS screen renders the
  QR client-side from the raw `qr_string` Duitku returns
  (`qrcode.react`, the one new frontend dependency this phase adds) and
  polls the existing branch-scoped order-detail endpoint every few
  seconds until status flips to `PAID` — no new polling endpoint needed.
- **Frontend**: a full `/pos` section with its own shell
  (`app/pos/layout.tsx`), deliberately separate from `/admin`'s — a
  cashier's till screen showing the back-office nav would be as
  confusing as the shop's own header would be here, so `SiteChrome`
  (Phase 9) now hides the customer chrome for `/pos/*` the same way it
  already does for `/admin/*`. The flow: pick a branch (reusing the same
  branch-store the shop already has) → open a shift if none is open yet
  → search products, build a sale, check out with cash or QRIS → a
  receipt screen with a browser-native "Cetak" button (`window.print()`,
  with a `print:hidden` class on the buttons themselves so only the
  receipt prints) → close the shift at end of day to see the
  reconciliation summary (expected vs. counted vs. discrepancy).
- **One incidental discipline check while touching `orders`**: adding
  `payment_method` and `cashier_shift_id` to the shared
  `orderColumns`/`scanOrder` (used by every order read path — `GetByID`,
  `ListForBranch`, `ListForCustomer`, Phase 9's `ListAdmin`, all of it)
  meant hand-counting the SELECT column list against `Scan()`'s argument
  list one more time — same SQL-correctness discipline as every other
  phase, see [Verification notes](#verification-notes).

Deliberately **not** in Phase 10: cash-drawer pay-outs mid-shift (petty
cash taken out before closing) aren't tracked, so the reconciliation math
assumes every rupiah that came in during the shift is still there at
close time — a real deployment eventually needs a way to record those
separately. There's no receipt-printer integration (thermal or
otherwise) — screen display and the browser's own print-to-PDF is what
this phase's scope asked for; a real thermal-printer integration is a
meaningfully different, hardware-specific piece of work. A POS sale is
always guest/walk-in — there's no "look up an existing customer by
phone" step at the counter the way Phase 7's guest checkout has one for
online orders; adding one is straightforward later but wasn't asked for
here. And the QRIS-at-counter flow has no idempotency guard — calling
`CreatePaymentForOrder` twice for the same order (a cashier's browser
retrying, or clicking "Bayar QRIS" twice) creates two separate payment
attempts at the gateway; a real deployment probably wants a guard
against that before going live.

## Tech stack

**Backend** — Go, Gin, PostgreSQL (pgx), Redis, JWT (Phase 2), WebSocket
(later phases), Docker.
**Frontend** — Next.js 16 (App Router), TypeScript, Tailwind CSS v4,
TanStack Query and Zustand (both wired up since Phase 5 — data fetching
and the branch/cart-token/auth-token stores), shadcn/ui (foundation only
— components added on demand via its CLI), `qrcode.react` (Phase 10, for
rendering a QRIS payload as an on-screen QR code client-side). React
Hook Form + Zod are still just declared: Phase 6's cart only needed a
+/− stepper, Phase 7's checkout form and Phase 9's admin forms all
turned out simple enough for plain controlled inputs, so they're still
waiting on the first form that actually needs real field-level or
cross-field validation.
**Mobile** — Capacitor wrapping the same Next.js app for Android/iOS.
No Flutter, no Laravel Blade — see the spec this was built from.

## Requirements

- Docker + Docker Compose (runs Postgres, Redis, the API, and the web app)
- Node.js 20+ and Go 1.22+ if you want to run either app outside Docker
- `go mod tidy` and `npm install` both need normal internet access — this
  scaffold was built and syntax/type/lint-checked in a network-restricted
  sandbox (see [Verification notes](#verification-notes)), so both steps
  below are things to run for the first time on your own machine.

## Installation

```bash
git clone <your-repo-url> satelit-parfume
cd satelit-parfume
cp .env.example .env
docker compose up -d --build
```

That builds and starts Postgres, Redis, the API, and the web app.

- Web: http://localhost:3000
- API health: http://localhost:8080/api/v1/health
- Through Nginx: http://localhost/

## Environment variables

See `.env.example` at the repo root for the full list (database, Redis,
JWT secrets + token TTLs, storage, payment keys, ports, CORS). Copy it to
`.env` before first run. Storage and payment groups are still placeholders
until the phase that uses them lands; database, Redis, JWT, port, and CORS
values are all actively read as of Phase 2.

## Database setup

Seven migrations so far:
- `000001_init` — enables the `pgcrypto` extension.
- `000002_auth` — roles, permissions, users, customers, refresh_tokens,
  plus the 5 staff roles as reference data.
- `000003_catalog` — brands, categories, products, variants, images.
- `000004_branches` — branches, `branch_staff`, `branch_inventory`, plus
  adding `categories.deleted_at` (a Phase 3 gap — see
  [What Phase 4 adds](#what-phase-4-adds)).
- `000005_cart` — `carts`, `cart_items`, with a `CHECK` constraint
  enforcing "customer or guest session, never both, never neither."
- `000006_orders` — `orders`, `order_items`, `order_status_histories`,
  and `order_number_counters` (backs atomic, human-friendly order
  numbers — see [What Phase 7 adds](#what-phase-7-adds)).
- `000007_payments` — `payments` (one row per attempt), `payment_transactions`
  (verbatim webhook audit log — see
  [What Phase 8 adds](#what-phase-8-adds)).

Remaining domain tables (promotions, memberships, reviews, ...) arrive
with their own phases, each as its own migration.

Recommended: [golang-migrate](https://github.com/golang-migrate/migrate).

```bash
docker run --rm -v $(pwd)/apps/api/migrations:/migrations \
  --network host migrate/migrate \
  -path=/migrations -database "postgres://satelit:satelit@localhost:5432/satelit_parfume?sslmode=disable" up
```

(Or install the `migrate` CLI locally and point it at the same path/URL.)

**Dev seed data** (optional, for testing the auth flow locally — see
`apps/api/seeds/dev_seed.sql` for the full "not real Satelit Parfume data"
disclaimer):

```bash
docker compose exec -T postgres psql -U satelit -d satelit_parfume < apps/api/seeds/dev_seed.sql
```

Creates `admin@satelitparfume.dev` (staff, SUPER_ADMIN) and
`customer@satelitparfume.dev` (customer), both password `ChangeMe123!`.

## Running in development

**With Docker (recommended):**
```bash
docker compose up -d
docker compose logs -f api web
```

**Without Docker:**
```bash
# API
cd apps/api
go mod tidy        # fetches deps + writes go.sum — needs real internet
go run ./cmd/api

# Web (separate terminal)
cd apps/web
npm install
npm run dev
```

## Trying the auth flow

With the stack up, migrations applied, and the dev seed loaded:

```bash
# Register a new customer (or use the seeded customer@satelitparfume.dev)
curl -sX POST localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Test Buyer","email":"buyer@example.com","password":"password123"}' | jq

# Staff login (uses the dev seed account)
curl -sX POST localhost:8080/api/v1/auth/staff/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@satelitparfume.dev","password":"ChangeMe123!"}' | jq

# → copy access_token from the response, then:
curl -s localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer <access_token>" | jq

# Role-gated route — 200 as SUPER_ADMIN/ADMIN, 403 as a customer or with no token:
curl -s localhost:8080/api/v1/admin/ping \
  -H "Authorization: Bearer <access_token>" | jq

# Refresh (rotates — the old refresh_token stops working after this):
curl -sX POST localhost:8080/api/v1/auth/refresh \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"<refresh_token>"}' | jq
```

## Trying the catalog

Import the real verified products (needs a staff access token — see
above and `apps/api/seeds/README.md`):

```bash
curl -sX POST localhost:8080/api/v1/admin/products/import \
  -H "Authorization: Bearer <access_token>" \
  -F "file=@apps/api/seeds/products_verified.csv" | jq
# → {"imported": 23, "skipped": 0, "errors": []}
```

Then browse what just got imported — no auth needed for any of this:

```bash
curl -s "localhost:8080/api/v1/products?search=konos" | jq
curl -s "localhost:8080/api/v1/products?category=mix-perfume&sort=price_desc" | jq
curl -s localhost:8080/api/v1/products/aqua-kiss | jq
curl -s localhost:8080/api/v1/categories | jq
```

## Trying branches and inventory

```bash
# Create a branch (staff access token, same as above):
curl -sX POST localhost:8080/api/v1/admin/branches \
  -H "Authorization: Bearer <access_token>" -H 'Content-Type: application/json' \
  -d '{"name":"Satelit Parfume Bekasi","code":"BKS","city":"Bekasi","latitude":-6.2383,"longitude":107.0}' | jq
# → note the returned "id" and "slug"

# Public: list branches, nearest-first if you pass a location
curl -s localhost:8080/api/v1/branches | jq
curl -s "localhost:8080/api/v1/branches?lat=-6.2&lng=106.9" | jq
curl -s localhost:8080/api/v1/branches/satelit-parfume-bekasi | jq

# Assign the dev admin as staff at this branch, then set stock for a variant
# (grab a product_variant_id from a product detail response first):
curl -sX POST localhost:8080/api/v1/admin/branches/<branch_id>/staff \
  -H "Authorization: Bearer <access_token>" -H 'Content-Type: application/json' \
  -d '{"user_id":"<staff_user_id>"}' | jq

curl -sX PUT localhost:8080/api/v1/admin/branches/<branch_id>/inventory/<variant_id> \
  -H "Authorization: Bearer <access_token>" -H 'Content-Type: application/json' \
  -d '{"stock_quantity":12,"minimum_stock":3}' | jq

curl -s localhost:8080/api/v1/admin/branches/<branch_id>/inventory \
  -H "Authorization: Bearer <access_token>" | jq

# Public: that stock now shows up on the product itself
curl -s "localhost:8080/api/v1/products/aqua-kiss?branch=satelit-parfume-bekasi" | jq
```

Try the last two with a *different* staff account that isn't assigned to
this branch (and isn't SUPER_ADMIN/ADMIN) to see `RequireBranchAccess`
reject it with `403 FORBIDDEN`.

## Trying the shop

This one's a browser, not `curl`. With the stack up and the catalog
imported:

1. Open `localhost:3000` — the real landing page, not Phase 1's
   placeholder.
2. Click the store selector (top right) → pick a branch, or try "Gunakan
   lokasiku" if you set one up with real coordinates.
3. Go to **Belanja** — search, filter by category, sort, and (once a
   branch is selected) see live stock badges on each card.
4. Click into a product — the same branch-aware availability shows up on
   the detail page.
5. Refresh the page. The selected branch survives — that's the
   `localStorage` persistence, not a fluke.

If you haven't imported the catalog or created a branch yet, every
section above shows an honest empty state instead of a blank screen or
fake data — worth seeing once, since a fresh `docker compose up` starts
from exactly that state.

## Trying the cart

Also a browser thing, continuing right on from the shop walkthrough
above:

1. On any product page, with a branch selected, use the +/− stepper and
   click **Tambah ke Keranjang**.
2. The cart icon in the header picks up a badge immediately — click it
   to open the panel, adjust quantity with +/−, or remove a line.
3. Try pushing quantity past what's shown as available — the button
   disables rather than letting you queue up more than the branch has.
4. Refresh the page. The cart survives (it's server-side, keyed by a
   token `localStorage` remembers), the same way the branch selection
   does.
5. Open dev tools → Application → Local Storage, and you'll see
   `satelit-parfume-cart-token` sitting next to
   `satelit-parfume-branch` — that token is also usable directly:

```bash
curl -s localhost:8080/api/v1/cart -H "X-Cart-Token: <token from localStorage>" | jq
```

## Trying checkout

**In the browser**, continuing right on from the cart above: with items
in your cart, click **Checkout (Ambil di Toko)**, fill in a name and
phone, and confirm. You'll get an order number back and a note that
payment happens at the counter — that's real, not a placeholder message.

**As a staff member (curl)**, confirming that order and watching stock
actually deduct:

```bash
# Staff login (see "Trying the auth flow" above for the dev account),
# then find the order:
curl -s localhost:8080/api/v1/admin/branches/<branch_id>/orders \
  -H "Authorization: Bearer <access_token>" | jq

# Confirm cash payment was received — this is the transition that
# deducts stock (stock_quantity and reserved_quantity both drop):
curl -sX PUT localhost:8080/api/v1/admin/branches/<branch_id>/orders/<order_id>/status \
  -H "Authorization: Bearer <access_token>" -H 'Content-Type: application/json' \
  -d '{"status":"PAID","note":"cash received at counter"}' | jq

# Check branch_inventory before/after — available_stock is unchanged
# (the unit was already excluded from it the moment it was reserved),
# but stock_quantity is now genuinely lower:
curl -s localhost:8080/api/v1/admin/branches/<branch_id>/inventory \
  -H "Authorization: Bearer <access_token>" | jq
```

**Testing expiry** without waiting 30 minutes: place an order, then in
`psql` (`docker compose exec postgres psql -U satelit -d satelit_parfume`)
run `UPDATE orders SET expires_at = now() - interval '1 minute' WHERE
order_number = '<your order number>';`, then either refresh the order in
the browser (lazy expiry) or:

```bash
curl -sX POST localhost:8080/api/v1/admin/orders/sweep-expired \
  -H "Authorization: Bearer <access_token>" | jq
```

Check the branch's inventory again — `reserved_quantity` should have
dropped back down, released by the expiry.

**Scheduling the sweep for real**: nothing in this project runs it
automatically — wire up a cron, a scheduled task, or your platform's
equivalent to `POST /api/v1/admin/orders/sweep-expired` every few
minutes. It's safe to call as often as you like; an empty result costs
one cheap query.

## Trying payments

Needs real (free) Duitku sandbox credentials — sign up at
[duitku.com](https://duitku.com), grab a sandbox merchant code + key,
and put them in `.env` as `DUITKU_MERCHANT_CODE`/`DUITKU_MERCHANT_KEY`.

**Creating a QRIS payment** for a `PENDING_PAYMENT` order (needs a
customer account, since guest checkout — Phase 7 — doesn't collect an
email and Duitku requires one; register a customer via `POST
/api/v1/auth/register` first, then check out while logged in):

```bash
curl -sX POST localhost:8080/api/v1/orders/<order_id>/pay \
  -H "Authorization: Bearer <customer_access_token>" -H 'Content-Type: application/json' \
  -d '{"payment_method":"SP"}' | jq
# → { "qr_string": "...", "reference": "...", ... } — the qr_string is
#   what a real QRIS-scanning app would render; there's no frontend UI
#   for it yet (see "What Phase 8 adds").

curl -s localhost:8080/api/v1/orders/<order_id>/payment \
  -H "Authorization: Bearer <customer_access_token>" | jq
```

**Receiving the webhook**: Duitku's sandbox needs to reach
`PUBLIC_API_URL` from the public internet, which `localhost` never is.
For local testing, run a tunnel (`ngrok http 8080` or similar), set
`PUBLIC_API_URL` to the tunnel's HTTPS URL, restart the API, and pay the
sandbox QR through Duitku's own simulator — their sandbox dashboard has
a way to trigger a test callback for a given reference.

**Simulating a webhook by hand** instead, without a tunnel — this
exercises the exact same signature verification and idempotency path
Duitku's real callback would:

```bash
# Compute the callback signature the way Duitku does:
# MD5(merchantCode + amount + merchantOrderId + merchantKey)
python3 -c "
import hashlib
merchant_code, amount, order_number, merchant_key = 'D1234', '35000', '<order_number>', '<your merchant key>'
print(hashlib.md5(f'{merchant_code}{amount}{order_number}{merchant_key}'.encode()).hexdigest())
"

curl -sX POST localhost:8080/api/v1/payments/webhook/duitku \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode "merchantCode=D1234" \
  --data-urlencode "merchantOrderId=<order_number>" \
  --data-urlencode "amount=35000" \
  --data-urlencode "resultCode=00" \
  --data-urlencode "reference=TEST-REF-001" \
  --data-urlencode "signature=<computed signature from above>"
```

Check the order afterward (`GET /api/v1/orders/<order_id>`) — status
should now be `PAID`, and the branch's inventory should show
`stock_quantity` actually reduced (not just `reserved_quantity`), per
[What Phase 7 adds](#what-phase-7-adds)'s deduct-on-`PAID` behavior.
Send the exact same request a second time — it should still return `200`
but leave everything unchanged, proving the idempotency behavior
described in [What Phase 8 adds](#what-phase-8-adds).

## Trying the admin dashboard

The dev seed's `admin@satelitparfume.dev` / `ChangeMe123!` (SUPER_ADMIN —
see `seeds/dev_seed.sql`) works for all of this, either through `curl` or
by visiting `/admin/login` in the browser.

**Logging in and checking the dashboard stats:**

```bash
curl -sX POST localhost:8080/api/v1/auth/staff/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@satelitparfume.dev","password":"ChangeMe123!"}' | jq
# → { "subject": {...}, "tokens": { "access_token": "...", ... } }

curl -s localhost:8080/api/v1/admin/dashboard/stats \
  -H "Authorization: Bearer <access_token>" | jq
# → { "revenue": ..., "order_counts": {"PAID": 2, ...}, "total_products": 23, ... }
```

**Creating a product by hand** (the manual path Phase 3 deferred —
`category`/`brand` resolve exactly like a CSV row would, creating either
if it doesn't already exist):

```bash
curl -sX POST localhost:8080/api/v1/admin/products \
  -H "Authorization: Bearer <access_token>" -H 'Content-Type: application/json' \
  -d '{"name":"Contoh Parfum 50ml","category":"Eau de Parfum","brand":"Contoh Brand","price":45000,"is_featured":false,"is_bestseller":false}' | jq
```

**Seeing every order across every branch** (Phase 7's own admin routes
are branch-scoped — `/admin/branches/:id/orders` — this one isn't):

```bash
curl -s "localhost:8080/api/v1/admin/orders?status=PENDING_PAYMENT&limit=5" \
  -H "Authorization: Bearer <access_token>" | jq
```

**Creating a staff account**, and seeing the SUPER_ADMIN safeguard reject
an ADMIN trying to grant it — the dev seed only creates one SUPER_ADMIN,
so this makes a second account with the ADMIN role first, logs in as
that one, then tries to use it to create a third account with the
SUPER_ADMIN role itself:

```bash
curl -sX POST localhost:8080/api/v1/admin/users \
  -H "Authorization: Bearer <super_admin_access_token>" -H 'Content-Type: application/json' \
  -d '{"name":"Admin Kedua","email":"admin2@satelitparfume.dev","password":"ChangeMe123!","roles":["ADMIN"]}' | jq

curl -sX POST localhost:8080/api/v1/auth/staff/login -H 'Content-Type: application/json' \
  -d '{"email":"admin2@satelitparfume.dev","password":"ChangeMe123!"}' | jq
# → grab this ADMIN's own access_token

curl -sX POST localhost:8080/api/v1/admin/users \
  -H "Authorization: Bearer <admin2_access_token>" -H 'Content-Type: application/json' \
  -d '{"name":"Mencoba Jadi Super Admin","email":"nope@satelitparfume.dev","password":"ChangeMe123!","roles":["SUPER_ADMIN"]}' | jq
# → 403 FORBIDDEN — an ADMIN can create CASHIER/BRANCH_MANAGER/etc.
#   accounts freely, but can't mint a SUPER_ADMIN.
```

## Trying the POS

Everything below uses the same `admin@satelitparfume.dev` login as the
admin dashboard section above (SUPER_ADMIN already has CASHIER-level
access to every POS route) and a `<branch_id>` from `GET /branches` —
either one you created following
[Trying branches and inventory](#trying-branches-and-inventory), or one
already in the database.

**Opening a shift, then trying to open a second one:**

```bash
curl -sX POST "localhost:8080/api/v1/admin/branches/<branch_id>/shifts" \
  -H "Authorization: Bearer <access_token>" -H 'Content-Type: application/json' \
  -d '{"opening_balance": 200000, "notes": "Shift pagi"}' | jq

curl -sX POST "localhost:8080/api/v1/admin/branches/<branch_id>/shifts" \
  -H "Authorization: Bearer <access_token>" -H 'Content-Type: application/json' \
  -d '{"opening_balance": 100000}' | jq
# → 409 SHIFT_ALREADY_OPEN — the partial unique index in migration
#   000008_pos catches this at the database level, not just in Go.
```

**Ringing up a cash sale** — build a cart exactly like a customer would
(same `X-Cart-Token` mechanism, see
[Trying the cart](#trying-the-cart)), then check out through the POS
route and confirm cash:

```bash
curl -sX POST localhost:8080/api/v1/cart/items -H 'Content-Type: application/json' \
  -d '{"product_variant_id":"<variant_id>","branch_id":"<branch_id>","quantity":1}' | jq
# → note the returned session_token

curl -sX POST "localhost:8080/api/v1/admin/branches/<branch_id>/pos/checkout" \
  -H "Authorization: Bearer <access_token>" -H "X-Cart-Token: <session_token>" \
  -H 'Content-Type: application/json' -d '{"order_type":"pickup"}' | jq
# → order created as PENDING_PAYMENT; note its id

curl -sX PUT "localhost:8080/api/v1/admin/branches/<branch_id>/orders/<order_id>/status" \
  -H "Authorization: Bearer <access_token>" -H 'Content-Type: application/json' \
  -d '{"status":"PAID","payment_method":"cash"}' | jq
# → PAID, and this order now counts toward the open shift's expected cash
```

**Ringing up a QRIS sale** instead of cash — same checkout, but pay via
the new branch-scoped payment endpoint rather than confirming cash:

```bash
curl -sX POST "localhost:8080/api/v1/admin/branches/<branch_id>/orders/<order_id>/pay" \
  -H "Authorization: Bearer <access_token>" -H 'Content-Type: application/json' \
  -d '{"payment_method":"SP"}' | jq
# → { "qr_string": "...", ... } — this is what app/pos/page.tsx renders
#   as a QR code client-side and polls the order for
```

**Closing the shift** and seeing the reconciliation — `expected_balance`
below should equal the cash sale above (the QRIS one doesn't count, by
design):

```bash
curl -sX PUT "localhost:8080/api/v1/admin/branches/<branch_id>/shifts/<shift_id>/close" \
  -H "Authorization: Bearer <access_token>" -H 'Content-Type: application/json' \
  -d '{"closing_balance": 245000, "notes": "Tutup shift pagi"}' | jq
# → { "opening_balance": 200000, "expected_balance": 245000,
#     "closing_balance": 245000, "discrepancy": 0, "status": "closed" }
```

## Testing

```bash
cd apps/api && go test ./...
```

`pkg/jwt`, `pkg/password`, `pkg/slug`, `internal/products` (its CSV
parsing + row validation), `internal/orders` (its status-transition
graph — every edge, the terminal states, and the pickup/delivery
type-matching rule), and `internal/payments/duitku` (all three signature
formulas, each checked against a value computed independently in Python —
not by reading the implementation back to itself — plus a test that
specifically asserts the three formulas produce *different* output for
the same inputs, the exact property that matters here) all have real
unit tests — issue/verify round-trips, wrong-secret rejection, expiry,
unique JTIs, bcrypt salting, slug edge cases, malformed/valid CSV rows,
valid and invalid status transitions, signature correctness — all pure
logic, no database needed, so they're the first thing worth running
after `go mod tidy`. Phases 4-6 (branches, inventory, shop frontend,
cart) added no test files of their own — almost entirely
database-touching CRUD, a SQL distance formula, and UI — see
[Verification notes](#verification-notes) for how each of those phases'
correctness was actually checked instead. Handler/service-level tests
that need a real database (register → login → refresh → revoke; import →
search → filter; branch-scoped access denial; checkout's stock
reservation under concurrent requests; webhook idempotency end to end)
are a good next addition,
alongside the rest of the spec's critical test list (payment webhooks,
duplicate delivery, ...) as those features land. Phase 9 added no new
test files either (the manual product CRUD, admin order list, dashboard
stats, and staff management are all straightforward database-touching
CRUD in the same shape as Phases 4-6) — but this is the first phase
where `go test ./...` actually ran, full stop, rather than staying a
"run this yourself" instruction; see
[Verification notes](#verification-notes) for how, and for the one
pre-existing bug (Phase 3's `ParseCSV`) it caught doing it. Phase 10 is
the same story as Phase 9 for test files — cashier shifts and POS
checkout are, again, database-touching CRUD reusing an already-tested
checkout path rather than new pure logic — but see that same
Verification notes section for exactly what got hand-checked instead
(every new SQL column against its migration, gin's routing tree for the
new branch-scoped paths).

## Building

```bash
# API
cd apps/api && go build -o bin/api ./cmd/api

# Web
cd apps/web && npm run build
```

## Docker

`docker-compose.yml` at the repo root defines five services: `postgres`,
`redis`, `api`, `web`, `nginx`. Both `api` and `web` build from their own
multi-stage Dockerfiles (`apps/api/Dockerfile`, `apps/web/Dockerfile`).
`nginx` reverse-proxies `/api/*` and `/ws` to the API and everything else
to the web app (`docker/nginx/nginx.conf`).

## Mobile build

Not started — Capacitor gets wired up in Phase 14, once there's an actual
storefront worth shipping in an app shell.

## API documentation

Not generated (OpenAPI/Swagger) yet — worth setting up once the surface
is bigger. For now, everything that exists:

| Method & path | Auth | Notes |
|---|---|---|
| `GET /api/v1/health` | none | API + database + Redis status |
| `POST /api/v1/auth/register` | none | creates a **customer** account, returns tokens |
| `POST /api/v1/auth/login` | none | customer login |
| `POST /api/v1/auth/staff/login` | none | staff login (separate from customer login) |
| `POST /api/v1/auth/refresh` | refresh token in body | rotates — old refresh token is revoked |
| `POST /api/v1/auth/logout` | refresh token in body | revokes it; always "succeeds" |
| `GET /api/v1/auth/me` | Bearer access token | current subject, re-fetched live (not from token claims) |
| `GET /api/v1/products` | none | search/filter/sort/paginate. Add `?branch=<slug>` for per-item stock (Phase 5) |
| `GET /api/v1/products/:slug` | none | full detail incl. variants + images. Add `?branch=<slug>` for that branch's stock+price |
| `GET /api/v1/categories` | none | flat list, for filter dropdowns |
| `GET /api/v1/branches` | none | active branches; `?lat=&lng=` sorts nearest-first |
| `GET /api/v1/branches/:slug` | none | single branch detail |
| `POST /api/v1/admin/products/import` | Bearer + `SUPER_ADMIN`/`ADMIN` role | multipart CSV upload, see [Trying the catalog](#trying-the-catalog) |
| `POST /api/v1/admin/branches` | Bearer + `SUPER_ADMIN`/`ADMIN` role | create a branch |
| `PUT /api/v1/admin/branches/:id` | Bearer + `SUPER_ADMIN`/`ADMIN` role | full-replace update |
| `DELETE /api/v1/admin/branches/:id` | Bearer + `SUPER_ADMIN`/`ADMIN` role | soft delete |
| `POST /.../branches/:id/staff` | Bearer + `SUPER_ADMIN`/`ADMIN` role | assign a staff user to this branch |
| `DELETE /.../branches/:id/staff/:userId` | Bearer + `SUPER_ADMIN`/`ADMIN` role | unassign |
| `GET /.../branches/:id/staff` | Bearer + role + assigned to `:id` | roster; SUPER_ADMIN/ADMIN/BRANCH_MANAGER |
| `GET /.../branches/:id/inventory` | Bearer + role + assigned to `:id` | full stock detail; +CASHIER, +INVENTORY_STAFF |
| `PUT /.../branches/:id/inventory/:variantId` | Bearer + role + assigned to `:id` | set stock; not CASHIER |
| `GET /api/v1/admin/ping` | Bearer + `SUPER_ADMIN`/`ADMIN` role | RBAC smoke test, see [What Phase 2 adds](#what-phase-2-adds) |
| `GET /api/v1/cart` | optional Bearer, or `X-Cart-Token` | current cart; guests with no token get a new cart + `session_token` back |
| `POST /api/v1/cart/items` | optional Bearer, or `X-Cart-Token` | add/increase a line; validates stock, enforces one branch per cart |
| `PUT /.../cart/items/:itemId` | optional Bearer, or `X-Cart-Token` | set a line's quantity |
| `DELETE /.../cart/items/:itemId` | optional Bearer, or `X-Cart-Token` | remove a line |
| `DELETE /api/v1/cart` | optional Bearer, or `X-Cart-Token` | clear the cart and release its branch lock |
| `POST /api/v1/orders` | optional Bearer, or `X-Cart-Token` | checkout — reserves stock, snapshots the cart into an order |
| `POST /api/v1/orders/lookup` | none | guest order tracking by order number + phone |
| `GET /api/v1/orders` | Bearer (customer) | your own orders, list view |
| `GET /api/v1/orders/:id` | Bearer (customer) | your own order, full detail — 403 if it isn't yours |
| `POST /api/v1/orders/:id/cancel` | Bearer (customer) | cancel — only valid from `PENDING_PAYMENT` |
| `GET /.../branches/:id/orders` | Bearer + role + assigned to `:id` | branch's orders; +CASHIER |
| `GET /.../branches/:id/orders/:orderId` | Bearer + role + assigned to `:id` | single order — 404 if it belongs to a different branch |
| `PUT /.../branches/:id/orders/:orderId/status` | Bearer + role + assigned to `:id` | validated status transition; `-> PAID` deducts stock |
| `POST /api/v1/admin/orders/sweep-expired` | Bearer + `SUPER_ADMIN`/`ADMIN` role | releases stock held by timed-out `PENDING_PAYMENT` orders — see [Trying checkout](#trying-checkout) |
| `POST /api/v1/orders/:id/pay` | Bearer (customer) | starts an online payment via Duitku — QR string, VA number, or redirect URL back, depending on method |
| `GET /api/v1/orders/:id/payment` | Bearer (customer) | current payment status for the order, for the frontend to poll |
| `POST /api/v1/payments/webhook/duitku` | none — verified by signature instead | Duitku's callback; see [Trying payments](#trying-payments) |

"assigned to `:id`" means `branches.RequireBranchAccess`: SUPER_ADMIN/ADMIN
reach any branch; other qualifying roles only reach branches they're
actually assigned to via `branch_staff` — see
[What Phase 4 adds](#what-phase-4-adds).

Every response uses the same envelope:
`{ "success": bool, "message": string, "code"?: string, "data": ... }` —
`code` is only present on errors (e.g. `INVALID_CREDENTIALS`,
`EMAIL_TAKEN`, `TOO_MANY_ATTEMPTS`), for clients that want to branch on
it instead of parsing `message`.

## Project structure

```text
satelit-parfume/
├── apps/
│   ├── api/                  Go backend
│   │   ├── cmd/api/          entrypoint (main.go) — wires every module together
│   │   ├── internal/
│   │   │   ├── auth/         register, login (staff+customer), refresh, logout, RBAC middleware
│   │   │   ├── users/        staff accounts — CRUD + role assignment as of Phase 9 (previously read-only)
│   │   │   ├── customers/    customer account model + repository
│   │   │   ├── categories/   category model + repository (auto-created on import)
│   │   │   ├── products/     products, variants, images, brands, CSV import, manual admin CRUD (Phase 9)
│   │   │   ├── branches/     branch CRUD, branch_staff, RequireBranchAccess middleware
│   │   │   ├── inventory/    branch_inventory: stock, price override, availability lookup
│   │   │   ├── cart/         carts, cart_items, stock validation, one-branch-per-cart
│   │   │   ├── orders/       checkout, stock reservation, status graph, order snapshots,
│   │   │   │                admin cross-branch view (Phase 9), POS checkout + cash/qris
│   │   │   │                payment_method tracking (Phase 10)
│   │   │   ├── payments/     Provider interface, payment_transactions audit log, staff-facing
│   │   │   │                QRIS-at-counter endpoint for POS (Phase 10, no customer ownership check)
│   │   │   │   └── duitku/   QRIS/VA adapter — three separate signature formulas, see below
│   │   │   ├── dashboard/    admin overview stats — revenue, order counts, low-stock (Phase 9)
│   │   │   ├── shifts/       cashier till sessions: open/close, cash reconciliation (Phase 10)
│   │   │   ├── health/
│   │   │   └── ...           everything else: empty (doc.go only) until its phase lands
│   │   ├── pkg/
│   │   │   ├── jwt/          access + refresh token issue/verify
│   │   │   ├── password/     bcrypt hashing
│   │   │   ├── random/       secure random (JTIs)
│   │   │   ├── response/     shared {success, message, code, data} envelope
│   │   │   ├── slug/         URL-slug generation (products AND branches)
│   │   │   ├── database/, logger/
│   │   │   └── storage/, websocket/   still stubs
│   │   ├── migrations/       000001_init … 000008_pos (see Database setup) — Phase 9 needed none
│   │   ├── seeds/            products_verified.csv (real, importable) + dev_seed.sql (fake, dev-only)
│   │   └── Dockerfile
│   └── web/                  Next.js frontend
│       ├── app/               /, /shop, /product/[slug] — customer site;
│       │                     /admin/* — staff dashboard (Phase 9): login,
│       │                     overview, products, orders, branches, staff;
│       │                     /pos/* — cashier till (Phase 10): shift open/close,
│       │                     product search, cart, cash/QRIS checkout, receipt
│       ├── components/       site-header/footer, site-chrome (Phase 9 — hides
│       │                     the above on /admin AND /pos routes as of Phase 10),
│       │                     branch-selector, cart-button, cart-panel, product-card,
│       │                     shop-page-client, product-detail-client,
│       │                     providers, backend-status, admin/ (stat-card,
│       │                     status-badge), ui/ (still empty)
│       ├── stores/           branch-store.ts (also drives which branch a POS session
│       │                     is working as of Phase 10), cart-store.ts, auth-store.ts
│       │                     (Phase 9, staff session) — all Zustand + localStorage,
│       │                     hydration-safe
│       ├── hooks/             use-click-outside.ts, use-cart.ts
│       ├── lib/               utils.ts (cn), format.ts (Rupiah), api-client.ts
│       └── Dockerfile
├── packages/                 intentionally empty — see packages/README.md
├── docker/nginx/nginx.conf
├── docker-compose.yml
└── .env.example
```

## Security

Implemented so far: CORS (explicit allow-list), fail-fast DB/Redis
connections, no secrets committed, password hashing (bcrypt), JWT access
+ refresh tokens with rotation and server-side revocation, role-based
authorization (backend-enforced, never just a hidden frontend button),
Redis-backed login rate limiting, branch-scoped authorization —
`RequireBranchAccess` checks `branch_staff` fresh on every request, so a
BRANCH_MANAGER/CASHIER/INVENTORY_STAFF account is confined to branches
they're actually assigned to (section 9's "branch users cannot access
unauthorized branches"), re-checked live rather than trusted from a JWT
claim — cart-level overselling prevention (live stock checks, never a
client-supplied quantity taken at face value), real stock reservation
with row-level locking (checkout's `SELECT ... FOR UPDATE` closes the
race a plain check-then-write would leave open), order access checks
(a customer can only reach their own orders; a branch-scoped order
lookup by ID 404s if it actually belongs to a different branch), and now
**webhook signature verification**: Duitku's callback is authenticated
by recomputing its MD5 signature server-side and comparing in constant
time — never by trusting the network origin or any header — with a
mismatched amount also rejected even when the signature itself checks
out, so a callback can't under-report or over-report what was actually
paid, **privilege-escalation guards on staff account management**:
creating or editing a staff account that grants the SUPER_ADMIN role is
rejected (`403`) unless the caller already holds SUPER_ADMIN — otherwise
any ADMIN, who can already create/edit staff at all, could mint
themselves the platform's top role — and **server-derived POS
ownership**: a cashier's shift id, for both the sale it rings up and the
till it's reconciled against, is always looked up server-side from the
authenticated caller (`shifts.GetOpenForUser`), never accepted as a
value the client supplies — `CheckoutRequest.CashierShiftID` has no JSON
tag at all, specifically so no request body can set it.

Still ahead, landing with the phase that needs it: fine-grained
permission-code checks (the `permissions`/`role_permissions` tables exist
but nothing reads them yet), input validation beyond struct binding,
general admin-action audit logging (section 64's `audit_logs` table —
`payment_transactions` only logs payment webhooks specifically, not
"ADMIN changed product price" or similar — Phases 9 and 10 both gave the
admin/staff surface more actions worth auditing this way, but neither
built the auditing system itself), a "how many active SUPER_ADMINs
remain" check before letting one deactivate or demote the last other
one, an idempotency guard on QRIS-at-counter payment creation (Phase
10's own "Deliberately not" note), and the rest of the spec's security
test list (SQL injection/XSS sweeps, expired/invalid JWT edge cases,
...) — those need the phases that introduce the actions being audited,
or are worth a dedicated pass now that the admin and POS surfaces
actually exist to test.

## Data notes

`apps/api/seeds/products_verified.csv` holds the 23 real, verified
products provided for this project — real names, real prices, nothing
fabricated. Fields that weren't given (SKU, barcode, brand, description,
size, gender, image) are left blank rather than guessed, per the project's
explicit "do not fabricate" rule. The target catalog is ~65 products; the
other ~42 are simply not in the file because they haven't been verified
yet — add them the same way (real name, real price, blank everything else)
as they get confirmed, then re-run the import in
[Trying the catalog](#trying-the-catalog).

## Verification notes

What was actually checked while building this, and how:

| Check | Backend (Go) | Frontend (Next.js) |
|---|---|---|
| Syntax/formatting | `gofmt -l` — clean across all 72 `.go` files (Phase 1-10) | — |
| Type check | — | `tsc --noEmit` — clean (Phase 9 is the first phase this sandbox could actually run it; see below) |
| Lint | — | `eslint .` — clean (Phase 9, same reason) |
| Dependency install | fully resolvable this phase, with a caveat (see below) | `npm install` succeeded outright — `registry.npmjs.org` is reachable here |
| Production build | not run for real (see below) | reaches `next build`'s TypeScript + Turbopack bundling step cleanly; fails at the same pre-existing font fetch every phase has hit (see below) |
| Unit tests | `go test ./...` run in full since Phase 9 (see below) — everything already there passed both times, and the Phase 9 run also caught (and that phase fixed) Phase 3's `ParseCSV` off-by-one | n/a — no frontend test runner configured, unchanged from every phase before this one |
| SQL correctness | Every new query's column list hand-checked against its migration's `CREATE TABLE` statement, line-by-line (Phase 9: `products`, `product_variants`, `product_images`, `orders`, `branch_inventory`, `users`, `user_roles`; Phase 10: `cashier_shifts`, plus `orders`' two new columns re-checked against the extended `orderColumns`/`scanOrder`) | n/a |
| Logic correctness | New route registrations hand-traced against gin's actual (per-HTTP-method) routing tree to rule out a static/param path conflict — e.g. `GET /admin/orders` next to `GET /admin/orders/:id` in Phase 9, `POST /admin/branches/:id/shifts` next to the existing `:id/staff`/`:id/inventory`/`:id/orders` siblings in Phase 10 — before trusting it, rather than assuming | n/a |

This phase's sandbox differs from whatever produced Phases 1-8's notes in
two ways worth being precise about, rather than letting the table above
imply more changed than actually did:

1. **`npm install` works here outright** — `registry.npmjs.org` is
   reachable. That's what made a real `tsc`/`eslint` run possible
   starting with Phase 9; nothing about either phase's own code caused
   that.
2. **The Go toolchain isn't preinstalled, but `apt-get install
   golang-go` is** — and once it's there, most of `go.mod`'s dependencies
   resolve via `GOPROXY=direct`, since `github.com` is reachable even
   though `proxy.golang.org` isn't. The exception is anything using a Go
   *vanity import path* (`golang.org/x/crypto`, `golang.org/x/net`,
   `gopkg.in/yaml.v3`, and a couple of their own transitive dependencies)
   — those need to reach `golang.org`/`gopkg.in` itself just to be told
   which real repository backs them, and neither domain is reachable
   here. Routing around that needed a handful of `replace` directives
   pointing each vanity path at its real GitHub mirror
   (`golang.org/x/crypto` → `github.com/golang/crypto`, and so on) —
   **added only in a throwaway copy used purely to verify each phase,
   never in the `go.mod` actually shipped in this repo.** With those in
   a scratch copy, `go build ./...`, `go vet ./...`, and `go test ./...`
   all ran for real and came back clean in both Phase 9 and Phase 10 —
   which is what caught Phase 3's `ParseCSV` bug in the first place. On a
   normal machine with ordinary internet access none of this is needed:
   `go mod tidy` resolves everything through the real module proxy, same
   as it always would.

`next build` gets further than any pre-Phase-9 notes describe — all the
way through TypeScript checking and Turbopack bundling, in both Phase 9
and Phase 10 (`qrcode.react` included) — before failing at the exact
same `next/font` → `fonts.googleapis.com` fetch Phase 1's notes already
named. That domain still isn't reachable here; it's a standard public
endpoint with no unusual access requirements, so it resolves itself the
moment this runs somewhere with real internet access.

Package versions (Next 16, React 19, Tailwind v4, ESLint 9) were checked
against the npm registry rather than assumed, and two were deliberately
*not* pinned to their newest release: TypeScript stays on 6.0.3 and ESLint
on 9.39.5 because `typescript-eslint` (via `eslint-config-next`) doesn't
yet support TypeScript 7 or run cleanly under ESLint 10 — confirmed by
actually running `lint` against both, not just reading changelogs. Worth
revisiting once `typescript-eslint` catches up.

## License

Not set — add one when you're ready to decide how the code can be reused.
