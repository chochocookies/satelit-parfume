-- Phase 12, part 2: product reviews. Verified-purchase only — a review
-- must cite a COMPLETED order (see internal/orders' transitions.go for
-- why that status specifically: it's the only terminal "this customer
-- actually received it" state, reached from either pickup or delivery)
-- that actually contained this product, checked in
-- internal/reviews' Service.Submit before this table is ever touched.
-- order_id is kept (not just checked-then-discarded) so that link stays
-- auditable — which real purchase earned this review's badge.
--
-- One review per customer per product: UNIQUE below, and Submit does an
-- upsert on it, so leaving a second review just edits the first rather
-- than erroring — closer to how a shopper expects "edit my review" to
-- work than a hard rejection would be.
CREATE TABLE product_reviews (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    product_id  UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    order_id    UUID NOT NULL REFERENCES orders(id),
    rating      SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (customer_id, product_id)
);

CREATE INDEX idx_product_reviews_product ON product_reviews (product_id);
