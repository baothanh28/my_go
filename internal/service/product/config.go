package product

// ProductConfig holds product service specific configuration
type ProductConfig struct {
	MaxPageSize     int
	DefaultPageSize int
}

// NewProductConfig creates product config with defaults
func NewProductConfig() *ProductConfig {
	return &ProductConfig{
		MaxPageSize:     100,
		DefaultPageSize: 10,
	}
}
