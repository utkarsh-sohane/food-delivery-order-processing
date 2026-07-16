package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vaurd/food-delivery/internal/models"
)

// ListFilters encapsulates the query parameters accepted by List.
type ListFilters struct {
	// Status filters orders to only those with the given status.
	// Empty string means "all statuses".
	Status string

	// SortField is the column to order by (e.g. "updated_at").
	SortField string

	// SortOrder is either "asc" or "desc".
	SortOrder string

	// Page is 1-indexed.
	Page int

	// Limit is the maximum number of records per page.
	Limit int
}

// OrderRepository defines the data-access contract for orders.
// Keeping an interface here allows the service layer to be tested without a
// real database.
type OrderRepository interface {
	// Create inserts a new order record.
	Create(ctx context.Context, order *models.Order) error

	// FindByID returns the order with the given ID, or gorm.ErrRecordNotFound.
	FindByID(ctx context.Context, id string) (*models.Order, error)

	// FindByIDForUpdate returns the order and holds a row-level lock (SELECT FOR UPDATE)
	// for the duration of the surrounding transaction.
	FindByIDForUpdate(ctx context.Context, tx *gorm.DB, id string) (*models.Order, error)

	// Update saves the changed fields of an existing order.
	Update(ctx context.Context, tx *gorm.DB, order *models.Order) error

	// List returns a page of orders with total count.
	List(ctx context.Context, filters ListFilters) ([]models.Order, int64, error)

	// DB exposes the underlying *gorm.DB so the service layer can begin
	// transactions when it needs to lock rows.
	DB() *gorm.DB
}

// ---------------------------------------------------------------------------
// gormOrderRepository — concrete implementation backed by PostgreSQL.
// ---------------------------------------------------------------------------

type gormOrderRepository struct {
	db *gorm.DB
}

// NewOrderRepository creates a new GORM-backed OrderRepository.
func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &gormOrderRepository{db: db}
}

func (r *gormOrderRepository) DB() *gorm.DB {
	return r.db
}

func (r *gormOrderRepository) Create(ctx context.Context, order *models.Order) error {
	if err := r.db.WithContext(ctx).Create(order).Error; err != nil {
		return fmt.Errorf("create order: %w", err)
	}
	return nil
}

func (r *gormOrderRepository) FindByID(ctx context.Context, id string) (*models.Order, error) {
	var order models.Order
	if err := r.db.WithContext(ctx).First(&order, "order_id = ?", id).Error; err != nil {
		return nil, err // callers check for gorm.ErrRecordNotFound
	}
	return &order, nil
}

// FindByIDForUpdate issues SELECT … FOR UPDATE within the provided transaction.
// This serialises concurrent updates to the same row at the database level.
func (r *gormOrderRepository) FindByIDForUpdate(ctx context.Context, tx *gorm.DB, id string) (*models.Order, error) {
	var order models.Order
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&order, "order_id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *gormOrderRepository) Update(ctx context.Context, tx *gorm.DB, order *models.Order) error {
	// Save updates every field, which is what we want for full-model replacements.
	if err := tx.WithContext(ctx).Save(order).Error; err != nil {
		return fmt.Errorf("update order: %w", err)
	}
	return nil
}

func (r *gormOrderRepository) List(ctx context.Context, filters ListFilters) ([]models.Order, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.Order{})

	// Apply optional status filter.
	if filters.Status != "" {
		query = query.Where("status = ?", filters.Status)
	}

	// Count total matching records before pagination.
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}

	// Sorting — default to updated_at DESC.
	sortField := filters.SortField
	if sortField == "" {
		sortField = "updated_at"
	}
	sortOrder := filters.SortOrder
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	query = query.Order(fmt.Sprintf("%s %s", sortField, sortOrder))

	// Pagination — default page=1, limit=20.
	page := filters.Page
	if page < 1 {
		page = 1
	}
	limit := filters.Limit
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit
	query = query.Offset(offset).Limit(limit)

	var orders []models.Order
	if err := query.Find(&orders).Error; err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}

	return orders, total, nil
}
