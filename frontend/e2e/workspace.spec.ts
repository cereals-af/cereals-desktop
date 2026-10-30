import { expect, test, type Page } from '@playwright/test';

const pageErrors = new WeakMap<Page, string[]>();
test.beforeEach(({ page }) => {
  const errors: string[] = [];
  pageErrors.set(page, errors);
  page.on('pageerror', (error) => errors.push(error.message));
});
test.afterEach(({ page }) => {
  expect(pageErrors.get(page)).toEqual([]);
});

test('imports contexts, browses resources, and opens bundled YAML viewer', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByText('还没有添加集群')).toBeVisible();
  await page.screenshot({ path: 'test-results/connections-empty.png', fullPage: true });
  await page.getByRole('button', { name: '导入 kubeconfig' }).click();
  const row = page.locator('.cluster-row').filter({ hasText: 'production-shanghai' });
  const topbar = page.locator('.topbar');
  await expect(topbar.getByRole('img', { name: '未连接', exact: true })).toBeVisible();
  await page.screenshot({ path: 'test-results/cluster-disconnected.png', fullPage: true });
  await topbar.getByRole('button', { name: '连接集群', exact: true }).click();
  await expect(topbar.getByRole('img', { name: '已连接', exact: true })).toBeVisible();
  await expect(topbar.getByRole('button', { name: '断开集群', exact: true })).toBeVisible();
  await page.getByRole('combobox', { name: '切换集群' }).click();
  await expect(
    page.locator('.ant-select-item-option').filter({ hasText: 'staging-hangzhou' }),
  ).toBeVisible();
  await page.screenshot({
    path: 'test-results/cluster-options.png',
    fullPage: true,
    animations: 'disabled',
  });
  await page.getByRole('combobox', { name: '切换集群' }).press('Escape');
  await row.getByRole('button', { name: '浏览资源' }).click();
  await expect(
    page.getByRole('button', { name: 'api-gateway-6f8d7c9b4-x2k5m', exact: true }),
  ).toBeVisible();
  await page.getByRole('textbox', { name: '搜索本页资源' }).fill('gateway');
  await expect(page.locator('.ant-table-tbody tr.ant-table-row')).toHaveCount(1);
  await page.getByRole('textbox', { name: '搜索本页资源' }).fill('');
  await page.screenshot({ path: 'test-results/resources.png', fullPage: true });
  await page.getByRole('button', { name: 'api-gateway-6f8d7c9b4-x2k5m', exact: true }).click();
  await expect(page.locator('.monaco-editor')).toBeVisible({ timeout: 15000 });
  await expect(page.locator('.view-lines')).toContainText('apiVersion');
});

test('separates clusters and displays namespace authorization failures', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig' }).click();
  await page
    .locator('.cluster-row')
    .filter({ hasText: 'production-shanghai' })
    .getByRole('button', { name: '连接', exact: true })
    .click();
  const staging = page.locator('.cluster-row').filter({ hasText: 'staging-hangzhou' });
  await staging.getByRole('button', { name: '连接', exact: true }).click();
  await staging.getByRole('button', { name: '浏览资源' }).click();
  await expect(
    page.getByRole('button', { name: 'staging-api-gateway-6f8d7c9b4-x2k5m', exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'api-gateway-6f8d7c9b4-x2k5m', exact: true }),
  ).toHaveCount(0);
  await expect(page.getByText('无法读取 Namespace 列表，请手动输入或重试')).toBeVisible();
  await page.getByRole('textbox', { name: '命名空间', exact: true }).fill('forbidden');
  await page.getByRole('button', { name: '应用筛选' }).click();
  await expect(page.getByText('权限不足：当前身份无权执行此操作')).toBeVisible();
  await page.getByRole('button', { name: '断开集群', exact: true }).click();
  await expect(page.getByRole('heading', { name: '集群连接', exact: true })).toBeVisible();
  await page.locator('.topbar').getByRole('button', { name: '连接集群', exact: true }).click();
  await expect(
    page.getByRole('button', { name: 'staging-api-gateway-6f8d7c9b4-x2k5m', exact: true }),
  ).toBeVisible();
});

