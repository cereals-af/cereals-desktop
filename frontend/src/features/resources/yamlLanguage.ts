import * as monaco from 'monaco-editor/editor/editor.api';
import 'monaco-editor/languages/definitions/yaml/register';
import 'monaco-editor/features/suggest/register';
// In 0.57 the granular suggest entry registers inline providers, not the popup controller.
import 'monaco-editor/editor/contrib/suggest/browser/suggestController';
import 'monaco-editor/features/hover/register';
import { configureMonacoYaml } from 'monaco-yaml';
import type { JSONSchema, SchemasSettings } from 'monaco-yaml';
import EditorWorker from 'monaco-editor/editor/editor.worker?worker';
import YAMLWorker from './yaml.worker?worker';

// Both workers and every schema are local. YAML modelines cannot fetch remote schemas.
self.MonacoEnvironment = {
  getWorker: (_, label) => (label === 'yaml' ? new YAMLWorker() : new EditorWorker()),
};
// monaco-yaml's worker manager still uses the pre-0.56 createData signature.
// Adapt only its worker factory; model sync, idle disposal and providers stay upstream.
const yamlMonaco = {
  ...monaco,
  editor: {
    ...monaco.editor,
    createWebWorker<T extends object>(options: { createData?: unknown }) {
      const worker = new YAMLWorker();
      worker.postMessage(options.createData);
      return monaco.editor.createWebWorker<T>({ worker });
    },
  },
};
const language = configureMonacoYaml(yamlMonaco, {
  enableSchemaRequest: false,
  isKubernetes: true,
  format: { enable: false },
  disableDefaultProperties: true,
  hoverSchemaSource: false,
  schemas: [],
});
if (import.meta.hot) {
  import.meta.hot.dispose(() => {
    // The plugin's legacy IDisposable type does not resolve against Monaco 0.57.
    if (isDisposable(language)) language.dispose();
  });
}

function isDisposable(value: unknown): value is { dispose: () => void } {
  return (
    typeof value === 'object' &&
    value !== null &&
    'dispose' in value &&
    typeof value.dispose === 'function'
  );
}
const schemas = new Map<string, SchemasSettings>();
let updating = Promise.resolve<undefined>(undefined);

function update() {
  const options = [...schemas.values()];
  updating = updating.catch(() => undefined).then(() => language.update({ schemas: options }));
  return updating;
}

export function registerSchema(uri: string, schema: JSONSchema) {
  schemas.set(uri, { uri: `${uri}.schema.json`, fileMatch: [uri], schema });
  return {
    ready: update(),
    dispose: () => {
      schemas.delete(uri);
      return update();
    },
  };
}

export function parseEditorSchema(text: string): JSONSchema {
  const value: unknown = JSON.parse(text);
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new Error('Schema 格式无效');
  }
  return value;
}
