/**
 * http.js
 *
 * Lightweight HTTP wrapper around the native Node.js fetch API.
 * Includes retry logic with exponential back-off so the generator keeps
 * running even when the backend is temporarily unavailable (e.g. during
 * container start-up).
 */

const MAX_RETRIES = 3;
const INITIAL_RETRY_DELAY_MS = 500;

/**
 * postJSON sends a POST request with a JSON body to the given URL.
 * Retries up to MAX_RETRIES times on network errors or 5xx responses.
 *
 * @param {string} url
 * @param {object} body
 * @returns {Promise<object>} Parsed JSON response body
 */
async function postJSON(url, body) {
  let lastError;

  for (let attempt = 1; attempt <= MAX_RETRIES; attempt++) {
    try {
      const response = await fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });

      // Parse response body regardless of status so we can log it.
      let data;
      try {
        data = await response.json();
      } catch {
        data = null;
      }

      if (!response.ok) {
        // Surface 4xx errors immediately — no point retrying client errors.
        if (response.status < 500) {
          console.warn(
            `[http] ${response.status} from backend — body:`,
            JSON.stringify(data)
          );
          return data;
        }
        throw new Error(`HTTP ${response.status}`);
      }

      return data;
    } catch (err) {
      lastError = err;
      if (attempt < MAX_RETRIES) {
        const delay = INITIAL_RETRY_DELAY_MS * Math.pow(2, attempt - 1);
        console.warn(
          `[http] attempt ${attempt}/${MAX_RETRIES} failed (${err.message}), retrying in ${delay}ms…`
        );
        await sleep(delay);
      }
    }
  }

  console.error(`[http] all ${MAX_RETRIES} attempts failed:`, lastError?.message);
  throw lastError;
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

module.exports = { postJSON };
