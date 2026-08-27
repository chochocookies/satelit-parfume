-- Phase 2: authentication schema.
--
-- Staff (internal, role-based) and customers (shop accounts) are kept as
-- two separate tables rather than one polymorphic "users" table: they have
-- different security postures, different session lifetimes, and staff need
-- RBAC while customers don't. refresh_tokens is shared between both via
-- subject_type/subject_id, so token rotation/revocation logic doesn't need
-- to be duplicated per subject kind.

CREATE TABLE roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Structural, per the spec's data model (section 5). Not yet enforced by
-- any endpoint — there's nothing to protect at a granular level until
-- Phase 3+ introduces resources. Role-level checks (RequireRole) are what
-- Phase 2 actually enforces; permission-code checks layer in naturally
-- once real endpoints need finer-grained control than "which role".
CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- Internal staff accounts (admin, branch manager, cashier, inventory staff).
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(150) NOT NULL,
    email         VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'active', -- active, suspended
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
    -- branch_id arrives via ALTER TABLE in the branches migration (Phase 4)
    -- rather than being forward-declared here against a table that
    -- doesn't exist yet.
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

-- Shop customers. Deliberately not part of the RBAC system above — a
-- customer's access is "is this their own cart/order/wishlist", not a
-- role/permission lookup.
CREATE TABLE customers (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(150) NOT NULL,
    email         VARCHAR(255) UNIQUE NOT NULL,
    phone         VARCHAR(30),
    password_hash TEXT NOT NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);

-- Refresh tokens are themselves JWTs (signed with JWT_REFRESH_SECRET, see
-- .env.example from Phase 1); this table tracks their JTI so a token can
-- be rotated and revoked server-side instead of remaining valid — stolen
-- or not — until it naturally expires.
CREATE TABLE refresh_tokens (
    id           UUID PRIMARY KEY,        -- equals the JWT's `jti` claim
    subject_type VARCHAR(10) NOT NULL,    -- 'user' | 'customer'
    subject_id   UUID NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_refresh_tokens_subject ON refresh_tokens (subject_type, subject_id);

-- Reference data (the 5 staff roles from section 40), not sample/dev data —
-- every environment needs these rows to exist for user_roles to mean
-- anything, the same way a currency or country-code table would ship seeded.
INSERT INTO roles (name, description) VALUES
    ('SUPER_ADMIN',     'Full access: all branches, products, users, reports, settings'),
    ('ADMIN',           'Products, orders, inventory, customers, promotions'),
    ('BRANCH_MANAGER',  'Own branch: inventory, POS, orders, staff'),
    ('CASHIER',         'POS, branch orders, customer lookup'),
    ('INVENTORY_STAFF', 'Inventory, stock transfer, stock opname');
