/**
 * server.js
 *
 * Food Delivery Order Event Generator
 * ------------------------------------
 * Continuously generates randomised order events and POSTs them to the
 * Go backend's POST /events endpoint.
 *
 * Event mix (configurable via .env):
 *   40%  order.create
 *   30%  order.update.status
 *   30%  order.update.items
 *
 * If no orders have been created yet, only order.create events are sent.
 *
 * Configuration (environment variables / .env):
 *   BACKEND_URL        Base URL of the Go backend (default: http://localhost:8080)
 *   EVENT_INTERVAL_MS  Milliseconds between events   (default: 500)
 *   LOG_EVENTS         Set to "true" to log each event payload (default: false)
 */

require("dotenv").config();

const {
  buildCreateEvent,
  buildStatusUpdateEvent,
  buildItemsUpdateEvent,
} = require("./services/eventService");

const { postJSON } = require("./utils/http");

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------
const BACKEND_URL = process.env.BACKEND_URL || "http://localhost:8080";
const EVENT_INTERVAL_MS = parseInt(process.env.EVENT_INTERVAL_MS || "500", 10);
const LOG_EVENTS = process.env.LOG_EVENTS === "true";
const EVENTS_ENDPOINT = `${BACKEND_URL}/events`;

// ---------------------------------------------------------------------------
// State — in-memory list of confirmed order IDs.
// Only orders that have been successfully created are tracked here.
// Update events are only sent for confirmed orders.
// ---------------------------------------------------------------------------
const confirmedOrderIds = [];

// ---------------------------------------------------------------------------
// Event generation weights.
// ---------------------------------------------------------------------------
// [0, 40) → create
// [40, 70) → status update
// [70, 100) → items update
const WEIGHT_CREATE = 40;
const WEIGHT_STATUS = 30;
// WEIGHT_ITEMS = 30 (remainder)

// ---------------------------------------------------------------------------
// Core loop
// ---------------------------------------------------------------------------

/**
 * generateAndSend picks a random event type, builds the payload, sends it
 * to the backend, and (for create events) stores the new orderId.
 */
async function generateAndSend() {
  try {
    const roll = Math.random() * 100;
    let event;

    // Only send update events if we have at least one confirmed order.
    if (confirmedOrderIds.length === 0 || roll < WEIGHT_CREATE) {
      event = buildCreateEvent();
    } else if (roll < WEIGHT_CREATE + WEIGHT_STATUS) {
      const orderId = randomOrderId();
      event = buildStatusUpdateEvent(orderId);
    } else {
      const orderId = randomOrderId();
      event = buildItemsUpdateEvent(orderId);
    }

    if (LOG_EVENTS) {
      console.log(`[generator] sending ${event.type}`, JSON.stringify(event));
    } else {
      process.stdout.write(`[generator] → ${event.type}\n`);
    }

    const response = await postJSON(EVENTS_ENDPOINT, event);

    // On a successful create, track the new orderId so future update events
    // can reference it.
    if (event.type === "order.create" && response?.data?.orderId) {
      confirmedOrderIds.push(response.data.orderId);
      console.log(
        `[generator] ✓ order created: ${response.data.orderId} (total tracked: ${confirmedOrderIds.length})`
      );
    }
  } catch (err) {
    // Log but never crash — the loop must keep running.
    console.error("[generator] error:", err.message);
  }
}

/**
 * randomOrderId picks a random ID from the confirmed orders pool.
 */
function randomOrderId() {
  const idx = Math.floor(Math.random() * confirmedOrderIds.length);
  return confirmedOrderIds[idx];
}

// ---------------------------------------------------------------------------
// Bootstrap
// ---------------------------------------------------------------------------

console.log("=".repeat(60));
console.log("  Food Delivery Order Event Generator");
console.log("=".repeat(60));
console.log(`  Backend URL  : ${BACKEND_URL}`);
console.log(`  Interval     : ${EVENT_INTERVAL_MS}ms (~${Math.round(1000 / EVENT_INTERVAL_MS * 10) / 10} events/sec)`);
console.log(`  Log payloads : ${LOG_EVENTS}`);
console.log("=".repeat(60));
console.log("");

// Start the loop.
setInterval(generateAndSend, EVENT_INTERVAL_MS);

// Run the first event immediately without waiting for the first interval tick.
generateAndSend();

// Graceful shutdown.
process.on("SIGINT", () => {
  console.log("\n[generator] shutting down…");
  process.exit(0);
});
process.on("SIGTERM", () => {
  console.log("\n[generator] shutting down…");
  process.exit(0);
});
