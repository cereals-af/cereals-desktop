import { expect, test, type Page } from '@playwright/test';

async function importClusters(page: Page) {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig' }).click();
}
function clusterRow(page: Page, name: string) {
  return page.locator('.cluster-row').filter({ hasText: name });
}
async function connect(page: Page, name: string) {
  await clusterRow(page, name).getByRole('button', { name: '连接', exact: true }).click();
}

test('connected cluster names open capacity and all-namespace resource counts', async ({
  page,
}) => {
  await importClusters(page);
  const row = clusterRow(page, 'production-shanghai');
  await expect(row.getByRole('button', { name: 'production-shanghai', exact: true })).toHaveCount(
    0,
  );
  await connect(page, 'production-shanghai');
  await row.getByRole('button', { name: 'production-shanghai', exact: true }).click();
  const drawer = page.getByRole('dialog', { name: /集群详情/ });
  await expect(drawer.getByText('24 核', { exact: true })).toHaveCount(2);
  await expect(drawer.getByText('96 GiB', { exact: true })).toHaveCount(2);
  await expect(drawer.getByText('就绪 2 · 未就绪 1 · 不可调度 1')).toBeVisible();
  await expect(drawer.getByText(/不代表当前剩余或实时使用量/)).toBeVisible();
  for (const name of ['刷新详情', '浏览资源']) {
    const box = await drawer.getByRole('button', { name, exact: true }).boundingBox();
    expect(box?.height).toBe(36);
    expect(box?.width).toBeGreaterThanOrEqual(112);
  }
  await page.screenshot({
    path: 'test-results/cluster-overview.png',
    fullPage: true,
    animations: 'disabled',
  });
  await drawer.getByRole('textbox', { name: '搜索资源统计' }).fill('cronjob');
  await expect(drawer.getByRole('row').filter({ hasText: '定时任务' })).toContainText(
    '全部命名空间',
  );
  await drawer.getByRole('textbox', { name: '搜索资源统计' }).fill('任务 · Job');
  const zero = drawer.getByRole('row').filter({ hasText: '任务 · Job' });
  await expect(zero.getByRole('cell', { name: '0', exact: true })).toBeVisible();
  await expect(zero.getByText('已统计')).toBeVisible();
  await drawer.getByRole('button', { name: '刷新详情' }).click();
  await expect(drawer.getByRole('button', { name: '刷新详情' })).not.toHaveClass(/loading/);
  await drawer.getByRole('button', { name: '浏览资源', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(
    page.getByRole('button', { name: 'api-gateway-6f8d7c9b4-x2k5m', exact: true }),
  ).toBeVisible();
});

test('unavailable counts are unknown and inspecting another cluster keeps data isolated', async ({
  page,
}) => {
  await importClusters(page);
  await connect(page, 'production-shanghai');
  await connect(page, 'staging-hangzhou');
  await clusterRow(page, 'staging-hangzhou')
    .getByRole('button', { name: 'staging-hangzhou', exact: true })
    .click();
  const drawer = page.getByRole('dialog');
  await expect(drawer.getByText('无法统计节点与容量')).toBeVisible();
  await expect(drawer.getByText('24 核', { exact: true })).toHaveCount(0);
  await drawer.getByRole('textbox', { name: '搜索资源统计' }).fill('secret');
  const secret = drawer.getByRole('row').filter({ hasText: '保密字典' });
  await expect(secret.getByRole('cell', { name: '—', exact: true })).toBeVisible();
  await expect(secret.getByText('无法统计', { exact: true })).toBeVisible();
  await page.screenshot({
    path: 'test-results/cluster-overview-partial.png',
    fullPage: true,
    animations: 'disabled',
  });
  await drawer.getByRole('button', { name: '关闭', exact: true }).click();
  await clusterRow(page, 'production-shanghai')
    .getByRole('button', { name: 'production-shanghai', exact: true })
    .click();
  await expect(drawer.getByText('96 GiB', { exact: true })).toHaveCount(2);
  await drawer.getByRole('button', { name: '浏览资源', exact: true }).click();
  await expect(
    page.getByRole('button', { name: 'api-gateway-6f8d7c9b4-x2k5m', exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'staging-api-gateway-6f8d7c9b4-x2k5m', exact: true }),
  ).toHaveCount(0);
});

test('all cluster management categories omit discovery action and redundant cluster strip', async ({
  page,
}) => {
  await importClusters(page);
  await connect(page, 'production-shanghai');
  for (const label of [
    '资源浏览器',
    '工作负载',
    '服务与网络',
    '存储管理',
    '配置管理',
    '自定义资源',
    '访问控制',
    '节点与空间',
  ]) {
    await page
      .getByRole('navigation', { name: '主导航' })
      .getByRole('button', { name: label, exact: true })
      .click();
    await expect(page.getByRole('heading', { name: label, exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: '重新发现 API' })).toHaveCount(0);
    await expect(page.locator('.context-strip')).toHaveCount(0);
    await expect(
      page.locator('.topbar').getByText('production-shanghai', { exact: true }),
    ).toBeVisible();
  }
  await page.screenshot({
    path: 'test-results/resource-page-simplified.png',
    fullPage: true,
    animations: 'disabled',
  });
});
