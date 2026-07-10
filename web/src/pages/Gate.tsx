import { FormEvent, useState } from "react";
import { api } from "../App";

type Props = {
  setupRequired: boolean;
  onDone: () => Promise<void>;
};

export default function Gate({ setupRequired, onDone }: Props) {
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    if (setupRequired && password !== confirm) {
      setError("Passwords do not match");
      return;
    }
    setBusy(true);
    try {
      if (setupRequired) {
        await api("/api/setup", {
          method: "POST",
          body: JSON.stringify({ password }),
        });
      } else {
        await api("/api/login", {
          method: "POST",
          body: JSON.stringify({ password }),
        });
      }
      await onDone();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Request failed");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="shell center">
      <div className="atmosphere" aria-hidden />
      <form className="card gate" onSubmit={submit}>
        <p className="brand">Boomerang</p>
        <h1>{setupRequired ? "First flight" : "Welcome back"}</h1>
        <p className="lede">
          {setupRequired
            ? "Set the single admin password for this appliance."
            : "Sign in to manage file servers, databases, and rollbacks."}
        </p>
        <div className="err" role="alert">
          {error}
        </div>
        <label htmlFor="password">Password</label>
        <input
          id="password"
          type="password"
          autoComplete={setupRequired ? "new-password" : "current-password"}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          minLength={setupRequired ? 8 : 1}
        />
        {setupRequired && (
          <>
            <label htmlFor="confirm">Confirm</label>
            <input
              id="confirm"
              type="password"
              autoComplete="new-password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              required
              minLength={8}
            />
          </>
        )}
        <button type="submit" disabled={busy}>
          {busy ? "Working…" : setupRequired ? "Create admin" : "Sign in"}
        </button>
      </form>
    </div>
  );
}
