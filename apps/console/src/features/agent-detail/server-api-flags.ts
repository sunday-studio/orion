/**
 * Core has no PATCH /v1/agents/{id} or decommission route yet (orion:backend builds them for F3).
 * Flip these when the routes land; until then the dialogs explain instead of failing silently.
 */
export const SERVER_EDIT_API_AVAILABLE = false;
export const SERVER_REMOVE_API_AVAILABLE = false;
