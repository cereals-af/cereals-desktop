import { expect, test, type Page } from '@playwright/test';
const errors = new WeakMap<Page, string[]>();
test.beforeEach(({ page }) => {
  const list: string[] = [];
  errors.set(page, list);
  page.on('pageerror', (error) => list.push(error.message));
});
test.afterEach(({ page }) => expect(errors.get(page)).toEqual([]));
async function browse(page: Page, kind: string, category = '工作负载') {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig', exact: true }).click();
  const cluster = page.locator('.cluster-row').filter({ hasText: 'production-shanghai' });
  await cluster.getByRole('button', { name: '连接', exact: true }).click();
  await cluster.getByRole('button', { name: '浏览资源', exact: true }).click();
  await page.getByRole('button', { name: category, exact: true }).click();
  await page.getByRole('combobox', { name: '资源类型' }).click();
  await page
    .locator('.ant-select-item-option')
    .filter({ hasText: `· ${kind}` })
    .click();
}
async function select(page: Page, label: string, value: string) {
  await page.getByRole('combobox', { name: label, exact: true }).click();
  const dropdown = page.locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)');
  await dropdown.getByTitle(value, { exact: true }).click();
  await expect(dropdown).toHaveCount(0);
}
async function commit(page: Page, label = '查看资源') {
  await page.getByRole('button', { name: '服务端预检并查看差异', exact: true }).click();
  await expect(page.getByText('服务端预检已通过，尚未写入集群')).toBeVisible();
  await page.getByRole('button', { name: '确认提交到集群', exact: true }).click();
  await page.getByRole('button', { name: label, exact: true }).click();
}

