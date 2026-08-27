# Seeds

## products_verified.csv

23 real, verified Satelit Parfume products with real names and real prices,
provided directly. Fields not yet verified (SKU, barcode, brand, description,
size, gender, image_url) are left blank — never guessed — per the project's
"do not fabricate" rule.

As of Phase 3, this file can actually be imported:

```bash
# get a staff access token first (see the root README's "Trying the auth flow"),
# then:
curl -sX POST localhost:8080/api/v1/admin/products/import \
  -H "Authorization: Bearer <access_token>" \
  -F "file=@apps/api/seeds/products_verified.csv" | jq
```

Each row becomes a product with one default variant (its price) — see
`internal/products/repository.go`'s `importRow`. Category names
("Perfume", "Mix Perfume") are created automatically on first use; brand
stays empty for all 23 since none was verified. Re-running the import is
safe — a product already present by name is skipped, not duplicated.

Target catalog size is ~65 products; only these 23 are currently verified.
The other ~42 simply aren't in this file, not filled with guesses.

## dev_seed.sql

Fake staff + customer accounts for testing auth locally — see the file
itself and the root README's "Database setup" section. Not Satelit
Parfume's real people, don't run against production.
