const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
function model() {
 const context = vm.createContext({});
 vm.runInContext(fs.readFileSync(__dirname + '/workflow-model.js', 'utf8'), context);
 return context.WorkflowGraph;
}
test('new Agent nodes save independent execution config without any shared Agent',()=>{
 const G=model(),g=new G(),a=g.add('agent',0,0),b=g.add('agent',300,0);
 const saved=JSON.parse(JSON.stringify(g.value));
 assert.equal(saved.nodes[0].agent.executor,'codex');
 assert.equal(saved.nodes[0].agent.sandbox,'workspace-write');
 assert.equal(saved.nodes[0].agent.network_access,false);
 assert.equal(saved.nodes[0].agent.inherit_env,true);
 assert.equal(saved.nodes[0].agent_id,undefined);
 a.agent.env.TASK='first';assert.equal(b.agent.env.TASK,undefined);
});
test('legacy migration takes only execution fields and isolates node and source changes',()=>{
 const G=model(),g=new G(),a=g.add('agent',0,0),b=g.add('end',300,0);
 g.connect(a.id,'done',b.id,'handoff','核验');a.prompt='本节点工作 {{handoff}}';a.agent_id='legacy';delete a.agent;
 const source={id:'source',name:'共享',enabled:true,authorized_users:['u1'],resolved_tools:[{secret:'private'}],executor:'codex',model:'m1',instructions:'角色',skills:['/skill'],tool_servers:[{server_id:'tools',tools:['read'],approvals:{read:'confirm'}}],env:{MODE:'one'}};
 a.agent=G.agentConfig(source);delete a.agent_id;
 const saved=JSON.parse(JSON.stringify(g.value));
 assert.equal(saved.nodes[0].agent_id,undefined);
 assert.deepEqual(Object.keys(saved.nodes[0].agent).sort(),['env','executor','instructions','model','skills','tool_servers']);
 source.skills.push('/other');source.tool_servers[0].approvals.read='auto';a.agent.env.MODE='two';
 assert.equal(a.agent.skills.length,1);assert.equal(a.agent.tool_servers[0].approvals.read,'confirm');assert.equal(source.env.MODE,'one');
 assert.equal(a.prompt,'本节点工作 {{handoff}}');assert.equal(g.value.edges[0].description,'核验');
});
test('loading a legacy reference does not migrate it and mutually exclusive sources are rejected',()=>{
 const G=model(),g=new G({nodes:[{id:'old',kind:'agent',agent_id:'shared',prompt:'旧工作'}],edges:[]});
 assert.equal(g.node('old').agent,undefined);assert.equal(g.node('old').agent_id,'shared');
 g.node('old').agent={executor:'codex'};assert.throws(()=>g.validateAgent(g.node('old')),/同时/);
});
test('import rejects malformed execution settings before opening the node drawer',()=>{
 const G=model(),g=new G();g.add('agent',0,0);
 for(const [changes,message]of [[{skills:{}},/Skill/],[{tool_servers:{}},/工具/],[{tool_servers:[{server_id:'x',tools:{}}]},/工具/],[{env:[]},/环境/]]){
  const value=JSON.parse(JSON.stringify(g.value));Object.assign(value.nodes[0].agent,changes);
  assert.throws(()=>G.import(JSON.stringify(value)),message);
 }
 const both=JSON.parse(JSON.stringify(g.value));both.nodes[0].agent_id='old';assert.throws(()=>G.import(JSON.stringify(both)),/同时/);
});
test('moving and deleting a node preserves other nodes and removes dangling routes',()=>{
 const G=model(),g=new G();
 const a=g.add('approval',25,40),b=g.add('end',380,40);
 g.connect(a.id,'approved',b.id);g.move(a.id,150,200);
 let saved=JSON.parse(JSON.stringify(g.value));
 assert.deepEqual([saved.nodes[0].x,saved.nodes[0].y],[150,200]);
 assert.equal(saved.edges[0].target,b.id);
 g.remove(b.id);assert.equal(g.value.edges.length,0);assert.equal(g.value.nodes.length,1);
});
test('editor rejects duplicate routes without erasing existing connection',()=>{
 const G=model(),g=new G(),a=g.add('approval',20,20),b=g.add('end',300,20),c=g.add('end',300,180);
 g.connect(a.id,'approved',b.id);
 assert.throws(()=>g.connect(a.id,'approved',c.id),/路由/);
 assert.equal(g.value.edges.length,1);assert.equal(g.value.edges[0].target,b.id);
 assert.throws(()=>g.connect(b.id,'next',a.id),/结束/);
});
test('import creates an independent unsaved copy and rejects malformed graph input',()=>{
 const G=model(),g=new G(),a=g.add('approval',0,0),b=g.add('end',300,0);g.connect(a.id,'next',b.id);
 g.value.id='a'.repeat(32);g.value.revision=9;g.value.authorized_users=['private-user'];
 const copy=G.import(JSON.stringify(g.value));
 assert.equal(copy.value.id,'');assert.equal(copy.value.revision,0);assert.equal(copy.value.authorized_users.length,0);
 copy.move(a.id,200,200);assert.equal(g.value.nodes[0].x,0);
 assert.throws(()=>G.import('{"nodes":"oops"}'));
 assert.throws(()=>G.import('{"nodes":[{"id":"a","kind":"script"}],"edges":[]}'));
});
test('adding nodes at the same palette position keeps the new node selectable',()=>{
 const G=model(),g=new G();const a=g.add('approval',40,60),b=g.add('end',40,60);
 assert.ok(Math.abs(a.x-b.x)>=184 || Math.abs(a.y-b.y)>=96,'new node must not cover the existing node');
});
test('edge choices survive export/import and invalid fixed Agent branches are rejected',()=>{
 const G=model(),g=new G(),a=g.add('agent',0,0),b=g.add('end',300,0),c=g.add('end',300,200);
 g.connect(a.id,'approved',b.id,'handoff','通过时交接');
 g.connect(a.id,'finished',c.id,'automatic');
 assert.equal(g.edgeMode(g.value.edges[0]),'handoff');
 assert.equal(g.edgeMode(g.value.edges[1]),'automatic');
 assert.throws(()=>g.connect(a.id,'another',b.id,'automatic'),/固定/);
 const copy=G.import(JSON.stringify(g.value));assert.equal(copy.value.edges[0].description,'通过时交接');
 assert.equal(copy.edgeMode(copy.value.edges[1]),'automatic');
 assert.throws(()=>G.import(JSON.stringify({...g.value,edges:[{source:a.id,target:b.id,route:'next',mode:'guess'}]})),/连线/);
});

