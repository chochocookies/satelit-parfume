-- Phase 12, part 4 (the last part): promo codes. The one Phase 12 slice
-- that actually touches Checkout — see internal/orders' own Service.go,
-- which already had "no promotions to discount it yet — Phase 12" on
-- its Total: c.Subtotal line, anticipating exactly this.
--
-- discount_type is PERCENTAGE (discount_value is a whole-number percent,
-- optionally capped by max_discount so "20% off" can't blow past a sane
-- rupiah ceiling) or FIXED (discount_value is a straight rupiah amount).
-- used_count is incremented inside the SAME transaction Checkout already
-- runs everything else in (see promotions.Repository.IncrementUsage) —
-- a limited-use code can't be oversold by two checkouts racing each
-- other the way an ungated column could be.
CREATE TABLE promo_codes (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code           VARCHAR(50) NOT NULL UNIQUE,
    description    VARCHAR(200),
    discount_type  VARCHAR(20) NOT NULL CHECK (discount_type IN ('PERCENTAGE', 'FIXED')),
    discount_value BIGINT NOT NULL CHECK (discount_value > 0),
    min_purchase   BIGINT NOT NULL DEFAULT 0,
    max_discount   BIGINT, -- caps a PERCENTAGE discount's rupiah value; NULL = uncapped. Unused for FIXED.
    max_uses       INTEGER, -- NULL = unlimited
    used_count     INTEGER NOT NULL DEFAULT 0,
    valid_from     TIMESTAMPTZ NOT NULL DEFAULT now(),
    valid_until    TIMESTAMPTZ, -- NULL = no expiry
    active         BOOLEAN NOT NULL DEFAULT true,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- promo_code is the code text, not a promo_codes.id FK: kept even if a
-- code is later deleted, the same way order_items snapshots
-- product_name/variant_name rather than only holding a foreign key —
-- an order's own record of what happened shouldn't change retroactively
-- because of a later, unrelated admin action.
ALTER TABLE orders ADD COLUMN promo_code VARCHAR(50);
ALTER TABLE orders ADD COLUMN discount_amount BIGINT NOT NULL DEFAULT 0 CHECK (discount_amount >= 0);

-- One example code, active immediately, so the feature is testable the
-- moment this migration runs rather than needing a manual INSERT first.
-- Real codes belong with whatever admin workflow comes next — see this
-- round's own summary on why that's not built yet.
INSERT INTO promo_codes (code, description, discount_type, discount_value, min_purchase, max_discount)
VALUES ('SATELIT10', 'Diskon 10% untuk pembelian pertama', 'PERCENTAGE', 10, 0, 20000);
