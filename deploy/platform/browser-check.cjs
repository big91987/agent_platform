// Read-only platform presentation check against a real installed instance.
// The operator passes an ephemeral login via stdin; no credential is printed.
const fs = require('node:fs');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
(async () => {
  const fixture = JSON.parse(fs.readFileSync(0, 'utf8'));
  const browser = await chromium.launch({ headless: true, args: ['--no-sandbox'] });
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
    page.setDefaultTimeout(10000);
    page.setDefaultNavigationTimeout(15000);
    const pageErrors = [];
    page.on('pageerror', error => pageErrors.push(error.message));
    await page.goto(fixture.url + '/workflows/' + fixture.workflow);
    await page.getByLabel('密码', { exact: true }).fill(fixture.password);
    fixture.password = '';
    await page.getByRole('button', { name: '登录', exact: true }).click();
    await page.locator('[data-node="python"] .wf-node-body').click();
    assert.deepEqual(await page.locator('[data-kind]').evaluateAll(items => items.map(el => el.dataset.kind)), ['agent', 'execution', 'approval', 'end']);
    assert.equal(await page.getByLabel('执行方式', { exact: true }).inputValue(), 'code');
    assert.equal(await page.getByLabel('语言', { exact: true }).inputValue(), 'python');
    assert.match(await page.getByLabel('代码', { exact: true }).inputValue(), /PLATFORM_PYTHON_PASS/);
    await page.locator('[data-node="typescript"] .wf-node-body').click();
    assert.equal(await page.getByLabel('语言', { exact: true }).inputValue(), 'typescript');
    await page.locator('[data-node="project"] .wf-node-body').click();
    assert.equal(await page.getByLabel('项目命令', { exact: true }).inputValue(), 'go run /workspace/main.go');
    const canvas = await page.screenshot({ fullPage: true, animations: 'disabled' });
    await page.goto(fixture.url + '/workflow-runs/' + fixture.run);
    await page.locator('#wf-run-header .status-completed').waitFor();
    await page.getByRole('heading', { name: '环境已释放', exact: true }).waitFor();
    assert.equal(await page.locator('#wf-resume-form, #wf-return-form').count(), 0);
    await page.locator('[data-step-details="4"]').click();
    assert.match(await page.locator('#wf-step-details').textContent(), /PLATFORM_PROJECT_COMMAND_PASS/);
    await page.locator('#wf-step-panel-close').click();
    await page.setViewportSize({ width: 390, height: 844 });
    const run = await page.screenshot({ fullPage: true, animations: 'disabled' });
    assert.deepEqual(pageErrors, []);
    const logout = await page.request.post(fixture.url + '/api/logout', { headers: { Origin: fixture.url }, data: {} });
    assert.equal(logout.status(), 200);
    console.log(JSON.stringify({ scope: 'platform read-only presentation; not product acceptance', canvas: canvas.toString('base64'), run: run.toString('base64'), four_node_types: true, persisted_execution_fields: true, released_run: true, original_output: true, page_errors: pageErrors, browser: browser.version() }));
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
