import { memo, useCallback, useRef, useState, useEffect } from 'react';
import Editor, { type OnMount, type OnChange } from '@monaco-editor/react';
import { Spin } from 'antd';
import { useTheme } from '../ThemeContext';
import '../monacoSetup';

interface JsonEditorProps {
  /** Current value as string */
  value?: string;
  /** Change callback - receives the raw string */
  onChange?: (value: string) => void;
  /** Height in px or CSS string (default: 200). Ignored when autoHeight is true. */
  height?: number | string;
  /** Whether the editor is read-only (default: false) */
  readOnly?: boolean;
  /** Placeholder text when empty */
  placeholder?: string;
  /** Language mode (default: 'json') - can also be 'plaintext' for non-JSON */
  language?: string;
  /** Font size (default: 12) */
  fontSize?: number;
  /** Whether to show minimap (default: false for compact use) */
  minimap?: boolean;
  /** Whether to show line numbers (default: false for compact use) */
  lineNumbers?: boolean;
  /** Whether to auto-format on mount (default: true) */
  autoFormat?: boolean;
  /** Auto-adjust height based on content (default: false) */
  autoHeight?: boolean;
  /** Minimum height in px when autoHeight is enabled (default: 120) */
  minHeight?: number;
  /** Maximum height in px when autoHeight is enabled (default: 360) */
  maxHeight?: number;
}

const LINE_HEIGHT = 19; // Monaco default line height for fontSize 12
const PADDING = 16; // top + bottom padding (8 + 8)

/**
 * Unified JSON/Code editor component wrapping Monaco Editor.
 * Auto-adapts to dark/light theme.
 * Best for single-instance editing scenarios (modals, forms).
 * 
 * Usage:
 *   <JsonEditor value={jsonStr} onChange={setJsonStr} height={200} />
 *   <JsonEditor value={code} readOnly language="plaintext" height={300} />
 *   <JsonEditor value={jsonStr} onChange={setJsonStr} autoHeight minHeight={120} maxHeight={400} />
 */
const JsonEditor = memo(({
  value = '',
  onChange,
  height = 200,
  readOnly = false,
  placeholder,
  language = 'json',
  fontSize = 12,
  minimap = false,
  lineNumbers = false,
  autoFormat = true,
  autoHeight = false,
  minHeight = 120,
  maxHeight = 360,
}: JsonEditorProps) => {
  const { isDark } = useTheme();
  const editorRef = useRef<Parameters<OnMount>[0] | null>(null);
  const [computedHeight, setComputedHeight] = useState<number>(
    autoHeight ? Math.max(minHeight, PADDING + LINE_HEIGHT * 3) : 0
  );

  // Compute height from content line count
  const updateHeight = useCallback(() => {
    if (!autoHeight || !editorRef.current) return;
    const model = editorRef.current.getModel();
    if (!model) return;
    const lineCount = model.getLineCount();
    const contentHeight = lineCount * LINE_HEIGHT + PADDING;
    setComputedHeight(Math.min(Math.max(contentHeight, minHeight), maxHeight));
  }, [autoHeight, minHeight, maxHeight]);

  // Re-compute when value prop changes (covers external updates)
  useEffect(() => {
    if (autoHeight) updateHeight();
  }, [value, autoHeight, updateHeight]);

  const handleMount: OnMount = useCallback((editor, monaco) => {
    editorRef.current = editor;

    // Configure JSON defaults if available
    try {
      const jsonLang = (monaco.languages as Record<string, unknown>).json as
        { jsonDefaults?: { setDiagnosticsOptions: (opts: Record<string, unknown>) => void } } | undefined;
      jsonLang?.jsonDefaults?.setDiagnosticsOptions({
        validate: true,
        allowComments: false,
        schemas: [],
        enableSchemaRequest: false,
      });
    } catch {
      // JSON language defaults may not be available
    }

    // Auto-format on mount if content exists
    if (autoFormat && value && language === 'json') {
      setTimeout(() => {
        editor.getAction('editor.action.formatDocument')?.run();
      }, 100);
    }

    // Listen for content changes to update height
    if (autoHeight) {
      updateHeight();
      editor.onDidChangeModelContent(() => updateHeight());
    }
  }, [autoFormat, value, language, autoHeight, updateHeight]);

  const handleChange: OnChange = useCallback((val) => {
    onChange?.(val || '');
  }, [onChange]);

  const effectiveHeight = autoHeight ? computedHeight : height;

  return (
    <div
      style={{
        border: `1px solid ${isDark ? '#333' : '#d9d9d9'}`,
        borderRadius: 6,
        overflow: 'hidden',
        position: 'relative',
        transition: autoHeight ? 'height 0.15s ease' : undefined,
      }}
    >
      <Editor
        height={effectiveHeight}
        language={language}
        theme={isDark ? 'vs-dark' : 'light'}
        value={value}
        onChange={handleChange}
        onMount={handleMount}
        loading={<Spin size="small" style={{ padding: 20 }} />}
        options={{
          readOnly,
          fontSize,
          fontFamily: '"JetBrains Mono", "Fira Code", "SF Mono", Menlo, monospace',
          minimap: { enabled: minimap },
          lineNumbers: lineNumbers ? 'on' : 'off',
          scrollBeyondLastLine: false,
          automaticLayout: true,
          tabSize: 2,
          wordWrap: 'on',
          folding: true,
          renderLineHighlight: readOnly ? 'none' : 'line',
          overviewRulerBorder: false,
          hideCursorInOverviewRuler: true,
          scrollbar: {
            verticalScrollbarSize: 8,
            horizontalScrollbarSize: 8,
          },
          padding: { top: 8, bottom: 8 },
          // Show placeholder-like behavior
          ...(placeholder && !value ? {
            // Monaco doesn't have native placeholder, but we can use emptySelectionClipboard
          } : {}),
        }}
      />
    </div>
  );
});

JsonEditor.displayName = 'JsonEditor';

export default JsonEditor;
