package product

import (
	"errors"
	"fmt"

	"myapp/internal/pkg/logger"

	"gorm.io/gorm"
)

// ProductService handles product business logic
type ProductService struct {
	repo   *ProductRepository
	logger *logger.Logger
}

// NewProductService creates a new product service
func NewProductService(repo *ProductRepository, log *logger.Logger) *ProductService {
	return &ProductService{
		repo:   repo,
		logger: log,
	}
}

// Create creates a new product
func (s *ProductService) Create(dto CreateProductDTO, userID uint) (*Product, error) {
	// Check if SKU already exists
	existingProduct, err := s.repo.GetBySKU(dto.SKU)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check existing product: %w", err)
	}
	if existingProduct != nil {
		return nil, errors.New("product with this SKU already exists")
	}

	// Create product
	product := &Product{
		Name:        dto.Name,
		Description: dto.Description,
		Price:       dto.Price,
		Stock:       dto.Stock,
		SKU:         dto.SKU,
		CreatedBy:   userID,
	}

	if err := s.repo.Insert(product); err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	return product, nil
}

// GetByID retrieves a product by ID
func (s *ProductService) GetByID(id uint) (*Product, error) {
	product, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("product not found")
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}
	return product, nil
}

// List retrieves a paginated list of products
func (s *ProductService) List(page, limit int) (*ProductListResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	// Get products
	products, err := s.repo.GetAll(limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list products: %w", err)
	}

	// Get total count
	total, err := s.repo.Count(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to count products: %w", err)
	}

	// Convert to response
	var productResponses []*ProductResponse
	for _, p := range products {
		resp := p.ToProductResponse()
		productResponses = append(productResponses, &resp)
	}

	return &ProductListResponse{
		Products: productResponses,
		Total:    total,
		Page:     page,
		Limit:    limit,
	}, nil
}

// Update updates a product
func (s *ProductService) Update(id uint, dto UpdateProductDTO, userID uint) (*Product, error) {
	// Get existing product
	product, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("product not found")
		}
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	// Check ownership (optional: depends on business logic)
	if product.CreatedBy != userID {
		return nil, errors.New("you don't have permission to update this product")
	}

	// Build updates map
	updates := make(map[string]interface{})
	if dto.Name != nil {
		updates["name"] = *dto.Name
	}
	if dto.Description != nil {
		updates["description"] = *dto.Description
	}
	if dto.Price != nil {
		updates["price"] = *dto.Price
	}
	if dto.Stock != nil {
		updates["stock"] = *dto.Stock
	}

	if len(updates) == 0 {
		return product, nil
	}

	// Update product
	if err := s.repo.UpdateFields(id, updates); err != nil {
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	// Get updated product
	return s.repo.GetByID(id)
}

// Delete deletes a product
func (s *ProductService) Delete(id uint, userID uint) error {
	// Get existing product
	product, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("product not found")
		}
		return fmt.Errorf("failed to get product: %w", err)
	}

	// Check ownership (optional: depends on business logic)
	if product.CreatedBy != userID {
		return errors.New("you don't have permission to delete this product")
	}

	// Delete product
	if err := s.repo.DeleteByID(id); err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}

	return nil
}

// Search searches products by name
func (s *ProductService) Search(name string, page, limit int) (*ProductListResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	products, err := s.repo.SearchByName(name, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to search products: %w", err)
	}

	// Convert to response
	var productResponses []*ProductResponse
	for _, p := range products {
		resp := p.ToProductResponse()
		productResponses = append(productResponses, &resp)
	}

	return &ProductListResponse{
		Products: productResponses,
		Total:    int64(len(productResponses)),
		Page:     page,
		Limit:    limit,
	}, nil
}
