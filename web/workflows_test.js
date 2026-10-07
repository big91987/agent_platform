const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
function editorPage(){
 const elements=new Map();
 const el=id=>{if(!elements.has(id))elements.set(id,{value:'',innerHTML:'',textContent:'',disabled:false,querySelectorAll:()=>[],classList:{add(){},remove(){},toggle(){}},setAttribute(){},focus(){}});return elements.get(id)};
 const ctx=vm.createContext({window:{addEventListener(){}},document:{querySelectorAll:()=>[]},$:el,state:{agents:[],me:{admin:true}},esc:x=>String(x??''),connectorKinds:{},dialog:html=>{el('#dialog').innerHTML=html;el('#dialog').close=()=>{}},toast:message=>{el('#toast').textContent=message}});
 for(const file of ['workflow-model.js','workflow-panel.js','workflows.js'])vm.runInContext(fs.readFileSync(__dirname+'/'+file,'utf8'),ctx);
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
 assert.match(el('#wf-preview-content').innerHTML,/展开后的工作说明/);
 assert.match(el('#wf-preview-content').innerHTML,/readonly/);
 assert.equal(vm.runInContext('editor.panelTab',ctx),'preview');
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
test('node user input permission is visible and unsaved preview respects disabling it',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 assert.match(el('#wf-inspector').innerHTML,/wf-allow-user-input/);
 for(const [id,value]of Object.entries({'#wf-node-name':'研究','#wf-node-agent':'agent1','#wf-node-prompt':'{{handoff}}','#wf-node-x':'0','#wf-node-y':'0','#wf-continuation':'3','#wf-timeout':'14400'}))el(id).value=value;
 el('#wf-allow-user-input').checked=false;
 ctx.requests=[];vm.runInContext("api=async(path,method,body)=>{requests.push(body);return {session_prompt:'预览',input:'输入'}}",ctx);
 await vm.runInContext('previewWorkflowNode(editor,a)',ctx);
 assert.equal(ctx.requests[0].workflow.nodes[0].allow_user_input,false);
});

test('click-selected node opens right tabs and closing retains unsaved draft',()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 assert.match(el('#wf-inspector').innerHTML,/role="tablist"/);
 assert.match(el('#wf-inspector').innerHTML,/wf-pane-config/);
 assert.match(el('#wf-inspector').innerHTML,/wf-pane-handoff/);
 assert.match(el('#wf-inspector').innerHTML,/wf-pane-preview/);
 for(const [id,value]of Object.entries({'#wf-node-name':'研究','#wf-node-agent':'agent1','#wf-node-prompt':'未保存输入 {{handoff}}','#wf-node-x':'0','#wf-node-y':'0','#wf-continuation':'3','#wf-timeout':'14400'}))el(id).value=value;
 el('#wf-node-prompt').oninput();el('#wf-tab-handoff').onclick();
 assert.equal(vm.runInContext('editor.panelTab',ctx),'handoff');
 el('#wf-panel-close').onclick();
 assert.equal(vm.runInContext('editor.selected',ctx),null);
 assert.equal(el('#wf-inspector').hidden,true);
 assert.equal(vm.runInContext('a.prompt',ctx),'未保存输入 {{handoff}}');
});
test('late preview of a previous node cannot replace current drawer details',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 for(const [id,value]of Object.entries({'#wf-node-name':'研究','#wf-node-agent':'agent1','#wf-node-prompt':'{{handoff}}','#wf-node-x':'0','#wf-node-y':'0','#wf-continuation':'3','#wf-timeout':'14400'}))el(id).value=value;
 ctx.resolvePreview=null;vm.runInContext("api=()=>new Promise(resolve=>resolvePreview=resolve);pending=previewWorkflowNode(editor,a)",ctx);
 vm.runInContext('editor.selected=c.id;renderWorkflowInspector(editor)',ctx);
 ctx.resolvePreview({session_prompt:'上一个节点过期预览',input:'过期输入'});await vm.runInContext('pending',ctx);
 assert.doesNotMatch(el('#wf-preview-content').innerHTML,/过期预览/);
});
test('closing a Connector drawer keeps its specific unsaved parameters',()=>{
 const {ctx,el}=editorPage();vm.runInContext("a.kind='connector';a.connector_id='c1';editor.connectors=[{id:'c1',kind:'github.pull_request'}];",ctx);
 vm.runInContext(fs.readFileSync(__dirname+'/connectors.js','utf8'),ctx);vm.runInContext('renderWorkflowInspector(editor)',ctx);
 for(const [id,value]of Object.entries({'#wf-node-name':'Agent','#wf-node-connector':'c1','#wf-node-x':'0','#wf-node-y':'0','#wf-c-head':'new-source-branch'}))el(id).value=value;
 el('#wf-panel-close').onclick();
 assert.equal(vm.runInContext('a.connector_input?.head',ctx),'new-source-branch');
 assert.equal(vm.runInContext('editor.dirty',ctx),true);
});
test('a late failed preview cannot overwrite a newer successful preview',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 for(const [id,value]of Object.entries({'#wf-node-name':'Agent','#wf-node-agent':'agent1','#wf-node-prompt':'{{handoff}}','#wf-node-x':'0','#wf-node-y':'0','#wf-continuation':'3','#wf-timeout':'14400'}))el(id).value=value;
 ctx.requests=[];vm.runInContext("api=()=>new Promise((resolve,reject)=>requests.push({resolve,reject}));old=previewWorkflowNode(editor,a);fresh=previewWorkflowNode(editor,a)",ctx);
 ctx.requests[1].resolve({session_prompt:'NEW SUCCESS',input:'new'});await vm.runInContext('fresh',ctx);
 ctx.requests[0].reject(new Error('OLD ERROR'));await vm.runInContext('old',ctx);
 assert.match(el('#wf-preview-content').innerHTML,/NEW SUCCESS/);assert.doesNotMatch(el('#wf-preview-content').innerHTML,/OLD ERROR/);
});
test('changing the same node outgoing paths invalidates its in-flight preview',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 for(const [id,value]of Object.entries({'#wf-node-name':'Agent','#wf-node-agent':'agent1','#wf-node-prompt':'{{handoff}}','#wf-node-x':'0','#wf-node-y':'0','#wf-continuation':'3','#wf-timeout':'14400'}))el(id).value=value;
 ctx.resolveOld=null;vm.runInContext("api=()=>new Promise(resolve=>resolveOld=resolve);old=previewWorkflowNode(editor,a);graph.removeEdge(0);workflowChanged(editor);renderWorkflowInspector(editor)",ctx);
 ctx.resolveOld({session_prompt:'REMOVED TARGET',input:'old'});await vm.runInContext('old',ctx);
 assert.doesNotMatch(el('#wf-preview-content').innerHTML,/REMOVED TARGET/);
});