test('new Agent drafts use session inputs while opening legacy graphs preserves their protocol',()=>{
 const G=model(),g=new G(),a=g.add('agent',0,0),b=g.add('end',300,0);
 g.connect(a.id,'done',b.id,'handoff','完成后交接');
 assert.equal(g.value.context_version,1);
 assert.equal(a.prompt,'{{handoff}}');
 assert.equal(a.continuation_limit,3);
 assert.equal(a.execution_timeout_seconds,14400);
 assert.doesNotThrow(()=>g.validateAgent(a));
 const legacy=new G({nodes:[{id:'old',kind:'agent',prompt:'原说明'}],edges:[]});
 assert.equal(legacy.value.context_version,undefined);
 assert.equal(legacy.node('old').prompt,'原说明');
});

test('changing handoff targets edits the graph edge and invalid edits leave its original path intact',()=>{
 const G=model(),g=new G(),a=g.add('agent',0,0),b=g.add('end',300,0),c=g.add('end',300,180);
 g.connect(a.id,'done',b.id,'handoff','完成');
 g.updateEdge(0,{target:c.id,description:'检查通过'});
 assert.equal(g.value.edges.length,1);
 assert.equal(g.value.edges[0].target,c.id);
 assert.equal(g.value.edges[0].description,'检查通过');
 assert.throws(()=>g.updateEdge(0,{target:'missing'}),/不存在/);
 assert.equal(g.value.edges[0].target,c.id);
 g.removeEdge(0);assert.equal(g.value.edges.length,0);
});

test('session prompt validation distinguishes autonomous fixed and mixed legacy paths',()=>{
 const G=model(),g=new G(),a=g.add('agent',0,0),b=g.add('end',300,0),c=g.add('end',300,180);
 g.connect(a.id,'done',b.id,'handoff');
 a.prompt='完成工作';assert.throws(()=>g.validateAgent(a),/handoff/);
 a.prompt='{{handoff}} {{handoff}}';assert.throws(()=>g.validateAgent(a),/handoff/);
 a.prompt='{{other}} {{handoff}}';assert.throws(()=>g.validateAgent(a),/占位符/);
 a.prompt='{{handoff}}';a.continuation_limit=0;a.execution_timeout_seconds=0;assert.doesNotThrow(()=>g.validateAgent(a));
 a.continuation_limit=11;assert.throws(()=>g.validateAgent(a),/继续/);a.continuation_limit=3;
 a.execution_timeout_seconds=86401;assert.throws(()=>g.validateAgent(a),/期限/);a.execution_timeout_seconds=14400;
 g.updateEdge(0,{mode:'automatic'});a.prompt='完成后提交结果';assert.doesNotThrow(()=>g.validateAgent(a));
 g.connect(a.id,'revise',c.id,'handoff');assert.equal(g.agentMode(a.id),'mixed');
 assert.equal(g.value.edges.length,2,'mixed compatibility must not drop either outgoing edge');
});
test('target-only handoff rejects duplicate targets but keeps legacy route graphs',()=>{
 const G=model(),g=new G(),a=g.add('agent',0,0),b=g.add('end',400,0);
 g.connect(a.id,'target_1',b.id,'handoff','第一策略');
 assert.throws(()=>g.connect(a.id,'target_2',b.id,'handoff','第二策略'),/同一.*目标/);
 g.value.context_version=0;
 g.connect(a.id,'target_2',b.id,'handoff','旧版不同route');
 assert.equal(g.value.edges.length,2);
});
test('new nodes expose a typed user input switch and preserve explicit disabled settings on import',()=>{
 const G=model(),g=new G(),a=g.add('agent',0,0);
 assert.equal(a.allow_user_input,true);
 a.allow_user_input=false;
 assert.equal(G.import(JSON.stringify(g.value)).node(a.id).allow_user_input,false);
 a.allow_user_input='false';assert.throws(()=>g.validateAgent(a),/用户输入/);
});
