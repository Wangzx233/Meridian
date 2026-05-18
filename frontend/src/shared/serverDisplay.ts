import type { Server } from "../types";

export function serverDisplayName(server: Pick<Server, "alias" | "name"> | null | undefined) {
  const alias = server?.alias?.trim();
  return alias || server?.name || "";
}
