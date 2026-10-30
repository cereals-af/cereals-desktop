import { expect, test, type Page } from '@playwright/test';
async function browse(page: Page, context = 'production-shanghai') {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig' }).click();
  const cluster = page.locator('.cluster-row').filter({ hasText: context });
  await cluster.getByRole('button', { name: '连接', exact: true }).click();
  await cluster.getByRole('button', { name: '浏览资源' }).click();
  await page.getByRole('button', { name: '自定义资源', exact: true }).click();
}
async function openInstances(page: Page) {
  await browse(page);
  await page.getByRole('button', { name: 'widgets.demo.example.com', exact: true }).click();
  await expect(page.getByText('Established', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '浏览 v1 实例', exact: true }).click();
}

test('browses CRD versions, server columns, namespace filters and pagination', async ({ page }) => {
  await browse(page);
  await page.getByRole('button', { name: 'widgets.demo.example.com', exact: true }).click();
  await expect(page.getByRole('button', { name: '浏览 v3 实例', exact: true })).toBeDisabled();
  await page.screenshot({
    path: 'test-results/crd-definition.png',
    fullPage: true,
    animations: 'disabled',
  });
  await page.getByRole('button', { name: '浏览 v1 实例', exact: true }).click();
  await expect(page.getByRole('cell', { name: '9007199254740993', exact: true })).toBeVisible();
  const instances = page.locator('.crd-instances');
  await expect(instances.getByRole('columnheader', { name: 'Advanced', exact: true })).toHaveCount(
    0,
  );
  await page.getByRole('checkbox', { name: '显示扩展列' }).check();
  await expect(
    instances.getByRole('columnheader', { name: 'Advanced', exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: 'test-results/crd-instances.png',
    fullPage: true,
    animations: 'disabled',
  });
  await page.getByRole('button', { name: '下一页实例', exact: true }).click();
  await expect(page.getByRole('cell', { name: 'second-widget', exact: true })).toBeVisible();
  await page.getByRole('button', { name: '上一页实例', exact: true }).click();
  await page.getByRole('combobox', { name: 'CRD API 版本' }).click();
  await page.locator('.ant-select-item-option').filter({ hasText: 'v2 · 已弃用' }).click();
  await expect(
    instances.getByRole('columnheader', { name: 'V2 Label', exact: true }),
  ).toBeVisible();
  await expect(page.getByText('测试版本，建议使用 v1', { exact: true })).toBeVisible();
  await instances.getByRole('combobox', { name: '命名空间', exact: true }).click();
  await page.locator('.ant-select-item-option').filter({ hasText: '全部命名空间' }).click();
  await expect(instances.getByRole('cell', { name: 'platform', exact: true })).toBeVisible();
  await page.getByRole('checkbox', { name: '使用通用列表' }).check();
  await expect(instances.getByRole('button', { name: '创建资源', exact: true })).toBeVisible();
});

test('edits typed fields, arrays and null while preserving unknown int64 through YAML', async ({
  page,
}) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await openInstances(page);
  await page.getByRole('button', { name: '表单编辑 sample-widget', exact: true }).click();
  await expect(page.getByRole('textbox', { name: '实例命名空间', exact: true })).toHaveValue(
    'platform',
  );
  await expect(page.getByRole('textbox', { name: 'spec.sequence', exact: true })).toHaveValue(
    '9007199254740993',
  );
  await page.getByRole('textbox', { name: 'spec.label', exact: true }).fill('changed');
  await page.getByRole('button', { name: '添加 spec.items', exact: true }).click();
  await page.getByRole('button', { name: '设置 spec.items[1].name', exact: true }).click();
  await page.getByRole('textbox', { name: 'spec.items[1].name', exact: true }).fill('second');
  await page.getByRole('button', { name: '设为 null spec.nullable', exact: true }).click();
  await page.getByRole('textbox', { name: '自定义资源名称', exact: true }).scrollIntoViewIfNeeded();
  await page.screenshot({
    path: 'test-results/crd-form.png',
    fullPage: true,
    animations: 'disabled',
  });
  await page.getByText('YAML 草稿', { exact: true }).click();
  await expect(page.locator('.view-lines')).toContainText('9223372036854775807');
  await expect(page.locator('.view-lines')).toContainText('9007199254740993');
  await page.getByText('可视化表单', { exact: true }).click();
  await expect(page.getByRole('textbox', { name: 'spec.label', exact: true })).toHaveValue(
    'changed',
  );
  await expect(page.getByRole('textbox', { name: 'spec.items[1].name', exact: true })).toHaveValue(
    'second',
  );
  await page
    .getByRole('textbox', { name: 'spec.sequence', exact: true })
    .fill('9223372036854775808');
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await expect(page.getByText('整数超出 int64 范围', { exact: true })).toBeVisible();
  await page.getByRole('textbox', { name: 'spec.sequence', exact: true }).fill('9007199254740993');
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await expect(page.getByText('服务端预检已通过，尚未写入集群')).toBeVisible();
  await page.getByRole('button', { name: '确认提交到集群' }).click();
  await page.getByRole('button', { name: '返回实例列表', exact: true }).click();
  await expect(
    page.locator('.crd-instances').getByRole('cell', { name: 'changed', exact: true }),
  ).toBeVisible();
  expect(errors).toEqual([]);
});

test('creates with required fields and preserves the draft on schema changes', async ({ page }) => {
  await openInstances(page);
  await page.getByRole('button', { name: '表单创建实例', exact: true }).click();
  await page.getByRole('textbox', { name: '自定义资源名称', exact: true }).fill('schema-changed');
  await page.getByRole('button', { name: '设置 spec', exact: true }).click();
  await page.getByRole('button', { name: '设置 spec.label', exact: true }).click();
  await page.getByRole('textbox', { name: 'spec.label', exact: true }).fill('new-value');
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await expect(
    page.getByText('CRD 定义已变化，请保留 YAML 草稿，刷新表单后重新检查', { exact: true }),
  ).toBeVisible();
  await expect(page.getByRole('textbox', { name: 'spec.label', exact: true })).toHaveValue(
    'new-value',
  );
  await page.getByRole('textbox', { name: '自定义资源名称', exact: true }).fill('created-widget');
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await page.getByRole('button', { name: '确认提交到集群' }).click();
  await expect(page.getByText('请求已提交', { exact: true })).toBeVisible();
});

test('reports CRD read denial without blocking the generic instance browser', async ({ page }) => {
  await browse(page, 'staging-hangzhou');
  await page.getByRole('combobox', { name: '资源类型', exact: true }).click();
  await page
    .locator('.ant-select-item-option')
    .filter({ hasText: 'Widget · demo.example.com' })
    .click();
  await page.getByRole('button', { name: 'CRD 定义与实例', exact: true }).click();
  await expect(page.getByText('权限不足：无法读取 CRD 定义', { exact: true })).toBeVisible();
  await page.getByRole('dialog').getByRole('button', { name: '关闭', exact: true }).click();
  await expect(
    page.getByRole('button', { name: 'staging-sample-widget', exact: true }),
  ).toBeVisible();
});
