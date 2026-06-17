import { memo } from 'react';
import Editor from '@monaco-editor/react';
import { Spin } from 'antd';
import { useTheme } from '../ThemeContext';
import '../monacoSetup';

interface CodeBlockProps {
  /** Code string to display */
  code: string;
  /** Language for syntax highlighting (default: 'json') */
  language?: string;
  /** Height - auto-calculates based on content if not provided */
  height?: number | string;
  /** Font size (default: 12) */
  fontSize?: number;
}

/**
 * Read-only code block with syntax highlighting via Monaco.
 * Ideal for displaying config snippets, commands, etc.
 * Auto-calculates height from content lines if not specified.
 * 
 * Usage:
 *   <CodeBlock code={jsonConfig} language="json" />
 *   <CodeBlock code={shellCommand} language="shell" />
 */
const CodeBlock = memo(({
  code,
  language = 'json',
  height,
  fontSize = 12,
}: CodeBlockProps) => {
  const { isDark } = useTheme();

  // Auto-calculate height based on line count
  const lineCount = code.split('\n').length;
  const lineHeight = fontSize * 1.5;
  const autoHeight = Math.min(Math.max(lineCount * lineHeight + 24, 50), 500);
  const finalHeight = height || autoHeight;

  return (
    <div
      style={{
        border: `1px solid ${isDark ? '#333' : '#e8e8e8'}`,
        borderRadius: 6,
        overflow: 'hidden',
      }}
    >
      <Editor
        height={typeof finalHeight === 'number' ? finalHeight : finalHeight}
        language={language}
        theme={isDark ? 'vs-dark' : 'light'}
        value={code}
        loading={<Spin size="small" style={{ padding: 20 }} />}
        options={{
          readOnly: true,
          domReadOnly: true,
          fontSize,
          fontFamily: '"JetBrains Mono", "Fira Code", "SF Mono", Menlo, monospace',
          minimap: { enabled: false },
          lineNumbers: 'off',
          scrollBeyondLastLine: false,
          automaticLayout: true,
          tabSize: 2,
          wordWrap: 'on',
          folding: false,
          renderLineHighlight: 'none',
          overviewRulerBorder: false,
          hideCursorInOverviewRuler: true,
          scrollbar: {
            verticalScrollbarSize: 6,
            horizontalScrollbarSize: 6,
          },
          padding: { top: 8, bottom: 8 },
          contextmenu: false,
          selectionHighlight: false,
          occurrencesHighlight: 'off',
        }}
      />
    </div>
  );
});

CodeBlock.displayName = 'CodeBlock';

export default CodeBlock;
