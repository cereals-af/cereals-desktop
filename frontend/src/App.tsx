import { useState } from 'react';
import { Alert, Button, Select, Spin, Tooltip } from 'antd';
import { ChevronRight, CircleHelp, Monitor, Plug, Unplug } from 'lucide-react';
import type { Profile } from './api/types';
import { Sidebar } from './components/Sidebar';
import { useDefaultConnection } from './features/connections/useDefaultConnection';
import { useWorkspace } from './features/connections/useWorkspace';
import { ConnectionsPage } from './features/connections/ConnectionsPage';
import { ConnectionDialog } from './features/connections/ConnectionDialog';
import { ResourceBrowser } from './features/resources/ResourceBrowser';
import { categoryLabels } from './features/resources/catalog';
import type { View } from './features/resources/catalog';
import { errorMessage } from './lib/errors';
import { version } from '../package.json';

export default function App() {
  const state = useWorkspace();
  const [view, setView] = useState<View>('connections');
  const [confirmProfile, setConfirmProfile] = useState<Profile | null>(null);
  const { selected } = state;
  const connectionBusy = state.connect.isPending || state.disconnect.isPending;
  const actualView = selected?.connected ? view : 'connections';
  const operationError =
    state.importConfig.error ??
    state.connect.error ??
    state.disconnect.error ??
    state.setDefault.error;
  const browse = (profile: Profile) => {
    state.setSelectedID(profile.id);
    setView('workloads');
  };
  const connect = (profile: Profile) => {
    state.connect.reset();
    if (profile.execCommand || profile.insecureTLS) setConfirmProfile(profile);
    else state.connect.mutate({ clusterID: profile.id, trustExec: false });
  };

  useDefaultConnection(state.workspace.data, (profile, needsConfirmation) => {
    state.setSelectedID(profile.id);
    if (needsConfirmation) setConfirmProfile(profile);
    else state.connect.mutate({ clusterID: profile.id, trustExec: Boolean(profile.execCommand) });
  });

  return (
    <div className="app-shell">
      <Sidebar view={actualView} onNavigate={setView} connected={Boolean(selected?.connected)} />
      <div className="app-body">
        <header className="topbar">
          <div className="breadcrumb">
            <Monitor size={16} />
            <span>工作空间</span>
            <ChevronRight size={13} />
            <strong>{categoryLabels[actualView]}</strong>
          </div>
          <div className="topbar-actions">
            <Select
              className="cluster-switcher"
              aria-label="切换集群"
              placeholder="选择 Context"
              value={selected?.id}
              disabled={connectionBusy || state.importConfig.isPending}
              options={state.profiles.map((profile) => ({
                value: profile.id,
                title: profile.context,
                label: (
                  <span className="cluster-option">
                    <span
                      role="img"
                      aria-label={profile.connected ? '已连接' : '未连接'}
                      className={`cluster-status-dot${profile.connected ? ' connected' : ''}`}
                    />
                    <span className="cluster-option-name">{profile.context}</span>
                  </span>
                ),
              }))}
              onChange={(id: string) => state.setSelectedID(id)}
            />
            {selected && (
              <Tooltip title={selected.connected ? '断开当前集群' : '连接当前集群'}>
                <Button
                  type="text"
                  aria-label={selected.connected ? '断开集群' : '连接集群'}
                  icon={selected.connected ? <Unplug size={17} /> : <Plug size={17} />}
                  loading={connectionBusy}
                  disabled={state.importConfig.isPending}
                  onClick={() =>
                    selected.connected ? state.disconnect.mutate(selected.id) : connect(selected)
                  }
                />
              </Tooltip>
            )}
            <span className="topbar-divider" />
            <Tooltip title="导入 kubeconfig 后连接 Context；资源写入须先预检并确认差异，可按需开启实时更新。">
              <CircleHelp size={18} className="muted" />
            </Tooltip>
            <span className="user-avatar">OP</span>
          </div>
        </header>
        <main className="main-content">
          {state.workspace.isError && (
            <Alert
              showIcon
              type="error"
              title="无法读取桌面服务"
              description="请通过 Wails 启动桌面客户端。浏览器预览不能访问本地配置。"
            />
          )}
          {(state.workspace.data?.warnings ?? []).map((warning) => (
            <Alert key={warning} showIcon type="warning" title={warning} />
          ))}
          {operationError && (
            <Alert
              showIcon
              type="error"
              title={errorMessage(operationError)}
              closable
              onClose={() => {
                state.importConfig.reset();
                state.connect.reset();
                state.disconnect.reset();
                state.setDefault.reset();
              }}
            />
          )}
          {state.connect.isPending && (
            <Alert
              type="info"
              showIcon
              icon={<Spin size="small" />}
              title="正在验证 API Server 与集群身份…"
              action={
                <Button size="small" onClick={state.cancelConnection}>
                  取消连接
                </Button>
              }
            />
          )}
          {actualView === 'connections' ? (
            <ConnectionsPage
              profiles={state.profiles}
              defaultClusterID={state.workspace.data?.defaultClusterID ?? ''}
              settingDefault={state.setDefault.isPending}
              onDefault={(profile) => state.setDefault.mutate(profile.id)}
              importing={state.importConfig.isPending}
              connecting={state.connect.isPending}
              onImport={(source) => {
                state.importConfig.reset();
                state.importConfig.mutate(source);
              }}
              onConnect={connect}
              onBrowse={browse}
            />
          ) : (
            selected && (
              <ResourceBrowser
                key={`${selected.id}:${actualView}`}
                profile={selected}
                category={actualView}
              />
            )
          )}
        </main>
        <footer className="app-footer">
          <span>
            <span className={`tiny-dot ${selected?.connected ? '' : 'gray'}`} />
            {selected?.connected ? `已连接 ${selected.context}` : '等待连接集群'}
          </span>
          <span>
            KUBERNETES DESKTOP CLIENT <span className="footer-separator">/</span> v{version}
          </span>
        </footer>
      </div>
      {confirmProfile && (
        <ConnectionDialog
          key={confirmProfile.id}
          profile={confirmProfile}
          isDefault={state.workspace.data?.defaultClusterID === confirmProfile.id}
          onClose={() => setConfirmProfile(null)}
          onConfirm={(trust) => {
            state.connect.mutate({ clusterID: confirmProfile.id, trustExec: trust });
            setConfirmProfile(null);
          }}
        />
      )}
    </div>
  );
}
