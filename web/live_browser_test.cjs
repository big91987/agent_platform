// Real HTTP/1.1 sockets exercise the per-origin connection limit, not a mocked EventSource.
const {test} = require('node:test');
const assert = require('node:assert/strict');
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const {chromium} = require('playwright');

async function fixture(t, status) {
  const id='a'.repeat(32), streams=new Set(), cursors=[], calls=[];
  const data={conversation:{id,agent_name:'Test Agent',status,created_at:new Date().toISOString(),user_id:'test'},messages:[],artifacts:[],approvals:[]};
  const events=[];
  const server=http.createServer((req,res)=>{
    const url=new URL(req.url,'http://localhost');calls.push(url.pathname);
    const json=value=>{res.writeHead(200,{'Content-Type':'application/json'});res.end(JSON.stringify(value));};
    if(url.pathname==='/api/me')return json({admin:false,user_id:'test'});
    if(url.pathname==='/api/agents')return json([]);
    if(url.pathname===`/api/conversations/${id}`)return json(data);
    if(url.pathname===`/api/conversations/${id}/events`){
      const after=Number(url.searchParams.get('after')||0);
      if(url.searchParams.get('format')==='json')return json(events.filter(e=>e.id>after).slice(0,300));
      res.writeHead(200,{'Content-Type':'text/event-stream'});res.write(': connected\n\n');
      streams.add(res);cursors.push(after);req.on('close',()=>streams.delete(res));return;
    }
    const name=url.pathname.startsWith('/conversations/')?'index.html':url.pathname.slice(1);
    if(!['index.html','app.js','style.css','request.js','markdown.js','transcript.js','favicon.svg'].includes(name)){res.writeHead(404);return res.end();}
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