test('requires explicit trust before running an authentication plugin', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig' }).click();
  await page.getByRole('combobox', { name: '切换集群' }).click();
  await page.locator('.ant-select-item-option').filter({ hasText: 'cloud-auth' }).click();
  await page.locator('.topbar').getByRole('button', { name: '连接集群', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByRole('button', { name: '连接集群' })).toBeDisabled();
  await expect(dialog.getByText('test-auth-plugin', { exact: true })).toBeVisible();
  await dialog.getByRole('checkbox').check();
  await dialog.getByRole('button', { name: '连接集群' }).click();
  await expect(
    page.locator('.cluster-row').filter({ hasText: 'cloud-auth' }).getByText('已连接'),
  ).toBeVisible();
});

const podName = 'api-gateway-6f8d7c9b4-x2k5m';
const podYAML = (name: string, image: string) =>
  `apiVersion: v1\nkind: Pod\nmetadata:\n  name: ${name}\n  namespace: platform\nspec:\n  containers:\n    - name: app\n      image: ${image}\n`;

async function browse(page: Page) {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig' }).click();
  const row = page.locator('.cluster-row').filter({ hasText: 'production-shanghai' });
  await row.getByRole('button', { name: '连接', exact: true }).click();
  await row.getByRole('button', { name: '浏览资源' }).click();
  await expect(page.getByRole('button', { name: podName, exact: true })).toBeVisible();
}

async function replaceYAML(page: Page, text: string) {
  const editor = page.getByRole('textbox', { name: 'YAML 资源编辑器', exact: true });
  await editor.focus();
  await editor.press('ControlOrMeta+A');
  // Use Monaco's paste path; typing a multiline string re-indents every newline.
  await editor.evaluate((element, value) => {
    const clipboard = new DataTransfer();
    clipboard.setData('text/plain', value);
    element.dispatchEvent(
      new ClipboardEvent('paste', { clipboardData: clipboard, bubbles: true, cancelable: true }),
    );
  }, text);
}

test('creates only after reviewing the server dry run', async ({ page }) => {
  await browse(page);
  await page.getByRole('button', { name: '创建资源', exact: true }).click();
  await replaceYAML(page, podYAML('created-by-ui', 'nginx:1.29'));
  await expect(page.getByRole('button', { name: '确认提交到集群' })).toHaveCount(0);
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await expect(page.getByText('服务端预检已通过，尚未写入集群')).toBeVisible();
  await expect(page.getByRole('tab', { name: '服务端处理差异' })).toBeVisible();
  await page.screenshot({ path: 'test-results/manifest-diff.png', fullPage: true });
  await page.getByRole('button', { name: '返回修改' }).click();
  await expect(page.locator('.view-lines')).toContainText('created-by-ui');
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await page.getByRole('button', { name: '确认提交到集群' }).click();
  await expect(page.getByText('请求已提交', { exact: true })).toBeVisible();
  await page.locator('.ant-drawer-close').click();
  await expect(page.getByRole('button', { name: 'created-by-ui', exact: true })).toBeVisible();
});

test('preserves the draft on validation failures and commit conflicts', async ({ page }) => {
  await browse(page);
  await page.getByRole('button', { name: podName, exact: true }).click();
  await page.getByRole('button', { name: '编辑 YAML', exact: true }).click();
  await replaceYAML(page, podYAML(podName, 'invalid-preview'));
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await expect(page.getByText('资源校验失败：spec.containers[0].image')).toBeVisible();
  await expect(page.locator('.view-lines')).toContainText('invalid-preview');
  await replaceYAML(page, podYAML(podName, 'conflict'));
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await page.getByRole('button', { name: '确认提交到集群' }).click();
  await expect(page.getByText(/草稿已保留/)).toBeVisible();
  await expect(page.locator('.view-lines')).toContainText('image: conflict');
  await page.getByRole('button', { name: '重新读取基线（保留草稿）' }).click();
  await expect(page.getByText(/已读取最新资源版本/)).toBeVisible();
  await replaceYAML(page, podYAML(podName, 'nginx:1.29'));
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await page.getByRole('button', { name: '确认提交到集群' }).click();
  await expect(page.getByText('请求已提交', { exact: true })).toBeVisible();
});

