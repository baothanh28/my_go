package product

import (
	"net/http"
	"strconv"

	"myapp/internal/pkg/logger"
	"myapp/internal/pkg/server"
	"myapp/internal/service/auth"

	"github.com/labstack/echo/v4"
)

// ProductHandler handles product HTTP requests
type ProductHandler struct {
	service *ProductService
	logger  *logger.Logger
}

// NewProductHandler creates a new product handler
func NewProductHandler(service *ProductService, log *logger.Logger) *ProductHandler {
	return &ProductHandler{
		service: service,
		logger:  log,
	}
}

// Create handles product creation
func (h *ProductHandler) Create(c echo.Context) error {
	// Get authenticated user
	userCtx, err := auth.GetUserFromContext(c)
	if err != nil {
		return server.ErrorResponse(c, http.StatusUnauthorized, err.Error(), "Unauthorized")
	}

	var dto CreateProductDTO
	if err := c.Bind(&dto); err != nil {
		return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Invalid request body")
	}

	// Basic validation
	if dto.Name == "" || dto.SKU == "" || dto.Price <= 0 || dto.Stock < 0 {
		return server.ErrorResponse(c, http.StatusBadRequest, nil, "Invalid product data")
	}

	product, err := h.service.Create(dto, userCtx.UserID)
	if err != nil {
		h.logger.Error("Failed to create product")
		return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Failed to create product")
	}

	return server.SuccessResponse(c, http.StatusCreated, product.ToProductResponse(), "Product created successfully")
}

// Get handles retrieving a product by ID
func (h *ProductHandler) Get(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Invalid product ID")
	}

	product, err := h.service.GetByID(uint(id))
	if err != nil {
		h.logger.Error("Failed to get product")
		return server.ErrorResponse(c, http.StatusNotFound, err.Error(), "Product not found")
	}

	return server.SuccessResponse(c, http.StatusOK, product.ToProductResponse(), "Product retrieved successfully")
}

// List handles listing products with pagination
func (h *ProductHandler) List(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	products, err := h.service.List(page, limit)
	if err != nil {
		h.logger.Error("Failed to list products")
		return server.ErrorResponse(c, http.StatusInternalServerError, err.Error(), "Failed to list products")
	}

	return server.SuccessResponse(c, http.StatusOK, products, "Products retrieved successfully")
}

// Update handles product updates
func (h *ProductHandler) Update(c echo.Context) error {
	// Get authenticated user
	userCtx, err := auth.GetUserFromContext(c)
	if err != nil {
		return server.ErrorResponse(c, http.StatusUnauthorized, err.Error(), "Unauthorized")
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Invalid product ID")
	}

	var dto UpdateProductDTO
	if err := c.Bind(&dto); err != nil {
		return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Invalid request body")
	}

	product, err := h.service.Update(uint(id), dto, userCtx.UserID)
	if err != nil {
		h.logger.Error("Failed to update product")
		return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Failed to update product")
	}

	return server.SuccessResponse(c, http.StatusOK, product.ToProductResponse(), "Product updated successfully")
}

// Delete handles product deletion
func (h *ProductHandler) Delete(c echo.Context) error {
	// Get authenticated user
	userCtx, err := auth.GetUserFromContext(c)
	if err != nil {
		return server.ErrorResponse(c, http.StatusUnauthorized, err.Error(), "Unauthorized")
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Invalid product ID")
	}

	if err := h.service.Delete(uint(id), userCtx.UserID); err != nil {
		h.logger.Error("Failed to delete product")
		return server.ErrorResponse(c, http.StatusBadRequest, err.Error(), "Failed to delete product")
	}

	return server.SuccessResponse(c, http.StatusOK, nil, "Product deleted successfully")
}

// Search handles product search
func (h *ProductHandler) Search(c echo.Context) error {
	name := c.QueryParam("name")
	if name == "" {
		return server.ErrorResponse(c, http.StatusBadRequest, nil, "Search name is required")
	}

	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	products, err := h.service.Search(name, page, limit)
	if err != nil {
		h.logger.Error("Failed to search products")
		return server.ErrorResponse(c, http.StatusInternalServerError, err.Error(), "Failed to search products")
	}

	return server.SuccessResponse(c, http.StatusOK, products, "Products found successfully")
}
