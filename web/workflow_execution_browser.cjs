// Real browser against the platform API and an actual MCP HTTP service supplied by Go.
const fs=require('node:fs');
const assert=require('node:assert/strict');
const {chromium}=require('playwright');
(async()=>{
 const fixture=JSON.parse(fs.readFileSync(0,'utf8'));
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage();page.setDefaultTimeout(3000);page.setDefaultNavigationTimeout(5000);
  page.on('pageerror',error=>console.error('PAGE_ERROR',error.message));
  console.log('phase: login');
  await page.goto(fixture.url+'/workflows/'+fixture.workflow);
  await page.getByLabel('密码',{exact:true}).fill('password');
  await page.getByRole('button',{name:'登录',exact:true}).click();
  console.log('phase: editor');await page.locator('[data-node="call"] .wf-node-body').click();
  assert.deepEqual(await page.locator('[data-kind]').evaluateAll(items=>items.map(el=>el.dataset.kind)),['agent','execution','approval','end']);
  await page.getByLabel('工具参数（JSON）').fill('{"text":"browser literal $(no shell)"}');
  await page.getByLabel('执行超时（秒）').fill('90');
  console.log('phase: save');const saved=page.waitForResponse(r=>r.url().endsWith('/api/workflows/'+fixture.workflow)&&r.request().method()==='PUT');
  await page.locator('#wf-save').click();
  assert.equal((await saved).status(),200);
  await page.reload();await page.locator('[data-node="call"] .wf-node-body').click();
  assert.equal(JSON.parse(await page.getByLabel('工具参数（JSON）').inputValue()).text,'browser literal $(no shell)');
  assert.equal(await page.getByLabel('执行超时（秒）').inputValue(),'90');
  console.log('phase: start');await page.locator('#wf-start').click();
  await page.getByLabel('你要完成什么？').fill('Harness execution browser check');
  await page.getByLabel('工作区目录').fill(fixture.workspace);
  await page.getByRole('button',{name:'开始运行',exact:true}).click();
  await page.waitForURL('**/workflow-runs/*');
  await page.locator('#wf-run-header .status-completed').waitFor({timeout:10000});
  await page.locator('[data-step-details="1"]').click();
  assert.match(await page.locator('#wf-step-details').textContent(),/browser literal \$\(no shell\)/);
  assert.match(await page.locator('#wf-step-details').textContent(),/native_response/);
  await page.setViewportSize({width:390,height:844});
  await page.screenshot({path:fixture.screenshot,fullPage:true,animations:'disabled'});
  console.log('EXECUTION_BROWSER_PASS: save/reload, API dispatch, original result, 390px screenshot');
 }finally{await browser.close()}
})().catch(error=>{console.error(error);process.exitCode=1});
