import { expect, test, type Page } from '@playwright/test';

const errors = new WeakMap<Page, string[]>();
test.beforeEach(({ page }) => {
  const messages: string[] = [];
  errors.set(page, messages);
  page.on('pageerror', (error) => messages.push(error.message));
});
test.afterEach(({ page }) => {
  expect(errors.get(page)).toEqual([]);
});
async function browse(page: Page, kind: string) {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig', exact: true }).click();
  const cluster = page.locator('.cluster-row').filter({ hasText: 'production-shanghai' });
  await cluster.getByRole('button', { name: '连接', exact: true }).click();
  await cluster.getByRole('button', { name: '浏览资源', exact: true }).click();
  if (kind === 'Node') {
    await page.getByRole('button', { name: '节点与空间', exact: true }).click();
    return;
  }
  await page.getByRole('combobox', { name: '资源类型' }).click();
  await page
    .locator('.ant-select-item-option')
    .filter({ hasText: `· ${kind}` })
    .click();
}
async function confirm(page: Page) {
  await page.getByRole('button', { name: '服务端预检并查看差异', exact: true }).click();
  await expect(page.getByText('服务端预检已通过，尚未写入集群')).toBeVisible();
  await page.getByRole('button', { name: '确认提交到集群', exact: true }).click();
}
async function container(page: Page, name = 'app') {
  await page.getByRole('combobox', { name: 'Pod 容器', exact: true }).click();
  await page
    .locator('.ant-select-dropdown:not(.ant-select-dropdown-hidden)')
    .getByTitle(`${name} · 普通 · running · 重启 ${name === 'app' ? 2 : 0} 次`)
    .click();
}

test('StatefulSet visual create includes PVC templates and shared review', async ({ page }) => {
  await browse(page, 'StatefulSet');
  await page.getByRole('button', { name: '创建 StatefulSet', exact: true }).click();
  await page.getByRole('textbox', { name: 'StatefulSet 名称', exact: true }).fill('new-cache');
  await page.getByRole('dialog').getByRole('combobox', { name: '命名空间', exact: true }).click();
  await page.locator('.ant-select-item-option').filter({ hasText: 'team-backend' }).click();
  await page.getByRole('textbox', { name: '容器镜像', exact: true }).fill('redis:8');
  await page.getByRole('textbox', { name: '治理 Service', exact: true }).fill('cache-headless');
  await page.getByRole('button', { name: '添加 PVC 模板', exact: true }).click();
  await page.getByRole('textbox', { name: 'PVC 容量 1', exact: true }).fill('20Gi');
  await page.screenshot({ path: 'test-results/sts-create.png', fullPage: true });
  await confirm(page);
  await page.getByRole('button', { name: '查看工作负载状态', exact: true }).click();
  await expect(page.getByText('StatefulSet · new-cache', { exact: true })).toBeVisible();
  await expect(page.locator('.drawer-title small')).toContainText('team-backend');
});

test('StatefulSet edits strategy, scales with PVC notice, and opens storage', async ({ page }) => {
  await browse(page, 'StatefulSet');
  await page.getByRole('button', { name: 'cache', exact: true }).click();
  await expect(page.getByText('revision-2', { exact: true }).first()).toBeVisible();
  await page.getByRole('button', { name: '更新配置', exact: true }).click();
  await expect(page.getByRole('textbox', { name: '治理 Service', exact: true })).toBeDisabled();
  await page.getByRole('spinbutton', { name: '更新分区', exact: true }).fill('1');
  await confirm(page);
  await page.getByRole('button', { name: '查看工作负载状态', exact: true }).click();
  await expect(page.getByText(/partition = 1/)).toBeVisible();
  await page.getByRole('button', { name: '扩缩容', exact: true }).click();
  await page.getByRole('spinbutton', { name: '期望副本数', exact: true }).fill('1');
  await expect(page.getByText('本次缩容 PVC 策略：Delete', { exact: true })).toBeVisible();
  await confirm(page);
  await page.getByRole('button', { name: '查看发布状态', exact: true }).click();
  await page.getByRole('tab', { name: '存储与 PVC', exact: true }).click();
  await expect(page.getByRole('button', { name: 'data-cache-0', exact: true })).toBeVisible();
  await page.screenshot({ path: 'test-results/sts-storage.png', fullPage: true });
  await page.getByRole('button', { name: '详情 / 扩容', exact: true }).click();
  await expect(page.getByText('PVC · data-cache-0', { exact: true })).toBeVisible();
});

