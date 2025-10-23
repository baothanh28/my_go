package database

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// BaseRepository provides common CRUD operations for entities
type BaseRepository[T any] struct {
	db *gorm.DB
}

// NewBaseRepository creates a new base repository instance
func NewBaseRepository[T any](db *Database) *BaseRepository[T] {
	return &BaseRepository[T]{
		db: db.DB,
	}
}

// Insert creates a new entity
func (r *BaseRepository[T]) Insert(entity *T) error {
	if err := r.db.Create(entity).Error; err != nil {
		return fmt.Errorf("failed to insert entity: %w", err)
	}
	return nil
}

// InsertBatch creates multiple entities in batch
func (r *BaseRepository[T]) InsertBatch(entities []*T) error {
	if len(entities) == 0 {
		return nil
	}
	if err := r.db.Create(entities).Error; err != nil {
		return fmt.Errorf("failed to insert batch: %w", err)
	}
	return nil
}

// UpdateByID updates an entity by its ID
func (r *BaseRepository[T]) UpdateByID(id uint, entity *T) error {
	result := r.db.Model(new(T)).Where("id = ?", id).Updates(entity)
	if result.Error != nil {
		return fmt.Errorf("failed to update entity: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateByField updates an entity by a specific field
func (r *BaseRepository[T]) UpdateByField(field string, value interface{}, updates map[string]interface{}) error {
	result := r.db.Model(new(T)).Where(field+" = ?", value).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update entity: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateFields updates specific fields of an entity
func (r *BaseRepository[T]) UpdateFields(id uint, updates map[string]interface{}) error {
	result := r.db.Model(new(T)).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update fields: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetByID retrieves an entity by its ID
func (r *BaseRepository[T]) GetByID(id uint) (*T, error) {
	var entity T
	err := r.db.First(&entity, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}
	return &entity, nil
}

// GetByField retrieves an entity by a specific field
func (r *BaseRepository[T]) GetByField(field string, value interface{}) (*T, error) {
	var entity T
	err := r.db.Where(field+" = ?", value).First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}
	return &entity, nil
}

// GetAll retrieves all entities with pagination
func (r *BaseRepository[T]) GetAll(limit, offset int) ([]*T, error) {
	var entities []*T
	query := r.db.Model(new(T))

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&entities).Error; err != nil {
		return nil, fmt.Errorf("failed to get all entities: %w", err)
	}
	return entities, nil
}

// GetWhere retrieves entities matching conditions
func (r *BaseRepository[T]) GetWhere(conditions map[string]interface{}, limit, offset int) ([]*T, error) {
	var entities []*T
	query := r.db.Model(new(T))

	// Apply conditions
	for field, value := range conditions {
		query = query.Where(field+" = ?", value)
	}

	// Apply pagination
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&entities).Error; err != nil {
		return nil, fmt.Errorf("failed to get entities: %w", err)
	}
	return entities, nil
}

// DeleteByID deletes an entity by its ID
func (r *BaseRepository[T]) DeleteByID(id uint) error {
	result := r.db.Delete(new(T), id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete entity: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteByField deletes entities by a specific field
func (r *BaseRepository[T]) DeleteByField(field string, value interface{}) error {
	result := r.db.Where(field+" = ?", value).Delete(new(T))
	if result.Error != nil {
		return fmt.Errorf("failed to delete entities: %w", result.Error)
	}
	return nil
}

// DeleteWhere deletes entities matching conditions
func (r *BaseRepository[T]) DeleteWhere(conditions map[string]interface{}) error {
	query := r.db.Model(new(T))
	for field, value := range conditions {
		query = query.Where(field+" = ?", value)
	}

	if err := query.Delete(new(T)).Error; err != nil {
		return fmt.Errorf("failed to delete entities: %w", err)
	}
	return nil
}

// Count counts entities matching conditions
func (r *BaseRepository[T]) Count(conditions map[string]interface{}) (int64, error) {
	var count int64
	query := r.db.Model(new(T))

	for field, value := range conditions {
		query = query.Where(field+" = ?", value)
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count entities: %w", err)
	}
	return count, nil
}

// Exists checks if an entity exists matching conditions
func (r *BaseRepository[T]) Exists(conditions map[string]interface{}) (bool, error) {
	count, err := r.Count(conditions)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// WithTx returns a new repository instance with the given transaction
func (r *BaseRepository[T]) WithTx(tx *gorm.DB) *BaseRepository[T] {
	return &BaseRepository[T]{
		db: tx,
	}
}

// GetDB returns the underlying gorm.DB instance
func (r *BaseRepository[T]) GetDB() *gorm.DB {
	return r.db
}
