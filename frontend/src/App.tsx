import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import { LoginScreen, SetupScreen } from "./features/auth/AuthScreens";
import { WorkbenchApp } from "./features/workbench/WorkbenchApp";
import { errorNotice } from "./shared/notices";
import type { Notice } from "./shared/notices";
import { LoadingState } from "./shared/ui";

export function App() {
  const queryClient = useQueryClient();
  const [notice, setNotice] = useState<Notice | null>(null);
  const sessionQuery = useQuery({
    queryKey: ["auth", "session"],
    queryFn: api.getAuthSession,
    retry: false,
  });
  const loginMutation = useMutation({
    mutationFn: api.login,
    onSuccess: (session) => {
      queryClient.setQueryData(["auth", "session"], session);
      setNotice(null);
    },
    onError: (error) => {
      setNotice(errorNotice(error, "Login failed."));
    },
  });
  const setupMutation = useMutation({
    mutationFn: api.setupAuth,
    onSuccess: (session) => {
      queryClient.setQueryData(["auth", "session"], session);
      setNotice(null);
    },
    onError: (error) => {
      setNotice(errorNotice(error, "Setup failed."));
    },
  });
  const logoutMutation = useMutation({
    mutationFn: api.logout,
    onSuccess: () => {
      queryClient.clear();
      queryClient.setQueryData(["auth", "session"], { authenticated: false, username: "" });
    },
  });
  useEffect(() => {
    const onUnauthorized = () => {
      queryClient.clear();
      queryClient.setQueryData(["auth", "session"], { authenticated: false, username: "" });
    };
    window.addEventListener("ctw:unauthorized", onUnauthorized);
    return () => window.removeEventListener("ctw:unauthorized", onUnauthorized);
  }, [queryClient]);

  if (sessionQuery.isLoading) {
    return (
      <div className="authShell">
        <LoadingState label="Checking session" />
      </div>
    );
  }

  if (!sessionQuery.data?.authenticated && sessionQuery.data?.setup_required) {
    return (
      <SetupScreen
        error={notice?.tone === "danger" ? notice.message : null}
        saving={setupMutation.isPending}
        onSetup={(username, password) => setupMutation.mutate({ username, password })}
      />
    );
  }

  if (!sessionQuery.data?.authenticated) {
    return (
      <LoginScreen
        error={notice?.tone === "danger" ? notice.message : null}
        loggingIn={loginMutation.isPending}
        onLogin={(username, password) => loginMutation.mutate({ username, password })}
      />
    );
  }

  return (
    <WorkbenchApp
      session={sessionQuery.data}
      onLogout={() => logoutMutation.mutate()}
      loggingOut={logoutMutation.isPending}
    />
  );
}
