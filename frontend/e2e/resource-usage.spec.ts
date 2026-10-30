import { expect, test, type Page } from '@playwright/test';

async function openResources(page: Page, context = 'production-shanghai') {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig' }).click();
  const cluster = page.locator('.cluster-row').filter({ hasText: context });
  await cluster.getByRole('button', { name: '连接', exact: true }).click();
  await cluster.getByRole('button', { name: '浏览资源' }).click();
}

test('shows single-line usage without increasing row height and remembers column choices', async ({
  page,
}) => {
  await openResources(page);
  const cell = page.locator('.resource-usage').first();
  await expect(cell).toHaveText('250m·256 MiB');
  await expect(cell).toHaveCSS('white-space', 'nowrap');
  const row = page.locator('tbody tr').first();
  const height = (await row.boundingBox())?.height;
  await cell.focus();
  await expect(page.getByRole('tooltip')).toContainText('采样于');
  await page.getByRole('button', { name: '自定义显示列' }).click();
  await page.getByRole('checkbox', { name: 'CPU / 内存用量', exact: true }).uncheck();
  await expect(page.getByRole('columnheader', { name: 'CPU / 内存用量' })).toHaveCount(0);
  expect((await row.boundingBox())?.height).toBe(height);
  await page.getByRole('checkbox', { name: '创建时间', exact: true }).uncheck();
  await page.reload();
  const cluster = page.locator('.cluster-row').filter({ hasText: 'production-shanghai' });
  await cluster.getByRole('button', { name: '连接', exact: true }).click();
  await cluster.getByRole('button', { name: '浏览资源' }).click();
  await expect(page.getByRole('columnheader', { name: '创建时间' })).toHaveCount(0);
  await expect(page.getByRole('columnheader', { name: 'CPU / 内存用量' })).toHaveCount(0);
  await page.getByRole('button', { name: '自定义显示列' }).click();
  await page.getByRole('checkbox', { name: 'CPU / 内存用量', exact: true }).check();
  await page.getByRole('checkbox', { name: '创建时间', exact: true }).check();
  await page.getByRole('button', { name: '自定义显示列' }).click();
  await expect(page.getByRole('checkbox', { name: 'CPU / 内存用量', exact: true })).toBeHidden();
  await expect(page.locator('.resource-usage').first()).toHaveText('250m·256 MiB');
  await page.screenshot({ path: 'test-results/resource-usage-pods.png', fullPage: true });
  await page.getByRole('button', { name: '节点与空间', exact: true }).click();
  await expect(page.locator('.resource-usage').first()).toHaveText('2.35 核·8.6 GiB');
  await expect(page.getByRole('columnheader', { name: 'CPU / 内存容量' })).toHaveCount(0);
  await expect(page.locator('.resource-panel .ant-spin-blur')).toHaveCount(0);
  await page.screenshot({ path: 'test-results/resource-usage-nodes.png', fullPage: true });
});

for (const [mode, message] of [
  ['unavailable', 'Metrics Server 未安装或暂不可用'],
  ['missing', '暂无采样'],
  ['stale', '采样已过期'],
]) {
  test(`keeps resource browsing usable when metrics are ${mode}`, async ({ page }) => {
    await page.addInitScript(
      (value) => sessionStorage.setItem('fixture-metrics', value ?? ''),
      mode,
    );
    await openResources(page);
    const cell = page.locator('.resource-usage').first();
    await expect(cell).toHaveText('—');
    await cell.focus();
    await expect(page.getByRole('tooltip')).toContainText(message ?? '');
    await expect(
      page.getByRole('button', { name: 'api-gateway-6f8d7c9b4-x2k5m', exact: true }),
    ).toBeVisible();
  });
}

test('isolates metrics by cluster and keeps permission failures separate from zero', async ({
  page,
}) => {
  await openResources(page, 'staging-hangzhou');
  const cell = page.locator('.resource-usage').first();
  await expect(cell).toHaveText('—');
  await cell.focus();
  await expect(page.getByRole('tooltip')).toContainText('当前身份无权读取资源用量');
});
