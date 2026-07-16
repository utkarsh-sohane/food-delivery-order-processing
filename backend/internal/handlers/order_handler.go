package handlers

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/vaurd/food-delivery/internal/repository"
	"github.com/vaurd/food-delivery/internal/services"
	"github.com/vaurd/food-delivery/internal/utils"
)

// OrderHandler handles order-query endpoints.
type OrderHandler struct {
	orderSvc services.OrderService
	logger   *zap.Logger
}

// NewOrderHandler creates a new OrderHandler.
func NewOrderHandler(orderSvc services.OrderService, logger *zap.Logger) *OrderHandler {
	return &OrderHandler{orderSvc: orderSvc, logger: logger}
}

// ListOrders returns a paginated, optionally filtered list of orders.
//
// GET /orders
//
// Query parameters:
//
//	page    int    (default 1)
//	limit   int    (default 20, max 100)
//	status  string (Received | Preparing | Complete | Cancelled)
//	sort    string (default "updated_at")
//	order   string ("asc" | "desc", default "desc")
func (h *OrderHandler) ListOrders(c *gin.Context) {
	filters := repository.ListFilters{
		Status:    c.Query("status"),
		SortField: c.Query("sort"),
		SortOrder: c.Query("order"),
		Page:      parseIntQuery(c, "page", 1),
		Limit:     parseIntQuery(c, "limit", 20),
	}

	orders, total, err := h.orderSvc.ListOrders(c.Request.Context(), filters)
	if err != nil {
		h.logger.Error("list orders failed", zap.Error(err))
		utils.RespondInternalError(c, "failed to retrieve orders")
		return
	}

	meta := utils.PaginationMeta{
		Page:       filters.Page,
		Limit:      filters.Limit,
		Total:      total,
		TotalPages: utils.ComputeTotalPages(total, filters.Limit),
	}

	utils.RespondList(c, orders, meta)
}

// GetOrder returns a single order by ID.
//
// GET /orders/:id
func (h *OrderHandler) GetOrder(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		utils.RespondBadRequest(c, "order id is required")
		return
	}

	order, err := h.orderSvc.GetOrder(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrOrderNotFound) {
			utils.RespondNotFound(c, "order not found: "+id)
			return
		}
		h.logger.Error("get order failed", zap.Error(err), zap.String("orderId", id))
		utils.RespondInternalError(c, "failed to retrieve order")
		return
	}

	utils.RespondSuccess(c, order)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseIntQuery parses a query parameter as an int, returning fallback on error.
func parseIntQuery(c *gin.Context, key string, fallback int) int {
	raw := c.Query(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return fallback
	}
	return v
}
