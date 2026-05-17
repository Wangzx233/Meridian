import { useEffect, useMemo, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type { ListResponse, Run, RunEvent, Task } from "./types";

const streamedEventNames = ["run.state", "codex.event", "process.output", "runner.error", "run.final"] as const;

export type StreamState = "idle" | "connecting" | "live" | "reconnecting" | "closed" | "error";

export function useRunEventStream(runId: string | null, enabled: boolean) {
  const queryClient = useQueryClient();
  const [events, setEvents] = useState<RunEvent[]>([]);
  const [state, setState] = useState<StreamState>("idle");
  const lastSeqRef = useRef(0);
  const sourceRef = useRef<EventSource | null>(null);
  const retryRef = useRef<number | null>(null);
  const terminalRef = useRef(false);

  useEffect(() => {
    setEvents([]);
    lastSeqRef.current = 0;
    terminalRef.current = false;

    if (!runId || !enabled) {
      setState("idle");
      return;
    }

    let disposed = false;

    const closeSource = () => {
      sourceRef.current?.close();
      sourceRef.current = null;
    };

    const mergeEvent = (event: RunEvent) => {
      setEvents((current) => {
        if (current.some((item) => item.seq === event.seq)) {
          return current;
        }
        return [...current, event].sort((a, b) => a.seq - b.seq);
      });
      lastSeqRef.current = Math.max(lastSeqRef.current, event.seq);

      if (event.event_type === "run.final") {
        terminalRef.current = true;
        setState("closed");
        closeSource();
        const status = event.payload.status;
        queryClient.setQueriesData<ListResponse<Run>>({ queryKey: ["runs"] }, (current) => {
          if (!current) {
            return current;
          }
          let changed = false;
          const items = current.items.map((run) => {
            if (run.id !== event.run_id) {
              return run;
            }
            changed = true;
            return {
              ...run,
              status: status ?? run.status,
              final_message:
                typeof event.payload.final_message === "string" ? event.payload.final_message : run.final_message,
              error_message:
                typeof event.payload.error_message === "string" ? event.payload.error_message : run.error_message,
              codex_session_id:
                typeof event.payload.codex_session_id === "string"
                  ? event.payload.codex_session_id
                  : run.codex_session_id,
              exit_code:
                typeof event.payload.exit_code === "number" || event.payload.exit_code === null
                  ? event.payload.exit_code
                  : run.exit_code,
              ended_at: event.occurred_at,
            };
          });
          return changed ? { ...current, items } : current;
        });
        queryClient.setQueriesData<Task>({ queryKey: ["task"] }, (current) => {
          if (!current || current.id !== event.task_id || current.active_run_id !== event.run_id) {
            return current;
          }
          return {
            ...current,
            status: current.status === "running" ? "waiting_user" : current.status,
            active_run_id: null,
            codex_session_id:
              typeof event.payload.codex_session_id === "string"
                ? event.payload.codex_session_id
                : current.codex_session_id,
          };
        });
        queryClient.setQueriesData<ListResponse<Task>>({ queryKey: ["tasks"] }, (current) => {
          if (!current) {
            return current;
          }
          let changed = false;
          const items = current.items.map((task) => {
            if (task.id !== event.task_id || task.active_run_id !== event.run_id) {
              return task;
            }
            changed = true;
            return {
              ...task,
              status: task.status === "running" ? "waiting_user" : task.status,
              active_run_id: null,
              codex_session_id:
                typeof event.payload.codex_session_id === "string"
                  ? event.payload.codex_session_id
                  : task.codex_session_id,
            };
          });
          return changed ? { ...current, items } : current;
        });
        void queryClient.invalidateQueries({ queryKey: ["runs"] });
        void queryClient.invalidateQueries({ queryKey: ["tasks"] });
        void queryClient.invalidateQueries({ queryKey: ["task"] });
      }
    };

    const connect = () => {
      if (disposed || terminalRef.current) {
        return;
      }

      setState(lastSeqRef.current > 0 ? "reconnecting" : "connecting");
      closeSource();

      const source = new EventSource(api.streamRunEventsUrl(runId, lastSeqRef.current), { withCredentials: true });
      sourceRef.current = source;

      const handleMessage = (message: MessageEvent<string>) => {
        if (disposed) {
          return;
        }
        setState("live");
        try {
          mergeEvent(JSON.parse(message.data) as RunEvent);
        } catch {
          setState("error");
        }
      };

      streamedEventNames.forEach((eventName) => {
        source.addEventListener(eventName, handleMessage);
      });

      source.onopen = () => {
        if (!disposed) {
          setState("live");
        }
      };

      source.onerror = () => {
        if (disposed || terminalRef.current) {
          return;
        }
        setState("reconnecting");
        closeSource();
        retryRef.current = window.setTimeout(connect, 1_500);
      };
    };

    void api
      .listRunEvents(runId, 0)
      .then((response) => {
        if (disposed) {
          return;
        }
        response.items.forEach(mergeEvent);
        if (!terminalRef.current) {
          connect();
        }
      })
      .catch(() => {
        if (!disposed) {
          connect();
        }
      });

    return () => {
      disposed = true;
      if (retryRef.current) {
        window.clearTimeout(retryRef.current);
      }
      closeSource();
    };
  }, [enabled, queryClient, runId]);

  return useMemo(() => ({ events, state }), [events, state]);
}
