-- Phase 3: catalog schema — brands, categories, products, variants, images.
--
-- brand_id/category_id are nullable: the real seed data (seeds/products_verified.csv)
-- has no brand for any of the 23 verified products, and a product without
-- a category shouldn't become impossible to save. NULL here just means
-- "not yet known", same as every other unverified field in this project.

-- pg_trgm powers the ILIKE-based fuzzy search on products.name
-- (idx_products_name_trgm below) — must exist before that index is created.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE brands (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(150) UNIQUE NOT NULL,
    slug       VARCHAR(170) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE categories (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(150) UNIQUE NOT NULL,
    slug       VARCHAR(170) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE products (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    brand_id          UUID REFERENCES brands(id),
    category_id       UUID REFERENCES categories(id),
    name              VARCHAR(200) NOT NULL,
    slug              VARCHAR(220) UNIQUE NOT NULL,
    sku               VARCHAR(100) UNIQUE,
    barcode           VARCHAR(100) UNIQUE,
    description       TEXT,
    short_description VARCHAR(500),
    size              VARCHAR(50),
    gender            VARCHAR(20),   -- men, women, unisex — free text, not enforced yet
    fragrance_family  VARCHAR(50),
    status            VARCHAR(20) NOT NULL DEFAULT 'active', -- active, draft, archived
    is_featured       BOOLEAN NOT NULL DEFAULT false,
    is_bestseller     BOOLEAN NOT NULL DEFAULT false,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);

CREATE INDEX idx_products_category ON products (category_id);
CREATE INDEX idx_products_brand ON products (brand_id);
CREATE INDEX idx_products_status ON products (status) WHERE deleted_at IS NULL;
CREATE INDEX idx_products_name_trgm ON products USING gin (name gin_trgm_ops);

-- Every Satelit Parfume product currently has exactly one size/price, so
-- most products get exactly one variant named 'Default' — never a
-- fabricated 30ml/50ml/100ml lineup (section 7: "do not create fake
-- variants"). Real size variants become real rows once they're verified.
CREATE TABLE product_variants (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name       VARCHAR(100) NOT NULL DEFAULT 'Default',
    sku        VARCHAR(100) UNIQUE,
    barcode    VARCHAR(100) UNIQUE,
    size       VARCHAR(50),
    base_price BIGINT NOT NULL CHECK (base_price >= 0), -- Rupiah, integer — see section 85
    cost_price BIGINT CHECK (cost_price IS NULL OR cost_price >= 0),
    weight     INTEGER, -- grams
    status     VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_product_variants_product ON product_variants (product_id);

CREATE TABLE product_images (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    image_url  TEXT NOT NULL,
    alt_text   VARCHAR(200),
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_product_images_product ON product_images (product_id);