test('preflights deletion and requires the exact resource name', async ({ page }) => {
  await browse(page);
  await page.getByRole('switch', { name: '实时更新' }).click();
  await expect(page.getByText('实时更新中', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: podName, exact: true }).click();
  await page.getByRole('button', { name: '删除资源', exact: true }).click();
  await page.getByRole('button', { name: '预检删除', exact: true }).click();
  await expect(page.getByRole('button', { name: '确认删除', exact: true })).toBeDisabled();
  await page.getByRole('textbox', { name: '确认删除资源名称' }).fill(podName);
  await page.getByRole('button', { name: '确认删除', exact: true }).click();
  await expect(page.getByText('删除请求已接受')).toBeVisible();
  await page.locator('.ant-drawer-close').click();
  await expect(page.getByRole('button', { name: podName, exact: true })).toHaveCount(0);
  await page.getByRole('switch', { name: '实时更新' }).click();
  await expect(page.getByText('手动刷新', { exact: true })).toBeVisible();
});

async function chooseResource(page: Page, label: string) {
  await page.getByRole('combobox', { name: '资源类型', exact: true }).click();
  await page.locator('.ant-select-item-option').filter({ hasText: label }).click();
}

test('loads namespace choices across pages, filters immediately, and supports all namespaces', async ({
  page,
}) => {
  await browse(page);
  const picker = page.getByRole('combobox', { name: '命名空间', exact: true });
  await picker.fill('team');
  await expect(
    page.locator('.ant-select-item-option').filter({ hasText: 'team-backend' }),
  ).toBeVisible();
  await page.screenshot({ path: 'test-results/namespace-select.png', fullPage: true });
  await page.locator('.ant-select-item-option').filter({ hasText: 'team-backend' }).click();
  await expect(page.locator('.ant-table-tbody tr.ant-table-row').first()).toContainText(
    'team-backend',
  );
  await picker.click();
  await picker.fill('');
  await page.locator('.ant-select-item-option').filter({ hasText: '全部命名空间' }).click();
  await expect(page.locator('.ant-table-tbody tr.ant-table-row').first()).toContainText('platform');
  await page.getByRole('button', { name: '资源浏览器', exact: true }).click();
  await chooseResource(page, '命名空间 · Namespace');
  await expect(page.getByRole('combobox', { name: '命名空间', exact: true })).toBeDisabled();
});

test('creates a Deployment form, reviews the diff, and follows rollout status', async ({
  page,
}) => {
  await browse(page);
  await chooseResource(page, '部署 · Deployment');
  await page.getByRole('button', { name: '创建 Deployment', exact: true }).click();
  const drawer = page.getByRole('dialog');
  await drawer.getByRole('textbox', { name: 'Deployment 名称', exact: true }).fill('frontend-app');
  await drawer.getByRole('textbox', { name: '容器镜像', exact: true }).fill('nginx:1.29');
  await drawer.getByRole('spinbutton', { name: '期望副本数' }).fill('2');
  await drawer.getByRole('textbox', { name: 'CPU 请求', exact: true }).fill('100m');
  await drawer.getByRole('textbox', { name: '内存上限', exact: true }).fill('128Mi');
  await drawer.getByRole('button', { name: '配置 环境变量', exact: true }).click();
  await drawer.getByRole('button', { name: '添加 环境变量', exact: true }).click();
  await drawer.getByRole('textbox', { name: '环境变量 1 / 变量名称', exact: true }).fill('MODE');
  await drawer.getByRole('combobox', { name: '环境变量 1 / 变量来源 类型', exact: true }).click();
  await page
    .locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)')
    .getByTitle('直接赋值', { exact: true })
    .click();
  await drawer
    .getByRole('textbox', { name: '环境变量 1 / 变量来源 / 直接赋值', exact: true })
    .fill('production');
  await page.screenshot({ path: 'test-results/deployment-create.png', fullPage: true });
  await drawer.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await expect(page.getByText('服务端预检已通过，尚未写入集群')).toBeVisible();
  await page.getByRole('button', { name: '返回修改' }).click();
  await expect(page.getByRole('textbox', { name: '容器镜像', exact: true })).toHaveValue(
    'nginx:1.29',
  );
  await expect(
    page.getByRole('textbox', { name: '环境变量 1 / 变量来源 / 直接赋值', exact: true }),
  ).toHaveValue('production');
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await page.getByRole('button', { name: '确认提交到集群' }).click();
  await expect(page.getByText('请求已提交，等待控制器处理', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '查看发布状态', exact: true }).click();
  await expect(page.getByText('等待控制器处理', { exact: true })).toBeVisible();
  await page.screenshot({ path: 'test-results/deployment-status.png', fullPage: true });
});

