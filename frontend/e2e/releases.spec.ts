import { expect, test, type Page } from '@playwright/test';

async function browseDeployments(page: Page, context = 'production-shanghai') {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig' }).click();
  const cluster = page.locator('.cluster-row').filter({ hasText: context });
  await cluster.getByRole('button', { name: '连接', exact: true }).click();
  await cluster.getByRole('button', { name: '浏览资源' }).click();
  await page.getByRole('combobox', { name: '资源类型' }).click();
  await page.locator('.ant-select-item-option').filter({ hasText: '部署 · Deployment' }).click();
}
async function confirm(page: Page) {
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await expect(page.getByText('服务端预检已通过，尚未写入集群')).toBeVisible();
  await page.getByRole('button', { name: '确认提交到集群' }).click();
}

test('previews historical templates and handles pause, resume and rollback', async ({ page }) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await browseDeployments(page);
  await page.getByRole('button', { name: 'api-gateway', exact: true }).click();
  await page.getByRole('tab', { name: '发布历史', exact: true }).click();
  await expect(page.getByText('api-gateway-previous', { exact: true })).toBeVisible();
  await page.screenshot({ path: 'test-results/release-history.png', fullPage: true });
  await page.getByRole('button', { name: '暂停发布', exact: true }).click();
  await confirm(page);
  await page.getByRole('button', { name: '查看发布状态', exact: true }).click();
  await expect(page.getByRole('button', { name: '重启发布', exact: true })).toBeDisabled();
  await page.getByRole('tab', { name: '发布历史', exact: true }).click();
  await expect(page.getByText('发布已暂停，请先恢复发布再回滚')).toBeVisible();
  await expect(page.getByRole('button', { name: '回滚到此版本' })).toBeDisabled();
  await page.getByRole('button', { name: '恢复发布', exact: true }).click();
  await confirm(page);
  await page.getByRole('button', { name: '查看发布状态', exact: true }).click();
  await page.getByRole('tab', { name: '发布历史', exact: true }).click();
  await page.getByRole('button', { name: '回滚到此版本' }).click();
  await expect(page.getByText('准备回滚到修订 3')).toBeVisible();
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await expect(page.getByText('服务端预检已通过，尚未写入集群')).toBeVisible();
  await expect(page.locator('.monaco-diff-editor')).toBeVisible();
  await page.screenshot({ path: 'test-results/rollback-preview.png', fullPage: true });
  await page.getByRole('button', { name: '返回修改', exact: true }).click();
  await expect(page.getByText('准备回滚到修订 3')).toBeVisible();
  await confirm(page);
  await page.getByRole('button', { name: '查看发布状态', exact: true }).click();
  await expect(page.getByText('nginx:1.28', { exact: true })).toBeVisible();
  await expect(page.locator('.replica-grid > div').filter({ hasText: '期望副本' })).toContainText(
    '3',
  );
  expect(errors).toEqual([]);
});

test('opens scale directly despite denied parent list and namespace discovery', async ({
  page,
}) => {
  await browseDeployments(page, 'staging-hangzhou');
  await page.getByRole('textbox', { name: '命名空间', exact: true }).fill('forbidden');
  await page.getByRole('button', { name: '应用筛选' }).click();
  await expect(page.getByText('权限不足：当前身份无权执行此操作')).toBeVisible();
  await page.getByRole('button', { name: '按名称扩缩容', exact: true }).click();
  const drawer = page.getByRole('dialog');
  await expect(drawer.getByRole('textbox', { name: '命名空间', exact: true })).toHaveValue(
    'forbidden',
  );
  await drawer.getByRole('textbox', { name: '扩缩容工作负载名称' }).fill('restricted-scale');
  await drawer.getByRole('button', { name: '读取 Scale' }).click();
  await expect(drawer.getByText('get · 允许', { exact: true })).toBeVisible();
  await drawer.getByRole('spinbutton', { name: '期望副本数' }).fill('0');
  await page.screenshot({ path: 'test-results/scale-only.png', fullPage: true });
  await confirm(page);
  await drawer.getByRole('button', { name: '重新读取 Scale', exact: true }).click();
  await expect(drawer.getByRole('spinbutton', { name: '期望副本数' })).toHaveValue('0');
  await drawer.getByRole('combobox', { name: 'Scale 提交方式' }).click();
  await page
    .locator('.ant-select-item-option')
    .filter({ hasText: '补丁 · 需要 patch scale' })
    .click();
  await drawer.getByRole('spinbutton', { name: '期望副本数' }).fill('2');
  await confirm(page);
  await drawer.getByRole('button', { name: '重新读取 Scale', exact: true }).click();
  await expect(drawer.getByRole('spinbutton', { name: '期望副本数' })).toHaveValue('2');
});
