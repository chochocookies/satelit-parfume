-- Phase 4: branches, branch staffing, and branch-scoped inventory.
--
-- opening_time/closing_time are stored as plain "HH:MM" text rather than
-- SQL TIME — a deliberate simplification. This project's Postgres access
-- goes through pgx, and TIME-without-timezone's Go mapping has enough
-- sharp edges that, unable to verify it against a live database from
-- this environment, VARCHAR is the safer choice: zero type-mapping risk,
-- fully sufficient for "what time does this branch open".

CREATE TABLE branches (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(150) NOT NULL,
    code         VARCHAR(20) UNIQUE NOT NULL,
    slug         VARCHAR(170) UNIQUE NOT NULL,
    address      TEXT,
    city         VARCHAR(100),
    province     VARCHAR(100),
    postal_code  VARCHAR(20),
    latitude     DOUBLE PRECISION,
    longitude    DOUBLE PRECISION,
    phone        VARCHAR(30),
    whatsapp     VARCHAR(30),
    opening_time VARCHAR(5),  -- "HH:MM", nullable — see note above
    closing_time VARCHAR(5),
    status       VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);

-- Many-to-many: a staff member can be assigned to more than one branch,
-- and RequireBranchAccess (internal/branches/middleware.go) checks this
-- table directly rather than trusting a role or a JWT claim, so revoking
-- access takes effect on the very next request, not the next login.
CREATE TABLE branch_staff (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    branch_id  UUID NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, branch_id)
);

-- Products are global (Phase 3); stock is per branch, per variant.
-- available_stock is a generated column rather than something every
-- caller re-derives — section 9 defines it as stock minus reserved, so
-- the database enforces that arithmetic once, in one place.
CREATE TABLE branch_inventory (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    branch_id          UUID NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    product_variant_id UUID NOT NULL REFERENCES product_variants(id) ON DELETE CASCADE,
    stock_quantity     INTEGER NOT NULL DEFAULT 0 CHECK (stock_quantity >= 0),
    reserved_quantity  INTEGER NOT NULL DEFAULT 0 CHECK (reserved_quantity >= 0),
    available_stock    INTEGER GENERATED ALWAYS AS (stock_quantity - reserved_quantity) STORED,
    minimum_stock      INTEGER NOT NULL DEFAULT 0,
    price              BIGINT CHECK (price IS NULL OR price >= 0), -- NULL = use the variant's base_price
    status             VARCHAR(20) NOT NULL DEFAULT 'active',
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (branch_id, product_variant_id)
);

CREATE INDEX idx_branch_inventory_branch ON branch_inventory (branch_id);
CREATE INDEX idx_branch_staff_branch ON branch_staff (branch_id);

-- Closing a Phase 3 gap: section 83 lists categories among the tables
-- that use soft delete, alongside products/branches/users. products and
-- users already had deleted_at; categories was missed when Phase 3 only
-- needed to create (never delete) them. No functional impact until a
-- category-delete endpoint exists, but the schema should be right now
-- rather than carrying the gap forward.
ALTER TABLE categories ADD COLUMN deleted_at TIMESTAMPTZ;
