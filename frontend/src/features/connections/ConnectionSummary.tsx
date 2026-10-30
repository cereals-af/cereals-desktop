import { Cable, Check, ShieldCheck } from 'lucide-react';
import type { Profile } from '../../api/types';

export function ConnectionSummary({ profiles }: { profiles: Profile[] }) {
  const connected = profiles.filter((profile) => profile.connected).length;
  return (
    <div className="summary-grid">
      <div className="summary-card">
        <div className="summary-icon">
          <Cable size={21} />
        </div>
        <div>
          <span>已导入 Context</span>
          <strong>
            {profiles.length}
            <small>个</small>
          </strong>
        </div>
      </div>
      <div className="summary-card">
        <div className="summary-icon green">
          <Check size={21} />
        </div>
        <div>
          <span>已建立连接</span>
          <strong>
            {connected}
            <small>个</small>
          </strong>
        </div>
      </div>
      <div className="summary-card">
        <div className="summary-icon">
          <ShieldCheck size={21} />
        </div>
        <div>
          <span>访问方式</span>
          <strong className="summary-text">
            本地直连<small>凭据保留在本机</small>
          </strong>
        </div>
      </div>
    </div>
  );
}