test('DaemonSet has strategy and ControllerRevision rollback without scale', async ({ page }) => {
  await browse(page, 'DaemonSet');
  await expect(page.getByRole('button', { name: '按名称扩缩容', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: 'ondelete-agent', exact: true }).click();
  await expect(page.getByRole('button', { name: '扩缩容', exact: true })).toHaveCount(0);
  await expect(page.getByText(/OnDelete：模板更新/).filter({ visible: true })).toBeVisible();
  await page.getByRole('button', { name: '重启工作负载', exact: true }).click();
  await expect(page.getByText('更新 Pod 模板的重启时间戳', { exact: true })).toBeVisible();
  await confirm(page);
  await page.getByRole('button', { name: '查看工作负载状态', exact: true }).click();
  await page.getByRole('tab', { name: '发布历史', exact: true }).click();
  await expect(
    page.getByText('不支持的 ControllerRevision 数据格式', { exact: true }),
  ).toBeVisible();
  await expect(
    page
      .getByRole('row')
      .filter({ hasText: 'unknown-revision' })
      .getByRole('button', { name: '回滚到此版本' }),
  ).toBeDisabled();
  await page
    .getByRole('row')
    .filter({ hasText: 'revision-1' })
    .getByRole('button', { name: '回滚到此版本' })
    .click();
  await expect(page.getByText(/OnDelete：模板更新/).filter({ visible: true })).toBeVisible();
  await confirm(page);
  await page.getByRole('button', { name: '查看发布状态', exact: true }).click();
  await page.getByRole('tab', { name: '容器配置', exact: true }).click();
  await expect(page.getByText('nginx:1.28', { exact: true })).toBeVisible();
  await page.screenshot({ path: 'test-results/daemonset-detail.png', fullPage: true });
});

test('ReplicaSet navigates to owner and standalone template explains behavior', async ({
  page,
}) => {
  await browse(page, 'ReplicaSet');
  await page.getByRole('button', { name: 'managed-rs', exact: true }).click();
  await expect(page.getByRole('tab', { name: '发布历史', exact: true })).toHaveCount(0);
  await expect(page.getByRole('button', { name: '暂停发布', exact: true })).toHaveCount(0);
  await page.getByRole('button', { name: '进入所属 Deployment', exact: true }).click();
  await expect(page.getByText('Deployment · api-gateway', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '关闭', exact: true }).last().click();
  await page.getByRole('button', { name: '关闭', exact: true }).click();
  await page.getByRole('button', { name: 'standalone-rs', exact: true }).click();
  await expect(page.getByText(/只影响后续新建 Pod/)).toBeVisible();
  await page.getByRole('button', { name: '更新配置', exact: true }).click();
  await page.getByRole('textbox', { name: '容器镜像', exact: true }).fill('nginx:1.30');
  await confirm(page);
  await page.getByRole('button', { name: '查看工作负载状态', exact: true }).click();
  await page.getByRole('tab', { name: '容器配置', exact: true }).click();
  await expect(page.getByText('nginx:1.30', { exact: true })).toBeVisible();
});

test('workload shortcut preserves multiple Pod sessions across detail tabs', async ({ page }) => {
  await browse(page, 'StatefulSet');
  await page.getByRole('button', { name: '日志 cache', exact: true }).click();
  await expect(page.getByRole('tab', { name: '容器组', exact: true })).toHaveAttribute(
    'aria-selected',
    'true',
  );
  await expect(page.getByRole('dialog').locator('.resource-usage').first()).toHaveText(
    '250m·256 MiB',
  );
  await page.getByRole('button', { name: 'cache-0', exact: true }).click();
  await expect(page.getByRole('button', { name: '连接终端', exact: true })).toBeDisabled();
  await container(page);
  await page.getByRole('button', { name: '打开日志', exact: true }).click();
  await expect(page.getByLabel('容器日志', { exact: true })).toContainText('服务已启动');
  await page.getByRole('tab', { name: '工作负载状态', exact: true }).click();
  await page.getByRole('tab', { name: '容器组', exact: true }).click();
  await expect(page.getByLabel('容器日志', { exact: true })).toContainText('服务已启动');
  await page.getByRole('button', { name: 'cache-1', exact: true }).click();
  await container(page, 'sidecar');
  await page.getByRole('button', { name: '连接终端', exact: true }).click();
  await expect(page.getByText('权限不足：当前身份无权连接容器终端', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '打开日志', exact: true }).click();
  await expect(page.getByLabel('容器日志', { exact: true }).last()).toContainText('sidecar ready');
  await page.getByRole('tab', { name: 'platform / cache-0', exact: true }).click();
  await expect(page.getByLabel('容器日志', { exact: true }).first()).toContainText('app ready');
  await page.screenshot({ path: 'test-results/workload-pod-sessions.png', fullPage: true });
  await page.getByRole('button', { name: '关闭', exact: true }).click();
  await expect(page.getByLabel('容器日志', { exact: true })).toHaveCount(0);
});

test('workload empty Pods and denied query remain distinct', async ({ page }) => {
  await browse(page, 'StatefulSet');
  await page.getByRole('button', { name: '日志 empty-cache', exact: true }).click();
  await expect(page.getByText('当前没有关联 Pod', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '关闭', exact: true }).click();
  await page.getByRole('button', { name: '终端 restricted-cache', exact: true }).click();
  await expect(
    page.getByText('Pod 查询失败：权限不足：无法列出 Pod', { exact: true }),
  ).toBeVisible();
  await expect(page.getByText('暂时不能判断是否存在 Pod', { exact: true })).toBeVisible();
});

test('Deployment and shared relation rows expose Pod operations in context', async ({ page }) => {
  await browse(page, 'Deployment');
  await page.getByRole('button', { name: '日志 api-gateway', exact: true }).click();
  await expect(page.getByRole('dialog').locator('.resource-usage').first()).toHaveText(
    '250m·256 MiB',
  );
  await expect(page.getByRole('combobox', { name: 'Pod 容器', exact: true })).toBeVisible();
  await container(page);
  await page.getByRole('button', { name: '连接终端', exact: true }).click();
  await expect(page.locator('.xterm')).toBeVisible();
  await page.getByRole('tab', { name: '发布状态', exact: true }).click();
  await page.getByRole('tab', { name: '容器组', exact: true }).click();
  await expect(page.locator('.xterm')).toBeVisible();
  await page.getByRole('tab', { name: '关联资源与事件', exact: true }).click();
  await page
    .getByRole('row')
    .filter({ hasText: 'api-gateway-pending' })
    .getByRole('button', { name: '日志', exact: true })
    .click();
  await expect(
    page.getByText('Pod 操作 · production-shanghai · api-gateway', { exact: true }),
  ).toBeVisible();
  await container(page);
  await page.getByRole('button', { name: '打开日志', exact: true }).click();
  await expect(page.getByLabel('容器日志', { exact: true })).toContainText('服务已启动');
});

test('Pod replacement closes old sessions and requires explicit new selection', async ({
  page,
}) => {
  await browse(page, 'StatefulSet');
  await page.getByRole('button', { name: '终端 cache', exact: true }).click();
  await page.getByRole('button', { name: 'cache-0', exact: true }).click();
  await container(page);
  await page.getByRole('button', { name: '连接终端', exact: true }).click();
  await expect(page.locator('.xterm')).toBeVisible();
  await page.evaluate(async () => {
    const module = await import('../src/test/sessionFixtures.ts');
    module.replaceFixturePod('cache-0');
  });
  await page.getByRole('button', { name: '刷新容器状态', exact: true }).click();
  await expect(
    page.getByText('Pod UID 已变化，原 Pod 已被替换。旧会话已结束，请返回列表刷新后重新选择。', {
      exact: true,
    }),
  ).toBeVisible();
  await expect(page.locator('.xterm')).toHaveCount(0);
  await expect
    .poll(() =>
      page.evaluate(async () => {
        const module = await import('../src/test/sessionFixtures.ts');
        return module.fixtureSessionCount();
      }),
    )
    .toBe(0);
  await page.getByRole('button', { name: '刷新容器组', exact: true }).click();
  await page.getByRole('button', { name: 'cache-0', exact: true }).click();
  await expect(page.getByRole('button', { name: '连接终端', exact: true })).toBeDisabled();
  await expect(page.locator('.xterm')).toHaveCount(0);
});

test('Node Pod rows provide direct terminal entry', async ({ page }) => {
  await browse(page, 'Node');
  await page.getByRole('button', { name: 'worker-01', exact: true }).click();
  await page.getByRole('tab', { name: '节点上的 Pod', exact: true }).click();
  await expect(page.getByRole('dialog').locator('.resource-usage').first()).toHaveText(
    '1.25 核·1.8 GiB',
  );
  await page
    .getByRole('row')
    .filter({ hasText: 'api-gateway-6f8d7c9b4-x2k5m' })
    .getByRole('button', { name: '终端', exact: true })
    .click();
  await expect(page.getByRole('tab', { name: '日志与终端', exact: true })).toHaveAttribute(
    'aria-selected',
    'true',
  );
  await container(page);
  await page.getByRole('button', { name: '连接终端', exact: true }).click();
  await expect(page.locator('.xterm')).toBeVisible();
});

test('DaemonSet creation uses node strategy fields and has no replicas', async ({ page }) => {
  await browse(page, 'DaemonSet');
  await page.getByRole('button', { name: '创建 DaemonSet', exact: true }).click();
  await expect(page.getByRole('spinbutton', { name: '期望副本数', exact: true })).toHaveCount(0);
  await page.getByRole('textbox', { name: 'DaemonSet 名称', exact: true }).fill('new-agent');
  await page.getByRole('textbox', { name: '容器镜像', exact: true }).fill('agent:v2');
  await page.getByRole('textbox', { name: '最大不可用', exact: true }).fill('20%');
  await page.getByRole('button', { name: '添加节点标签', exact: true }).click();
  await page.getByRole('textbox', { name: '节点标签键 1', exact: true }).fill('kubernetes.io/os');
  await page.getByRole('textbox', { name: '节点标签值 1', exact: true }).fill('linux');
  await confirm(page);
  await page.getByRole('button', { name: '查看工作负载状态', exact: true }).click();
  await expect(page.getByText('DaemonSet · new-agent', { exact: true })).toBeVisible();
  await expect(page.getByText(/maxUnavailable = 20%/)).toBeVisible();
});

test('workload Pod rows open UID checked events, YAML and port forwarding', async ({ page }) => {
  await browse(page, 'ReplicaSet');
  await page.getByRole('button', { name: '日志 standalone-rs', exact: true }).click();
  const row = page.getByRole('row').filter({ hasText: 'standalone-rs-0' });
  await row.getByRole('button', { name: '事件', exact: true }).click();
  await expect(page.getByText('BackOff', { exact: true })).toBeVisible();
  await page.getByRole('dialog').last().getByRole('button', { name: '关闭', exact: true }).click();
  await row.getByRole('button', { name: 'YAML', exact: true }).click();
  await expect(page.locator('.monaco-editor')).toBeVisible();
  await page.getByRole('dialog').last().getByRole('button', { name: '关闭', exact: true }).click();
  await expect(row.getByRole('button', { name: /^(日志|终端|转发)$/ })).toHaveCount(0);
  await expect(row.getByRole('button')).toHaveCount(3);
  await row.getByRole('button', { name: 'standalone-rs-0', exact: true }).click();
  await page.getByRole('button', { name: '开始转发', exact: true }).click();
  await expect(page.getByRole('heading', { name: /127.0.0.1:49152/ })).toBeVisible();
});
