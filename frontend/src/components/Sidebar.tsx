import {
  Box,
  Boxes,
  Cable,
  Database,
  FileSliders,
  Grid2X2,
  Layers3,
  Network,
  ShieldCheck,
  SquareStack,
} from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import type { View } from '../features/resources/catalog';
import { categoryLabels } from '../features/resources/catalog';
import { version } from '../../package.json';
import { UpdateButton } from './UpdateButton';

const navigation: { view: View; icon: LucideIcon }[] = [
  { view: 'all', icon: Grid2X2 },
  { view: 'workloads', icon: Boxes },
  { view: 'network', icon: Network },
  { view: 'storage', icon: Database },
  { view: 'configuration', icon: FileSliders },
  { view: 'custom', icon: SquareStack },
  { view: 'access', icon: ShieldCheck },
  { view: 'nodes', icon: Layers3 },
];

export function Sidebar({
  view,
  onNavigate,
  connected,
}: {
  view: View;
  onNavigate: (view: View) => void;
  connected: boolean;
}) {
  return (
    <aside className="sidebar">
      <div className="brand">
        <div className="brand-mark">
          <Box size={25} />
        </div>
        <div>
          <strong>
            cereals<span>.</span>
          </strong>
          <small>KUBERNETES WORKSPACE</small>
        </div>
      </div>
      <div className="workspace-label">
        <span className="workspace-avatar">C</span>
        <div>
          <strong>本地工作空间</strong>
          <span>LOCAL WORKSPACE</span>
        </div>
        <span className="tiny-dot" />
      </div>
      <nav aria-label="主导航">
        <div className="nav-section">工作空间</div>
        <button
          className={`nav-item ${view === 'connections' ? 'active' : ''}`}
          onClick={() => onNavigate('connections')}
        >
          <Cable size={18} />
          集群连接
        </button>
        <div className="nav-section resource-section">集群管理</div>
        {navigation.map(({ view: target, icon: Icon }) => (
          <button
            key={target}
            className={`nav-item ${view === target ? 'active' : ''}`}
            onClick={() => onNavigate(target)}
            disabled={!connected}
            title={!connected ? '请先连接集群' : undefined}
          >
            <Icon size={18} />
            {categoryLabels[target]}
          </button>
        ))}
      </nav>
      <UpdateButton />
      <div className="sidebar-footer">
        <ShieldCheck size={17} />
        <div>
          <strong>本地运行 · 直接连接</strong>
          <span>cereals {version}</span>
        </div>
      </div>
    </aside>
  );
}
