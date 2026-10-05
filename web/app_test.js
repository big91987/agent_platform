const {test} = require('node:test');
const assert = require('node:assert/strict');
const {readFileSync} = require('node:fs');
const vm = require('node:vm');

test('fixed Issue links retain the destination through login and open the original conversation', async () => {
  for (const authenticated of [true, false]) {
    let authenticatedAfterLogin = authenticated;
    const id = 'a'.repeat(32);
    const location = {pathname: '/conversations/' + id, hash: '', search: ''};
    const context = vm.createContext({
      URLSearchParams, location, clearInterval, clearTimeout, setTimeout,
      window: {addEventListener() {}},
      document: {addEventListener() {}, querySelector() {}},
      history: {replaceState(_state, _title, path) {location.pathname = path; location.hash = ''; }},
      fetch: async path => {
        if (path === '/api/me') return {ok: authenticatedAfterLogin, status: authenticatedAfterLogin ? 200 : 401, json: async () => authenticatedAfterLogin ? {admin: true} : {error: '请登录'}};
        if (path === '/api/agents') return {ok: true, json: async () => []};
        throw new Error('Unexpected API: ' + path);
      },
    });
    const source = readFileSync(__dirname + '/app.js', 'utf8').replace(/\nroute\(\)\.catch\(showRouteError\);\s*$/, '');
    vm.runInContext(source, context);
    vm.runInContext(`
      let opened = null, loginNote = null;
      login = note => { loginNote = note || ''; };
      shell = () => {};
      toast = () => {};
      conversationView = async id => { opened = id; };
    `, context);
    await vm.runInContext('route()', context);
    assert.equal(location.hash, '');
    if (authenticated) {
      assert.equal(vm.runInContext('opened', context), id, 'valid login must open the original conversation');
      assert.equal(vm.runInContext('loginNote', context), null);
    } else {
      assert.equal(vm.runInContext('opened', context), null, 'expired grant must not authorize anonymous access');
      assert.equal(vm.runInContext('loginNote', context), '');
      assert.equal(location.pathname, '/conversations/' + id);
      authenticatedAfterLogin = true;
      await vm.runInContext('route()', context);
      assert.equal(vm.runInContext('opened', context), id);
    }
  }
});

function conversationPage(data) {
  const nodes = new Map();
  const node = selector => {
    if (!nodes.has(selector)) nodes.set(selector, {
      innerHTML: '', textContent: '', scrollHeight: 100, scrollTop: 0,
      clientHeight: 100, querySelectorAll: () => [],
    });
    return nodes.get(selector);
  };
  const context = vm.createContext({
    URLSearchParams, location: {}, clearInterval, clearTimeout, setTimeout,
    window: {addEventListener() {}}, document: {addEventListener() {}, querySelector: node},
  });
  vm.runInContext(readFileSync(__dirname + '/markdown.js', 'utf8'), context);
  vm.runInContext(readFileSync(__dirname + '/transcript.js', 'utf8'), context);
  vm.runInContext(readFileSync(__dirname + '/app.js', 'utf8').replace(/\nroute\(\)\.catch\(showRouteError\);\s*$/, ''), context);
  context.snapshot = data;
  vm.runInContext("api = async path => path.includes('/events?') ? [] : snapshot; state.me = {admin:false}; state.id = 'c1'; state.events = [];", context);
  return {context, node};
}

