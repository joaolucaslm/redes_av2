import { Component, type ErrorInfo, type ReactNode } from "react";

type Props = { children: ReactNode };

type State = { error: Error | null };

export class Boundary extends Component<Props, State> {
  override state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  override componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error("[bancada]", error, info.componentStack);
  }

  override render(): ReactNode {
    if (this.state.error) {
      return (
        <div className="offline">
          <div className="label">falha na interface</div>
          <pre className="reject-reason">{this.state.error.message}</pre>
          <button className="key" onClick={() => this.setState({ error: null })}>
            tentar de novo
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
