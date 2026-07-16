package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/vaurd/food-delivery/internal/models"
	"github.com/vaurd/food-delivery/internal/repository"
)

// ---------------------------------------------------------------------------
// Input DTOs — one per event type, as parsed by the handler.
// ---------------------------------------------------------------------------

// CreateOrderInput carries the data needed to create a new order.
type CreateOrderInput struct {
	CustomerID   string       `json:"customerId"   binding:"required"`
	RestaurantID string       `json:"restaurantId" binding:"required"`
	Items        models.Items `json:"items"        binding:"required,min=1"`
}

// UpdateStatusInput carries the data needed to change an order's status.
type UpdateStatusInput struct {
	OrderID string       `json:"orderId" binding:"required,uuid"`
	Status  models.Status `json:"status"  binding:"required"`
}

// UpdateItemsInput carries the data needed to replace an order's item list.
type UpdateItemsInput struct {
	OrderID string       `json:"orderId" binding:"required,uuid"`
	Items   models.Items `json:"items"   binding:"required,min=1"`
}

// ---------------------------------------------------------------------------
// Sentinel errors returned by the service layer.
// ---------------------------------------------------------------------------

var (
	// ErrOrderNotFound is returned when an update references a non-existent order.
	ErrOrderNotFound = errors.New("order not found")

	// ErrInvalidStatusTransition is returned when the requested status change is not allowed.
	ErrInvalidStatusTransition = errors.New("invalid status transition")

	// ErrInvalidStatus is returned when the status string is not a known enum value.
	ErrInvalidStatus = errors.New("invalid status")

	// ErrInvalidItems is returned when items contain invalid data (e.g. qty ≤ 0).
	ErrInvalidItems = errors.New("invalid items")
)

// ---------------------------------------------------------------------------
// OrderService interface.
// ---------------------------------------------------------------------------

// OrderService defines the business-logic contract for order operations.
type OrderService interface {
	CreateOrder(ctx context.Context, input CreateOrderInput) (*models.Order, error)
	UpdateStatus(ctx context.Context, input UpdateStatusInput) (*models.Order, error)
	UpdateItems(ctx context.Context, input UpdateItemsInput) (*models.Order, error)
	GetOrder(ctx context.Context, id string) (*models.Order, error)
	ListOrders(ctx context.Context, filters repository.ListFilters) ([]models.Order, int64, error)
}

// ---------------------------------------------------------------------------
// Concrete implementation.
// ---------------------------------------------------------------------------

type orderService struct {
	repo   repository.OrderRepository
	logger *zap.Logger
}

// NewOrderService creates a new OrderService backed by the given repository.
func NewOrderService(repo repository.OrderRepository, logger *zap.Logger) OrderService {
	return &orderService{repo: repo, logger: logger}
}

// CreateOrder generates a new UUID, inserts the order with status "Received",
// and returns the persisted record.
func (s *orderService) CreateOrder(ctx context.Context, input CreateOrderInput) (*models.Order, error) {
	if err := validateItems(input.Items); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	order := &models.Order{
		OrderID:      uuid.New().String(),
		CustomerID:   input.CustomerID,
		RestaurantID: input.RestaurantID,
		Status:       models.StatusReceived,
		Items:        input.Items,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	s.logger.Info("order created",
		zap.String("orderId", order.OrderID),
		zap.String("customerId", order.CustomerID),
	)
	return order, nil
}

// UpdateStatus changes the status of an existing order, enforcing the allowed
// transition rules.  The update runs inside a transaction with a row-level lock
// (SELECT FOR UPDATE) to prevent lost updates under concurrent writes.
func (s *orderService) UpdateStatus(ctx context.Context, input UpdateStatusInput) (*models.Order, error) {
	if !input.Status.IsValid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidStatus, input.Status)
	}

	var updated *models.Order

	err := s.repo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := s.repo.FindByIDForUpdate(ctx, tx, input.OrderID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrOrderNotFound
			}
			return fmt.Errorf("find order for update: %w", err)
		}

		// Enforce status transition rules.
		if !order.Status.CanTransitionTo(input.Status) {
			return fmt.Errorf("%w: %s → %s", ErrInvalidStatusTransition, order.Status, input.Status)
		}

		order.Status = input.Status
		order.UpdatedAt = time.Now().UTC()

		if err := s.repo.Update(ctx, tx, order); err != nil {
			return err
		}

		updated = order
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("order status updated",
		zap.String("orderId", updated.OrderID),
		zap.String("status", string(updated.Status)),
	)
	return updated, nil
}

// UpdateItems replaces the item list on an existing order. Also runs inside a
// transaction with a row-level lock.
func (s *orderService) UpdateItems(ctx context.Context, input UpdateItemsInput) (*models.Order, error) {
	if err := validateItems(input.Items); err != nil {
		return nil, err
	}

	var updated *models.Order

	err := s.repo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := s.repo.FindByIDForUpdate(ctx, tx, input.OrderID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrOrderNotFound
			}
			return fmt.Errorf("find order for update: %w", err)
		}

		order.Items = input.Items
		order.UpdatedAt = time.Now().UTC()

		if err := s.repo.Update(ctx, tx, order); err != nil {
			return err
		}

		updated = order
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("order items updated",
		zap.String("orderId", updated.OrderID),
		zap.Int("itemCount", len(updated.Items)),
	)
	return updated, nil
}

// GetOrder returns a single order by ID.
func (s *orderService) GetOrder(ctx context.Context, id string) (*models.Order, error) {
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("get order: %w", err)
	}
	return order, nil
}

// ListOrders returns a paginated, optionally filtered list of orders.
func (s *orderService) ListOrders(ctx context.Context, filters repository.ListFilters) ([]models.Order, int64, error) {
	orders, total, err := s.repo.List(ctx, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	return orders, total, nil
}

// ---------------------------------------------------------------------------
// Helpers.
// ---------------------------------------------------------------------------

// validateItems ensures every item has a non-empty ItemID and a positive Qty.
func validateItems(items models.Items) error {
	if len(items) == 0 {
		return fmt.Errorf("%w: items must not be empty", ErrInvalidItems)
	}
	for i, it := range items {
		if it.ItemID == "" {
			return fmt.Errorf("%w: item[%d] itemId is required", ErrInvalidItems, i)
		}
		if it.Qty <= 0 {
			return fmt.Errorf("%w: item[%d] qty must be > 0", ErrInvalidItems, i)
		}
	}
	return nil
}
