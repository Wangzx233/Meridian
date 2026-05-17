import { runnerFeatureCapabilities } from "./constants";
import type { Server } from "../types";

export function runnerCapabilitySummary(server: Server) {
  if (!server.runner_connected) {
    return "No active websocket";
  }
  const missing = missingRunnerCapabilities(server);
  if (missing.length === 0) {
    return "Files and terminal ready";
  }
  return `Missing ${missing.join(", ")}`;
}


export function runnerCapabilityPillLabel(server: Server | null) {
  if (!server?.runner_connected) {
    return "Runner disconnected";
  }
  return "Runner update required";
}


export function runnerCapabilityBlockedReason(server: Server | null, capability: string, feature: string) {
  if (!server) {
    return `Select the server for this project before using ${feature}.`;
  }
  if (!server.runner_connected) {
    return `No runner websocket is connected for ${server.name}. Start or reinstall the runner with runner id ${server.runner_id}.`;
  }
  const version = server.runner_connection?.version ? `version ${server.runner_connection.version}` : "an unknown version";
  const hostname = server.runner_connection?.hostname ? ` on ${server.runner_connection.hostname}` : "";
  return `The connected runner${hostname} is ${version}, but it did not report ${capability}. Re-run the runner installer for ${server.runner_id} after the latest deployment.`;
}


export function missingRunnerCapabilities(server: Server) {
  return runnerFeatureCapabilities.filter((capability) => server.runner_capabilities?.[capability] !== true);
}
