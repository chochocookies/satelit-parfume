-- Phase 10: POS — cashier shifts (till sessions) plus the two columns
-- orders needs to support them.
--
-- Deliberately NO new order_type: a POS sale reuses 'pickup' exactly as
-- Phase 7's own notes already anticipated ("a cashier confirming cash at
-- the counter, section 28's POS flow") — a walk-in sale genuinely *is* a
-- pickup, just rung up by staff instead of placed online by the
-- customer. cashier_shift_id is what actually distinguishes a POS sale
-- from an ordinary online pickup order for reporting — order_type alone
-- can't, and doesn't need to.
CREATE TABLE cashier_shifts (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    branch_id        UUID NOT NULL REFERENCES branches(id),
    user_id          UUID NOT NULL REFERENCES users(id),
    opening_balance  BIGINT NOT NULL CHECK (opening_balance >= 0),
    closing_balance  BIGINT CHECK (closing_balance >= 0), -- null while open
    expected_balance BIGINT, -- opening_balance + cash sales during the shift; computed at close, null while open
    discrepancy      BIGINT, -- closing_balance - expected_balance; null while open
    status           VARCHAR(20) NOT NULL DEFAULT 'open', -- open, closed
    notes            TEXT,
    opened_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at        TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cashier_shifts_branch ON cashier_shifts (branch_id);

-- Enforces "one open shift per cashier at a time" at the database level,
-- not just in application code — a partial unique index rather than a
-- plain UNIQUE(user_id) so a cashier can still accumulate many *closed*
-- shifts in their history. A second INSERT while one is already open
-- fails with a real constraint violation instead of silently succeeding.
CREATE UNIQUE INDEX idx_cashier_shifts_one_open_per_user
    ON cashier_shifts (user_id) WHERE status = 'open';

ALTER TABLE orders
    ADD COLUMN cashier_shift_id UUID REFERENCES cashier_shifts(id),
    -- 'cash' | 'qris' | null. Deliberately separate from payments.payment_method
    -- (Phase 8): that table only ever gets a row for online/gateway methods —
    -- see internal/payments/provider.go's own doc comment, unchanged since
    -- Phase 8, on why cash never reaches that package at all. This column is
    -- what a *cash* sale actually gets recorded under, and QRIS-at-counter
    -- gets stamped here too (in addition to its real payments row) purely so
    -- a shift-close reconciliation query never has to join out to payments
    -- to answer "was this order's cashier_shift_id a cash sale or not".
    ADD COLUMN payment_method VARCHAR(20);

CREATE INDEX idx_orders_cashier_shift ON orders (cashier_shift_id) WHERE cashier_shift_id IS NOT NULL;
