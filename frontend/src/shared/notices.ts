import { ApiError } from "../api";
import type { RunnerUpdateAllResponse } from "../types";

export type Notice = {
  tone: "info" | "success" | "danger";
  message: string;
};


export function errorNotice(error: unknown, fallback: string): Notice {
  if (error instanceof ApiError) {
    return {
      tone: "danger",
      message: `${error.message} (${error.code})`,
    };
  }

  if (error instanceof Error) {
    return {
      tone: "danger",
      message: error.message || fallback,
    };
  }

  return {
    tone: "danger",
    message: fallback,
  };
}


export function runnerUpdateNotice(response: RunnerUpdateAllResponse): Notice {
  const tone = response.failed > 0 ? "danger" : response.accepted > 0 ? "success" : "info";
  const details = [`${response.accepted} started`, `${response.skipped} skipped`];
  if (response.failed > 0) {
    details.push(`${response.failed} failed`);
  }
  const skippedReason = response.results.find((result) => result.status === "skipped" && result.message.trim());
  return {
    tone,
    message: `Runner update requested: ${details.join(", ")}.${skippedReason ? ` ${skippedReason.message}` : ""}`,
  };
}
