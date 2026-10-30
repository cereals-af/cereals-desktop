import { expect, test, type Page } from '@playwright/test';

async function openStorage(page: Page, context = 'production-shanghai') {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig' }).click();
  const cluster = page.locator('.cluster-row').filter({ hasText: context });
  await cluster.getByRole('button', { name: '连接', exact: true }).click();
  await cluster.getByRole('button', { name: '浏览资源' }).click();
  await page.getByRole('button', { name: '存储管理', exact: true }).click();
}
async function selectType(page: Page, label: string) {
  await page.getByRole('combobox', { name: '资源类型', exact: true }).click();
  await page.locator('.ant-select-item-option').filter({ hasText: label }).click();
}
async function selectClassMode(page: Page, label: string) {
  await page.getByRole('combobox', { name: '存储类策略', exact: true }).click();
  await page.locator('.ant-select-item-option').filter({ hasText: label }).click();
}
async function closeDetail(page: Page) {
  await page.getByRole('dialog').last().getByRole('button', { name: '关闭', exact: true }).click();
}

test('inspects PVC, PV and StorageClass and follows consumer and event links', async ({ page }) => {
  await openStorage(page);
  await page.getByRole('button', { name: 'app-data', exact: true }).click();
  await expect(page.getByText('申请与完成状态分别显示', { exact: true })).toBeVisible();
  await expect(page.getByText('example.csi.test', { exact: true })).toBeVisible();
  await page.screenshot({
    path: 'test-results/storage-claim.png',
    fullPage: true,
    animations: 'disabled',
  });
  await page.getByRole('tab', { name: '关联资源与事件' }).click();
  await expect(page.getByRole('button', { name: 'storage-consumer', exact: true })).toBeVisible();
  await expect(page.getByText('等待使用者参与调度', { exact: true })).toBeVisible();
  await closeDetail(page);
  await selectType(page, '持久卷 · PersistentVolume');
  await page.getByRole('button', { name: 'data-volume', exact: true }).click();
  await expect(page.getByText('Retain', { exact: true })).toBeVisible();
  await expect(page.getByText('platform / app-data', { exact: true })).toBeVisible();
  await page.screenshot({
    path: 'test-results/storage-volume.png',
    fullPage: true,
    animations: 'disabled',
  });
  await closeDetail(page);
  await selectType(page, '存储类型 · StorageClass');
  await page.getByRole('button', { name: '下一页', exact: true }).click();
  await page.getByRole('button', { name: 'fast-csi', exact: true }).click();
  await expect(page.getByText('等待首个使用者', { exact: true })).toBeVisible();
  await page.screenshot({
    path: 'test-results/storage-class.png',
    fullPage: true,
    animations: 'disabled',
  });
});

test('creates a PVC using a paginated StorageClass selector after reviewing the diff', async ({
  page,
}) => {
  await openStorage(page);
  await page.getByRole('button', { name: '创建 PVC', exact: true }).click();
  await page.getByRole('textbox', { name: 'PVC 名称', exact: true }).fill('new-data');
  await selectClassMode(page, '指定 StorageClass');
  await page.getByRole('combobox', { name: 'StorageClass 名称', exact: true }).click();
  await page.locator('.ant-select-item-option').filter({ hasText: 'fast-csi' }).click();
  await page.getByRole('textbox', { name: '申请容量', exact: true }).fill('20Gi');
  await page.screenshot({
    path: 'test-results/storage-create.png',
    fullPage: true,
    animations: 'disabled',
  });
  await page.getByRole('button', { name: '服务端预检并查看差异', exact: true }).click();
  await expect(page.locator('.monaco-diff-editor')).toContainText('fast-csi');
  await expect(page.locator('.monaco-diff-editor')).toContainText('20Gi');
  await page.getByRole('button', { name: '确认提交到集群', exact: true }).click();
  await page.getByRole('button', { name: '查看资源', exact: true }).click();
  await expect(page.getByText('PVC 尚未处于 Bound 状态', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: '申请扩容', exact: true })).toBeDisabled();
});

