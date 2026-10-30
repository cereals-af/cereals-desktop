import { Button } from 'antd';
import { ArrowDownToLine, Box, FolderOpen } from 'lucide-react';

interface Props {
  importing: boolean;
  connecting: boolean;
  onImport: (source: 'file' | 'default') => void;
}

export function ConnectionHero({ importing, connecting, onImport }: Props) {
  return (
    <section className="connection-hero">
      <div className="hero-text">
        <span className="section-kicker">
          <span className="tiny-dot" /> 本地工作台
        </span>
        <h2>
          连接集群，
          <br />
          专注每一次运维。
        </h2>
        <p>
          统一浏览 Kubernetes 资源与自定义 API。
          <br />
          使用你已有的身份，在熟悉的工作流中操作。
        </p>
        <div className="hero-actions">
          <Button
            type="primary"
            icon={<FolderOpen size={16} />}
            loading={importing}
            disabled={connecting}
            onClick={() => onImport('file')}
          >
            选择配置文件
          </Button>
          <Button
            icon={<ArrowDownToLine size={15} />}
            loading={importing}
            disabled={connecting}
            onClick={() => onImport('default')}
          >
            读取 ~/.kube/config
          </Button>
        </div>
      </div>
      <div className="cluster-illustration" aria-hidden="true">
        <div className="orbit orbit-outer" />
        <div className="orbit orbit-inner" />
        <div className="cluster-core">
          <Box size={48} strokeWidth={1.4} />
        </div>
        <div className="satellite satellite-a">
          <Box size={23} />
        </div>
        <div className="satellite satellite-b">
          <Box size={23} />
        </div>
        <div className="satellite satellite-c">
          <Box size={23} />
        </div>
        <div className="cluster-caption">
          <span className="tiny-dot" /> YOUR INFRASTRUCTURE, CONNECTED
        </div>
      </div>
    </section>
  );
}
