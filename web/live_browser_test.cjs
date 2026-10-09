// Real HTTP/1.1 sockets exercise the per-origin connection limit, not a mocked EventSource.
const {test} = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const {chromium} = require('playwright');

async function fixture(t, status, admin=false) {
  const id='a'.repeat(32), streams=new Set(), cursors=[], calls=[];
  const data={conversation:{id,agent_name:'Test Agent',status,created_at:new Date().toISOString(),user_id:'test'},messages:[],artifacts:[],approvals:[],execution_permissions:{network_access:true,allow_elevation:true,approvals_reviewer:'user'}};
  const events=[];
  const server=http.createServer((req,res)=>{
    const url=new URL(req.url,'http://localhost');calls.push(url.pathname);
    const json=value=>{res.writeHead(200,{'Content-Type':'application/json'});res.end(JSON.stringify(value));};
    if(url.pathname==='/api/me')return json({admin,user_id:'test'});
    if(url.pathname==='/api/agents')return json([]);
    if(url.pathname===`/api/conversations/${id}/execution-permissions` && req.method==='PATCH'){
      if(!admin || ['running','stopping','closed'].includes(data.conversation.status)){res.writeHead(admin?409:403,{'Content-Type':'application/json'});return res.end(JSON.stringify({error:'permission update refused'}));}
      let body='';req.on('data',chunk=>body+=chunk);req.on('end',()=>{data.execution_permissions=JSON.parse(body);json({ok:true});});return;
    }
    if(url.pathname===`/api/conversations/${id}/apply-agent-permissions` && req.method==='POST'){
      data.execution_permissions={network_access:false,allow_elevation:false,approvals_reviewer:'user'};return json({ok:true});
    }
    if(url.pathname===`/api/conversations/${id}`)return json(data);
    if(url.pathname===`/api/conversations/${id}/events`){
      const after=Number(url.searchParams.get('after')||0);
      if(url.searchParams.get('format')==='json')return json(events.filter(e=>e.id>after).slice(0,300));
      res.writeHead(200,{'Content-Type':'text/event-stream'});res.write(': connected\n\n');
      streams.add(res);cursors.push(after);req.on('close',()=>streams.delete(res));return;
    }
    const name=url.pathname.startsWith('/conversations/')?'index.html':url.pathname.slice(1);
    if(!['index.html','app.js','agent-config.js','style.css','request.js','markdown.js','transcript.js','favicon.svg'].includes(name)){res.writeHead(404);return res.end();}
    res.setHeader('Content-Type',name.endsWith('.js')?'text/javascript':name.endsWith('.css')?'text/css':'text/html');res.end(fs.readFileSync(path.join(__dirname,name)));
  });
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
  const browser=await chromium.launch({headless:true});
  t.after(async()=>{await browser.close();server.closeAllConnections();await new Promise(resolve=>server.close(resolve));});
  const context=await browser.newContext();
  const url=`http://127.0.0.1:${server.address().port}/conversations/${id}`;
  const until=async predicate=>{const end=Date.now()+3000;while(!predicate()){if(Date.now()>end)assert.fail('Timed out waiting for connection state');await new Promise(r=>setTimeout(r,25));}};
  async function open(){const page=await context.newPage();await page.goto(url,{timeout:2500});await page.waitForFunction(()=>typeof state!=='undefined' && state.current?.id,{},{timeout:2500});return page;}
  return {context,open,data,events,streams,cursors,calls,until};
}

test('eight idle conversation tabs leave connections available for Issue navigation',async t=>{
  const f=await fixture(t,'idle');
  for(let i=0;i<8;i++)await f.open();
  assert.equal(f.streams.size,0,'completed conversations must not hold permanent SSE sockets');
});