test('distinguishes omitted default StorageClass from an explicitly empty class', async ({
  page,
}) => {
  await openStorage(page);
  await page.getByRole('button', { name: '创建 PVC', exact: true }).click();
  await page.getByRole('textbox', { name: 'PVC 名称', exact: true }).fill('default-data');
  await page.getByRole('button', { name: '服务端预检并查看差异', exact: true }).click();
  await expect(page.locator('.monaco-diff-editor')).toContainText('PersistentVolumeClaim');
  await expect(page.locator('.monaco-diff-editor')).not.toContainText('storageClassName');
  await page.getByRole('button', { name: '返回修改', exact: true }).click();
  await selectClassMode(page, '显式无存储类');
  await page.getByRole('button', { name: '服务端预检并查看差异', exact: true }).click();
  await expect(page.locator('.monaco-diff-editor')).toContainText('storageClassName: ""');
});

test('preserves expansion drafts on validation/conflict and separates requested from reported capacity', async ({
  page,
}) => {
  await openStorage(page);
  await page.getByRole('button', { name: 'app-data', exact: true }).click();
  await page.getByRole('button', { name: '申请扩容', exact: true }).click();
  const capacity = page.getByRole('textbox', { name: '新的申请容量', exact: true });
  await capacity.fill('5Gi');
  await page.getByRole('button', { name: '服务端预检并查看差异', exact: true }).click();
  await expect(
    page.getByText('扩容容量必须大于当前申请容量；不支持缩容', { exact: true }),
  ).toBeVisible();
  await expect(capacity).toHaveValue('5Gi');
  await capacity.fill('99Gi');
  await page.getByRole('button', { name: '服务端预检并查看差异', exact: true }).click();
  await expect(
    page.getByText('PVC 已变化或被重建，请重新读取基线后预检', { exact: true }),
  ).toBeVisible();
  await expect(capacity).toHaveValue('99Gi');
  await page.getByRole('button', { name: '重新读取基线（保留草稿）', exact: true }).click();
  await expect(capacity).toBeEnabled();
  await capacity.fill('20Gi');
  await page.getByRole('button', { name: '服务端预检并查看差异', exact: true }).click();
  await expect(page.locator('.monaco-diff-editor')).toContainText('10Gi');
  await expect(page.locator('.monaco-diff-editor')).toContainText('20Gi');
  await page.screenshot({
    path: 'test-results/storage-expansion.png',
    fullPage: true,
    animations: 'disabled',
  });
  await page.getByRole('button', { name: '确认提交到集群', exact: true }).click();
  await page.getByRole('button', { name: '查看资源', exact: true }).click();
  await expect(page.getByText('FileSystemResizePending', { exact: true })).toBeVisible();
  await expect(page.getByText('20Gi', { exact: true })).toBeVisible();
  await expect(page.getByText('10Gi', { exact: true })).toBeVisible();
});

test('keeps PVC details and events usable when StorageClass and Pod permissions are denied', async ({
  page,
}) => {
  await openStorage(page, 'staging-hangzhou');
  await page.getByRole('button', { name: 'app-data', exact: true }).click();
  await expect(page.getByText('权限不足：不能读取 StorageClass', { exact: true })).toBeVisible();
  await expect(page.getByText('申请与完成状态分别显示', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '申请扩容', exact: true }).click();
  await page.getByRole('textbox', { name: '新的申请容量', exact: true }).fill('20Gi');
  await page.getByRole('button', { name: '服务端预检并查看差异', exact: true }).click();
  await expect(page.getByText('权限不足：不能读取 StorageClass', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '放弃草稿，返回详情', exact: true }).click();
  await page.getByRole('tab', { name: '关联资源与事件' }).click();
  await expect(
    page.getByText('引用此 PVC 的 Pod 读取失败：权限不足：无法列出 Pod', { exact: true }),
  ).toBeVisible();
  await expect(page.getByText('等待使用者参与调度', { exact: true })).toBeVisible();
});
