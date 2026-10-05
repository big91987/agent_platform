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
