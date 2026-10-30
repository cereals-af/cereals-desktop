import { expect, test, type Page } from '@playwright/test';

const row = (page: Page, name: string) => page.locator('.cluster-row').filter({ hasText: name });

test('remembers the selected default, connects at launch and respects manual disconnect', async ({
  page,
}) => {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig' }).click();
  await row(page, 'staging-hangzhou')
    .getByRole('button', { name: '将 staging-hangzhou 设为默认集群' })
    .click();
  await expect(
    row(page, 'staging-hangzhou').getByRole('button', { name: 'staging-hangzhou 默认集群' }),
  ).toBeVisible();
  await page.reload();
  await expect(row(page, 'staging-hangzhou').getByText('已连接', { exact: true })).toBeVisible();
  await expect(row(page, 'production-shanghai').getByText('未连接', { exact: true })).toBeVisible();
  await page.screenshot({
    path: 'test-results/default-cluster.png',
    fullPage: true,
    animations: 'disabled',
  });
  await page.getByRole('button', { name: '断开集群', exact: true }).click();
  await expect(row(page, 'staging-hangzhou').getByText('未连接', { exact: true })).toBeVisible();
  await row(page, 'production-shanghai')
    .getByRole('button', { name: '将 production-shanghai 设为默认集群' })
    .click();
  await expect(row(page, 'staging-hangzhou').getByText('未连接', { exact: true })).toBeVisible();
  await expect(row(page, 'production-shanghai').getByText('未连接', { exact: true })).toBeVisible();
});

test('a single imported cluster is the default and connects automatically', async ({ page }) => {
  await page.addInitScript(() => sessionStorage.setItem('fixture-clusters', 'test-production'));
  await page.goto('/');
  await expect(row(page, 'production-shanghai').getByText('已连接', { exact: true })).toBeVisible();
  await expect(
    row(page, 'production-shanghai').getByRole('button', { name: 'production-shanghai 默认集群' }),
  ).toBeVisible();
});

test('default authentication plugin requires initial consent and remembers it for startup', async ({
  page,
}) => {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig' }).click();
  await row(page, 'cloud-auth').getByRole('button', { name: '将 cloud-auth 设为默认集群' }).click();
  await page.reload();
  const dialog = page.getByRole('dialog', { name: '确认集群连接' });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole('button', { name: '连接集群' })).toBeDisabled();
  await dialog.getByRole('checkbox').check();
  await dialog.getByRole('button', { name: '连接集群' }).click();
  await expect(row(page, 'cloud-auth').getByText('已连接', { exact: true })).toBeVisible();
  await page.reload();
  await expect(row(page, 'cloud-auth').getByText('已连接', { exact: true })).toBeVisible();
  await expect(page.getByRole('dialog')).toHaveCount(0);
});
