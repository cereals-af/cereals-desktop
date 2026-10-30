import { expect, test, type Page } from '@playwright/test';

const failures = new WeakMap<Page, string[]>();
test.beforeEach(({ page }) => {
  const errors: string[] = [];
  failures.set(page, errors);
  page.on('pageerror', (error) => errors.push(error.message));
});
test.afterEach(({ page }) => {
  expect(failures.get(page)).toEqual([]);
});

async function openPod(page: Page) {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig', exact: true }).click();
  const row = page.locator('.cluster-row').filter({ hasText: 'production-shanghai' });
  await row.getByRole('button', { name: '连接', exact: true }).click();
  await row.getByRole('button', { name: '浏览资源', exact: true }).click();
  await page.getByRole('button', { name: 'api-gateway-6f8d7c9b4-x2k5m', exact: true }).click();
  await page.getByRole('tab', { name: '日志与终端', exact: true }).click();
  await expect(page.getByRole('button', { name: '打开日志', exact: true })).toBeEnabled();
}

test('reads previous logs, selects init containers and stops follow', async ({ page }) => {
  await openPod(page);
  await page.getByRole('button', { name: '打开日志', exact: true }).click();
  await expect(page.getByLabel('容器日志', { exact: true })).toContainText('服务已启动');
  await page.getByRole('button', { name: '停止会话', exact: true }).click();
  await expect(page.getByText('已停止', { exact: true })).toBeVisible();
  await page.getByRole('checkbox', { name: '上一次运行', exact: true }).check();
  await expect(page.getByRole('checkbox', { name: '持续接收新日志', exact: true })).toBeDisabled();
  await page.getByRole('button', { name: '打开日志', exact: true }).click();
  await expect(page.getByLabel('容器日志', { exact: true }).last()).toContainText(
    'previous: stopped before restart',
  );
  await expect(page.getByText('已完成', { exact: true })).toBeVisible();
  await page.getByRole('combobox', { name: 'Pod 容器', exact: true }).click();
  await page.getByTitle('init-config · Init · terminated · 重启 0 次').click();
  await expect(page.getByRole('button', { name: '连接终端', exact: true })).toBeDisabled();
  await page.screenshot({
    path: 'test-results/pod-logs.png',
    fullPage: true,
    animations: 'disabled',
  });
});

test('opens multiple terminals and preserves sessions across tabs', async ({ page }) => {
  await openPod(page);
  await page.getByRole('button', { name: '连接终端', exact: true }).click();
  const input = page.locator('.xterm-helper-textarea').first();
  await expect(input).toBeVisible();
  await input.pressSequentially('echo hello');
  await input.press('Enter');
  await page.getByRole('tab', { name: 'YAML', exact: true }).click();
  await page.getByRole('tab', { name: '日志与终端', exact: true }).click();
  await expect(input).toBeVisible();
  await page.getByRole('button', { name: '连接终端', exact: true }).click();
  await expect(page.locator('.xterm')).toHaveCount(2);
  await expect(page.getByText('运行中', { exact: true }).last()).toBeVisible();
  await page.screenshot({
    path: 'test-results/pod-terminal.png',
    fullPage: true,
    animations: 'disabled',
  });
  await page.getByRole('button', { name: '停止会话', exact: true }).last().click();
  await expect(page.getByText('已停止', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '关闭', exact: true }).first().click();
  await expect(page.locator('.xterm')).toHaveCount(0);
});

test('reports loopback port allocation and stops the session', async ({ page }) => {
  await openPod(page);
  await page.getByRole('button', { name: '开始转发', exact: true }).click();
  await expect(page.getByRole('heading', { name: /127.0.0.1:49152/ })).toBeVisible();
  await expect(page.getByText('运行中', { exact: true })).toBeVisible();
  await page.screenshot({
    path: 'test-results/pod-forward.png',
    fullPage: true,
    animations: 'disabled',
  });
  await page.getByRole('button', { name: '停止会话', exact: true }).click();
  await expect(page.getByText('已停止', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '关闭', exact: true }).first().click();
  await page.getByRole('button', { name: '断开集群', exact: true }).click();
  await expect(page.getByRole('heading', { name: '集群连接', exact: true })).toBeVisible();
});

