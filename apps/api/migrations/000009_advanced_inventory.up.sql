-- Phase 11: advanced inventory — an auditable movement log
-- (stock_movements), branch-to-branch transfers, and stock opname
-- (physical count reconciliation). See internal/inventory's own Phase 4
-- doc comment for why none of this existed until now: "stock_movements
-- with a reason/actor/history trail (receive, adjust, transfer, opname)
-- is Phase 11's job (section 32)."

-- One row per actual stock_quantity change, whatever caused it.
-- Deliberately does NOT log reservation holds/releases (branch_inventory
-- .reserved_quantity) — those aren't a physical stock change, just a
-- temporary bookkeeping hold already well served by that column; this
-- table is a ledger of real, physical stock moving in or out.
CREATE TABLE stock_movements (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    branch_id          UUID NOT NULL REFERENCES branches(id),
    product_variant_id UUID NOT NULL REFERENCES product_variants(id),
    -- Positive = stock increased (receive, transfer in, opname found more
    -- than expected, a cancelled transfer's stock returning); negative =
    -- stock decreased (sale, transfer out, opname found less). The sign
    -- is what lets a running balance be reconstructed by summing
    -- history, same as any ledger.
    quantity_change    INTEGER NOT NULL CHECK (quantity_change != 0),
    reason             VARCHAR(20) NOT NULL, -- sale, receive, adjust, transfer_in, transfer_out, transfer_cancelled, opname
    reference_type     VARCHAR(20), -- order, transfer, opname; null for a plain manual adjust/receive
    reference_id       UUID,
    note               TEXT,
    actor_user_id      UUID REFERENCES users(id), -- null for system-driven movements (a sale)
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_stock_movements_branch_variant ON stock_movements (branch_id, product_variant_id, created_at DESC);
CREATE INDEX idx_stock_movements_reference ON stock_movements (reference_type, reference_id) WHERE reference_type IS NOT NULL;

-- Branch-to-branch transfers. Creating one immediately deducts the
-- source branch's stock (see internal/stock/transfers.go) — so a
-- transfer sitting "pending" can't be oversold from the source branch
-- while it's in transit — and completing it adds that same stock to the
-- destination. Nothing models the physical truck in between; the stock
-- is simply "in transit, counted at neither branch" while pending.
CREATE TABLE stock_transfers (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_branch_id UUID NOT NULL REFERENCES branches(id),
    to_branch_id   UUID NOT NULL REFERENCES branches(id) CHECK (to_branch_id != from_branch_id),
    status         VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, completed, cancelled
    requested_by   UUID NOT NULL REFERENCES users(id),
    completed_by   UUID REFERENCES users(id),
    notes          TEXT,
    completed_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE stock_transfer_items (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transfer_id        UUID NOT NULL REFERENCES stock_transfers(id) ON DELETE CASCADE,
    product_variant_id UUID NOT NULL REFERENCES product_variants(id),
    quantity           INTEGER NOT NULL CHECK (quantity > 0)
);

CREATE INDEX idx_stock_transfers_from ON stock_transfers (from_branch_id);
CREATE INDEX idx_stock_transfers_to ON stock_transfers (to_branch_id);
CREATE INDEX idx_stock_transfer_items_transfer ON stock_transfer_items (transfer_id);

-- Stock opname (physical count) sessions — one branch counting its shelf
-- stock against what the system thinks it has. system_quantity is
-- snapshotted when the session starts, not read live at completion: the
-- whole point is comparing against what the system said *at the moment
-- counting began*, not a moving target if a sale happens mid-count.
CREATE TABLE stock_opnames (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    branch_id    UUID NOT NULL REFERENCES branches(id),
    status       VARCHAR(20) NOT NULL DEFAULT 'open', -- open, completed
    started_by   UUID NOT NULL REFERENCES users(id),
    completed_by UUID REFERENCES users(id),
    notes        TEXT,
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE stock_opname_items (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    opname_id          UUID NOT NULL REFERENCES stock_opnames(id) ON DELETE CASCADE,
    product_variant_id UUID NOT NULL REFERENCES product_variants(id),
    system_quantity    INTEGER NOT NULL,
    counted_quantity   INTEGER, -- null until staff actually counts this line
    UNIQUE (opname_id, product_variant_id)
);

CREATE INDEX idx_stock_opnames_branch ON stock_opnames (branch_id);
-- One open count per branch at a time — same reasoning and same partial-
-- unique-index technique as migration 000008_pos's one-open-shift-per-user.
CREATE UNIQUE INDEX idx_stock_opnames_one_open_per_branch
    ON stock_opnames (branch_id) WHERE status = 'open';
