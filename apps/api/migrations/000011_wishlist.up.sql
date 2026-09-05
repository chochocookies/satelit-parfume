-- Phase 12, part 1: wishlist. Keyed off products, not variants — every
-- other product-facing surface in this schema (products.ListItem,
-- cart's own price_from) already treats "the product" as the
-- shopper-facing unit and leaves variant choice to add-to-cart time
-- (see product_variants' own migration comment: most products only
-- have one variant anyway, so this loses nothing in practice).
--
-- Customer-only, unlike cart_items: a wishlist is only meaningful
-- across visits, which only means anything for an identity that itself
-- persists across visits — a guest's X-Cart-Token isn't meant to.
CREATE TABLE wishlist_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    product_id  UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (customer_id, product_id)
);

CREATE INDEX idx_wishlist_items_customer ON wishlist_items (customer_id);
