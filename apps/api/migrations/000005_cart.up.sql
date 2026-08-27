-- Phase 6: cart schema.
--
-- A cart belongs to EITHER a customer OR a guest session, never both and
-- never neither (section 20: "Cart belongs to a customer/session") — the
-- CHECK constraint enforces that at the database level, not just in Go.
-- The partial unique index means a customer can only ever have one cart;
-- session_token is already unique on its own for the guest side.
--
-- branch_id lives on both carts and cart_items, matching section 20's
-- literal schema. In practice cart_items.branch_id always equals its
-- parent cart's — internal/cart's application logic is what actually
-- enforces "a cart is associated with a selected branch" (section 20)
-- by rejecting an item for a different branch than the cart already has.
CREATE TABLE carts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id   UUID REFERENCES customers(id) ON DELETE CASCADE,
    session_token TEXT UNIQUE,
    branch_id     UUID REFERENCES branches(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (customer_id IS NOT NULL AND session_token IS NULL) OR
        (customer_id IS NULL AND session_token IS NOT NULL)
    )
);

CREATE UNIQUE INDEX idx_carts_customer ON carts (customer_id) WHERE customer_id IS NOT NULL;

CREATE TABLE cart_items (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cart_id            UUID NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    product_variant_id UUID NOT NULL REFERENCES product_variants(id),
    branch_id          UUID NOT NULL REFERENCES branches(id),
    quantity           INTEGER NOT NULL CHECK (quantity > 0),
    unit_price         BIGINT NOT NULL CHECK (unit_price >= 0), -- snapshot at add-time, section 20
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (cart_id, product_variant_id)
);

CREATE INDEX idx_cart_items_cart ON cart_items (cart_id);