test('keeps logs usable after terminal permission failure', async ({ page }) => {
  await openPod(page);
  await page.getByRole('combobox', { name: 'Pod 容器', exact: true }).click();
  await page.getByTitle('sidecar · 普通 · running · 重启 0 次').click();
  await page.getByRole('button', { name: '连接终端', exact: true }).click();
  await expect(page.getByText('权限不足：当前身份无权连接容器终端', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '打开日志', exact: true }).click();
  await expect(page.getByLabel('容器日志', { exact: true })).toContainText('sidecar ready');
});

test('following logs stay at the latest line, including after switching tabs', async ({ page }) => {
  await openPod(page);
  await page.getByRole('spinbutton', { name: '日志末尾行数' }).fill('1000');
  await page.getByRole('button', { name: '打开日志', exact: true }).click();
  const logs = page.getByLabel('容器日志', { exact: true });
  await expect(logs).toContainText('follow line');
  const bottomGap = () => logs.evaluate((el) => el.scrollHeight - el.scrollTop - el.clientHeight);
  await expect.poll(bottomGap).toBeLessThan(2);
  await logs.evaluate((el) => {
    el.scrollTop = 0;
  });
  await expect(page.getByRole('button', { name: '恢复自动滚动', exact: true })).toBeVisible();
  await expect.poll(() => logs.evaluate((el) => el.scrollTop)).toBe(0);
  await page.getByRole('button', { name: '恢复自动滚动', exact: true }).click();
  await expect.poll(bottomGap).toBeLessThan(2);
  await page.getByRole('tab', { name: 'YAML', exact: true }).click();
  await page.getByRole('tab', { name: '日志与终端', exact: true }).click();
  await expect.poll(bottomGap).toBeLessThan(2);
  await logs.scrollIntoViewIfNeeded();
  await page.screenshot({
    path: 'test-results/log-follow.png',
    fullPage: true,
    animations: 'disabled',
  });
});

test('logs without follow preserve manual scroll position', async ({ page }) => {
  await openPod(page);
  await page.getByRole('checkbox', { name: '持续接收新日志', exact: true }).uncheck();
  await page.getByRole('spinbutton', { name: '日志末尾行数' }).fill('1000');
  await page.getByRole('button', { name: '打开日志', exact: true }).click();
  const logs = page.getByLabel('容器日志', { exact: true });
  await expect(logs).toContainText('history line 159');
  expect(await logs.evaluate((el) => el.scrollTop)).toBe(0);
  await logs.evaluate((el) => {
    el.scrollTop = 90;
  });
  await page.getByRole('tab', { name: 'YAML', exact: true }).click();
  await page.getByRole('tab', { name: '日志与终端', exact: true }).click();
  await expect.poll(() => logs.evaluate((el) => el.scrollTop)).toBe(90);
});

test('pauses only rendering and exports all received logs after stopping', async ({ page }) => {
  await openPod(page);
  await page.getByRole('spinbutton', { name: '日志末尾行数' }).fill('1000');
  await page.getByRole('button', { name: '打开日志', exact: true }).click();
  const logs = page.getByLabel('容器日志', { exact: true });
  await expect(logs).toContainText('follow line');
  await page.getByRole('button', { name: '暂停自动滚动', exact: true }).click();
  const frozen = await logs.textContent();
  await expect(page.getByText('已暂停滚动 · 有新日志', { exact: true })).toBeVisible();
  expect(await logs.textContent()).toBe(frozen);
  await page.evaluate(async () => {
    const module = await import('../src/test/sessionFixtures.ts');
    module.appendFixtureLogs(
      Array.from({ length: 6000 }, (_, i) => `bulk line ${i} ${'x'.repeat(100)}\n`).join(''),
    );
  });
  await page.getByRole('button', { name: '下载本次会话日志', exact: true }).click();
  await expect(page.getByText(/已保存 app.log/)).toBeVisible();
  const saved = await page.evaluate(async () => {
    const module = await import('../src/test/sessionFixtures.ts');
    return module.fixtureLogExports().at(-1)?.text ?? '';
  });
  expect(saved).toContain('服务已启动');
  expect(saved).toContain('bulk line 5999');
  await page.getByRole('button', { name: '恢复自动滚动', exact: true }).click();
  await expect(logs).toContainText('bulk line 5999');
  await expect(logs).not.toContainText('服务已启动');
  await page.getByRole('button', { name: '停止会话', exact: true }).click();
  await expect(page.getByText('已停止', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: '下载本次会话日志', exact: true })).toBeEnabled();
  await page.getByRole('button', { name: '下载本次会话日志', exact: true }).click();
  await page.screenshot({ path: 'test-results/session-log-controls.png', fullPage: true });
});

