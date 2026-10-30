import Editor, { loader } from '@monaco-editor/react';
import { useEffect, useRef, useState } from 'react';
import { Alert, Button } from 'antd';
import type { JSONSchema } from 'monaco-yaml';
import * as monaco from 'monaco-editor/editor/editor.api';
import '@monaco-codicons.css';
import { registerSchema } from './yamlLanguage';

// Bundle workers and language support locally; the desktop app must not depend on a CDN.
loader.config({ monaco });

export default function YAMLViewer({
  value,
  readOnly = true,
  onChange,
  schema,
}: {
  value: string;
  readOnly?: boolean;
  onChange?: (value: string) => void;
  schema?: JSONSchema;
}) {
  const [uri] = useState(() => `file:///cereals/${crypto.randomUUID()}.yaml`);
  const [markers, setMarkers] = useState<monaco.editor.IMarker[]>([]);
  const [languageError, setLanguageError] = useState('');
  const editor = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);
  useEffect(() => {
    if (!schema) return;
    let active = true;
    const registration = registerSchema(uri, schema);
    void registration.ready.catch(() => {
      if (active) setLanguageError('Schema 语言服务加载失败，仍可使用服务端预检。');
    });
    return () => {
      active = false;
      void registration.dispose().catch(() => undefined);
    };
  }, [schema, uri]);
  useEffect(() => {
    const subscription = monaco.editor.onDidChangeMarkers((resources) => {
      if (resources.some((resource) => resource.toString() === uri)) {
        setMarkers(monaco.editor.getModelMarkers({ resource: monaco.Uri.parse(uri) }).slice(0, 20));
      }
    });
    return () => subscription.dispose();
  }, [uri]);
  return (
    <>
      {languageError && <Alert type="warning" title={languageError} />}
      {schema && !readOnly && (
        <div className="yaml-editor-tools">
          <Button
            size="small"
            onClick={() => {
              editor.current?.focus();
              editor.current?.trigger('schema-toolbar', 'editor.action.triggerSuggest', {});
            }}
          >
            字段补全
          </Button>
        </div>
      )}
      <div className="yaml-viewer">
        <Editor
          path={uri}
          onMount={(instance) => {
            editor.current = instance;
          }}
          height="100%"
          language="yaml"
          value={value}
          onChange={(text) => onChange?.(text ?? '')}
          options={{
            readOnly,
            ariaLabel: readOnly ? 'YAML 只读预览' : 'YAML 资源编辑器',
            minimap: { enabled: false },
            fontSize: 13,
            scrollBeyondLastLine: false,
            wordWrap: 'on',
            padding: { top: 18 },
            automaticLayout: true,
            tabSize: 2,
            autoIndent: 'keep',
            formatOnPaste: false,
            formatOnType: false,
          }}
        />
      </div>
      {!readOnly && markers.length > 0 && (
        <div className="yaml-diagnostics" aria-label="YAML 编辑提示">
          {markers.map((marker, index) => (
            <Button
              key={`${marker.startLineNumber}:${marker.startColumn}:${index}`}
              type="text"
              size="small"
              onClick={() => {
                editor.current?.setPosition({
                  lineNumber: marker.startLineNumber,
                  column: marker.startColumn,
                });
                editor.current?.revealLineInCenter(marker.startLineNumber);
                editor.current?.focus();
              }}
            >
              第 {marker.startLineNumber} 行：{marker.message}
            </Button>
          ))}
        </div>
      )}
    </>
  );
}

export function YAMLDiff({ before, after }: { before: string; after: string }) {
  const container = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!container.current) return;
    const original = monaco.editor.createModel(before, 'yaml');
    const modified = monaco.editor.createModel(after, 'yaml');
    const editor = monaco.editor.createDiffEditor(container.current, {
      readOnly: true,
      originalEditable: false,
      minimap: { enabled: false },
      fontSize: 13,
      automaticLayout: true,
      renderSideBySide: true,
      scrollBeyondLastLine: false,
      ariaLabel: 'YAML 差异预览',
    });
    editor.setModel({ original, modified });
    return () => {
      // Monaco 0.57 requires detaching the diff models before disposing them.
      editor.setModel(null);
      editor.dispose();
      original.dispose();
      modified.dispose();
    };
  }, [before, after]);
  return <div className="yaml-viewer" ref={container} />;
}
