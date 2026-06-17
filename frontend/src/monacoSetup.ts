/**
 * Configures @monaco-editor/react to use the locally bundled monaco-editor
 * package instead of loading it from the jsdelivr CDN at runtime.
 *
 * Gaze is a Wails desktop app — without this, the plugin editor, JSON editor
 * and code blocks all fail to load when the machine is offline (or the CDN is
 * blocked). Import this module (side-effect only) from every component that
 * renders an <Editor> from @monaco-editor/react.
 */
import * as monaco from "monaco-editor";
import { loader } from "@monaco-editor/react";

// Vite worker imports: each becomes its own emitted chunk, loaded on demand.
import EditorWorker from "monaco-editor/esm/vs/editor/editor.worker?worker";
import JsonWorker from "monaco-editor/esm/vs/language/json/json.worker?worker";
import TsWorker from "monaco-editor/esm/vs/language/typescript/ts.worker?worker";

self.MonacoEnvironment = {
  getWorker(_workerId: string, label: string) {
    if (label === "json") {
      return new JsonWorker();
    }
    if (label === "typescript" || label === "javascript") {
      return new TsWorker();
    }
    return new EditorWorker();
  },
};

loader.config({ monaco });
