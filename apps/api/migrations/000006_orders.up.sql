-- Phase 7: orders, order items, status history.
--
-- Guest checkout is supported the same way guest carts are (Phase 6):
-- customer_id is nullable, guest_name/guest_phone stand in when there's
-- no account — there's no customer-login UI yet, so in practice every
-- order today goes through the guest path, same situation as the cart.
--
-- order_items snapshots product_name/variant_name/sku/unit_price at the
-- moment of purchase (section 84) — nothing here is a live join back to
-- products/product_variants for *display* purposes, only product_variant_id
-- itself is kept (nullable) as a reference for internal bookkeeping
-- (stock release/deduction), never relied on to render the order.
CREATE TABLE orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_number    VARCHAR(30) UNIQUE NOT NULL,
    customer_id     UUID REFERENCES customers(id),
    guest_name      VARCHAR(150),
    guest_phone     VARCHAR(30),
    guest_email     VARCHAR(255),
    branch_id       UUID NOT NULL REFERENCES branches(id), -- snapshot: the branch at time of purchase (section 22), never re-derived later
    order_type      VARCHAR(20) NOT NULL, -- 'pickup' | 'delivery'
    status          VARCHAR(30) NOT NULL DEFAULT 'PENDING_PAYMENT',
    subtotal        BIGINT NOT NULL CHECK (subtotal >= 0),
    total           BIGINT NOT NULL CHECK (total >= 0), -- equals subtotal until promotions (Phase 12) can discount it
    recipient_name  VARCHAR(150), -- delivery orders only
    recipient_phone VARCHAR(30),
    address_line    TEXT,
    city            VARCHAR(100),
    province        VARCHAR(100),
    postal_code     VARCHAR(20),
    notes           TEXT,
    expires_at      TIMESTAMPTZ, -- PENDING_PAYMENT orders past this are lazily expired — see internal/orders' doc comment
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (customer_id IS NOT NULL OR guest_name IS NOT NULL)
);

CREATE INDEX idx_orders_customer ON orders (customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX idx_orders_branch ON orders (branch_id);
CREATE INDEX idx_orders_status ON orders (status);
CREATE INDEX idx_orders_guest_lookup ON orders (order_number, guest_phone);

CREATE TABLE order_items (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id           UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_variant_id UUID REFERENCES product_variants(id), -- kept for stock release/deduction bookkeeping only
    branch_id          UUID NOT NULL REFERENCES branches(id),
    product_name       VARCHAR(200) NOT NULL,
    variant_name       VARCHAR(100) NOT NULL,
    sku                VARCHAR(100),
    unit_price         BIGINT NOT NULL CHECK (unit_price >= 0),
    quantity           INTEGER NOT NULL CHECK (quantity > 0),
    subtotal           BIGINT NOT NULL CHECK (subtotal >= 0),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_items_order ON order_items (order_id);

CREATE TABLE order_status_histories (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id   UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    status     VARCHAR(30) NOT NULL,
    note       TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_status_histories_order ON order_status_histories (order_id);

-- Backs human-friendly order numbers (SP-20260814-0001, section 25's own
-- example format) — a dedicated counter table rather than COUNT(*)+1,
-- which would race under concurrent checkouts; the INSERT ... ON CONFLICT
-- DO UPDATE ... RETURNING pattern internal/orders uses against this table
-- is a single atomic statement, safe without extra locking.
CREATE TABLE order_number_counters (
    counter_date DATE PRIMARY KEY,
    count        INTEGER NOT NULL DEFAULT 0
);