test('Job creates with container configuration and protects immutable template in edit', async ({
  page,
}) => {
  await browse(page, 'Job');
  await page.getByRole('button', { name: '创建 Job', exact: true }).click();
  await page.getByRole('textbox', { name: 'Job 名称', exact: true }).fill('batch-created');
  await page
    .getByRole('textbox', {
      name: 'Job 配置 / Pod 模板 / Pod 配置 / 容器 1 / 容器镜像',
      exact: true,
    })
    .fill('busybox:stable');
  await page.screenshot({ path: 'test-results/job-form.png' });
  await commit(page);
  await expect(page.getByText('Job · batch-created', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '可视化编辑', exact: true }).click();
  await expect(page.getByText(/已有 Job 的 Pod 模板不可直接修改/)).toBeVisible();
  await expect(page.getByRole('textbox', { name: /容器镜像/ })).toHaveCount(0);
});
test('CronJob pause, resume and manual Job share reviewed writes', async ({ page }) => {
  await browse(page, 'CronJob');
  await page.getByRole('button', { name: 'demo-cronjobs', exact: true }).click();
  await page.getByRole('button', { name: '暂停', exact: true }).click();
  await commit(page);
  await expect(page.getByRole('button', { name: '恢复', exact: true })).toBeVisible();
  await page.getByRole('button', { name: '手动运行', exact: true }).click();
  await page.getByRole('textbox', { name: '新 Job 名称', exact: true }).fill('manual-job');
  await commit(page, '查看 Job');
  await expect(page.getByText('Job · manual-job', { exact: true })).toBeVisible();
  await expect(page.getByRole('tab', { name: '容器组', exact: true })).toBeVisible();
});
test('Ingress creates HTTP routing and TLS and preserves entered fields through review', async ({
  page,
}) => {
  await browse(page, 'Ingress', '服务与网络');
  await page.getByRole('button', { name: '创建 Ingress', exact: true }).click();
  await page.getByRole('textbox', { name: 'Ingress 名称', exact: true }).fill('public-api');
  await page
    .getByRole('textbox', { name: 'Ingress 配置 / IngressClass', exact: true })
    .fill('nginx');
  await page
    .getByRole('textbox', { name: 'Ingress 配置 / 路由规则 1 / 域名', exact: true })
    .fill('api.example.test');
  await page
    .getByRole('textbox', {
      name: 'Ingress 配置 / 路由规则 1 / HTTP 路由 / 路径 1 / 后端 / Service / Service 名称',
      exact: true,
    })
    .fill('api');
  await page.getByRole('button', { name: '配置 Ingress 配置 / TLS 配置', exact: true }).click();
  await page.getByRole('button', { name: '添加 Ingress 配置 / TLS 配置', exact: true }).click();
  await page
    .getByRole('textbox', { name: 'Ingress 配置 / TLS 配置 1 / TLS Secret 名称', exact: true })
    .fill('api-tls');
  await page
    .getByRole('textbox', { name: 'Ingress 配置 / IngressClass', exact: true })
    .scrollIntoViewIfNeeded();
  await page.screenshot({ path: 'test-results/ingress-form.png' });
  await commit(page);
  await expect(page.locator('.visual-readonly-value').filter({ hasText: /^nginx$/ })).toBeVisible();
});
test('NetworkPolicy distinguishes deny-all from allow rules and supports named ports', async ({
  page,
}) => {
  await browse(page, 'NetworkPolicy', '服务与网络');
  await page.getByRole('button', { name: '创建 NetworkPolicy', exact: true }).click();
  await page.getByRole('textbox', { name: 'NetworkPolicy 名称', exact: true }).fill('isolate-api');
  await expect(page.getByText(/空规则列表 \[\] 拒绝所有入站流量/)).toBeVisible();
  await page.getByRole('button', { name: '添加 网络策略配置 / 入站规则', exact: true }).click();
  await page
    .getByRole('button', { name: '配置 网络策略配置 / 入站规则 1 / 允许端口', exact: true })
    .click();
  await page
    .getByRole('button', { name: '添加 网络策略配置 / 入站规则 1 / 允许端口', exact: true })
    .click();
  await page
    .getByRole('textbox', { name: '网络策略配置 / 入站规则 1 / 允许端口 1 / 端口', exact: true })
    .fill('http');
  await page.screenshot({ path: 'test-results/network-policy-form.png' });
  await commit(page);
  await expect(page.locator('.visual-readonly-value').filter({ hasText: /^http$/ })).toBeVisible();
});
test('Secret reveals decoded text explicitly, hides again, and edits text or binary', async ({
  page,
}) => {
  await browse(page, 'Secret', '配置管理');
  await page.getByRole('button', { name: 'demo-secrets', exact: true }).click();
  await expect(page.getByRole('textbox', { name: 'Secret 值 1', exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: '可视化编辑', exact: true })).toBeDisabled();
  await page.getByRole('button', { name: '解码查看', exact: true }).click();
  await expect(page.getByRole('textbox', { name: 'Secret 值 1', exact: true })).toHaveValue(
    '服务:\n  密码: 测试密钥\n',
  );
  await expect(page.getByRole('textbox', { name: 'Secret 值 2', exact: true })).toHaveValue('AP8=');
  await page.screenshot({ path: 'test-results/secret-decoded.png' });
  await page.getByRole('button', { name: '隐藏配置', exact: true }).click();
  await expect(page.getByRole('textbox', { name: 'Secret 值 1', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: '解码查看', exact: true }).click();
  await page.getByRole('button', { name: '可视化编辑', exact: true }).click();
  await page.getByRole('textbox', { name: 'Secret 值 1', exact: true }).fill('新的配置\n');
  await commit(page);
  await expect(page.getByRole('textbox', { name: 'Secret 值 1', exact: true })).toHaveValue(
    '新的配置\n',
  );
  const persisted = await page.evaluate(() =>
    JSON.stringify({ ...localStorage, ...sessionStorage }),
  );
  expect(persisted).not.toContain('新的配置');
});
test('Deployment advanced editor adds Secret refs and init containers with uniform controls', async ({
  page,
}) => {
  await browse(page, 'Deployment');
  await page.getByRole('button', { name: 'api-gateway', exact: true }).click();
  await page.getByRole('button', { name: '可视化编辑', exact: true }).click();
  await page.getByRole('tab', { name: '环境变量', exact: true }).click();
  await page.getByRole('button', { name: '添加 环境变量', exact: true }).click();
  await page
    .getByRole('textbox', { name: '环境变量 2 / 变量名称', exact: true })
    .fill('EXTRA_TOKEN');
  await select(page, '环境变量 2 / 变量来源', 'Secret 键');
  await page
    .getByRole('textbox', {
      name: '环境变量 2 / 资源名称',
      exact: true,
    })
    .fill('credentials');
  await page.getByRole('textbox', { name: '环境变量 2 / 键名', exact: true }).fill('token');
  await page.getByRole('heading', { name: '环境变量', exact: true }).click();
  await page.getByRole('button', { name: '添加 环境变量批量引用', exact: true }).click();
  await page
    .getByRole('textbox', { name: '环境变量批量引用 1 / 变量前缀', exact: true })
    .fill('APP_');
  await select(page, '环境变量批量引用 1 / 引用来源', 'Secret');
  await page
    .getByRole('textbox', { name: '环境变量批量引用 1 / 资源名称', exact: true })
    .fill('settings');
  await page.getByRole('heading', { name: '环境变量', exact: true }).click();
  await page.getByRole('region', { name: '容器配置 app', exact: true }).scrollIntoViewIfNeeded();
  await page.screenshot({ path: 'test-results/environment-flat-editor.png' });
  await page.getByRole('tab', { name: '初始化容器', exact: true }).click();
  await page.getByRole('button', { name: '添加 初始化容器', exact: true }).click();
  await page.getByRole('textbox', { name: '初始化容器 1 / 容器名称', exact: true }).fill('init');
  await page.getByRole('textbox', { name: '初始化容器 1 / 容器镜像', exact: true }).fill('busybox');
  await page.getByRole('tab', { name: '端口', exact: true }).last().click();
  await page.getByRole('button', { name: '添加 初始化容器 1 / 端口映射', exact: true }).click();
  await page
    .getByRole('textbox', { name: '初始化容器 1 / 端口映射 1 / 容器端口', exact: true })
    .fill('8080');
  const dimensions = await page.locator('.form-item-action').evaluateAll((nodes) =>
    nodes
      .map((node) => ({
        w: node.getBoundingClientRect().width,
        h: node.getBoundingClientRect().height,
      }))
      .filter((r) => r.w > 0),
  );
  expect(dimensions.every((r) => r.h === 36 && r.w >= 112)).toBe(true);
  await page
    .getByRole('textbox', { name: '初始化容器 1 / 容器名称', exact: true })
    .scrollIntoViewIfNeeded();
  await page.screenshot({ path: 'test-results/advanced-pod-form.png' });
  await page.setViewportSize({ width: 1024, height: 900 });
  await page.locator('.generic-port-row').scrollIntoViewIfNeeded();
  const drawerBounds = await page.getByRole('dialog').boundingBox();
  expect(drawerBounds?.x).toBeGreaterThanOrEqual(0);
  expect(drawerBounds?.width).toBeLessThanOrEqual(1024);
  const overflowing = await page
    .locator('.generic-port-row, .environment-entry')
    .evaluateAll(
      (nodes) =>
        nodes.filter((node) => node.clientWidth > 0 && node.scrollWidth > node.clientWidth + 1)
          .length,
    );
  expect(overflowing).toBe(0);
  await page.screenshot({ path: 'test-results/compact-form-narrow.png' });
  await commit(page, '查看发布状态');
  await page.getByRole('button', { name: '可视化编辑', exact: true }).click();
  await page.getByRole('tab', { name: '环境变量', exact: true }).click();
  await expect(
    page.getByRole('textbox', { name: '环境变量 2 / 资源名称', exact: true }),
  ).toHaveValue('credentials');
  await expect(
    page.getByRole('textbox', { name: '环境变量批量引用 1 / 变量前缀', exact: true }),
  ).toHaveValue('APP_');
  await expect(
    page.getByRole('textbox', { name: '环境变量批量引用 1 / 资源名称', exact: true }),
  ).toHaveValue('settings');
});

test('flat affinity sections preserve drafts across type changes, review and reopening', async ({
  page,
}) => {
  await browse(page, 'Deployment');
  await page.getByRole('button', { name: 'api-gateway', exact: true }).click();
  await page.getByRole('button', { name: '可视化编辑', exact: true }).click();
  const required = '亲和性 / 节点亲和性 / 必须满足';
  const expression = `${required} 1 / 节点标签表达式`;
  await page.getByRole('button', { name: `添加 ${required}`, exact: true }).click();
  await page.getByRole('button', { name: `添加 ${expression}`, exact: true }).click();
  await page.getByRole('textbox', { name: `${expression} 1 / 键`, exact: true }).fill('disk');
  await page.getByRole('combobox', { name: `${expression} 1 / 匹配值`, exact: true }).fill('ssd');
  await page
    .getByRole('combobox', { name: `${expression} 1 / 匹配值`, exact: true })
    .press('Enter');
  await page.getByRole('heading', { name: '亲和性', exact: true }).click();
  const preferred = '亲和性 / 节点亲和性 / 优先满足';
  await page.getByRole('button', { name: `添加 ${preferred}`, exact: true }).click();
  await page
    .getByRole('textbox', { name: `${preferred} 1 / 权重（1–100）`, exact: true })
    .fill('50');
  const preference = `${preferred} 1 / 节点匹配条件 / 节点标签表达式`;
  await page.getByRole('button', { name: `添加 ${preference}`, exact: true }).click();
  await page.getByRole('textbox', { name: `${preference} 1 / 键`, exact: true }).fill('fast');
  await select(page, `${preference} 1 / 运算符`, 'Exists');
  await expect(
    page.getByRole('combobox', { name: `${preference} 1 / 匹配值`, exact: true }),
  ).toBeDisabled();
  await page
    .getByRole('combobox', { name: '亲和性 / 亲和性类型', exact: true })
    .scrollIntoViewIfNeeded();
  await page.getByRole('heading', { name: '亲和性', exact: true }).click();
  await page.screenshot({ path: 'test-results/affinity-flat-editor.png' });

  await select(page, '亲和性 / 亲和性类型', 'Pod 反亲和性');
  const anti = '亲和性 / Pod 反亲和性 / 必须满足';
  await page.getByRole('button', { name: `添加 ${anti}`, exact: true }).click();
  await page
    .getByRole('textbox', { name: `${anti} 1 / 拓扑键`, exact: true })
    .fill('kubernetes.io/hostname');
  await page.getByRole('button', { name: `配置 ${anti} 1 / Pod 选择器`, exact: true }).click();
  await select(page, '亲和性 / 亲和性类型', '节点亲和性 · 2 条规则');
  await expect(
    page.getByRole('textbox', { name: `${expression} 1 / 键`, exact: true }),
  ).toHaveValue('disk');
  await expect(
    page.getByRole('textbox', { name: `${preferred} 1 / 权重（1–100）`, exact: true }),
  ).toHaveValue('50');
  await page
    .getByRole('region', { name: 'Pod 配置', exact: true })
    .getByRole('tab', { name: '网络与 DNS', exact: true })
    .click();
  await page
    .getByRole('region', { name: 'Pod 配置', exact: true })
    .getByRole('tab', { name: '调度配置', exact: true })
    .click();
  await expect(
    page.getByRole('textbox', { name: `${expression} 1 / 键`, exact: true }),
  ).toHaveValue('disk');
  await commit(page, '查看发布状态');
  await page.getByRole('button', { name: '可视化编辑', exact: true }).click();
  await expect(
    page.getByRole('textbox', { name: `${expression} 1 / 键`, exact: true }),
  ).toHaveValue('disk');
  await select(page, '亲和性 / 亲和性类型', 'Pod 反亲和性 · 1 条规则');
  await expect(page.getByRole('textbox', { name: `${anti} 1 / 拓扑键`, exact: true })).toHaveValue(
    'kubernetes.io/hostname',
  );
  await page.getByRole('button', { name: `清除 ${anti}`, exact: true }).click();
  await page.getByRole('button', { name: `添加 ${anti}`, exact: true }).click();
  await expect(
    page.getByRole('textbox', { name: `${anti} 1 / 拓扑键`, exact: true }),
  ).toBeVisible();
  await expect(page.getByRole('textbox', { name: `${anti} 1 / 拓扑键`, exact: true })).toHaveValue(
    '',
  );
});
