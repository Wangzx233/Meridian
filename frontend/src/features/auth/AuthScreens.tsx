import { FolderKanban, Save, Loader2 } from "lucide-react";
import { useState } from "react";
import type { FormEvent } from "react";
import { productName, productTagline } from "../../shared/constants";
import { MeridianIcon } from "../../shared/MeridianIcon";
import { InlineNotice } from "../../shared/ui";

export function LoginScreen(props: {
  error: string | null;
  loggingIn: boolean;
  onLogin: (username: string, password: string) => void;
}) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!username.trim() || !password) {
      return;
    }
    props.onLogin(username.trim(), password);
  };

  return (
    <main className="authShell" aria-label="Workbench login">
      <section className="loginPanel">
        <div className="loginHeader">
          <span className="brandMark" aria-hidden="true">
            <MeridianIcon size={24} />
          </span>
          <div>
            <h1>{productName}</h1>
            <p>{productTagline}</p>
          </div>
        </div>
        <form className="loginForm" onSubmit={submit}>
          <label htmlFor="login-username">
            Username
            <input
              id="login-username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              autoComplete="username"
              disabled={props.loggingIn}
              autoFocus
            />
          </label>
          <label htmlFor="login-password">
            Password
            <input
              id="login-password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="current-password"
              disabled={props.loggingIn}
            />
          </label>
          {props.error ? <InlineNotice tone="danger">{props.error}</InlineNotice> : null}
          <button className="primaryButton" type="submit" disabled={props.loggingIn || !username.trim() || !password}>
            {props.loggingIn ? <Loader2 className="spin" size={16} /> : <FolderKanban size={16} />}
            Sign in
          </button>
        </form>
      </section>
    </main>
  );
}


export function SetupScreen(props: {
  error: string | null;
  saving: boolean;
  onSetup: (username: string, password: string) => void;
}) {
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const passwordMismatch = Boolean(confirmPassword) && password !== confirmPassword;
  const canSubmit = username.trim() && password.length >= 8 && password === confirmPassword;

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!canSubmit) {
      return;
    }
    props.onSetup(username.trim(), password);
  };

  return (
    <main className="authShell" aria-label="Workbench setup">
      <section className="loginPanel">
        <div className="loginHeader">
          <span className="brandMark" aria-hidden="true">
            <MeridianIcon size={24} />
          </span>
          <div>
            <h1>{productName}</h1>
            <p>Create the first access account</p>
          </div>
        </div>
        <form className="loginForm" onSubmit={submit}>
          <label htmlFor="setup-username">
            Username
            <input
              id="setup-username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              autoComplete="username"
              disabled={props.saving}
              autoFocus
            />
          </label>
          <label htmlFor="setup-password">
            Password
            <input
              id="setup-password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="new-password"
              disabled={props.saving}
            />
          </label>
          <label htmlFor="setup-confirm-password">
            Confirm password
            <input
              id="setup-confirm-password"
              type="password"
              value={confirmPassword}
              onChange={(event) => setConfirmPassword(event.target.value)}
              autoComplete="new-password"
              disabled={props.saving}
            />
          </label>
          {password && password.length < 8 ? (
            <InlineNotice tone="danger">Password must be at least 8 characters.</InlineNotice>
          ) : null}
          {passwordMismatch ? <InlineNotice tone="danger">Passwords do not match.</InlineNotice> : null}
          {props.error ? <InlineNotice tone="danger">{props.error}</InlineNotice> : null}
          <button className="primaryButton" type="submit" disabled={props.saving || !canSubmit}>
            {props.saving ? <Loader2 className="spin" size={16} /> : <Save size={16} />}
            Create account
          </button>
        </form>
      </section>
    </main>
  );
}