test('administrator configures existing node permissions through the page, preserves edits across refresh and reloads saved settings',async t=>{
  const f=await fixture(t,'stopped',true);
  f.data.workflow_run_id='b'.repeat(32);f.data.conversation.agent_id='';
  f.data.conversation.workspace_path='/fixture/workspace';
  const page=await f.open();
  await page.selectOption('#panel-choice','permissions');
  const mode=page.getByLabel('权限申请审批方式');
  await mode.selectOption('auto_review');await page.getByLabel('允许联网',{exact:true}).uncheck();
  await page.evaluate(()=>refreshConversation(state.id));
  assert.equal(await mode.inputValue(),'auto_review','live refresh must preserve unsaved permission choices');
  assert.equal(await page.getByLabel('允许联网',{exact:true}).isChecked(),false);
  await page.getByRole('button',{name:'保存权限',exact:true}).click();
  await f.until(()=>f.data.execution_permissions.approvals_reviewer==='auto_review');
  assert.equal(f.data.execution_permissions.network_access,false);
  assert.equal(f.data.conversation.workspace_path,'/fixture/workspace');
  await page.reload();await page.selectOption('#panel-choice','permissions');
  assert.equal(await page.getByLabel('权限申请审批方式').inputValue(),'auto_review');
  await page.getByRole('button',{name:'恢复节点原执行权限',exact:true}).click();
  await f.until(()=>f.data.execution_permissions.approvals_reviewer==='user');
  assert.equal(f.data.execution_permissions.allow_elevation,false);
});

test('permission editor is read-only for callers and locked while execution is active',async t=>{
  const ordinary=await fixture(t,'idle');let page=await ordinary.open();
  await page.selectOption('#panel-choice','permissions');
  assert.equal(await page.getByLabel('权限申请审批方式').isDisabled(),true);
  assert.equal(await page.getByRole('button',{name:'保存权限',exact:true}).count(),0);
  const running=await fixture(t,'running',true);page=await running.open();
  await page.selectOption('#panel-choice','permissions');
  assert.equal(await page.getByLabel('权限申请审批方式').isDisabled(),true);
  assert.equal(await page.getByRole('button',{name:'保存权限',exact:true}).isDisabled(),true);
  running.data.conversation.status='stopped';
  await page.evaluate(()=>refreshConversation(state.id));
  assert.equal(await page.getByLabel('权限申请审批方式').isDisabled(),false);
});

test('hidden conversation releases sockets and polling; resume catches up before reconnecting',async t=>{
  const f=await fixture(t,'running');
  f.data.messages.push({id:1,role:'user',kind:'input',status:'running',content:'Read the current directory'});
  const page=await f.open();await f.until(()=>f.streams.size===1);
  // Supply the browser visibility signal; all request/render/subscription code remains real.
  await page.evaluate(()=>{Object.defineProperty(document,'hidden',{configurable:true,get:()=>true});document.dispatchEvent(new Event('visibilitychange'));});
  await f.until(()=>f.streams.size===0);
  const count=f.calls.length;await new Promise(r=>setTimeout(r,2200));assert.equal(f.calls.length,count,'background pages must not poll');
  f.events.push({id:7,message_id:1,type:'item.completed',created_at:new Date().toISOString(),raw:{item:{id:'tool-one',type:'command_execution',command:'pwd',status:'completed',exitCode:0}}});
  await page.evaluate(()=>{Object.defineProperty(document,'hidden',{configurable:true,get:()=>false});document.dispatchEvent(new Event('visibilitychange'));});
  await f.until(()=>f.streams.size===1 && f.cursors.at(-1)===7);
  assert.equal(await page.locator('#timeline .tool-event').count(),1,'hidden interval tools must be restored once');
  const before=f.calls.filter(p=>p===new URL(page.url()).pathname.replace('/conversations/','/api/conversations/')).length;
  await page.evaluate(()=>Promise.all(Array.from({length:8},()=>refreshConversation(state.id))));
  const after=f.calls.filter(p=>p===new URL(page.url()).pathname.replace('/conversations/','/api/conversations/')).length;
  assert.equal(after-before,1,'concurrent refresh triggers share one request');
  assert.equal(await page.locator('#timeline .tool-event').count(),1,'catch-up must not duplicate tools');
  f.data.conversation.status='idle';f.data.messages[0].status='completed';
  for(const stream of f.streams)stream.write('id: 8\ndata: '+JSON.stringify({id:8,message_id:1,type:'platform.execution.completed'})+'\n\n');
  await f.until(()=>f.streams.size===0);
  f.data.conversation.status='queued';
  await f.until(()=>f.streams.size===1);
  await page.evaluate(()=>window.dispatchEvent(new PageTransitionEvent('pagehide')));
  await f.until(()=>f.streams.size===0);
});
