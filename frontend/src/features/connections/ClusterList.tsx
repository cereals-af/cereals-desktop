import { Button, Empty, Tag } from 'antd';
import { ArrowRight, Box, FileKey2, Star } from 'lucide-react';
import type { Profile } from '../../api/types';

interface Props {
  profiles: Profile[];
  defaultClusterID: string;
  settingDefault: boolean;
  onDefault: (profile: Profile) => void;
  importing: boolean;
  connecting: boolean;
  onConnect: (profile: Profile) => void;
  onBrowse: (profile: Profile) => void;
  onDetails: (profile: Profile) => void;
}

export function ClusterList({
  profiles,
  defaultClusterID,
  settingDefault,
  onDefault,
  importing,
  connecting,
  onConnect,
  onBrowse,
  onDetails,
}: Props) {
  return (
    <section className="panel">
      <div className="panel-heading">
        <div>
          <h2>
            我的集群 <span className="count-badge">{profiles.length}</span>
          </h2>
          <p>启动时自动连接默认集群；只有一个集群时自动设为默认。</p>
        </div>
        <FileKey2 size={21} className="muted" />
      </div>
      {profiles.length === 0 ? (
        <div className="empty-connections">
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              <>
                <strong>还没有添加集群</strong>
                <p>选择配置文件，或读取本机默认配置。</p>
              </>
            }
          />
        </div>
      ) : (
        <div className="cluster-list">
          {profiles.map((profile) => (
            <div className="cluster-row" key={profile.id}>
              <div className="cluster-avatar">
                <Box size={24} />
              </div>
              <div className="cluster-row-main">
                {profile.connected ? (
                  <button className="cluster-name-link" onClick={() => onDetails(profile)}>
                    {profile.context}
                  </button>
                ) : (
                  <strong>{profile.context}</strong>
                )}
                <span>{profile.server}</span>
              </div>
              <Button
                type="text"
                className={
                  profile.id === defaultClusterID
                    ? 'default-cluster-button selected'
                    : 'default-cluster-button'
                }
                icon={
                  <Star
                    size={15}
                    fill={profile.id === defaultClusterID ? 'currentColor' : 'none'}
                  />
                }
                aria-label={
                  profile.id === defaultClusterID
                    ? `${profile.context} 默认集群`
                    : `将 ${profile.context} 设为默认集群`
                }
                disabled={
                  settingDefault || importing || connecting || profile.id === defaultClusterID
                }
                onClick={() => onDefault(profile)}
              >
                {profile.id === defaultClusterID ? '默认集群' : '设为默认'}
              </Button>
              <div className="cluster-row-meta">
                <span>{profile.namespace}</span>
                <small>{profile.version || '尚未连接'}</small>
              </div>
              <Tag color={profile.connected ? 'success' : 'default'}>
                {profile.connected ? '已连接' : '未连接'}
              </Tag>
              <Button
                disabled={connecting || importing}
                icon={<ArrowRight size={14} />}
                onClick={() => (profile.connected ? onBrowse(profile) : onConnect(profile))}
              >
                {profile.connected ? '浏览资源' : '连接'}
              </Button>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
