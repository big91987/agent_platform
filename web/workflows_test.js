const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
function editorPage(){
 const elements=new Map();
 const el=id=>{if(!elements.has(id))elements.set(id,{value:'',innerHTML:'',textContent:'',disabled:false,querySelectorAll:()=>[],classList:{add(){}}});return elements.get(id)};
 const ctx=vm.createContext({window:{addEventListener(){}},document:{querySelectorAll:()=>[]},$:el,state:{agents:[],me:{admin:true}},esc:x=>String(x??''),connectorKinds:{},dialog:html=>{el('#dialog').innerHTML=html;el('#dialog').close=()=>{}},toast:message=>{el('#toast').textContent=message}});
 for(const file of ['workflow-model.js','workflows.js'])vm.runInContext(fs.readFileSync(__dirname+'/'+file,'utf8'),ctx);
 vm.runInContext("const graph=new WorkflowGraph();const a=graph.add('agent',0,0),b=graph.add('end',300,0),c=graph.add('end',300,180);graph.connect(a.id,'done',b.id,'handoff','完成后');const editor=workflowEditor={graph,selected:a.id,readOnly:false,dirty:false,connectors:[]};renderWorkflowNodes=()=>{};workflowSaveStatus=()=>{};",ctx);
 return {ctx,el};
}
test('connection settings change the displayed target and policy in the single graph edge',()=>{
 const {ctx,el}=editorPage();vm.runInContext('workflowEdgeDialog(editor,0)',ctx);
 assert.match(el('#dialog').innerHTML,/wf-edge-target/);
 el('#wf-edge-target').value='node_3';el('#wf-edge-mode').value='handoff';el('#wf-edge-description').value='核验通过';
 el('#wf-connect').onsubmit({preventDefault(){}});
 assert.equal(vm.runInContext('editor.graph.value.edges[0].target',ctx),'node_3');
 assert.equal(vm.runInContext('editor.graph.value.edges[0].description',ctx),'核验通过');
 assert.match(el('#wf-inspector').innerHTML,/核验通过/);
});
test('preview uses unsaved visible session prompt without mutating or saving the graph',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 el('#wf-node-name').value='研究';el('#wf-node-agent').value='agent1';el('#wf-node-prompt').value='实时草稿 {{handoff}}';el('#wf-node-x').value='0';el('#wf-node-y').value='0';el('#wf-continuation').value='3';el('#wf-timeout').value='14400';
 ctx.requests=[];vm.runInContext("api=async(path,method,body)=>{requests.push({path,method,body});return {session_prompt:'展开后的工作说明',input:'预览输入'}}",ctx);
 await vm.runInContext('previewWorkflowNode(editor,a)',ctx);
 assert.equal(ctx.requests.length,1);
 assert.equal(ctx.requests[0].path,'/api/workflows/preview');
 assert.equal(ctx.requests[0].body.workflow.nodes[0].prompt,'实时草稿 {{handoff}}');
 assert.equal(vm.runInContext('a.prompt',ctx),'{{handoff}}');
 assert.match(el('#dialog').innerHTML,/展开后的工作说明/);
 assert.match(el('#dialog').innerHTML,/readonly/);
});
test('mixed Agent paths expose compatibility warning while keeping both editable edges',()=>{
 const {ctx,el}=editorPage();vm.runInContext("graph.connect(a.id,'fixed',c.id,'automatic');renderWorkflowInspector(editor)",ctx);
 assert.match(el('#wf-inspector').innerHTML,/混合/);
 assert.equal(vm.runInContext('graph.value.edges.length',ctx),2);
});

test('editing a shared Agent uses the existing editor and returns to the workflow inspector',async()=>{
 const {ctx,el}=editorPage();
 vm.runInContext("state.agents=[{id:'agent1',name:'角色',instructions:'角色职责'}];a.agent_id='agent1';renderWorkflowInspector(editor);editAgent=async(agent,done)=>{edited=agent.id;await done()}",ctx);
 for(const [id,value]of Object.entries({'#wf-node-name':'研究','#wf-node-agent':'agent1','#wf-node-prompt':'{{handoff}}','#wf-node-x':'0','#wf-node-y':'0','#wf-continuation':'3','#wf-timeout':'14400'}))el(id).value=value;
 el('#wf-edit-agent').onclick();await new Promise(resolve=>setImmediate(resolve));
 assert.equal(vm.runInContext('edited',ctx),'agent1');
 assert.match(el('#wf-inspector').innerHTML,/角色职责/);
});

test('editing a Session Prompt keeps its current draft when a target is changed and inspector redraws',()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 for(const [id,value]of Object.entries({'#wf-node-name':'研究','#wf-node-agent':'agent1','#wf-node-prompt':'修改后的说明 {{handoff}}','#wf-node-x':'0','#wf-node-y':'0','#wf-continuation':'3','#wf-timeout':'14400'}))el(id).value=value;
 el('#wf-node-prompt').oninput();
 vm.runInContext('graph.updateEdge(0,{target:c.id});renderWorkflowInspector(editor)',ctx);
 assert.match(el('#wf-inspector').innerHTML,/修改后的说明/);
 assert.equal(vm.runInContext('graph.value.edges[0].target',ctx),'node_3');
});
