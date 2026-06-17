import React, { useRef, useEffect } from "react";
import Editor, { OnMount } from "@monaco-editor/react";
import { useTheme } from "../ThemeContext";
import * as monaco from "monaco-editor";
import "../monacoSetup";

// 全局标记：completion providers 只需注册一次
let completionProvidersRegistered = false;
// 全局 disposables 列表：在组件卸载时不 dispose（因为是全局注册）
// 但可以在应用重新加载时清理
const globalDisposables: monaco.IDisposable[] = [];

interface MonacoPluginEditorProps {
  value: string;
  onChange: (value: string) => void;
  language: "javascript" | "typescript" | "json";
  height?: string;
}

const MonacoPluginEditor: React.FC<MonacoPluginEditorProps> = ({
  value,
  onChange,
  language,
  height = "500px",
}) => {
  const { isDark } = useTheme();
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);

  const handleEditorDidMount: OnMount = (editor, monaco) => {
    editorRef.current = editor;

    // 配置 JSONC (JSON with Comments) - 必须在其他配置之前
    monaco.languages.json.jsonDefaults.setDiagnosticsOptions({
      validate: true,
      allowComments: true,
      schemas: [],
      enableSchemaRequest: false,
    });

    // Debug: Check model language
    const model = editor.getModel();
    if (model) {
      console.log("[Monaco Debug] Model language:", model.getLanguageId());
      console.log("[Monaco Debug] Model URI:", model.uri.toString());
      console.log("[Monaco Debug] Prop language:", language);
      
      // Explicitly set model language to match the prop
      if (model.getLanguageId() !== language) {
        console.warn(`[Monaco Debug] Model language mismatch! Model is "${model.getLanguageId()}", but prop is "${language}". Setting to "${language}"...`);
        monaco.editor.setModelLanguage(model, language);
        console.log("[Monaco Debug] Model language after update:", model.getLanguageId());
      }
    }

    // 加载插件 API 类型定义
    fetch("/plugin.d.ts")
      .then((res) => res.text())
      .then((dts) => {
        // 添加类型定义到 Monaco
        monaco.languages.typescript.typescriptDefaults.addExtraLib(
          dts,
          "file:///node_modules/@types/gaze-plugin/index.d.ts"
        );

        monaco.languages.typescript.javascriptDefaults.addExtraLib(
          dts,
          "file:///node_modules/@types/gaze-plugin/index.d.ts"
        );
      })
      .catch((err) => {
        console.error("Failed to load plugin.d.ts:", err);
      });

    // 配置 TypeScript 编译器选项
    monaco.languages.typescript.typescriptDefaults.setCompilerOptions({
      target: monaco.languages.typescript.ScriptTarget.ES2020,
      allowNonTsExtensions: true,
      moduleResolution: monaco.languages.typescript.ModuleResolutionKind.NodeJs,
      module: monaco.languages.typescript.ModuleKind.CommonJS,
      noEmit: true,
      esModuleInterop: true,
      allowJs: true,
      checkJs: false,
      strict: true,
      noImplicitAny: false,
      strictNullChecks: false,
      strictFunctionTypes: true,
      strictPropertyInitialization: false,
      noImplicitThis: true,
      alwaysStrict: false,
      noUnusedLocals: false,
      noUnusedParameters: false,
      noImplicitReturns: false,
      noFallthroughCasesInSwitch: true,
      lib: ["es2020"],
    });

    // 配置 JavaScript 编译器选项
    monaco.languages.typescript.javascriptDefaults.setCompilerOptions({
      target: monaco.languages.typescript.ScriptTarget.ES2020,
      allowNonTsExtensions: true,
      allowJs: true,
      checkJs: true,
      noEmit: true,
      lib: ["es2020"],
    });

    // 禁用诊断（减少干扰）
    monaco.languages.typescript.typescriptDefaults.setDiagnosticsOptions({
      noSemanticValidation: false,
      noSyntaxValidation: false,
      diagnosticCodesToIgnore: [
        1375, // 'await' expressions are only allowed at the top level
        2792, // Cannot find module
        2304, // Cannot find name (for some edge cases)
        8010, // Type annotations can only be used in TypeScript files
        8016, // Type assertion expressions can only be used in TypeScript files
      ],
    });

    monaco.languages.typescript.javascriptDefaults.setDiagnosticsOptions({
      noSemanticValidation: false,
      noSyntaxValidation: false,
      diagnosticCodesToIgnore: [2304],
    });

    // 添加自定义代码补全（只注册一次，避免多实例累积）
    if (!completionProvidersRegistered) {
    completionProvidersRegistered = true;
    globalDisposables.push(monaco.languages.registerCompletionItemProvider("typescript", {
      provideCompletionItems: (model: any, position: any) => {
        const word = model.getWordUntilPosition(position);
        const range = {
          startLineNumber: position.lineNumber,
          endLineNumber: position.lineNumber,
          startColumn: word.startColumn,
          endColumn: word.endColumn,
        };

        const suggestions: monaco.languages.CompletionItem[] = [
          {
            label: "plugin",
            kind: monaco.languages.CompletionItemKind.Variable,
            insertText: [
              "// Plugin metadata is managed by the form (Basic Info & Filters tabs)",
              "// This code only defines the event processing logic",
              "",
              "const plugin: Plugin = {",
              "  // Called for each event matching your filters",
              "  onEvent: (event, context) => {",
              "    // Access plugin configuration",
              "    // const config = context.config;",
              "    ",
              '    context.log("Processing: " + event.id);',
              "    ",
              "    // Your logic here",
              "    $0",
              "    ",
              "    return {",
              "      derivedEvents: [],  // New events to emit",
              "      tags: [],           // Tags to add",
              "      metadata: {}        // Additional metadata",
              "    };",
              "  }",
              "};",
            ].join("\n"),
            insertTextRules:
              monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
            documentation: "Complete plugin template (metadata in form)",
            range: range,
          },
          {
            label: "derivedEvent",
            kind: monaco.languages.CompletionItemKind.Snippet,
            insertText: [
              "{",
              '  source: "plugin",',
              '  category: "plugin",',
              '  type: "${1:custom_event}",',
              '  level: "${2|info,warn,error,fatal|}",',
              '  title: "${3:Event Title}",',
              "  data: {",
              "    $0",
              "  },",
              '  tags: ["${4:tag}"],',
              "  metadata: {}",
              "}",
            ].join("\n"),
            insertTextRules:
              monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
            documentation: "Create a derived event",
            range: range,
          },
        ];

        return { suggestions };
      },
    }));

    // JavaScript 也添加相同的代码补全
    globalDisposables.push(monaco.languages.registerCompletionItemProvider("javascript", {
      provideCompletionItems: (model: any, position: any) => {
        const word = model.getWordUntilPosition(position);
        const range = {
          startLineNumber: position.lineNumber,
          endLineNumber: position.lineNumber,
          startColumn: word.startColumn,
          endColumn: word.endColumn,
        };

        const suggestions: monaco.languages.CompletionItem[] = [
          {
            label: "plugin",
            kind: monaco.languages.CompletionItemKind.Variable,
            insertText: [
              "// Plugin metadata is managed by the form (Basic Info & Filters tabs)",
              "// This code only defines the event processing logic",
              "",
              "const plugin = {",
              "  // Called for each event matching your filters",
              "  onEvent: function(event, context) {",
              "    // Access plugin configuration",
              "    // const config = context.config;",
              "    ",
              '    context.log("Processing: " + event.id);',
              "    ",
              "    // Your logic here",
              "    $0",
              "    ",
              "    return {",
              "      derivedEvents: [],  // New events to emit",
              "      tags: [],           // Tags to add",
              "      metadata: {}        // Additional metadata",
              "    };",
              "  }",
              "};",
            ].join("\n"),
            insertTextRules:
              monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
            documentation: "Complete plugin template (metadata in form)",
            range: range,
          },
        ];

        return { suggestions };
      },
    }));
    } // end if (!completionProvidersRegistered)
  };

  return (
    <Editor
      height={height}
      language={language}
      value={value}
      onChange={(value) => onChange(value || "")}
      onMount={handleEditorDidMount}
      theme={isDark ? "vs-dark" : "vs-light"}
      options={{
        minimap: { enabled: false },
        fontSize: 13,
        lineNumbers: "on",
        roundedSelection: true,
        scrollBeyondLastLine: false,
        automaticLayout: true,
        tabSize: 2,
        insertSpaces: true,
        wordWrap: "on",
        formatOnPaste: true,
        formatOnType: true,
        suggestOnTriggerCharacters: true,
        quickSuggestions: {
          other: true,
          comments: false,
          strings: true,
        },
        suggest: {
          snippetsPreventQuickSuggestions: false,
        },
        acceptSuggestionOnCommitCharacter: true,
        acceptSuggestionOnEnter: "on",
        folding: true,
        foldingStrategy: "indentation",
        showFoldingControls: "always",
        bracketPairColorization: {
          enabled: true,
        },
      }}
    />
  );
};

export default MonacoPluginEditor;