test('uploads with explicit overwrite while terminal remains interactive', async ({ page }) => {
  await openPod(page);
  await page.getByRole('button', { name: '连接终端', exact: true }).click();
  await expect(page.locator('.xterm-helper-textarea')).toBeVisible();
  await page.getByRole('button', { name: '上传文件', exact: true }).click();
  await page.getByRole('button', { name: '选择文件', exact: true }).click();
  await page.getByRole('textbox', { name: '容器目标目录', exact: true }).fill('/etc');
  await page.getByRole('button', { name: '开始上传', exact: true }).click();
  await expect(
    page.getByText('容器中已存在同名文件，请确认覆盖后重试', { exact: true }),
  ).toBeVisible();
  await page.getByRole('button', { name: '确认覆盖并上传', exact: true }).click();
  await expect(page.getByRole('button', { name: '取消传输', exact: true })).toBeVisible();
  await page.locator('.xterm-helper-textarea').pressSequentially('echo independent');
  await page.locator('.xterm-helper-textarea').press('Enter');
  await expect(page.getByText('文件传输完成', { exact: true })).toBeVisible();
  await page.screenshot({ path: 'test-results/session-file-upload.png', fullPage: true });
  await page.getByRole('button', { name: '关闭文件传输', exact: true }).click();
  await expect
    .poll(() =>
      page.evaluate(async () => {
        const module = await import('../src/test/transferFixtures.ts');
        return module.fixtureTransferCount();
      }),
    )
    .toBe(0);
  await expect(page.locator('.xterm')).toBeVisible();
});

test('downloads are cancellable and failures do not interrupt another terminal', async ({
  page,
}) => {
  await openPod(page);
  await page.getByRole('button', { name: '连接终端', exact: true }).click();
  await page.getByRole('button', { name: '连接终端', exact: true }).click();
  const active = page.locator('.pod-session:visible');
  await active.getByRole('button', { name: '下载文件', exact: true }).click();
  await active
    .getByRole('textbox', { name: '容器文件绝对路径', exact: true })
    .fill('/tmp/support.log');
  await active.getByRole('button', { name: '选择保存位置并下载', exact: true }).click();
  await active.getByRole('button', { name: '取消传输', exact: true }).click();
  await expect(active.getByText('传输已取消', { exact: true })).toBeVisible();
  await active
    .getByRole('textbox', { name: '容器文件绝对路径', exact: true })
    .fill('/tmp/denied.log');
  await active.getByRole('button', { name: '选择保存位置并下载', exact: true }).click();
  await expect(
    active.getByText('权限不足：当前身份无权访问 pods/exec', { exact: true }),
  ).toBeVisible();
  await expect(page.locator('.xterm')).toHaveCount(2);
  await page
    .getByRole('tab', { name: /终端 · api-gateway/ })
    .first()
    .click();
  await expect(
    page.locator('.pod-session:visible').getByRole('button', { name: '上传文件', exact: true }),
  ).toBeEnabled();
  await page.getByRole('button', { name: '关闭', exact: true }).first().click();
  await expect
    .poll(() =>
      page.evaluate(async () => {
        const module = await import('../src/test/transferFixtures.ts');
        return module.fixtureTransferCount();
      }),
    )
    .toBe(0);
});
