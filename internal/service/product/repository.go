package product

import (
	"errors"
	"fmt"

	"myapp/internal/pkg/database"

	"gorm.io/gorm"
)

// ProductRepository handles product data access
type ProductRepository struct {
	*database.BaseRepository[Product]
	db *database.Database
}

// NewProductRepository creates a new product repository
func NewProductRepository(db *database.Database) *ProductRepository {
	return &ProductRepository{
		BaseRepository: database.NewBaseRepository[Product](db),
		db:             db,
	}
}

// GetBySKU retrieves a product by SKU (domain-specific method)
func (r *ProductRepository) GetBySKU(sku string) (*Product, error) {
	return r.GetByField("sku", sku)
}

// GetByCreatedBy retrieves products created by a specific user (domain-specific method)
func (r *ProductRepository) GetByCreatedBy(userID uint, limit, offset int) ([]*Product, error) {
	conditions := map[string]interface{}{"created_by": userID}
	return r.GetWhere(conditions, limit, offset)
}

// CountByCreatedBy counts products created by a specific user
func (r *ProductRepository) CountByCreatedBy(userID uint) (int64, error) {
	conditions := map[string]interface{}{"created_by": userID}
	return r.Count(conditions)
}

// UpdateStock updates product stock (domain-specific method with transaction example)
func (r *ProductRepository) UpdateStock(id uint, quantity int) error {
	// Start transaction
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Get current product within transaction
		var product Product
		if err := tx.First(&product, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("product not found")
			}
			return fmt.Errorf("failed to get product: %w", err)
		}

		// Calculate new stock
		newStock := product.Stock + quantity
		if newStock < 0 {
			return errors.New("insufficient stock")
		}

		// Update stock
		if err := tx.Model(&product).Update("stock", newStock).Error; err != nil {
			return fmt.Errorf("failed to update stock: %w", err)
		}

		return nil
	})
}

// SearchByName searches products by name (domain-specific method)
func (r *ProductRepository) SearchByName(name string, limit, offset int) ([]*Product, error) {
	var products []*Product
	query := r.GetDB().Model(&Product{})

	// Use LIKE for partial matching
	query = query.Where("name ILIKE ?", "%"+name+"%")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&products).Error; err != nil {
		return nil, fmt.Errorf("failed to search products: %w", err)
	}

	return products, nil
}
