/**
 * eventService.js
 *
 * Builds randomised event payloads for all three event types.
 * Uses @faker-js/faker for realistic-looking random data.
 */

const { faker } = require("@faker-js/faker");

// Fixed pools of restaurant and item IDs make the data feel more realistic —
// a small number of restaurants each serving a realistic menu.
const RESTAURANT_IDS = Array.from({ length: 10 }, () => faker.string.uuid());
const ITEM_IDS = Array.from({ length: 30 }, () => faker.string.uuid());

// Valid statuses in the order they can be applied by the generator.
// The generator only moves orders forward (no backwards transitions),
// which avoids sending invalid transition events.
const STATUS_PROGRESSION = ["Preparing", "Complete", "Cancelled"];

/**
 * buildCreateEvent creates a randomised order.create payload.
 *
 * @returns {object}
 */
function buildCreateEvent() {
  return {
    type: "order.create",
    customerId: faker.string.uuid(),
    restaurantId: faker.helpers.arrayElement(RESTAURANT_IDS),
    items: buildRandomItems(),
  };
}

/**
 * buildStatusUpdateEvent creates a randomised order.update.status payload.
 *
 * The generator intentionally picks any of the four statuses at random.
 * Invalid transitions (e.g. Complete → Preparing) are rejected by the
 * backend with a 400, which is expected behaviour and is logged.
 *
 * @param {string} orderId
 * @returns {object}
 */
function buildStatusUpdateEvent(orderId) {
  // Pick a status from the valid progression (skipping Received — that's only set on create).
  const status = faker.helpers.arrayElement(STATUS_PROGRESSION);
  return {
    type: "order.update.status",
    orderId,
    status,
  };
}

/**
 * buildItemsUpdateEvent creates a randomised order.update.items payload.
 *
 * @param {string} orderId
 * @returns {object}
 */
function buildItemsUpdateEvent(orderId) {
  return {
    type: "order.update.items",
    orderId,
    items: buildRandomItems(),
  };
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * buildRandomItems generates a random non-empty list of order items.
 *
 * @returns {Array<{itemId: string, qty: number}>}
 */
function buildRandomItems() {
  const count = faker.number.int({ min: 1, max: 5 });
  // Use a Set to avoid duplicate itemIds within the same order.
  const picked = new Set();
  while (picked.size < count) {
    picked.add(faker.helpers.arrayElement(ITEM_IDS));
  }
  return Array.from(picked).map((itemId) => ({
    itemId,
    qty: faker.number.int({ min: 1, max: 10 }),
  }));
}

module.exports = {
  buildCreateEvent,
  buildStatusUpdateEvent,
  buildItemsUpdateEvent,
};
