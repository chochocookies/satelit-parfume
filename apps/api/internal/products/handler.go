package products

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"satelit-parfume-api/internal/categories"
	"satelit-parfume-api/internal/inventory"
	"satelit-parfume-api/pkg/response"
)

type Handler struct {
	repo       *Repository
	categories *categories.Repository
	inventory  *inventory.Repository
}

func NewHandler(repo *Repository, categoriesRepo *categories.Repository, inventoryRepo *inventory.Repository) *Handler {
	return &Handler{repo: repo, categories: categoriesRepo, inventory: inventoryRepo}
}

// List backs GET /api/v1/products — search, category/brand/gender
// filters, sort, and pagination, per section 15/19 of the spec. Add
// ?branch=<slug> to also resolve each item's stock at that branch
// (section 16's product card availability) — omit it and items just
// come back without a branch_stock value, same as before this existed.
func (h *Handler) List(c *gin.Context) {
	filter := ListFilter{
		Search:       c.Query("search"),
		CategorySlug: c.Query("category"),
		BrandSlug:    c.Query("brand"),
		Gender:       c.Query("gender"),
		BranchSlug:   c.Query("branch"),
		Sort:         c.Query("sort"),
		Page:         queryInt(c, "page", 1),
		Limit:        queryInt(c, "limit", 24),
	}

	result, err := h.repo.List(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, err, "could not load products")
		return
	}

	response.OK(c, http.StatusOK, "products", result)
}

func queryInt(c *gin.Context, key string, fallback int) int {
	v := c.Query(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// GetBySlug backs GET /api/v1/products/:slug — full product detail with
// variants and images. Pass ?branch=<slug> to also get that branch's
// stock/price for this product (see model.go's Detail.Availability
// comment for why it's opt-in rather than always resolved).
func (h *Handler) GetBySlug(c *gin.Context) {
	detail, err := h.repo.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(c, http.StatusNotFound, "NOT_FOUND", "product not found")
			return
		}
		response.InternalError(c, err, "could not load product")
		return
	}

	if branchSlug := c.Query("branch"); branchSlug != "" {
		availability, err := h.inventory.AvailabilityForProduct(c.Request.Context(), branchSlug, detail.Slug)
		if err == nil {
			detail.Availability = availability
		}
		// A missing/unknown branch just means Availability stays nil —
		// the product itself is still valid and worth returning.
	}

	response.OK(c, http.StatusOK, "product", detail)
}

// Import backs POST /api/v1/admin/products/import — admin-only (see
// main.go's route registration). Accepts multipart/form-data with a
// "file" field holding a CSV in the section-48 schema; returns a
// row-by-row report rather than a single pass/fail, so a batch with a
// few bad rows still imports everything that was valid.
func (h *Handler) Import(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "expected a multipart file field named \"file\"")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "could not open uploaded file")
		return
	}
	defer file.Close()

	rows, err := ParseCSV(file)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_CSV", err.Error())
		return
	}

	result, err := h.repo.Import(c.Request.Context(), h.categories, rows)
	if err != nil {
		response.InternalError(c, err, "import failed partway through: "+err.Error())
		return
	}

	response.OK(c, http.StatusOK, "import complete", result)
}

// ── Admin manual CRUD (Phase 9 — see repository.go's section comment) ──

// AdminList backs GET /api/v1/admin/products. It's List's admin sibling:
// every status shows up (not just active), since the admin table needs
// to manage drafts and archived products too, not only what a customer
// can currently see. ?status=draft narrows to one specific status; leave
// it off to see everything.
func (h *Handler) AdminList(c *gin.Context) {
	filter := ListFilter{
		Search:       c.Query("search"),
		CategorySlug: c.Query("category"),
		BrandSlug:    c.Query("brand"),
		Gender:       c.Query("gender"),
		Status:       c.Query("status"),
		AdminView:    true,
		Sort:         c.Query("sort"),
		Page:         queryInt(c, "page", 1),
		Limit:        queryInt(c, "limit", 24),
	}

	result, err := h.repo.List(c.Request.Context(), filter)
	if err != nil {
		response.InternalError(c, err, "could not load products")
		return
	}
	response.OK(c, http.StatusOK, "products", result)
}

// AdminCreate backs POST /api/v1/admin/products — the manual single-
// product create path Phase 3 deliberately deferred to here.
func (h *Handler) AdminCreate(c *gin.Context) {
	var req ProductUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	product, err := h.repo.AdminCreate(c.Request.Context(), h.categories, req)
	if err != nil {
		respondUpsertError(c, err)
		return
	}
	response.OK(c, http.StatusCreated, "product created", product)
}

func (h *Handler) AdminUpdate(c *gin.Context) {
	var req ProductUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	product, err := h.repo.AdminUpdate(c.Request.Context(), h.categories, c.Param("id"), req)
	if err != nil {
		respondUpsertError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "product updated", product)
}

func (h *Handler) AdminDelete(c *gin.Context) {
	if err := h.repo.AdminSoftDelete(c.Request.Context(), c.Param("id")); err != nil {
		respondUpsertError(c, err)
		return
	}
	response.OK(c, http.StatusOK, "product deleted", nil)
}

func respondUpsertError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "product not found")
	case errors.Is(err, ErrDuplicateName):
		response.Error(c, http.StatusConflict, "DUPLICATE_NAME", err.Error())
	case errors.Is(err, ErrDuplicateSKU):
		response.Error(c, http.StatusConflict, "DUPLICATE_SKU", err.Error())
	default:
		response.InternalError(c, err, "could not save product")
	}
}
