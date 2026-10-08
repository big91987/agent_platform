const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
function editorPage(){
 const elements=new Map();
 const el=id=>{if(!elements.has(id))elements.set(id,{value:'',textContent:'',disabled:false,querySelectorAll:()=>[],checkValidity:()=>true,classList:{add(){},remove(){},toggle(){}},setAttribute(){},focus(){},set innerHTML(html){this.html=html;for(const match of html.matchAll(/<(textarea|select)\b([^>]*\bid="([^"]+)"[^>]*)>([\s\S]*?)(?=<\/(?:textarea|select)>|$)/g)){const [,tag,attrs,key,tail]=match,target=el('#'+key);target.value=tag==='input'?(attrs.match(/\bvalue="([^"]*)"/)?.[1]||''):tag==='textarea'?tail:(tail.match(/<option[^>]*value="([^"]*)"[^>]*selected/)?.[1]||tail.match(/<option[^>]*value="([^"]*)"/)?.[1]||'');target.checked=/\bchecked\b/.test(attrs);}for(const match of html.matchAll(/<input\b([^>]*\bid="([^"]+)"[^>]*)>/g)){const target=el('#'+match[2]);target.value=match[1].match(/\bvalue="([^"]*)"/)?.[1]||'';target.checked=/\bchecked\b/.test(match[1]);}},get innerHTML(){return this.html||''}});return elements.get(id)};
 const ctx=vm.createContext({window:{addEventListener(){}},document:{querySelectorAll:()=>[]},$:el,state:{agents:[],me:{admin:true}},esc:x=>String(x??''),connectorKinds:{},dialog:html=>{el('#dialog').innerHTML=html;el('#dialog').close=()=>{}},toast:message=>{el('#toast').textContent=message}});
 for(const file of ['workflow-model.js','agent-config.js','workflow-panel.js','workflows.js'])vm.runInContext(fs.readFileSync(__dirname+'/'+file,'utf8'),ctx);
 vm.runInContext("const graph=new WorkflowGraph();graph.value.context_version=2;const a=graph.add('agent',0,0),b=graph.add('end',300,0),c=graph.add('end',300,180);graph.connect(a.id,'done',b.id,'handoff','完成后');const editor=workflowEditor={graph,selected:a.id,readOnly:false,dirty:false,connectors:[]};renderWorkflowNodes=()=>{};workflowSaveStatus=()=>{};",ctx);
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
test('preview uses unsaved visible role instructions without mutating or saving the graph',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 el('#wf-node-name').value='研究';el('#wf-agent-instructions').value='实时职责草稿';el('#wf-node-x').value='0';el('#wf-node-y').value='0';el('#wf-continuation').value='3';el('#wf-timeout').value='14400';
 ctx.requests=[];vm.runInContext("api=async(path,method,body)=>{requests.push({path,method,body});return {session_prompt:'展开后的工作说明',input:'预览输入'}}",ctx);
 await vm.runInContext('previewWorkflowNode(editor,a)',ctx);
 assert.equal(ctx.requests.length,1);
 assert.equal(ctx.requests[0].path,'/api/workflows/preview');
 assert.equal(ctx.requests[0].body.workflow.nodes[0].agent.instructions,'实时职责草稿');
 assert.equal(vm.runInContext('a.agent.instructions',ctx),'');
 assert.match(el('#wf-preview-content').innerHTML,/预览输入/);
 assert.match(el('#wf-preview-content').innerHTML,/readonly/);
 assert.equal(vm.runInContext('editor.panelTab',ctx),'preview');
});
test('mixed stored Agent paths become one handoff mode with every target retained while keeping both editable edges',()=>{
 const {ctx,el}=editorPage();vm.runInContext("delete a.exit_mode;graph.connect(a.id,'fixed',c.id,'automatic');graph.upgradeForEditing(state.agents);renderWorkflowInspector(editor)",ctx);
 assert.equal(vm.runInContext('graph.node(a.id).exit_mode',ctx),'handoff');assert.equal(vm.runInContext('graph.agentMode(a.id)',ctx),'handoff');
 assert.equal(vm.runInContext('graph.value.edges.length',ctx),2);
});

test('opening a legacy Agent reference leaves stored graph intact until direct node edits migrate it',()=>{
 const {ctx,el}=editorPage();
 vm.runInContext("state.agents=[{id:'agent1',name:'角色',executor:'codex',instructions:'角色职责',env:{MODE:'old'}}];delete a.agent;a.agent_id='agent1';renderWorkflowInspector(editor)",ctx);
 assert.equal(vm.runInContext('a.agent_id',ctx),'agent1');assert.equal(vm.runInContext('a.agent',ctx),undefined);
 assert.equal(el('#wf-agent-instructions').value,'角色职责');
 for(const [id,value]of Object.entries({'#wf-node-name':'研究','#wf-node-prompt':'{{handoff}}','#wf-node-x':'0','#wf-node-y':'0','#wf-continuation':'3','#wf-timeout':'14400'}))el(id).value=value;
 el('#wf-agent-instructions').value='节点角色';el('#wf-agent-instructions').oninput();
 assert.equal(vm.runInContext('a.agent_id',ctx),undefined);assert.equal(vm.runInContext('a.agent.instructions',ctx),'节点角色');
 assert.equal(vm.runInContext('state.agents[0].instructions',ctx),'角色职责');
});

test('editing role instructions keeps its current draft when a target is changed and inspector redraws',()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 for(const [id,value]of Object.entries({'#wf-node-name':'研究','#wf-agent-instructions':'修改后的职责','#wf-node-x':'0','#wf-node-y':'0','#wf-continuation':'3','#wf-timeout':'14400'}))el(id).value=value;
 el('#wf-agent-instructions').oninput();
 vm.runInContext('graph.updateEdge(0,{target:c.id});renderWorkflowInspector(editor)',ctx);
 assert.match(el('#wf-inspector').innerHTML,/修改后的职责/);
 assert.equal(vm.runInContext('graph.value.edges[0].target',ctx),'node_3');
});
test('node user input permission is visible and unsaved preview respects disabling it',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 assert.match(el('#wf-inspector').innerHTML,/wf-allow-user-input/);
 for(const [id,value]of Object.entries({'#wf-node-name':'研究','#wf-node-prompt':'{{handoff}}','#wf-node-x':'0','#wf-node-y':'0','#wf-continuation':'3','#wf-timeout':'14400'}))el(id).value=value;
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
 for(const [id,value]of Object.entries({'#wf-node-name':'研究','#wf-agent-instructions':'未保存职责','#wf-node-x':'0','#wf-node-y':'0','#wf-continuation':'3','#wf-timeout':'14400'}))el(id).value=value;
 el('#wf-agent-instructions').oninput();el('#wf-tab-handoff').onclick();
 assert.equal(vm.runInContext('editor.panelTab',ctx),'handoff');
 el('#wf-panel-close').onclick();
 assert.equal(vm.runInContext('editor.selected',ctx),null);
 assert.equal(el('#wf-inspector').hidden,true);
 assert.equal(vm.runInContext('a.agent.instructions',ctx),'未保存职责');
});
test('late preview of a previous node cannot replace current drawer details',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 for(const [id,value]of Object.entries({'#wf-node-name':'研究','#wf-node-prompt':'{{handoff}}','#wf-node-x':'0','#wf-node-y':'0','#wf-continuation':'3','#wf-timeout':'14400'}))el(id).value=value;
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
 for(const [id,value]of Object.entries({'#wf-node-name':'Agent','#wf-node-prompt':'{{handoff}}','#wf-node-x':'0','#wf-node-y':'0','#wf-continuation':'3','#wf-timeout':'14400'}))el(id).value=value;
 ctx.requests=[];vm.runInContext("api=()=>new Promise((resolve,reject)=>requests.push({resolve,reject}));old=previewWorkflowNode(editor,a);fresh=previewWorkflowNode(editor,a)",ctx);
 ctx.requests[1].resolve({input:'NEW SUCCESS'});await vm.runInContext('fresh',ctx);
 ctx.requests[0].reject(new Error('OLD ERROR'));await vm.runInContext('old',ctx);
 assert.match(el('#wf-preview-content').innerHTML,/NEW SUCCESS/);assert.doesNotMatch(el('#wf-preview-content').innerHTML,/OLD ERROR/);
});
test('changing the same node outgoing paths invalidates its in-flight preview',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 for(const [id,value]of Object.entries({'#wf-node-name':'Agent','#wf-node-prompt':'{{handoff}}','#wf-node-x':'0','#wf-node-y':'0','#wf-continuation':'3','#wf-timeout':'14400'}))el(id).value=value;
 ctx.resolveOld=null;vm.runInContext("api=()=>new Promise(resolve=>resolveOld=resolve);old=previewWorkflowNode(editor,a);graph.removeEdge(0);workflowChanged(editor);renderWorkflowInspector(editor)",ctx);
 ctx.resolveOld({session_prompt:'REMOVED TARGET',input:'old'});await vm.runInContext('old',ctx);
 assert.doesNotMatch(el('#wf-preview-content').innerHTML,/REMOVED TARGET/);
});
test('save submits the visible independent execution settings and migrates unopened legacy nodes',async()=>{
 const {ctx,el}=editorPage();
 vm.runInContext("state.agents=[{id:'old',name:'Shared',executor:'codex',instructions:'Old role',env:{MODE:'source'},authorized_users:['private'],resolved_tools:[{}]}];c.kind='agent';c.agent_id='old';c.prompt='旧任务';renderWorkflowInspector(editor)",ctx);
 for(const [id,value]of Object.entries({'#wf-agent-model':'model-local','#wf-agent-instructions':'节点职责','#wf-agent-skills':'/skills/a\n/skills/b','#wf-agent-native':'model_reasoning_effort = "high"','#wf-agent-env':'{"MODE":"node","REMOVE":null}','#wf-agent-seed':'/template','#wf-node-prompt':'当前工作 {{handoff}}'}))el(id).value=value;
 el('#wf-agent-network').checked=true;el('#wf-agent-elevation').checked=true;el('#wf-agent-trust').checked=true;el('#wf-agent-inherit').checked=false;
 ctx.requests=[];vm.runInContext("history={replaceState(){}};api=async(path,method,body)=>{requests.push(JSON.parse(JSON.stringify({path,method,body})));return {...body,id:'saved',revision:1}}",ctx);
 await vm.runInContext('saveWorkflow(editor)',ctx);
 assert.equal(ctx.requests.length,1,el('#toast').textContent);
 const nodes=JSON.parse(JSON.stringify(ctx.requests[0].body.nodes));
 assert.equal(nodes[0].agent_id,undefined);assert.equal(nodes[0].agent.instructions,'节点职责');assert.equal(nodes[0].agent.model,'model-local');
 assert.deepEqual(nodes[0].agent.skills,['/skills/a','/skills/b']);assert.deepEqual(nodes[0].agent.env,{MODE:'node',REMOVE:null});
 assert.equal(nodes[0].agent.network_access,true);assert.equal(nodes[0].agent.allow_elevation,true);assert.equal(nodes[0].agent.trust_hooks,true);assert.equal(nodes[0].agent.inherit_env,false);assert.equal(nodes[0].agent.seed_dir,undefined);assert.equal(nodes[0].agent.native_config,'model_reasoning_effort = "high"');
 assert.equal(nodes[2].agent_id,undefined);assert.equal(nodes[2].agent.instructions,'Old role\n\n旧任务');assert.equal(nodes[2].agent.authorized_users,undefined);assert.equal(nodes[2].agent.resolved_tools,undefined);
 vm.runInContext("state.agents[0].env.MODE='changed'",ctx);assert.equal(vm.runInContext('editor.graph.value.nodes[2].agent.env.MODE',ctx),'source');
});
test('invalid environment draft survives closing and reopening and blocks save until corrected',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 el('#wf-agent-env').value='{"MODE":';el('#wf-agent-env').oninput();el('#wf-panel-close').onclick();
 vm.runInContext('editor.selected=a.id;renderWorkflowInspector(editor)',ctx);
 assert.equal(el('#wf-agent-env').value,'{"MODE":');
 ctx.requests=[];vm.runInContext("api=async(...args)=>requests.push(args)",ctx);await vm.runInContext('saveWorkflow(editor)',ctx);
 assert.equal(ctx.requests.length,0);assert.match(el('#toast').textContent,/环境/);
 el('#wf-agent-env').value='{"MODE":"repaired"}';el('#wf-agent-env').oninput();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 assert.equal(vm.runInContext('a.agent.env.MODE',ctx),'repaired');
});
test('invalid environment in a closed drawer prevents silently saving its previous value',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 el('#wf-agent-env').value='{"PARTIAL":';el('#wf-agent-env').oninput();el('#wf-panel-close').onclick();
 vm.runInContext('editor.selected=b.id;renderWorkflowInspector(editor)',ctx);
 ctx.requests=[];vm.runInContext("api=async(...args)=>requests.push(args)",ctx);await vm.runInContext('saveWorkflow(editor)',ctx);
 assert.equal(ctx.requests.length,0);assert.match(el('#toast').textContent,/环境/);
});
test('closing an unedited legacy node leaves migration for save',()=>{
 const {ctx,el}=editorPage();vm.runInContext("state.agents=[{id:'old',executor:'codex',instructions:'Role'}];delete a.agent;a.agent_id='old';renderWorkflowInspector(editor)",ctx);
 el('#wf-panel-close').onclick();
 assert.equal(vm.runInContext('a.agent_id',ctx),'old');assert.equal(vm.runInContext('editor.dirty',ctx),false);
});
test('opening and closing a stored node without editing preserves absent optional fields and clean state',()=>{
 const {ctx,el}=editorPage();
 vm.runInContext("a.agent={executor:'codex',instructions:'Role'};delete a.allow_user_input;delete a.continuation_limit;delete a.execution_timeout_seconds;before=JSON.stringify(a);renderWorkflowInspector(editor)",ctx);
 el('#wf-panel-close').onclick();
 assert.equal(vm.runInContext('JSON.stringify(a)===before',ctx),true);
 assert.equal(vm.runInContext('editor.dirty',ctx),false);
});
test('node tool selection and approval policy are submitted with its independent config',()=>{
 const {ctx,el}=editorPage();vm.runInContext("editor.toolServers=[{id:'browser',name:'Browser',enabled:true,tools:[{name:'navigate',description:'Open page'}]}];renderWorkflowInspector(editor)",ctx);
 el('#wf-agent-tools').querySelectorAll=selector=>selector==='[data-agent-approval]'?[{dataset:{server:'browser',tool:'navigate'},value:'confirm'}]:selector==='[data-agent-tool]:checked'?[{dataset:{server:'browser'},value:'navigate'}]:[];
 vm.runInContext('applyWorkflowNode(editor,a)',ctx);
 assert.deepEqual(JSON.parse(vm.runInContext('JSON.stringify(a.agent.tool_servers)',ctx)),[{server_id:'browser',tools:['navigate'],approvals:{navigate:'confirm'}}]);
});
for(const removedDraft of ['{"PRIVATE_VALUE":"old-node-only"}','{"PARTIAL":'])test('deleting a node clears its environment draft before its ID is reused: '+removedDraft,async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 el('#wf-agent-env').value=removedDraft;el('#wf-agent-env').oninput();el('#wf-delete-node').onclick();
 vm.runInContext("addWorkflowNode(editor,'agent',0,0)",ctx);
 assert.equal(vm.runInContext('editor.selected',ctx),'node_1');
 assert.equal(el('#wf-agent-env').value,'{}');
 ctx.requests=[];vm.runInContext("history={replaceState(){}};api=async(path,method,body)=>{requests.push(JSON.parse(JSON.stringify(body)));return {...body,id:'saved',revision:1}}",ctx);
 await vm.runInContext('saveWorkflow(editor)',ctx);
 assert.equal(ctx.requests.length,1,el('#toast').textContent);
 assert.deepEqual(JSON.parse(JSON.stringify(ctx.requests[0].nodes.find(n=>n.id==='node_1').agent.env)),{});
});
test('share export captures visible config and legacy snapshots while omitting secrets without mutating graph',()=>{
 const {ctx,el}=editorPage();el('#wf-export-json').select=()=>{};
 vm.runInContext("state.agents=[{id:'old',name:'Shared',executor:'codex',instructions:'Legacy role',model:'legacy-model',env:{TOKEN:'PRIVATE_MARKER'},native_config:'token = \"PRIVATE_MARKER\"',tool_servers:[{server_id:'t',tools:['read']}]}];c.kind='agent';c.agent_id='old';c.prompt='Old prompt';a.agent.env={TOKEN:'PRIVATE_MARKER'};a.agent.native_config='token = \"PRIVATE_MARKER\"';renderWorkflowInspector(editor);before=JSON.stringify(graph.value)",ctx);
 el('#wf-agent-instructions').value='Visible role';el('#wf-node-prompt').value='Visible task {{handoff}}';
 vm.runInContext('exportWorkflow(editor)',ctx);
 const exported=JSON.parse(el('#wf-export-json').value);
 assert.equal(exported.nodes[0].agent.instructions,'Visible role');assert.equal(exported.nodes[0].prompt,undefined);
 assert.equal(exported.nodes[2].agent_id,undefined);assert.equal(exported.nodes[2].agent.model,'legacy-model');assert.equal(exported.nodes[2].agent.instructions,'Legacy role\n\nOld prompt');assert.deepEqual(exported.nodes[2].agent.tool_servers,[{server_id:'t',tools:['read']}]);
 assert.equal(exported.nodes[0].agent.native_config,'');assert.deepEqual(exported.nodes[0].agent.env,{});assert.doesNotMatch(el('#wf-export-json').value,/PRIVATE_MARKER/);
 assert.equal(vm.runInContext('JSON.stringify(graph.value)===before',ctx),true);
});
test('share export blocks invalid environment drafts instead of silently exporting stale settings',()=>{
 const {ctx,el}=editorPage();el('#wf-export-json').select=()=>{};vm.runInContext('renderWorkflowInspector(editor)',ctx);
 el('#wf-agent-env').value='{"PARTIAL":';el('#wf-agent-env').oninput();el('#wf-panel-close').onclick();
 vm.runInContext('exportWorkflow(editor)',ctx);
 assert.equal(el('#wf-export-json').value,'');assert.match(el('#toast').textContent,/环境/);
});
test('a failed save keeps the visible unsynchronized draft out of the stored graph',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor);before=JSON.stringify(graph.value);api=async()=>{throw new Error("offline")}',ctx);
 el('#wf-agent-instructions').value='Unsynchronized role';await vm.runInContext('saveWorkflow(editor)',ctx);
 assert.equal(vm.runInContext('JSON.stringify(graph.value)===before',ctx),true);assert.equal(el('#wf-agent-instructions').value,'Unsynchronized role');
});
test('new input protocol saves node name and common settings without a separate prompt',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('graph.value.context_version=2;delete a.prompt;renderWorkflowInspector(editor)',ctx);
 assert.doesNotMatch(el('#wf-inspector').innerHTML,/id="wf-node-prompt"/);
 assert.doesNotMatch(el('#wf-inspector').innerHTML,/id="wf-agent-seed"/);
 el('#wf-node-name').value='独立验收';el('#wf-node-name').oninput();
 el('#wf-agent-model').value='gpt-6.1-sol';el('#wf-agent-instructions').value='核对实际证据';
 ctx.requests=[];vm.runInContext("history={replaceState(){}};api=async(path,method,body)=>{requests.push(JSON.parse(JSON.stringify(body)));return {...body,id:'saved',revision:1}}",ctx);
 await vm.runInContext('saveWorkflow(editor)',ctx);
 assert.equal(ctx.requests.length,1,el('#toast').textContent);
 assert.equal(ctx.requests[0].nodes[0].name,'独立验收');
 assert.equal(ctx.requests[0].nodes[0].agent.model,'gpt-6.1-sol');
 assert.equal(ctx.requests[0].nodes[0].prompt,undefined);
 vm.runInContext('renderWorkflowInspector(editor)',ctx);
 assert.equal(el('#wf-node-name').value,'独立验收');
 assert.equal(el('#wf-agent-instructions').value,'核对实际证据');
});
test('standalone and workflow forms read the same execution settings and preserve unavailable tools',()=>{
 const {ctx,el}=editorPage();
 const values={executor:'codex',model:'gpt-6.1-sol',instructions:'核对证据',skills:'/skills/review\n/skills/report',sandbox:'read-only',seed:'/template',native:'model_reasoning_effort = "high"',env:'{"MODE":"test","REMOVE":null}'};
 for(const prefix of ['agent','wf-agent']){
  for(const [key,value]of Object.entries(values))el('#'+prefix+'-'+key).value=value;
  for(const key of ['network','elevation','trust','inherit'])el('#'+prefix+'-'+key).checked=key==='inherit';
  el('#'+prefix+'-tools').querySelectorAll=selector=>selector==='[data-agent-approval]'?[{dataset:{server:'unavailable',tool:'inspect'},value:'confirm'}]:[{dataset:{server:'unavailable'},value:'inspect'}];
 }
 const standalone=JSON.parse(vm.runInContext("JSON.stringify(readAgentConfig({},'agent'))",ctx));
 const node=JSON.parse(vm.runInContext("JSON.stringify(readAgentConfig({},'wf-agent'))",ctx));
 assert.deepEqual(standalone,node);
 assert.deepEqual(node.env,{MODE:'test',REMOVE:null});
 assert.deepEqual(node.tool_servers,[{server_id:'unavailable',tools:['inspect'],approvals:{inspect:'confirm'}}]);
});
test('fixed output drafts survive closing and invalid JSON blocks save even after switching nodes',async()=>{
 const {ctx,el}=editorPage();vm.runInContext("graph.value.context_version=2;delete a.prompt;graph.setExitMode(a.id,'complete');renderWorkflowInspector(editor)",ctx);
 assert.match(el('#wf-inspector').innerHTML,/wf-completion-schema/);
 el('#wf-completion-schema').value='{"type":"object","properties":{"passed":{"type":"boolean"}}}';el('#wf-completion-instructions').value='根据验证填写 passed';el('#wf-completion-schema').oninput();
 ctx.requests=[];vm.runInContext("api=async(path,method,body)=>{requests.push(body);return {input:'preview'}}",ctx);await vm.runInContext('previewWorkflowNode(editor,a)',ctx);
 assert.equal(ctx.requests[0].workflow.nodes[0].completion_schema.properties.passed.type,'boolean');
 el('#wf-completion-schema').value='{bad';el('#wf-completion-schema').oninput();el('#wf-panel-close').onclick();
 vm.runInContext('editor.selected=a.id;renderWorkflowInspector(editor)',ctx);assert.equal(el('#wf-completion-schema').value,'{bad');
 vm.runInContext('editor.selected=b.id;renderWorkflowInspector(editor)',ctx);ctx.requests.length=0;await vm.runInContext('saveWorkflow(editor)',ctx);
 assert.equal(ctx.requests.length,0);assert.match(el('#toast').textContent,/输出格式/);
});
test('schema drafts reject numbers that JSON.parse would silently round',()=>{
 const {ctx}=editorPage();
 assert.throws(()=>vm.runInContext('applyCompletionDraft(a,{schema:\'{"type":"object","properties":{"id":{"const":9007199254740993}}}\',instructions:""})',ctx),/数值/);
 assert.throws(()=>vm.runInContext('applyCompletionDraft(a,{schema:\'{"type":"object","properties":{"n":{"minimum":1.0000000000000001}}}\',instructions:""})',ctx),/数值/);
 assert.doesNotThrow(()=>vm.runInContext('applyCompletionDraft(a,{schema:\'{"type":"object","properties":{"n":{"minimum":0.1,"maximum":1e3}}}\',instructions:""})',ctx));
});
test('stored workflows open the same current form regardless of their input version',async()=>{
 for(const version of [0,1]){
  const {ctx,el}=editorPage();ctx.version=version;
  vm.runInContext("graph.value.context_version=version;graph.value.id='stored';a.prompt='原职责 {{handoff}}';state.agents=[{id:'shared',executor:'codex',model:'configured-model',instructions:'角色'}];delete a.agent;delete a.exit_mode;a.agent_id='shared';api=async()=>[]",ctx);
  await vm.runInContext('openWorkflowEditor(graph)',ctx);
  vm.runInContext('workflowEditor.selected=a.id;renderWorkflowInspector(workflowEditor)',ctx);
  assert.equal(vm.runInContext('graph.value.context_version',ctx),2);
  assert.match(el('#wf-inspector').innerHTML,/wf-exit-handoff/);assert.match(el('#wf-inspector').innerHTML,/wf-exit-complete/);assert.match(el('#wf-inspector').innerHTML,/wf-pane-preview/);
  assert.doesNotMatch(el('#wf-inspector').innerHTML,/Session Prompt|wf-node-prompt|{{handoff}}/);
  assert.equal(el('#wf-agent-model').value,'configured-model');assert.equal(el('#wf-agent-instructions').value,'角色\n\n原职责');
  assert.equal(vm.runInContext('workflowEditor.dirty',ctx),true);
 }
});
