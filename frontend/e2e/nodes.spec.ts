import { expect, test, type Page } from '@playwright/test';
async function openNode(page: Page, context = 'production-shanghai') {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig' }).click();
  const cluster = page.locator('.cluster-row').filter({ hasText: context });
  await cluster.getByRole('button', { name: '连接', exact: true }).click();
  await cluster.getByRole('button', { name: '浏览资源' }).click();
  await page.getByRole('button', { name: '节点与空间', exact: true }).click();
  await expect(page.getByRole('cell', { name: '192.0.2.31', exact: false })).toBeVisible();
  await expect(page.getByRole('columnheader', { name: 'CPU / 内存用量' })).toBeVisible();
  await page.getByRole('button', { name: 'worker-01', exact: true }).click();
}
async function allowDrain(page: Page, timeout = 600) {
  await page.getByRole('button', { name: '排空节点', exact: true }).click();
  await page.getByRole('checkbox', { name: '保留 DaemonSet Pod 并继续排空' }).check();
  await page.getByRole('checkbox', { name: '允许驱逐无控制器引用的 Pod（可能不会重建）' }).check();
  await page
    .getByRole('checkbox', { name: '接受 emptyDir / hostPath 本地数据丢失或不可迁移的影响' })
    .check();
  await page.getByRole('spinbutton', { name: '排空超时秒数' }).fill(String(timeout));
  await page.getByRole('button', { name: '检查排空影响范围', exact: true }).click();
  await page.getByRole('textbox', { name: '确认排空节点名称' }).fill('worker-01');
  await page.getByRole('button', { name: '确认封锁并排空', exact: true }).click();
}
test('inspects nodes, filters assigned Pods and reviews cordon and uncordon', async ({ page }) => {
  await openNode(page);
  await expect(page.getByRole('cell', { name: '7800m', exact: true })).toBeVisible();
  await expect(page.getByRole('cell', { name: '32 GiB', exact: true })).toBeVisible();
  await expect(page.getByRole('cell', { name: '30 GiB', exact: true })).toBeVisible();
  await page.screenshot({
    path: 'test-results/node-details.png',
    fullPage: true,
    animations: 'disabled',
  });
  await page.getByRole('tab', { name: '节点上的 Pod', exact: true }).click();
  await expect(page.getByRole('button', { name: 'node-agent', exact: true })).toBeVisible();
  await page.locator('.node-pods').getByRole('combobox', { name: '命名空间', exact: true }).click();
  await page.locator('.ant-select-item-option').filter({ hasText: 'platform' }).click();
  await expect(page.getByRole('button', { name: 'node-agent', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'api-gateway-6f8d7c9b4-x2k5m', exact: true }).click();
  await expect(page.getByRole('tab', { name: '日志与终端' })).toBeVisible();
  await page.getByRole('dialog').last().getByRole('button', { name: '关闭', exact: true }).click();
  for (const name of ['封锁调度', '解封调度']) {
    await page.getByRole('button', { name, exact: true }).click();
    await page.getByRole('button', { name: '服务端预检并查看差异', exact: true }).click();
    await expect(page.locator('.monaco-diff-editor')).toContainText('unschedulable');
    await expect(page.locator('.monaco-diff-editor')).toContainText('kind: Node');
    await expect(page.locator('.monaco-diff-editor')).not.toContainText('kind: Pod');
    await expect(page.getByRole('button', { name: '确认提交到集群', exact: true })).toBeVisible();
    await page.getByRole('button', { name: '确认提交到集群', exact: true }).click();
    await page.getByRole('button', { name: '返回节点详情', exact: true }).click();
  }
  await expect(page.getByRole('button', { name: '封锁调度', exact: true })).toBeVisible();
});
test('requires explicit drain policies and node-name confirmation', async ({ page }) => {
  await openNode(page);
  await page.getByRole('button', { name: '排空节点', exact: true }).click();
  await page.getByRole('button', { name: '检查排空影响范围', exact: true }).click();
  await expect(
    page.getByText('存在阻塞项，请返回调整策略或处理权限问题后重新预检', { exact: true }),
  ).toBeVisible();
  await page.getByRole('textbox', { name: '确认排空节点名称' }).fill('worker-01');
  await expect(page.getByRole('button', { name: '确认封锁并排空', exact: true })).toBeDisabled();
  await expect(page.locator('.monaco-diff-editor')).toBeVisible();
  await page
    .locator('.ant-drawer-body')
    .last()
    .evaluate((element) => {
      element.scrollTop = 0;
    });
  await page.screenshot({
    path: 'test-results/node-drain-preview.png',
    fullPage: true,
    animations: 'disabled',
  });
});
test('shows per-Pod drain results and keeps the node cordoned', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await openNode(page);
  await allowDrain(page);
  await expect(page.getByRole('heading', { name: '排空任务结果', exact: true })).toBeVisible({
    timeout: 15000,
  });
  await expect(
    page.getByText('已确认的可驱逐 Pod 均已离开；节点保持封锁。', { exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: 'test-results/node-drain-result.png',
    fullPage: true,
    animations: 'disabled',
  });
  await page.getByRole('button', { name: '返回节点详情', exact: true }).click();
  await expect(page.getByRole('button', { name: '解封调度', exact: true })).toBeVisible();
  expect(errors).toEqual([]);
});
test('cancels PDB waiting and reports a timed-out task without automatic uncordon', async ({
  page,
}) => {
  await openNode(page);
  await allowDrain(page);
  await page.getByRole('button', { name: '停止后续操作', exact: true }).click();
  await expect(
    page.getByText('已停止后续操作；已接受的驱逐不会回滚。', { exact: true }),
  ).toBeVisible();
  await page.getByRole('button', { name: '返回节点详情', exact: true }).click();
  await allowDrain(page, 30);
  await expect(page.getByText('排空超时；节点不会自动解封。', { exact: true })).toBeVisible({
    timeout: 15000,
  });
});
test('keeps node details usable when Pod listing and drain are denied', async ({ page }) => {
  await openNode(page, 'staging-hangzhou');
  await page.getByRole('tab', { name: '节点上的 Pod', exact: true }).click();
  await expect(page.getByText('权限不足：无法跨命名空间列出 Pod', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '排空节点', exact: true }).click();
  await page.getByRole('button', { name: '检查排空影响范围', exact: true }).click();
  await expect(
    page.getByText('权限不足：排空需要完整节点 Pod 清单', { exact: true }),
  ).toBeVisible();
});
