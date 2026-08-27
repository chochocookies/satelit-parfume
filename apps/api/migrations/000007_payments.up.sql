-- Phase 8: payment abstraction's persistence — one row per payment
-- attempt, plus a verbatim audit log of every webhook delivery received.
--
-- payments.status is deliberately separate from orders.status: a payment
-- can be PENDING/PAID/FAILED/EXPIRED on its own timeline, and
-- orders.status only advances to PAID once a payment here is confirmed —
-- internal/payments' service is what connects the two, never a trigger
-- or a shared column.
CREATE TABLE payments (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id       UUID NOT NULL REFERENCES orders(id),
    provider       VARCHAR(30) NOT NULL,  -- 'duitku' today; the abstraction exists so a second one doesn't mean rewriting this table
    payment_method VARCHAR(20) NOT NULL,  -- provider-specific code, e.g. Duitku's "SP" for QRIS
    reference      VARCHAR(100) NOT NULL, -- the provider's own transaction reference
    amount         BIGINT NOT NULL CHECK (amount >= 0),
    status         VARCHAR(20) NOT NULL DEFAULT 'PENDING', -- PENDING, PAID, FAILED
    qr_string      TEXT,    -- QRIS payload, when payment_method is a QRIS variant
    va_number      VARCHAR(50), -- virtual account number, when payment_method is a VA
    payment_url    TEXT,    -- redirect URL, for methods that need one
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, reference)
);

CREATE INDEX idx_payments_order ON payments (order_id);

-- Verbatim log of every webhook/callback delivery, valid or not — not a
-- dedup mechanism (that's orders' own guarded status transition, see
-- internal/orders' UpdateStatus), just the audit trail section 82 asks
-- for ("important operations must be audited") and the record you'd
-- actually want if a real gateway integration ever needs debugging.
CREATE TABLE payment_transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider        VARCHAR(30) NOT NULL,
    reference       VARCHAR(100),
    raw_payload     TEXT NOT NULL,
    signature_valid BOOLEAN NOT NULL,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payment_transactions_reference ON payment_transactions (provider, reference);
