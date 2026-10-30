import { useState } from 'react';
import { Alert, Button } from 'antd';
import { Plus } from 'lucide-react';
import type { Profile } from '../../api/types';
import { ConnectionHero } from './ConnectionHero';
import { ConnectionSummary } from './ConnectionSummary';
import { ClusterList } from './ClusterList';
import { ClusterOverview } from './ClusterOverview';

interface Props {
  profiles: Profile[];
  defaultClusterID: string;
  settingDefault: boolean;
  onDefault: (profile: Profile) => void;
  importing: boolean;
  connecting: boolean;
  onImport: (source: 'file' | 'default') => void;
  onConnect: (profile: Profile) => void;
  onBrowse: (profile: Profile) => void;
}

export function ConnectionsPage({
  profiles,
  defaultClusterID,
  settingDefault,
  onDefault,
  importing,
  connecting,
  onImport,
  onConnect,
  onBrowse,
}: Props) {
  const [detailsID, setDetailsID] = useState('');
  const details = profiles.find((profile) => profile.id === detailsID && profile.connected);
  return (
    <>
      <div className="page-heading">
        <div>
          <div className="eyebrow">YOUR LOCAL KUBERNETES WORKSPACE</div>
          <h1>集群连接</h1>
          <p>从一份 kubeconfig 开始，把集群管理带到桌面。</p>
        </div>
        <Button
          type="primary"
          icon={<Plus size={16} />}
          loading={importing}
          disabled={connecting}
          onClick={() => onImport('file')}
        >
          导入 kubeconfig
        </Button>
      </div>
      <ConnectionHero importing={importing} connecting={connecting} onImport={onImport} />
      <ConnectionSummary profiles={profiles} />
      <ClusterList
        profiles={profiles}
        defaultClusterID={defaultClusterID}
        settingDefault={settingDefault}
        onDefault={onDefault}
        importing={importing}
        connecting={connecting}
        onConnect={onConnect}
        onBrowse={onBrowse}
        onDetails={(profile) => setDetailsID(profile.id)}
      />
      {details && (
        <ClusterOverview
          key={details.id}
          profile={details}
          onClose={() => setDetailsID('')}
          onBrowse={() => {
            setDetailsID('');
            onBrowse(details);
          }}
        />
      )}
      <Alert
        className="local-note"
        showIcon
        type="info"
        title="配置文件在本地读取，连接信息不上传。导入后先检查 Context，再建立集群连接。"
      />
    </>
  );
}
