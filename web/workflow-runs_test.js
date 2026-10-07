const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');

test('owned run history opens after workflow access is revoked',async()=>{
 const id='a'.repeat(32),run={id:'b'.repeat(32),workflow_id:id,definition:{name:'旧流程'},input:'旧任务',status:'completed'};
 const content={innerHTML:''},calls=[];
 const ctx=vm.createContext({document:{querySelector:()=>content},setInterval,clearInterval,$:()=>content,calls,id,run});
 vm.runInContext(fs.readFileSync(__dirname+'/workflow-runs.js','utf8'),ctx);
 vm.runInContext(`api=async path=>{calls.push(path);if(path==='/api/workflows/'+id){const e=new Error('forbidden');e.status=403;throw e}if(path==='/api/workflow-runs?workflow_id='+id)return [run];throw new Error(path)};heading=(title,description,action)=>'<h1>'+title+'</h1>'+action;esc=x=>x;`,ctx);
 await vm.runInContext('workflowRunsView(id)',ctx);
 assert.match(content.innerHTML,/旧流程 · 运行记录/);
 assert.match(content.innerHTML,new RegExp(run.id));
 assert.doesNotMatch(content.innerHTML,/打开编排/);
 assert.deepEqual(calls,['/api/workflow-runs?workflow_id='+id,'/api/workflows/'+id]);
});

test('workflow overview groups inaccessible history under its own Runs entry',async()=>{
 const id='c'.repeat(32),content={innerHTML:''};
 const ctx=vm.createContext({window:{addEventListener(){}},document:{querySelector:()=>content},state:{me:{admin:false}},$:()=>content});
 vm.runInContext(fs.readFileSync(__dirname+'/workflows.js','utf8'),ctx);
 ctx.id=id;
 vm.runInContext("api=async path=>path==='/api/workflows'?[]:path==='/api/workflow-run-groups'?[{id,name:'历史编排'}]:Promise.reject(Error(path));heading=()=>'<h1>智能体编排</h1>';esc=x=>x;",ctx);
 await vm.runInContext('workflowsView()',ctx);
 assert.match(content.innerHTML,/历史运行/);
 assert.match(content.innerHTML,new RegExp('/workflows/'+id+'/runs'));
 assert.doesNotMatch(content.innerHTML,/workflow-runs\//,'overview must group by workflow, not list all runs');
});

test('run delivery shows only verified deployment links and warns when PR head changed',()=>{
 const ctx=vm.createContext({URL,esc:x=>x});
 vm.runInContext(fs.readFileSync(__dirname+'/workflow-runs.js','utf8'),ctx);
 const base={pull_request_url:'https://github.com/demo/repo/pull/7',actions_url:'https://github.com/demo/repo/actions',run_head_sha:'a'.repeat(40),current_head_sha:'b'.repeat(40),state:'open',draft:true,merged:false,deployments:[]};
 ctx.delivery=base;
 const before=vm.runInContext('workflowDeliveryHTML(delivery)',ctx);
 assert.match(before,/尚未合并/);
 assert.match(before,/原 QA 结论不覆盖当前 PR/);
 assert.doesNotMatch(before,/打开效果地址/);
 ctx.delivery={...base,state:'closed',draft:false,merged:true,main_sha:'c'.repeat(40),deployments:[{environment:'local-preview',state:'success',version:'c'.repeat(40),url:'http://127.0.0.1:5545/admin/',log_url:'https://github.com/demo/repo/actions/runs/1'}]};
 const after=vm.runInContext('workflowDeliveryHTML(delivery)',ctx);
 assert.match(after,/已合并提交/);
 assert.match(after,/查看已合并 PR/);
 assert.match(after,/打开效果地址/);
 assert.match(after,/127\.0\.0\.1:5545/);
 ctx.delivery.deployments.push({...ctx.delivery.deployments[0],version:'d'.repeat(40)});
 const history=vm.runInContext('workflowDeliveryHTML(delivery)',ctx);
 assert.equal((history.match(/打开效果地址/g)||[]).length,1,'historical successes must not link the current fixed address as an old version');
 ctx.delivery.deployments[0].url='javascript:alert(1)';
 assert.doesNotMatch(vm.runInContext('workflowDeliveryHTML(delivery)',ctx),/href="javascript:/);
});

test('actual input dialog reads frozen step context and explains legacy input',async()=>{
 const rendered=[];const ctx=vm.createContext({esc:x=>String(x??''),dialog:html=>rendered.push(html),api:async path=>{assert.equal(path,'/api/workflow-runs/run1/steps/2/context');return {role_instructions:'角色正文',project_instructions:['AGENTS.md'],tools:['complete_node'],input:'冻结输入',context_version:0,workflow_revision:7}}});
 vm.runInContext(fs.readFileSync(__dirname+'/workflow-runs.js','utf8'),ctx);
 await vm.runInContext("workflowStepContextDialog('run1',2)",ctx);
 assert.match(rendered.at(-1),/冻结输入/);assert.match(rendered.at(-1),/AGENTS.md/);assert.match(rendered.at(-1),/complete_node/);assert.match(rendered.at(-1),/旧版/);assert.match(rendered.at(-1),/readonly/);
});
test('continuation budget is paused for inspection and does not invent a user question',async()=>{
 const elements=new Map(),el=id=>{if(!elements.has(id))elements.set(id,{innerHTML:'',querySelectorAll:()=>[]});return elements.get(id)};
 const run={id:'run',workflow_id:'workflow',status:'waiting',seq:1,input:'task',definition:{name:'预算验收',revision:1,nodes:[{id:'dev',name:'研发',kind:'agent',prompt:'执行'}],edges:[]},steps:[{seq:1,node_id:'dev',status:'waiting',wait_kind:'limit',wait_reason:'已达到2次上限',continuations:2,conversation_id:'conv'}]};
 const ctx=vm.createContext({esc:x=>String(x??''),$:el,document:{querySelectorAll:()=>[]},api:async path=>path.endsWith('/notifications')?[]:run});
 vm.runInContext(fs.readFileSync(__dirname+'/workflow-runs.js','utf8'),ctx);
 vm.runInContext("workflowRunPage={id:'run'}",ctx);await vm.runInContext("refreshWorkflowRun('run')",ctx);
 assert.match(el('#wf-run-header').innerHTML,/自动继续上限/);
 assert.match(el('#wf-run-controls').innerHTML,/节点尚未完成/);
 assert.doesNotMatch(el('#wf-run-controls').innerHTML,/继续澄清/);
});