test('conversation renders native tools in the timeline without inferring business state', async () => {
  const data = {
    conversation: {id:'c1', agent_name:'助手', status:'running'}, artifacts:[],
    messages:[{id:7,role:'user',kind:'input',status:'running',content:'请处理'},
      {id:8,parent_id:7,role:'agent',kind:'progress',status:'completed',content:'正在核对需求'}],
  };
  const {context,node} = conversationPage(data);
  const progress=data.messages.pop();
  await vm.runInContext("refreshConversation('c1')", context);
  assert.match(node('#timeline').innerHTML,/avatar agent-active/,'show activity before the first native output');
  assert.match(node('#timeline').innerHTML,/正在处理/);
  data.messages.push(progress);
  vm.runInContext(`state.events = [{id:12,message_id:7,type:'item.started',created_at:new Date().toISOString(),raw:{item:{id:'item_1',type:'command_execution',command:'cat docs/prd.md',status:'in_progress'}}}];`, context);
  await vm.runInContext("refreshConversation('c1')", context);
  assert.match(node('#execution-status').innerHTML, /执行中/);
  assert.match(node('#timeline').innerHTML, /cat docs\/prd.md/, 'native command must be visible in the conversation');
  assert.doesNotMatch(node('#execution-status').innerHTML, /cat docs\/prd.md|正在执行命令/);
  vm.runInContext(`receiveConversationEvent({id:13,message_id:7,type:'item.completed',raw:{item:{id:'file',type:'file_change',changes:[{path:'notes.md'}]}}})`, context);
  assert.doesNotMatch(node('#execution-status').innerHTML, /文件已更新|正在更新文件/);
  await vm.runInContext("refreshConversation('c1')", context);
  assert.match(node('#timeline').innerHTML, /文件变更/);
  assert.match(node('#timeline').innerHTML, /class="message agent agent-turn"[\s\S]*<strong>助手<\/strong>[\s\S]*class="tool-event/, 'Agent identity must precede and contain tool calls');
  assert.doesNotMatch(node('#timeline').innerHTML, /notes.md/, 'completed tool details default to collapsed');
  assert.doesNotMatch(node('#timeline').innerHTML, /class="agent-process"/);
  assert.match(node('#execution-status').innerHTML, /引导/);
  assert.match(node('#timeline').innerHTML, /正在核对需求/);
  assert.doesNotMatch(node('#timeline').innerHTML, /awaiting-output/, 'real output replaces the waiting placeholder');
  assert.equal((node('#timeline').innerHTML.match(/<strong>助手<\/strong>/g)||[]).length,1,'one Agent name per turn, including progress, tools and final reply');

  const disclosure={open:true,dataset:{disclosure:'tool:7:item_1'}};
  node('#timeline').querySelectorAll=()=>[disclosure];
  vm.runInContext(`receiveConversationEvent({id:14,message_id:7,type:'item.completed',raw:{item:{id:'item_1',type:'command_execution',aggregatedOutput:'full-command-result',exitCode:0}}})`,context);
  await vm.runInContext("refreshConversation('c1')",context);
  assert.equal(disclosure.open,true,'manual expansion survives tool completion');
  assert.match(node('#timeline').innerHTML,/full-command-result/);

  data.conversation.status = 'idle';
  data.messages[0].status = 'completed';
  data.messages.push({id:9,parent_id:7,role:'agent',kind:'reply',status:'completed',content:JSON.stringify({status:'needs_input',message:'请选择笔记数量',artifacts:[]})});
  await vm.runInContext("refreshConversation('c1')", context);
  assert.match(node('#chat-status').innerHTML, /本轮已结束/);
  assert.match(node('#execution-status').innerHTML, /本轮已结束/);
  assert.doesNotMatch(node('#timeline').innerHTML, /avatar agent-active/, 'finished turn stops avatar activity');
  assert.match(node('#execution-status').innerHTML, /^<summary/, 'status is a compact disclosure in the composer');
  assert.doesNotMatch(node('#execution-status').innerHTML, /平台状态：/);
  assert.doesNotMatch(node('#execution-status').innerHTML, /正在执行命令/);
  assert.match(node('#timeline').innerHTML, /请选择笔记数量/);
  assert.match(node('#timeline').innerHTML, /needs_input/);
  assert.match(node('#timeline').innerHTML, /cat docs\/prd.md/);
  assert.match(node('#timeline').innerHTML, /正在核对需求/);
  assert.doesNotMatch(node('#timeline').innerHTML, /awaiting-output/, 'real output replaces the waiting placeholder');
  assert.equal((node('#timeline').innerHTML.match(/<strong>助手<\/strong>/g)||[]).length,1,'one Agent name per turn, including progress, tools and final reply');
  assert.doesNotMatch(node('#timeline').innerHTML, /notes.md/);
  assert.doesNotMatch(node('#timeline').innerHTML, /class="agent-process"/);
  assert.doesNotMatch(node('#execution-status').innerHTML, /等待你回复/);
  assert.match(data.messages.at(-1).content, /needs_input/, 'presentation must retain stored raw text');

  data.conversation.status = 'failed';
  data.conversation.error = '原生执行退出';
  await vm.runInContext("refreshConversation('c1')", context);
  assert.match(node('#execution-status').innerHTML, /执行失败/);
  assert.doesNotMatch(node('#execution-status').innerHTML, /等待你回复/);
});

test('pending tool approval is visible and decisions preserve the conversation', async () => {
 const data={conversation:{id:'c1',agent_name:'助手',status:'running'},messages:[],artifacts:[],approvals:[{id:'approval-1',decision:'',request:{serverName:'external',message:'Save <record>?'}}]};
 const {context,node}=conversationPage(data);
 let buttons=[];
 node('#tool-approvals').querySelectorAll=()=>buttons;
 const accept={dataset:{approval:'approval-1',decision:'accept'}},decline={dataset:{approval:'approval-1',decision:'decline'}};
 buttons=[accept,decline];
 await vm.runInContext("refreshConversation('c1')",context);
 assert.match(node('#tool-approvals').innerHTML,/Save &lt;record&gt;\?/);
 assert.match(node('#execution-status').innerHTML,/等待你确认工具调用/);
 assert.match(node('#chat-status').innerHTML,/等待工具审批/);
 context.calls=[];
 vm.runInContext("api = async (path,method,body) => {if(method==='POST'){calls.push({path,body});snapshot.approvals[0].decision=body.decision;return {ok:true}}return path.includes('/events?')?[]:snapshot}",context);
 await accept.onclick();
 assert.equal(context.calls[0].path,'/api/conversations/c1/approvals/approval-1');
 assert.equal(context.calls[0].body.decision,'accept');
 assert.equal(node('#tool-approvals').innerHTML,'');
 assert.equal(data.conversation.id,'c1');
});


test('queued input offers steering only while a turn is running', async () => {
  const data = {conversation:{id:'c1',agent_name:'助手',status:'running'},artifacts:[],messages:[
    {id:1,role:'user',kind:'input',status:'running',content:'原任务'},
    {id:2,role:'user',kind:'input',status:'queued',content:'补充要求'}]};
  const {context,node}=conversationPage(data);
  await vm.runInContext("refreshConversation('c1')",context);
  assert.match(node('#timeline').innerHTML,/data-steer="2"/);
  assert.doesNotMatch(node('#timeline').innerHTML,/data-steer="1"/);
  data.messages[1].status='steered';
  await vm.runInContext("refreshConversation('c1')",context);
  assert.match(node('#timeline').innerHTML,/已引导/);
  assert.doesNotMatch(node('#timeline').innerHTML,/data-steer/);
  data.messages[1].status='queued';data.conversation.status='idle';
  await vm.runInContext("refreshConversation('c1')",context);
  assert.doesNotMatch(node('#timeline').innerHTML,/data-steer/);
});

test('refresh restores paginated tool and thinking history even without SSE', async () => {
 const data={conversation:{id:'c1',agent_name:'助手',status:'idle'},artifacts:[],messages:[{id:7,role:'user',kind:'input',status:'completed',content:'处理',created_at:'2026-10-03T00:00:00Z'},{id:8,parent_id:7,role:'agent',kind:'reply',content:'完成',created_at:'2026-10-03T00:00:10Z'}]};
 const {context,node}=conversationPage(data);
 const events=Array.from({length:300},(_,i)=>({id:i+1,message_id:7,type:'native.notification',created_at:'2026-10-03T00:00:01Z',raw:{}}));
 events.push({id:301,message_id:7,type:'item.completed',created_at:'2026-10-03T00:00:02Z',raw:{item:{id:'tool',type:'command_execution',command:'pwd',status:'completed',exitCode:0}}});
 events.push({id:302,message_id:7,type:'item.completed',created_at:'2026-10-03T00:00:03Z',raw:{item:{id:'thought',type:'reasoning',summary:['Public summary']}}});
 context.eventsFixture=events; context.calls=[];
 vm.runInContext(`api=async path=>{calls.push(path);if(path.includes('/events?')){const after=Number(path.split('after=')[1]);return eventsFixture.filter(e=>e.id>after).slice(0,300)}return snapshot};state.lastEventID=0;receiveConversationEvent(eventsFixture[301]);`,context);
 await vm.runInContext("refreshConversation('c1')",context);
 assert.match(node('#timeline').innerHTML,/tool-event/,'polling restores tools when stream is absent');
 assert.match(node('#timeline').innerHTML,/Thinking/);
 assert.equal(vm.runInContext('state.events.length',context),302,'SSE arriving ahead of history must not discard or duplicate older events');
 assert.ok(node('#timeline').innerHTML.indexOf('tool-event')<node('#timeline').innerHTML.indexOf('Thinking'));
 assert.doesNotMatch(node('#timeline').innerHTML,/Public summary/,'thinking remains collapsed');
 await vm.runInContext("refreshConversation('c1')",context);
 assert.equal(vm.runInContext('state.events.length',context),302);
 assert.ok(context.calls.some(p=>p.endsWith('after=302')),'incremental refresh uses persisted-history cursor');
});

test('native elevation shows requested permissions and only enables administrators',async()=>{
 const data={conversation:{id:'c1',agent_name:'助手',status:'running'},messages:[],artifacts:[],approvals:[{id:'p',decision:'',request:{platform_permission_request:true,method:'item/permissions/requestApproval',reason:'Read public sources',permissions:{network:{enabled:true}}}}]};
 const {context,node}=conversationPage(data);
 await vm.runInContext("loadConversation('c1')",context);
 assert.match(node('#tool-approvals').innerHTML,/等待管理员批准提权/);
 assert.match(node('#tool-approvals').innerHTML,/network/);
 assert.match(node('#tool-approvals').innerHTML,/data-decision="accept" disabled/);
 vm.runInContext('state.me.admin=true',context);
 await vm.runInContext("loadConversation('c1')",context);
 assert.doesNotMatch(node('#tool-approvals').innerHTML,/data-decision="accept" disabled/);
 assert.equal(node('#apply-agent-permissions').disabled,true);
 data.conversation.status='idle';
 await vm.runInContext("loadConversation('c1')",context);
 assert.equal(node('#apply-agent-permissions').disabled,false);
});

test('tool catalog leads with currently assigned shared connection and folds unassigned history',async()=>{
 const content={innerHTML:''};
 const context=vm.createContext({URLSearchParams,location:{},clearInterval,clearTimeout,setTimeout,window:{addEventListener(){}},document:{addEventListener(){},querySelector:()=>content,querySelectorAll:()=>[]}});
 vm.runInContext(readFileSync(__dirname+'/app.js','utf8').replace(/\nroute\(\)\.catch\(showRouteError\);\s*$/, ''),context);
 vm.runInContext(`state.agents=[{tool_servers:[{server_id:'browser-validation'}]}];heading=()=>'<h1>外部工具</h1>';api=async()=>[
 {id:'browser-validation',name:'浏览器验证',enabled:true,connection:{}},
 {id:'old-a',name:'旧项目 A 浏览器',enabled:false,connection:{}},
 {id:'old-b',name:'旧项目 B 浏览器',enabled:true,connection:{}}
 ];`,context);
 await vm.runInContext('toolsView()',context);
 assert.match(content.innerHTML,/<h2>浏览器验证<\/h2>/);
 assert.match(content.innerHTML,/<details[^>]*>\s*<summary>未分配给当前 Agent 的连接（2）<\/summary>/);
 assert.match(content.innerHTML,/<summary>未分配给当前 Agent 的连接（2）<\/summary>[\s\S]*旧项目 A 浏览器[\s\S]*旧项目 B 浏览器/);
});
