import { Button, Checkbox, Popover, Tooltip } from 'antd';
import { Settings2 } from 'lucide-react';
import type { useTableColumns } from './useTableColumns';

interface Option {
  key: string;
  label: string;
}

export function TableColumnSettings({
  options,
  settings,
}: {
  options: Option[];
  settings: ReturnType<typeof useTableColumns>;
}) {
  return (
    <Popover
      trigger="click"
      placement="bottomRight"
      title="显示列"
      content={
        <div className="table-column-options">
          {options.map((option) => (
            <Checkbox
              key={option.key}
              checked={settings.shown(option.key)}
              onChange={(event) => settings.toggle(option.key, event.target.checked)}
            >
              {option.label}
            </Checkbox>
          ))}
          {settings.unavailable && <small className="muted">偏好仅在本次页面有效</small>}
        </div>
      }
    >
      <Tooltip title="自定义显示列">
        <Button aria-label="自定义显示列" icon={<Settings2 size={15} />} />
      </Tooltip>
    </Popover>
  );
}
