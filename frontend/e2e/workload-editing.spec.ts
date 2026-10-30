import { expect, test, type Page } from '@playwright/test';

async function openWorkload(page: Page, kind: string, name: string) {
  await page.goto('/');
  await page.getByRole('button', { name: '导入 kubeconfig', exact: true }).click();
  const cluster = page.locator('.cluster-row').filter({ hasText: 'production-shanghai' });
  await cluster.getByRole('button', { name: '连接', exact: true }).click();
  await cluster.getByRole('button', { name: '浏览资源', exact: true }).click();
  await page.getByRole('combobox', { name: '资源类型' }).click();
  await page
    .locator('.ant-select-item-option')
    .filter({ hasText: `· ${kind}` })
    .click();
  await page.getByRole('button', { name, exact: true }).click();
}

async function commit(page: Page, kind: string) {
  await page.getByRole('button', { name: '服务端预检并查看差异', exact: true }).click();
  await expect(page.getByText('服务端预检已通过，尚未写入集群')).toBeVisible();
  await page.getByRole('button', { name: '确认提交到集群', exact: true }).click();
  await page
    .getByRole('button', {
      name: kind === 'Deployment' ? '查看发布状态' : '查看工作负载状态',
      exact: true,
    })
    .click();
}

for (const [kind, name] of [
  ['Deployment', 'api-gateway'],
  ['StatefulSet', 'cache'],
  ['DaemonSet', 'node-agent'],
] as const) {
  test(`${kind} edits individual containers, ports and exact command arguments`, async ({
    page,
  }) => {
    const errors: string[] = [];
    page.on('pageerror', (error) => errors.push(error.message));
    await openWorkload(page, kind, name);
    const edit = page.getByRole('button', {
      name: kind === 'Deployment' ? '可视化编辑' : '更新配置',
      exact: true,
    });
    await edit.click();
    const app = page.getByRole('region', { name: '容器配置 app', exact: true });
    await expect(app.getByRole('button', { name: '移除此容器', exact: true })).toBeDisabled();
    await expect(app.getByRole('textbox', { name: '容器名称', exact: true })).toBeDisabled();
    await page.getByRole('button', { name: '添加容器', exact: true }).click();
    await page
      .getByRole('region', { name: '容器配置 app-2', exact: true })
      .getByRole('textbox', { name: '容器镜像', exact: true })
      .fill('busybox:remove');
    await page.getByRole('button', { name: '添加容器', exact: true }).click();
    const sidecar = page.getByRole('region', { name: '容器配置 app-3', exact: true });
    await sidecar.getByRole('textbox', { name: '容器镜像', exact: true }).fill('busybox:keep');
    await page
      .getByRole('region', { name: '容器配置 app-2', exact: true })
      .getByRole('button', { name: '移除此容器', exact: true })
      .click();
    await expect(sidecar.getByRole('textbox', { name: '容器镜像', exact: true })).toHaveValue(
      'busybox:keep',
    );

    await app.getByRole('button', { name: '添加命令项', exact: true }).click();
    await app.getByRole('textbox', { name: '命令项 1', exact: true }).fill('/bin/sh');
    await app.getByRole('button', { name: '添加命令项', exact: true }).click();
    await app.getByRole('textbox', { name: '命令项 2', exact: true }).fill('-c');
    await app.getByRole('button', { name: '添加参数', exact: true }).click();
    const script = 'printf "%s" "hello world"\nprintf "second line"';
    await app.getByRole('textbox', { name: '参数 1', exact: true }).fill(script);
    await app.getByRole('button', { name: '添加参数', exact: true }).click();
    await app.getByRole('tab', { name: '端口', exact: true }).click();
    await app.getByRole('button', { name: '添加端口', exact: true }).click();
    await app.getByRole('textbox', { name: '容器端口名称 1', exact: true }).fill('http');
    await app.getByRole('button', { name: '添加端口', exact: true }).click();
    await app.getByRole('textbox', { name: '容器端口名称 2', exact: true }).fill('metrics');
    await app.getByRole('spinbutton', { name: '容器端口 2', exact: true }).fill('9090');
    await app.getByRole('spinbutton', { name: '主机端口 2', exact: true }).fill('19090');
    await app.getByRole('textbox', { name: '主机 IP 2', exact: true }).fill('127.0.0.1');
    await app.getByRole('button', { name: '移除此端口', exact: true }).first().click();
    await expect(app.getByRole('textbox', { name: '容器端口名称 1', exact: true })).toHaveValue(
      'metrics',
    );
    if (kind === 'Deployment') {
      await app.getByRole('heading', { name: '容器 · app', exact: true }).scrollIntoViewIfNeeded();
      await page.screenshot({ path: 'test-results/workload-container-editor.png' });
      await app.locator('.container-port-row').scrollIntoViewIfNeeded();
      await page.screenshot({ path: 'test-results/workload-container-ports.png' });
    }
    await page.getByRole('button', { name: '服务端预检并查看差异', exact: true }).click();
    await expect(page.getByText('服务端预检已通过，尚未写入集群')).toBeVisible();
    await page.getByRole('button', { name: '返回修改', exact: true }).click();
    await app.getByRole('tab', { name: '基础配置', exact: true }).click();
    await expect(app.getByRole('textbox', { name: '参数 1', exact: true })).toHaveValue(script);
    await expect(app.getByRole('textbox', { name: '参数 2', exact: true })).toHaveValue('');
    await commit(page, kind);
    await page.getByRole('tab', { name: '容器配置', exact: true }).click();
    await expect(page.getByText(JSON.stringify(['/bin/sh', '-c']), { exact: true })).toBeVisible();
    await expect(page.getByText(JSON.stringify([script, '']), { exact: true })).toBeVisible();
    await expect(page.getByRole('cell', { name: '19090', exact: true })).toBeVisible();
    await expect(
      page
        .getByRole('tabpanel', { name: '容器配置', exact: true })
        .getByText('busybox:keep', { exact: true }),
    ).toBeVisible();
    await expect(page.getByText('busybox:remove', { exact: true })).toHaveCount(0);

    await edit.click();
    await expect(sidecar.getByRole('textbox', { name: '容器名称', exact: true })).toBeDisabled();
    await sidecar.getByRole('button', { name: '移除此容器', exact: true }).click();
    await expect(app.getByRole('button', { name: '移除此容器', exact: true })).toBeDisabled();
    for (let i = 0; i < 2; i++) {
      await app.getByRole('button', { name: '移除此命令项', exact: true }).first().click();
      await app.getByRole('button', { name: '移除此参数', exact: true }).first().click();
    }
    await app.getByRole('tab', { name: '端口', exact: true }).click();
    await app.getByRole('button', { name: '移除此端口', exact: true }).click();
    await commit(page, kind);
    await page.getByRole('tab', { name: '容器配置', exact: true }).click();
    await expect(page.getByText('使用镜像默认命令', { exact: true })).toBeVisible();
    await expect(page.getByText('使用镜像默认参数', { exact: true })).toBeVisible();
    await expect(page.getByText('未声明容器端口', { exact: true })).toBeVisible();
    expect(errors).toEqual([]);
  });
}
