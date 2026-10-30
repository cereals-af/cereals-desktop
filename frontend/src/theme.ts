import type { ThemeConfig } from 'antd';

export const theme: ThemeConfig = {
  token: {
    colorPrimary: '#159c7a',
    colorInfo: '#159c7a',
    colorSuccess: '#159c7a',
    colorWarning: '#bc8524',
    colorError: '#d65555',
    colorText: '#263445',
    colorTextSecondary: '#78858e',
    colorBorder: '#dde5e7',
    colorBgLayout: '#f4f7f7',
    borderRadius: 6,
    controlHeight: 36,
    fontSize: 13,
    fontFamily:
      '-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif',
  },
  components: {
    Button: { fontWeight: 500, primaryShadow: 'none' },
    Table: { headerBg: '#f7f9fa', headerColor: '#687985', cellPaddingBlock: 15 },
    Select: { optionSelectedBg: '#e9f6f1' },
    Modal: { titleFontSize: 18 },
  },
};
