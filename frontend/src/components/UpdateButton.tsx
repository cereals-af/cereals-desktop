import { useState } from 'react';
import { Alert } from 'antd';
import { RefreshCw } from 'lucide-react';
import { CheckForUpdates } from '../../bindings/github.com/cereals-af/cereals-desktop/internal/appupdate/service';
import { errorMessage } from '../lib/errors';

export function UpdateButton() {
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState('');

  async function check() {
    setChecking(true);
    setError('');
    try {
      await CheckForUpdates();
    } catch (error: unknown) {
      setError(errorMessage(error));
    } finally {
      setChecking(false);
    }
  }

  return (
    <>
      <button className="nav-item" disabled={checking} onClick={() => void check()}>
        <RefreshCw size={17} />
        {checking ? '正在检查更新…' : '检查软件更新'}
      </button>
      {error && <Alert type="error" title={error} showIcon />}
    </>
  );
}
