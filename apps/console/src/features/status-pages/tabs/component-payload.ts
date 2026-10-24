import type { ApiStatusPageComponentRequest, ApiStatusPageComponentResponse } from "@/orion-sdk";

/** Full update body for a component; the API replaces every field it is sent. */
export const componentPayload = (
  component: ApiStatusPageComponentResponse,
  changes: Partial<ApiStatusPageComponentRequest> = {},
): ApiStatusPageComponentRequest => ({
  display_mode: component.display_mode,
  manual_status: component.manual_status,
  manual_status_reason: component.manual_status_reason,
  public_description: component.public_description,
  public_name: component.public_name,
  section_id: component.section_id,
  sort_order: component.sort_order,
  visible: component.visible ?? true,
  ...changes,
});