test('edits Deployment images, scales and restarts through separate previews', async ({ page }) => {
  await browse(page);
  await chooseResource(page, '部署 · Deployment');
  await page.getByRole('button', { name: 'api-gateway', exact: true }).click();
  await expect(page.getByText('已就绪', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '可视化编辑', exact: true }).click();
  await expect(
    page.getByRole('textbox', { name: '环境变量 1 / 变量名称', exact: true }),
  ).toBeEnabled();
  await expect(
    page.getByRole('textbox', {
      name: '环境变量 1 / 变量来源 / 引用 / Secret 键 / 资源名称',
      exact: true,
    }),
  ).toHaveValue('credentials');
  await page.getByRole('textbox', { name: '容器镜像', exact: true }).fill('invalid-preview');
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await expect(page.getByText('资源校验失败：spec.containers[0].image')).toBeVisible();
  await expect(page.getByRole('textbox', { name: '容器镜像', exact: true })).toHaveValue(
    'invalid-preview',
  );
  await page.getByRole('textbox', { name: '容器镜像', exact: true }).fill('conflict');
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await page.getByRole('button', { name: '确认提交到集群' }).click();
  await expect(page.getByText(/草稿已保留/)).toBeVisible();
  await page.getByRole('textbox', { name: '容器镜像', exact: true }).fill('nginx:updated');
  await page.getByRole('button', { name: '重新读取基线（保留草稿）' }).click();
  await expect(page.getByText(/已读取最新基线/)).toBeVisible();
  await expect(page.getByRole('textbox', { name: '容器镜像', exact: true })).toHaveValue(
    'nginx:updated',
  );
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await page.getByRole('button', { name: '确认提交到集群' }).click();
  await page.getByRole('button', { name: '查看发布状态', exact: true }).click();
  await expect(page.getByText('nginx:updated', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '扩缩容', exact: true }).click();
  await page.getByRole('spinbutton', { name: '期望副本数' }).fill('0');
  await expect(page.getByRole('textbox', { name: '容器镜像', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await expect(page.getByText('服务端预检已通过，尚未写入集群')).toBeVisible();
  await page.getByRole('button', { name: '确认提交到集群' }).click();
  await page.getByRole('button', { name: '查看发布状态', exact: true }).click();
  await expect(page.locator('.replica-grid > div').filter({ hasText: '期望副本' })).toContainText(
    '0',
  );
  await page.getByRole('button', { name: '重启发布', exact: true }).click();
  await expect(page.getByText(/重启将更新 Pod 模板时间戳/)).toBeVisible();
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await page.getByRole('button', { name: '确认提交到集群' }).click();
  await expect(page.getByText('请求已提交，等待控制器处理', { exact: true })).toBeVisible();
});

test('keeps namespace queries isolated when switching connected clusters', async ({ page }) => {
  await browse(page);
  await page.getByRole('button', { name: '集群连接', exact: true }).click();
  const staging = page.locator('.cluster-row').filter({ hasText: 'staging-hangzhou' });
  await staging.getByRole('button', { name: '连接', exact: true }).click();
  await staging.getByRole('button', { name: '浏览资源' }).click();
  await expect(page.getByRole('textbox', { name: '命名空间', exact: true })).toHaveValue('default');
  await expect(page.getByText('无法读取 Namespace 列表，请手动输入或重试')).toBeVisible();
  await page.getByRole('combobox', { name: '切换集群' }).click();
  await page.locator('.ant-select-item-option').filter({ hasText: 'production-shanghai' }).click();
  await expect(page.getByRole('combobox', { name: '命名空间', exact: true })).toBeVisible();
  await expect(page.getByText('无法读取 Namespace 列表，请手动输入或重试')).toHaveCount(0);
  await page.getByRole('combobox', { name: '命名空间', exact: true }).click();
  await expect(
    page.locator('.ant-select-item-option').filter({ hasText: 'team-backend' }),
  ).toBeVisible();
});

test('creates a Service with selectors and ports only after reviewing its preview', async ({
  page,
}) => {
  await browse(page);
  await page.getByRole('button', { name: '服务与网络', exact: true }).click();
  await page.getByRole('button', { name: '创建 Service', exact: true }).click();
  await page.getByRole('textbox', { name: 'Service 名称', exact: true }).fill('new-service');
  await page.getByRole('combobox', { name: '服务类型', exact: true }).click();
  await page
    .locator('.ant-select-item-option')
    .filter({ hasText: /^NodePort$/ })
    .click();
  await page.getByRole('button', { name: '添加选择器', exact: true }).click();
  await page.getByRole('textbox', { name: '选择器键 1', exact: true }).fill('app');
  await page.getByRole('textbox', { name: '选择器值 1', exact: true }).fill('gateway');
  await page.getByRole('textbox', { name: '目标端口 1', exact: true }).fill('http');
  await page.getByRole('spinbutton', { name: '节点端口 1', exact: true }).fill('30088');
  await page.screenshot({ path: 'test-results/service-create.png', fullPage: true });
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await expect(page.getByText('服务端预检已通过，尚未写入集群')).toBeVisible();
  await page.getByRole('button', { name: '返回修改' }).click();
  await expect(page.getByRole('textbox', { name: '目标端口 1', exact: true })).toHaveValue('http');
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await page.getByRole('button', { name: '确认提交到集群' }).click();
  await expect(page.getByText('请求已提交', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '查看资源', exact: true }).click();
  await expect(page.getByRole('dialog').getByText('30088', { exact: true })).toBeVisible();
  await page.screenshot({
    path: 'test-results/service-details.png',
    fullPage: true,
    animations: 'disabled',
  });
});

test('edits Service type with explicit cleanup and opens YAML from resource details', async ({
  page,
}) => {
  await browse(page);
  await page.getByRole('button', { name: '服务与网络', exact: true }).click();
  await page.getByRole('button', { name: 'gateway-service', exact: true }).click();
  await expect(page.getByRole('dialog').getByText('10.96.0.12', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '可视化编辑', exact: true }).click();
  await expect(page.getByRole('spinbutton', { name: '节点端口 1', exact: true })).toHaveValue(
    '30080',
  );
  await page.getByRole('combobox', { name: '服务类型', exact: true }).click();
  await page
    .locator('.ant-select-item-option')
    .filter({ hasText: /^ClusterIP$/ })
    .click();
  await expect(page.getByText(/切换类型会清理不再适用/)).toBeVisible();
  await expect(page.getByRole('spinbutton', { name: '节点端口 1', exact: true })).toHaveCount(0);
  await page.getByRole('textbox', { name: '目标端口 1', exact: true }).fill('9090');
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await page.getByRole('button', { name: '确认提交到集群' }).click();
  await page.getByRole('button', { name: '查看资源', exact: true }).click();
  await expect(page.getByRole('dialog').getByText('ClusterIP', { exact: true })).toBeVisible();
  await page.locator('.ant-drawer-close').click();
  await page.getByRole('button', { name: 'gateway-service', exact: true }).click();
  await page.getByRole('button', { name: '查看 / 编辑 YAML', exact: true }).click();
  await expect(page.getByRole('button', { name: '编辑 YAML', exact: true })).toBeVisible();
  await expect(page.locator('.monaco-editor')).toBeVisible();
});

test('validates ConfigMap binary data, preserves multiline text and locks immutable data', async ({
  page,
}) => {
  await browse(page);
  await page.getByRole('button', { name: '配置管理', exact: true }).click();
  await page.getByRole('button', { name: '创建 ConfigMap', exact: true }).click();
  await page.getByRole('textbox', { name: 'ConfigMap 名称', exact: true }).fill('new-config');
  await page.getByRole('button', { name: '添加文本配置', exact: true }).click();
  await page.getByRole('textbox', { name: '文本键名 1', exact: true }).fill('application.yaml');
  const config = 'server:\n  port: 8080\n  name: 中文配置\n';
  await page.getByRole('textbox', { name: '文本内容 1', exact: true }).fill(config);
  await page.screenshot({ path: 'test-results/configmap-create.png', fullPage: true });
  await page.getByRole('tab', { name: /二进制数据/ }).click();
  await page.getByRole('button', { name: '添加二进制配置', exact: true }).click();
  await page.getByRole('textbox', { name: '二进制键名 1', exact: true }).fill('payload.bin');
  await page.getByRole('textbox', { name: '二进制内容 1', exact: true }).fill('not base64!');
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await expect(page.getByText('binaryData 必须填写有效的标准 Base64')).toBeVisible();
  await expect(page.getByRole('textbox', { name: '二进制内容 1', exact: true })).toHaveValue(
    'not base64!',
  );
  await page.getByRole('textbox', { name: '二进制内容 1', exact: true }).fill('AAEC/w==');
  await page.getByRole('checkbox', { name: /设为不可变/ }).check();
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await page.getByRole('button', { name: '返回修改' }).click();
  await expect(page.getByRole('textbox', { name: '文本内容 1', exact: true })).toHaveValue(config);
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await page.getByRole('button', { name: '确认提交到集群' }).click();
  await page.getByRole('button', { name: '查看资源', exact: true }).click();
  await expect(page.getByText('已锁定', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: '可视化编辑', exact: true })).toBeDisabled();
  await page.getByRole('button', { name: 'application.yaml', exact: true }).click();
  await expect(page.locator('.config-preview')).toBeVisible();
  await expect(page.locator('.config-preview')).toHaveText(config);
  await expect(page.locator('.config-preview')).toBeInViewport({ ratio: 1 });
  await page.getByRole('button', { name: 'payload.bin', exact: true }).click();
  await expect(page.locator('.config-preview').filter({ hasText: 'AAEC/w==' })).toBeInViewport({
    ratio: 1,
  });
  await page.screenshot({
    path: 'test-results/configmap-details.png',
    fullPage: true,
    animations: 'disabled',
  });
});

test('edits ConfigMap keys and rejects duplicates without discarding the draft', async ({
  page,
}) => {
  await browse(page);
  await page.getByRole('button', { name: '配置管理', exact: true }).click();
  await page.getByRole('button', { name: 'app-config', exact: true }).click();
  await page.getByRole('button', { name: '可视化编辑', exact: true }).click();
  await page.getByRole('button', { name: '添加文本配置', exact: true }).click();
  await page.getByRole('textbox', { name: '文本键名 2', exact: true }).fill('application.yaml');
  await page.getByRole('textbox', { name: '文本内容 2', exact: true }).fill('second');
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await expect(page.getByText('配置键名不能为空或重复')).toBeVisible();
  await page.getByRole('textbox', { name: '文本键名 2', exact: true }).fill('second.txt');
  await page.getByRole('button', { name: '重新读取基线（保留草稿）' }).click();
  await expect(page.getByRole('textbox', { name: '文本内容 2', exact: true })).toHaveValue(
    'second',
  );
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await page.getByRole('button', { name: '确认提交到集群' }).click();
  await page.getByRole('button', { name: '查看资源', exact: true }).click();
  await expect(page.getByRole('button', { name: 'second.txt', exact: true })).toBeVisible();
});

test('provides cluster schema diagnostics and completion without downloading remote schemas', async ({
  page,
  context,
}) => {
  const remoteRequests: string[] = [];
  context.on('request', (request) => {
    if (request.url().startsWith('https://schema.example.invalid/'))
      remoteRequests.push(request.url());
  });
  await browse(page);
  await page.getByRole('button', { name: '创建资源', exact: true }).click();
  await expect(page.getByText('OpenAPI v3 · Pod', { exact: true })).toBeVisible();
  const header =
    'apiVersion: v1\nkind: Pod\nmetadata:\n  name: schema-test\n  namespace: platform\nspec:\n';
  await replaceYAML(page, `${header}  containers: wrong\n`);
  await expect(page.locator('.yaml-diagnostics')).toContainText('array', { timeout: 15000 });
  await page.locator('.yaml-diagnostics').getByRole('button').first().click();
  await expect(page.getByRole('textbox', { name: 'YAML 资源编辑器', exact: true })).toBeFocused();
  await expect(page.getByRole('button', { name: '服务端预检并查看差异' })).toBeInViewport({
    ratio: 1,
  });
  await page.screenshot({
    path: 'test-results/schema-diagnostics.png',
    fullPage: true,
    animations: 'disabled',
  });
  await replaceYAML(page, `${header}  `);
  await page.getByRole('button', { name: '字段补全', exact: true }).click();
  await expect(page.locator('.suggest-widget.visible')).toContainText('containers', {
    timeout: 15000,
  });
  await page.keyboard.press('Escape');
  await replaceYAML(
    page,
    `# yaml-language-server: $schema=https://schema.example.invalid/private.json\n${podYAML('schema-test', 'nginx:stable')}`,
  );
  await page.getByRole('button', { name: '刷新 Schema', exact: true }).click();
  await expect(page.locator('.view-lines')).toContainText('nginx:stable');
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await expect(page.getByText('服务端预检已通过，尚未写入集群')).toBeVisible();
  expect(remoteRequests).toEqual([]);
});

test('retains CRD integer text while loading and refreshing schema', async ({ page }) => {
  await browse(page);
  await page.getByRole('button', { name: '自定义资源', exact: true }).click();
  await chooseResource(page, 'Widget · demo.example.com');
  await page.getByRole('button', { name: '创建资源', exact: true }).click();
  await expect(page.getByText('OpenAPI v3 · Widget', { exact: true })).toBeVisible();
  await replaceYAML(
    page,
    'apiVersion: demo.example.com/v1\nkind: Widget\nmetadata:\n  name: exact-integer\n  namespace: platform\nspec:\n  sequence: 9007199254740993\n',
  );
  await page.getByRole('button', { name: '刷新 Schema', exact: true }).click();
  await expect(page.locator('.view-lines')).toContainText('9007199254740993');
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await expect(page.locator('.monaco-diff-editor')).toContainText('9007199254740993');
});

test('opens Deployment descendants and drills into Pod events', async ({ page }) => {
  await browse(page);
  await chooseResource(page, '部署 · Deployment');
  await page.getByRole('button', { name: 'api-gateway', exact: true }).click();
  await page.getByRole('tab', { name: '关联资源与事件', exact: true }).click();
  await expect(page.getByText('修订 3', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'api-gateway-rs-3', exact: true })).toBeVisible();
  await page.screenshot({
    path: 'test-results/deployment-relations.png',
    fullPage: true,
    animations: 'disabled',
  });
  await page.getByRole('button', { name: 'api-gateway-pending', exact: true }).click();
  await expect(page.getByText('BackOff', { exact: true })).toBeVisible();
  await expect(
    page.getByText('容器镜像拉取失败，请检查镜像地址与镜像仓库权限。', { exact: true }),
  ).toBeVisible();
  await page.getByRole('button', { name: '返回上级关联', exact: true }).click();
  const row = page.locator('.ant-table-row').filter({ hasText: 'api-gateway-pending' });
  await row.getByRole('button', { name: 'YAML', exact: true }).click();
  await expect(page.locator('.view-lines')).toContainText('api-gateway-pending');
});

test('keeps EndpointSlice results visible when Pod access is denied', async ({ page }) => {
  await browse(page);
  await page.getByRole('button', { name: '服务与网络', exact: true }).click();
  await page.getByRole('button', { name: 'gateway-service', exact: true }).click();
  await page.getByRole('tab', { name: '关联资源与事件', exact: true }).click();
  await expect(page.getByRole('button', { name: 'gateway-endpoints', exact: true })).toBeVisible();
  await expect(page.getByText('1 / 2 就绪 · 1 未知', { exact: true })).toBeVisible();
  await expect(
    page.getByText('选择器匹配的 Pod 读取失败：权限不足：无法列出 Pod', { exact: true }),
  ).toBeVisible();
  await expect(page.getByText('当前没有可展示的资源，查询未完成', { exact: true })).toBeVisible();
  await page.screenshot({
    path: 'test-results/service-relations.png',
    fullPage: true,
    animations: 'disabled',
  });
});

test('keeps YAML usable when the current cluster cannot provide a schema', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig' }).click();
  const staging = page.locator('.cluster-row').filter({ hasText: 'staging-hangzhou' });
  await staging.getByRole('button', { name: '连接', exact: true }).click();
  await staging.getByRole('button', { name: '浏览资源' }).click();
  await page.getByRole('button', { name: '创建资源', exact: true }).click();
  await expect(page.getByText('Schema 不可用', { exact: true })).toBeVisible();
  await replaceYAML(
    page,
    podYAML('schema-fallback', 'nginx:stable').replace('platform', 'default'),
  );
  await page.getByRole('button', { name: '服务端预检并查看差异' }).click();
  await expect(page.getByText('服务端预检已通过，尚未写入集群')).toBeVisible();
});
