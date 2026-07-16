package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/vaurd/food-delivery/internal/models"
	"github.com/vaurd/food-delivery/internal/services"
	"github.com/vaurd/food-delivery/internal/utils"
)

// ---------------------------------------------------------------------------
// Event envelope — the top-level JSON wrapper every inbound event must have.
// ---------------------------------------------------------------------------

// eventEnvelope is used to peek at the "type" field before full deserialization.
type eventEnvelope struct {
	Type string `json:"type"`
}

// ---------------------------------------------------------------------------
// EventHandler handles POST /events.
// ---------------------------------------------------------------------------

// EventHandler handles incoming order events.
type EventHandler struct {
	orderSvc services.OrderService
	logger   *zap.Logger
}

// NewEventHandler creates a new EventHandler.
func NewEventHandler(orderSvc services.OrderService, logger *zap.Logger) *EventHandler {
	return &EventHandler{orderSvc: orderSvc, logger: logger}
}

// Handle dispatches an incoming event to the correct processing path based on
// the "type" field in the JSON body.
//
// POST /events
func (h *EventHandler) Handle(c *gin.Context) {
	// Read the raw body so we can inspect the "type" field without consuming
	// the reader – we'll re-decode the full payload afterwards.
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		utils.RespondBadRequest(c, "invalid JSON: "+err.Error())
		return
	}

	// Unmarshal just the envelope to read the event type.
	var envelope eventEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		utils.RespondBadRequest(c, "cannot read event type: "+err.Error())
		return
	}

	h.logger.Info("event received", zap.String("type", envelope.Type))

	switch envelope.Type {
	case "order.create":
		h.handleCreate(c, raw)
	case "order.update.status":
		h.handleUpdateStatus(c, raw)
	case "order.update.items":
		h.handleUpdateItems(c, raw)
	default:
		utils.RespondBadRequest(c, "unknown event type: "+envelope.Type)
	}
}

// ---------------------------------------------------------------------------
// order.create
// ---------------------------------------------------------------------------

// createPayload is the expected body for an order.create event.
type createPayload struct {
	Type         string       `json:"type"`
	CustomerID   string       `json:"customerId"`
	RestaurantID string       `json:"restaurantId"`
	Items        models.Items `json:"items"`
}

func (h *EventHandler) handleCreate(c *gin.Context, raw json.RawMessage) {
	var payload createPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		utils.RespondBadRequest(c, "invalid order.create payload: "+err.Error())
		return
	}

	if payload.CustomerID == "" {
		utils.RespondBadRequest(c, "customerId is required")
		return
	}
	if payload.RestaurantID == "" {
		utils.RespondBadRequest(c, "restaurantId is required")
		return
	}
	if len(payload.Items) == 0 {
		utils.RespondBadRequest(c, "items must not be empty")
		return
	}

	input := services.CreateOrderInput{
		CustomerID:   payload.CustomerID,
		RestaurantID: payload.RestaurantID,
		Items:        payload.Items,
	}

	order, err := h.orderSvc.CreateOrder(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, services.ErrInvalidItems) {
			utils.RespondBadRequest(c, err.Error())
			return
		}
		h.logger.Error("create order failed", zap.Error(err))
		utils.RespondInternalError(c, "failed to create order")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": order})
}

// ---------------------------------------------------------------------------
// order.update.status
// ---------------------------------------------------------------------------

// updateStatusPayload is the expected body for an order.update.status event.
type updateStatusPayload struct {
	Type    string        `json:"type"`
	OrderID string        `json:"orderId"`
	Status  models.Status `json:"status"`
}

func (h *EventHandler) handleUpdateStatus(c *gin.Context, raw json.RawMessage) {
	var payload updateStatusPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		utils.RespondBadRequest(c, "invalid order.update.status payload: "+err.Error())
		return
	}

	if payload.OrderID == "" {
		utils.RespondBadRequest(c, "orderId is required")
		return
	}
	if payload.Status == "" {
		utils.RespondBadRequest(c, "status is required")
		return
	}

	input := services.UpdateStatusInput{
		OrderID: payload.OrderID,
		Status:  payload.Status,
	}

	order, err := h.orderSvc.UpdateStatus(c.Request.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrOrderNotFound):
			utils.RespondNotFound(c, "order not found: "+payload.OrderID)
		case errors.Is(err, services.ErrInvalidStatus):
			utils.RespondBadRequest(c, err.Error())
		case errors.Is(err, services.ErrInvalidStatusTransition):
			utils.RespondBadRequest(c, err.Error())
		default:
			h.logger.Error("update status failed", zap.Error(err), zap.String("orderId", payload.OrderID))
			utils.RespondInternalError(c, "failed to update order status")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": order})
}

// ---------------------------------------------------------------------------
// order.update.items
// ---------------------------------------------------------------------------

// updateItemsPayload is the expected body for an order.update.items event.
type updateItemsPayload struct {
	Type    string       `json:"type"`
	OrderID string       `json:"orderId"`
	Items   models.Items `json:"items"`
}

func (h *EventHandler) handleUpdateItems(c *gin.Context, raw json.RawMessage) {
	var payload updateItemsPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		utils.RespondBadRequest(c, "invalid order.update.items payload: "+err.Error())
		return
	}

	if payload.OrderID == "" {
		utils.RespondBadRequest(c, "orderId is required")
		return
	}
	if len(payload.Items) == 0 {
		utils.RespondBadRequest(c, "items must not be empty")
		return
	}

	input := services.UpdateItemsInput{
		OrderID: payload.OrderID,
		Items:   payload.Items,
	}

	order, err := h.orderSvc.UpdateItems(c.Request.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrOrderNotFound):
			utils.RespondNotFound(c, "order not found: "+payload.OrderID)
		case errors.Is(err, services.ErrInvalidItems):
			utils.RespondBadRequest(c, err.Error())
		default:
			h.logger.Error("update items failed", zap.Error(err), zap.String("orderId", payload.OrderID))
			utils.RespondInternalError(c, "failed to update order items")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": order})
}
