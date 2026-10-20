import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "@fontsource/jetbrains-mono/400.css";
import "@fontsource/jetbrains-mono/600.css";
import "@fontsource/ibm-plex-sans-condensed/500.css";
import "@fontsource/ibm-plex-sans-condensed/600.css";
import "./styles/tokens.css";
import "./styles/app.css";
import "./styles/panels.css";
import { App } from "./App";
import { Boundary } from "./Boundary";

const root = document.getElementById("root");
if (root) {
  createRoot(root).render(
    <StrictMode>
      <Boundary>
        <App />
      </Boundary>
    </StrictMode>,
  );
}
