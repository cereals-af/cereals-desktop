import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ConfigProvider } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import App from './App';
import { APIContext } from './api/context';
import { desktopAPI } from './api/desktop';
import { theme } from './theme';
import './styles.css';

async function bootstrap() {
  const container = document.getElementById('root');
  if (!container) throw new Error('Application root is missing');
  // Fixtures are available only to the development-only Playwright harness.
  const api =
    import.meta.env.DEV && import.meta.env.VITE_E2E === 'true'
      ? (await import('./test/fixtureAPI')).fixtureAPI
      : desktopAPI;
  const client = new QueryClient({
    defaultOptions: { queries: { refetchOnWindowFocus: false, retry: false } },
  });
  createRoot(container).render(
    <StrictMode>
      <ConfigProvider theme={theme} locale={zhCN}>
        <QueryClientProvider client={client}>
          <APIContext.Provider value={api}>
            <App />
          </APIContext.Provider>
        </QueryClientProvider>
      </ConfigProvider>
    </StrictMode>,
  );
}

void bootstrap().catch((error: unknown) => {
  console.error('Application initialization failed', error);
});
