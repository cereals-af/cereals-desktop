import { expect, test } from '@playwright/test';

test('workload statuses use the agreed colors and remain readable as text', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig' }).click();
  const cluster = page.locator('.cluster-row').filter({ hasText: 'production-shanghai' });
  await cluster.getByRole('button', { name: '连接', exact: true }).click();
  await cluster.getByRole('button', { name: '浏览资源' }).click();
  for (const [status, color] of [
    ['Ready', 'rgb(21, 156, 122)'],
    ['Completed', 'rgb(91, 141, 184)'],
    ['Processing', 'rgb(188, 133, 36)'],
    ['Error', 'rgb(214, 85, 85)'],
  ]) {
    const badge = page
      .locator('.status-badge')
      .filter({ hasText: status ?? '' })
      .first();
    await expect(badge).toBeVisible();
    await expect(badge).toHaveCSS('color', color ?? '');
  }
  await page.screenshot({
    path: 'test-results/workload-status-colors.png',
    fullPage: true,
    animations: 'disabled',
  });
  await page.getByRole('combobox', { name: '资源类型', exact: true }).click();
  await page.locator('.ant-select-item-option').filter({ hasText: '部署 · Deployment' }).click();
  const stopped = page.locator('.status-badge').filter({ hasText: 'Stopped' });
  await expect(stopped).toBeVisible();
  await expect(stopped).toHaveCSS('color', 'rgb(138, 150, 163)');
});
