import { Component, type ErrorInfo, type ReactNode } from "react";

type Props = { children: ReactNode };
type State = { error: Error | null };

export default class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("Boomerang UI error", error, info);
  }

  render() {
    if (this.state.error) {
      return (
        <div className="shell center">
          <div className="card gate">
            <p className="brand">Boomerang</p>
            <h1>Something went wrong</h1>
            <p className="lede">The page failed to load. Try a refresh or sign in again.</p>
            <p className="err">{this.state.error.message}</p>
            <button type="button" onClick={() => window.location.assign("/")}>
              Back to sign in
            </button>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}
