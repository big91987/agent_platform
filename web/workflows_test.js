const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
function editorPage(){
 const elements=new Map();
 const el=id=>{if(!elements.has(id))elements.set(id,{style:{},clientWidth:900,clientHeight:600,scrollLeft:0,scrollTop:0,scrollTo(){},insertAdjacentHTML(where,html){this.innerHTML=this.innerHTML+html},addEventListener(){},removeEventListener(){},value:'',textContent:'',disabled:false,querySelectorAll:()=>[],checkValidity:()=>true,classList:{add(){},remove(){},toggle(){}},setAttribute(){},focus(){},set innerHTML(html){this.html=html;for(const match of html.matchAll(/<(textarea|select)\b([^>]*\bid="([^"]+)"[^>]*)>([\s\S]*?)(?=<\/(?:textarea|select)>|$)/g)){const [,tag,attrs,key,tail]=match,target=el('#'+key);target.value=tag==='input'?(attrs.match(/\bvalue="([^"]*)"/)?.[1]||''):tag==='textarea'?tail:(tail.match(/<option[^>]*value="([^"]*)"[^>]*selected/)?.[1]||tail.match(/<option[^>]*value="([^"]*)"/)?.[1]||'');target.checked=/\bchecked\b/.test(attrs);}for(const match of html.matchAll(/<input\b([^>]*\bid="([^"]+)"[^>]*)>/g)){const target=el('#'+match[2]);target.value=match[1].match(/\bvalue="([^"]*)"/)?.[1]||'';target.checked=/\bchecked\b/.test(match[1]);}},get innerHTML(){return this.html||''}});return elements.get(id)};
 const ctx=vm.createContext({clearTimeout,ResizeObserver:class {observe(){} disconnect(){}},window:{addEventListener(){}},confirm:()=>true,document:{querySelectorAll:()=>[]},$:el,state:{agents:[],me:{admin:true}},esc:x=>String(x??''),connectorKinds:{},dialog:html=>{el('#dialog').innerHTML=html;el('#dialog').close=()=>{}},toast:message=>{el('#toast').textContent=message}});
 for(const file of ['workflow-model.js','agent-config.js','workflow-panel.js','workflow-rules.js','workflows.js'])vm.runInContext(fs.readFileSync(__dirname+'/'+file,'utf8'),ctx);
 vm.runInContext("const graph=new WorkflowGraph();graph.value.context_version=2;const a=graph.add('agent',0,0),b=graph.add('end',300,0),c=graph.add('end',300,180);graph.connect(a.id,'done',b.id,'handoff','完成后');const editor=workflowEditor={graph,selected:a.id,readOnly:false,dirty:false,connectors:[]};renderWorkflowNodes=()=>{};workflowSaveStatus=()=>{};",ctx);
 return {ctx,el};
}
test('clicking an Agent edge opens its inline target settings without a second dialog',()=>{
 const {ctx,el}=editorPage();vm.runInContext('workflowEdgeDialog(editor,0)',ctx);
 assert.equal(el('#dialog').innerHTML,'');assert.equal(vm.runInContext('editor.panelTab',ctx),'handoff');
 assert.match(el('#wf-inspector').innerHTML,/wf-agent-target-0/);
 vm.runInContext("editor.nodeEditor.graph.updateEdge(0,{target:c.id,description:'核验通过'});renderWorkflowInspector(editor)",ctx);
 assert.equal(el('#wf-agent-target-0').value,'node_3');assert.equal(el('#wf-agent-condition-0').value,'核验通过');
 assert.equal(vm.runInContext('graph.value.edges[0].target',ctx),'node_2');
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
test('Agent handoff targets render inline without mode buttons or a configuration dialog',()=>{
 const {ctx,el}=editorPage();vm.runInContext("renderWorkflowInspector(editor)",ctx);
 assert.match(el('#wf-inspector').innerHTML,/data-agent-target/);assert.match(el('#wf-inspector').innerHTML,/data-agent-condition/);
 assert.doesNotMatch(el('#wf-inspector').innerHTML,/wf-exit-handoff|wf-exit-complete/);
 assert.match(el('#wf-inspector').innerHTML,/wf-pane-advanced/);
});

test('opening a legacy Agent reference leaves stored graph intact until direct node edits migrate it',()=>{
 const {ctx,el}=editorPage();vm.runInContext("state.agents=[{id:'agent1',name:'角色',executor:'codex',instructions:'角色职责',env:{MODE:'old'}}];delete a.agent;a.agent_id='agent1';renderWorkflowInspector(editor)",ctx);
 assert.equal(el('#wf-agent-instructions').value,'角色职责');el('#wf-agent-instructions').value='节点角色';el('#wf-agent-instructions').oninput();
 assert.equal(vm.runInContext('a.agent_id',ctx),'agent1');el('#wf-node-apply').onclick();
 assert.equal(vm.runInContext('a.agent_id',ctx),undefined);assert.equal(vm.runInContext('a.agent.instructions',ctx),'节点角色');assert.equal(vm.runInContext('state.agents[0].instructions',ctx),'角色职责');
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

test('closing a dirty node can continue editing or explicitly discard changes',()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 assert.match(el('#wf-inspector').innerHTML,/wf-pane-preview/);
 el('#wf-agent-instructions').value='未应用';el('#wf-agent-instructions').oninput();el('#wf-tab-handoff').onclick();
 ctx.confirm=()=>false;el('#wf-panel-close').onclick();assert.equal(vm.runInContext('editor.selected',ctx),'node_1');assert.equal(el('#wf-agent-instructions').value,'未应用');
 ctx.confirm=()=>true;el('#wf-panel-close').onclick();assert.equal(vm.runInContext('editor.selected',ctx),null);assert.equal(vm.runInContext('a.agent.instructions',ctx),'');
});test('late preview of a previous node cannot replace current drawer details',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 for(const [id,value]of Object.entries({'#wf-node-name':'研究','#wf-node-prompt':'{{handoff}}','#wf-node-x':'0','#wf-node-y':'0','#wf-continuation':'3','#wf-timeout':'14400'}))el(id).value=value;
 ctx.resolvePreview=null;vm.runInContext("api=()=>new Promise(resolve=>resolvePreview=resolve);pending=previewWorkflowNode(editor,a)",ctx);
 vm.runInContext('editor.selected=c.id;renderWorkflowInspector(editor)',ctx);
 ctx.resolvePreview({session_prompt:'上一个节点过期预览',input:'过期输入'});await vm.runInContext('pending',ctx);
 assert.doesNotMatch(el('#wf-preview-content').innerHTML,/过期预览/);
});
test('Connector parameters update only on Apply',()=>{
 const {ctx,el}=editorPage();vm.runInContext("a.kind='connector';a.connector_id='c1';editor.connectors=[{id:'c1',kind:'github.pull_request'}];",ctx);vm.runInContext(fs.readFileSync(__dirname+'/connectors.js','utf8'),ctx);vm.runInContext('renderWorkflowInspector(editor)',ctx);
 el('#wf-c-head').value='new-source-branch';el('#wf-node-apply').onclick();assert.equal(vm.runInContext('a.connector_input?.head',ctx),'new-source-branch');
 el('#wf-panel-close').onclick();assert.equal(vm.runInContext('editor.dirty',ctx),true);
});test('a late failed preview cannot overwrite a newer successful preview',async()=>{
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
 el('#wf-agent-network').checked=true;el('#wf-agent-elevation').checked=true;el('#wf-agent-reviewer').value='auto_review';el('#wf-agent-trust').checked=true;el('#wf-agent-inherit').checked=false;
 ctx.requests=[];vm.runInContext("history={replaceState(){}};api=async(path,method,body)=>{requests.push(JSON.parse(JSON.stringify({path,method,body})));return {...body,id:'saved',revision:1}}",ctx);
 el('#wf-node-apply').onclick();await vm.runInContext('saveWorkflow(editor)',ctx);
 assert.equal(ctx.requests.length,1,el('#toast').textContent);
 const nodes=JSON.parse(JSON.stringify(ctx.requests[0].body.nodes));
 assert.equal(nodes[0].agent_id,undefined);assert.equal(nodes[0].agent.instructions,'节点职责');assert.equal(nodes[0].agent.model,'model-local');
 assert.deepEqual(nodes[0].agent.skills,['/skills/a','/skills/b']);assert.deepEqual(nodes[0].agent.env,{MODE:'node',REMOVE:null});
 assert.equal(nodes[0].agent.network_access,true);assert.equal(nodes[0].agent.allow_elevation,true);assert.equal(nodes[0].agent.approvals_reviewer,'auto_review');assert.equal(nodes[0].agent.trust_hooks,true);assert.equal(nodes[0].agent.inherit_env,false);assert.equal(nodes[0].agent.seed_dir,undefined);assert.equal(nodes[0].agent.native_config,'model_reasoning_effort = "high"');
 assert.equal(nodes[2].agent_id,undefined);assert.equal(nodes[2].agent.instructions,'Old role\n\n旧任务');assert.equal(nodes[2].agent.authorized_users,undefined);assert.equal(nodes[2].agent.resolved_tools,undefined);
 vm.runInContext("state.agents[0].env.MODE='changed'",ctx);assert.equal(vm.runInContext('editor.graph.value.nodes[2].agent.env.MODE',ctx),'source');
});
test('invalid environment blocks Apply and Save without discarding the editable text',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);el('#wf-agent-env').value='{"MODE":';el('#wf-agent-env').oninput();el('#wf-node-apply').onclick();
 assert.match(el('#toast').textContent,/环境/);assert.equal(el('#wf-agent-env').value,'{"MODE":');assert.equal(vm.runInContext('a.agent.env.MODE',ctx),undefined);
 ctx.requests=[];vm.runInContext("api=async(...args)=>requests.push(args)",ctx);await vm.runInContext('saveWorkflow(editor)',ctx);assert.equal(ctx.requests.length,0);
 el('#wf-agent-env').value='{"MODE":"repaired"}';el('#wf-agent-env').oninput();el('#wf-node-apply').onclick();assert.equal(vm.runInContext('a.agent.env.MODE',ctx),'repaired');
});test('switching nodes cannot silently apply or drop an invalid draft',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);el('#wf-agent-env').value='{"PARTIAL":';el('#wf-agent-env').oninput();
 ctx.confirm=()=>false;vm.runInContext('editor.selected=b.id;renderWorkflowInspector(editor)',ctx);assert.equal(vm.runInContext('editor.selected',ctx),'node_1');assert.equal(el('#wf-agent-env').value,'{"PARTIAL":');
 ctx.requests=[];vm.runInContext("api=async(...args)=>requests.push(args)",ctx);await vm.runInContext('saveWorkflow(editor)',ctx);assert.equal(ctx.requests.length,0);assert.match(el('#toast').textContent,/应用/);
});test('closing an unedited legacy node leaves migration for save',()=>{
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
test('share export includes applied configuration and removes Agent secrets',()=>{
 const {ctx,el}=editorPage();el('#wf-export-json').select=()=>{};vm.runInContext("a.agent.env={TOKEN:'PRIVATE_MARKER'};a.agent.native_config='token = \"PRIVATE_MARKER\"';renderWorkflowInspector(editor)",ctx);
 el('#wf-agent-instructions').value='Visible role';el('#wf-agent-instructions').oninput();el('#wf-node-apply').onclick();vm.runInContext('before=JSON.stringify(graph.value);exportWorkflow(editor)',ctx);
 const exported=JSON.parse(el('#wf-export-json').value);assert.equal(exported.nodes[0].agent.instructions,'Visible role');assert.deepEqual(exported.nodes[0].agent.env,{});assert.equal(exported.nodes[0].agent.native_config,'');assert.doesNotMatch(el('#wf-export-json').value,/PRIVATE_MARKER/);assert.equal(vm.runInContext('JSON.stringify(graph.value)===before',ctx),true);
});test('export requires applying or cancelling an unfinished node edit',()=>{
 const {ctx,el}=editorPage();el('#wf-export-json').select=()=>{};vm.runInContext('renderWorkflowInspector(editor)',ctx);el('#wf-agent-env').value='{"PARTIAL":';el('#wf-agent-env').oninput();vm.runInContext('exportWorkflow(editor)',ctx);assert.equal(el('#wf-export-json').value,'');assert.match(el('#toast').textContent,/应用/);
});test('a failed save keeps the visible unsynchronized draft out of the stored graph',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor);before=JSON.stringify(graph.value);api=async()=>{throw new Error("offline")}',ctx);
 el('#wf-agent-instructions').value='Unsynchronized role';await vm.runInContext('saveWorkflow(editor)',ctx);
 assert.equal(vm.runInContext('JSON.stringify(graph.value)===before',ctx),true);assert.equal(el('#wf-agent-instructions').value,'Unsynchronized role');
});
test('new input protocol saves node name and common settings without a separate prompt',async()=>{
 const {ctx,el}=editorPage();vm.runInContext('graph.value.context_version=2;delete a.prompt;renderWorkflowInspector(editor)',ctx);
 assert.doesNotMatch(el('#wf-inspector').innerHTML,/id="wf-node-prompt"/);
 assert.doesNotMatch(el('#wf-inspector').innerHTML,/id="wf-agent-seed"/);
 el('#wf-node-name').value='独立验收';el('#wf-node-name').onblur();
 el('#wf-agent-model').value='gpt-6.1-sol';el('#wf-agent-instructions').value='核对实际证据';
 ctx.requests=[];vm.runInContext("history={replaceState(){}};api=async(path,method,body)=>{requests.push(JSON.parse(JSON.stringify(body)));return {...body,id:'saved',revision:1}}",ctx);
 el('#wf-node-apply').onclick();await vm.runInContext('saveWorkflow(editor)',ctx);
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
test('optional params retain Schema and instructions across toggles and validate on Apply',()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 el('#wf-handoff-params-node_2').checked=true;el('#wf-handoff-params-node_2').onchange();el('#wf-completion-schema-node_2').value='{"type":"object","properties":{"passed":{"type":"boolean"}}}';el('#wf-completion-instructions-node_2').value='根据实际验证填写 passed';el('#wf-completion-schema-node_2').oninput();
 el('#wf-handoff-params-node_2').checked=false;el('#wf-handoff-params-node_2').onchange();el('#wf-handoff-params-node_2').checked=true;el('#wf-handoff-params-node_2').onchange();assert.match(el('#wf-completion-schema-node_2').value,/passed/);assert.match(el('#wf-completion-instructions-node_2').value,/实际验证/);
 el('#wf-completion-schema-node_2').value='{bad';el('#wf-completion-schema-node_2').oninput();el('#wf-node-apply').onclick();assert.equal(vm.runInContext('a.handoff_parameters?.node_2?.enabled',ctx),undefined);assert.match(el('#toast').textContent,/JSON/);
 el('#wf-completion-schema-node_2').value='{"type":"object"}';el('#wf-completion-schema-node_2').oninput();el('#wf-node-apply').onclick();assert.equal(vm.runInContext('a.handoff_parameters?.node_2?.enabled',ctx),true);assert.match(vm.runInContext('a.handoff_parameters.node_2.instructions',ctx),/实际验证/);
});test('schema drafts reject numbers that JSON.parse would silently round',()=>{
 const {ctx}=editorPage();
 assert.throws(()=>vm.runInContext('applyCompletionDraft(a,{node_2:{schema:\'{"type":"object","properties":{"id":{"const":9007199254740993}}}\',instructions:""}})',ctx),/数值/);
 assert.throws(()=>vm.runInContext('applyCompletionDraft(a,{node_2:{schema:\'{"type":"object","properties":{"n":{"minimum":1.0000000000000001}}}\',instructions:""}})',ctx),/数值/);
 assert.doesNotThrow(()=>vm.runInContext('applyCompletionDraft(a,{node_2:{schema:\'{"type":"object","properties":{"n":{"minimum":0.1,"maximum":1e3}}}\',instructions:""}})',ctx));
});
test('stored workflows open the same current form regardless of their input version',async()=>{
 for(const version of [0,1]){
  const {ctx,el}=editorPage();ctx.version=version;
  vm.runInContext("graph.value.context_version=version;graph.value.id='stored';a.prompt='原职责 {{handoff}}';state.agents=[{id:'shared',executor:'codex',model:'configured-model',instructions:'角色'}];delete a.agent;delete a.exit_mode;a.agent_id='shared';api=async()=>[]",ctx);
  await vm.runInContext('openWorkflowEditor(graph)',ctx);
  vm.runInContext('workflowEditor.selected=a.id;renderWorkflowInspector(workflowEditor)',ctx);
  assert.equal(vm.runInContext('graph.value.context_version',ctx),2);
  assert.match(el('#wf-inspector').innerHTML,/data-agent-target/);assert.doesNotMatch(el('#wf-inspector').innerHTML,/wf-exit-complete/);assert.match(el('#wf-inspector').innerHTML,/wf-pane-preview/);
  assert.doesNotMatch(el('#wf-inspector').innerHTML,/Session Prompt|wf-node-prompt|{{handoff}}/);
  assert.equal(el('#wf-agent-model').value,'configured-model');assert.equal(el('#wf-agent-instructions').value,'角色\n\n原职责');
  assert.equal(vm.runInContext('workflowEditor.dirty',ctx),true);
 }
});

test('MCP arguments survive redraw and invalid arguments cannot replace applied settings',()=>{
 const {ctx,el}=editorPage();vm.runInContext("const run=graph.add('execution',0,300);editor.selected=run.id;run.rule_handoff=workflowRulePolicy('mcp');editor.toolServers=[{id:'github',name:'GitHub',enabled:true,tools:[{name:'create_issue',inputSchema:{type:'object',properties:{title:{type:'string'}}}}]}];renderWorkflowInspector(editor)",ctx);
 el('#wf-execution-server').value='github';el('#wf-execution-server').onchange();el('#wf-execution-tool').value='create_issue';el('#wf-execution-tool').onchange();el('#wf-execution-arguments').value='{"title":"literal $(test)","count":2}';el('#wf-execution-arguments').oninput();el('#wf-execution-timeout').value='120';el('#wf-execution-timeout').oninput();
 vm.runInContext('renderWorkflowInspector(editor)',ctx);assert.match(el('#wf-execution-arguments').value,/literal/);el('#wf-node-apply').onclick();assert.equal(vm.runInContext('run.execution.arguments.count',ctx),2);assert.equal(vm.runInContext('run.execution_timeout_seconds',ctx),120);
 el('#wf-execution-arguments').value='{broken';el('#wf-execution-arguments').oninput();el('#wf-node-apply').onclick();assert.equal(vm.runInContext('run.execution.arguments.count',ctx),2);assert.equal(el('#wf-execution-arguments').value,'{broken');
});
test('applied Python code remains after closing and reopening the node',()=>{
 const {ctx,el}=editorPage();vm.runInContext("a.kind='execution';delete a.agent;a.execution={mode:'code',language:'python',code:'print(1)'};a.execution_timeout_seconds=60;graph.value.environment='opensandbox';a.rule_handoff=workflowRulePolicy('code');renderWorkflowInspector(editor)",ctx);
 el('#wf-execution-code').value="import json\nprint('literal $(no shell)')";el('#wf-execution-code').oninput();el('#wf-node-apply').onclick();el('#wf-panel-close').onclick();vm.runInContext('editor.selected=a.id;renderWorkflowInspector(editor)',ctx);assert.equal(el('#wf-execution-code').value,"import json\nprint('literal $(no shell)')");assert.equal(vm.runInContext('graph.value.environment',ctx),'opensandbox');
});
test('workflow environment setting is retained and project command supports arbitrary repository language',()=>{
 const {ctx,el}=editorPage();vm.runInContext('editor.users=[];workflowSettings(editor)',ctx);el('#wf-environment').value='opensandbox';el('#wf-settings-form').onsubmit({preventDefault(){}});assert.equal(vm.runInContext('graph.value.environment',ctx),'opensandbox');
 vm.runInContext("a.kind='execution';delete a.agent;a.execution={mode:'command',command:'go test ./...'};a.execution_timeout_seconds=60;graph.value.environment='opensandbox';a.rule_handoff=workflowRulePolicy('command');renderWorkflowInspector(editor)",ctx);el('#wf-execution-command').value='mvn verify';el('#wf-node-apply').onclick();assert.equal(vm.runInContext('a.execution.command',ctx),'mvn verify');
});

test('MCP parameter bindings persist after Apply and invalid references cannot be applied',()=>{
 const {ctx,el}=editorPage();vm.runInContext("const run=graph.add('execution',0,300);editor.selected=run.id;run.execution={mode:'mcp',server_id:'github',tool:'create_issue',arguments:{title:'fallback'}};run.rule_handoff=workflowRulePolicy('mcp');renderWorkflowInspector(editor)",ctx);
 const raw='{"title":{"path":"/input"},"body":{"node":"node_1","path":"/result/params/report"}}';el('#wf-execution-bindings').value=raw;el('#wf-execution-bindings').oninput();el('#wf-node-apply').onclick();el('#wf-panel-close').onclick();vm.runInContext('editor.selected=run.id;renderWorkflowInspector(editor)',ctx);assert.equal(el('#wf-execution-bindings').value,JSON.stringify(JSON.parse(raw),null,2));
 el('#wf-execution-bindings').value='{broken';el('#wf-execution-bindings').oninput();el('#wf-node-apply').onclick();assert.match(el('#toast').textContent,/JSON/);assert.equal(vm.runInContext('run.execution.bindings.title.path',ctx),'/input');
 el('#wf-execution-bindings').value='{"title":{"node":"absent","path":"/result"}}';el('#wf-execution-bindings').oninput();el('#wf-node-apply').onclick();assert.match(el('#toast').textContent,/节点/);
});
test('execution environment references persist after Apply and preserve invalid text for correction',()=>{
 const {ctx,el}=editorPage();vm.runInContext("a.kind='execution';delete a.agent;a.execution={mode:'command',command:'git fetch'};a.rule_handoff=workflowRulePolicy('command');graph.value.environment='opensandbox';renderWorkflowInspector(editor)",ctx);
 el('#wf-execution-env').value='{"GITHUB_TOKEN":"PROJECT_GITHUB_TOKEN"}';el('#wf-execution-env').oninput();el('#wf-node-apply').onclick();el('#wf-panel-close').onclick();vm.runInContext('editor.selected=a.id;renderWorkflowInspector(editor)',ctx);assert.equal(JSON.parse(el('#wf-execution-env').value).GITHUB_TOKEN,'PROJECT_GITHUB_TOKEN');
 el('#wf-execution-env').value='{broken';el('#wf-execution-env').oninput();el('#wf-node-apply').onclick();assert.match(el('#toast').textContent,/JSON/);vm.runInContext('renderWorkflowInspector(editor)',ctx);assert.equal(el('#wf-execution-env').value,'{broken');
});
// Input packet frequency must not change how far a continuous gesture moves.
test('elastic zoom is independent of event subdivision and reverses without hidden overshoot',()=>{
 const {ctx}=editorPage();
 const zoom=(start,delta,parts)=>vm.runInContext(`(()=>{let z=${start};for(let i=0;i<${parts};i++)z=workflowGestureZoom(z,Math.exp(${delta}/${parts}));return z})()`,ctx);
 for(const [start,delta]of [[.5,.4],[1.9,.3],[.03,-.4]])assert.ok(Math.abs(zoom(start,delta,1)-zoom(start,delta,100))<1e-10);
 const upper=zoom(2,100,100);
 assert.ok(upper<2.18);
 assert.ok(zoom(upper,-.1,1)<2);
});

test('execution missing its required handoff does not fabricate a policy from edges',()=>{
 const {ctx,el}=editorPage();
 vm.runInContext("var exec=graph.add('execution',0,0);exec.execution={mode:'command',command:'make verify'};graph.connect(exec.id,'next',b.id,'automatic');editor.selected=exec.id;var original=JSON.stringify(graph.value);renderWorkflowInspector(editor)",ctx);
 assert.match(el('#wf-inspector').innerHTML,/role="alert"/);
 assert.doesNotMatch(el('#wf-inspector').innerHTML,/id="wf-result-source"|id="wf-enable-rules"/);
 assert.equal(vm.runInContext('JSON.stringify(graph.value)===original',ctx),true);
});

test('header rename trims text, rejects blank names and is absent in read-only details',()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);el('#wf-node-name').value='  新节点名称  ';el('#wf-node-name').onblur();assert.notEqual(vm.runInContext('a.name',ctx),'新节点名称');el('#wf-node-apply').onclick();assert.equal(vm.runInContext('a.name',ctx),'新节点名称');
 el('#wf-node-name').value=' ';el('#wf-node-name').onblur();assert.equal(el('#wf-node-name').value,'新节点名称');vm.runInContext('editor.readOnly=true;renderWorkflowInspector(editor)',ctx);assert.doesNotMatch(el('#wf-inspector').innerHTML,/id="wf-node-name"/);assert.match(el('#wf-inspector').innerHTML,/<h2[^>]*>新节点名称<\/h2>/);
});
test('node edits stay isolated until Apply and Cancel restores the applied node',()=>{
 const {ctx,el}=editorPage();vm.runInContext('renderWorkflowInspector(editor)',ctx);
 el('#wf-agent-instructions').value='暂存职责';el('#wf-agent-instructions').oninput();
 assert.equal(vm.runInContext('a.agent.instructions',ctx),'');assert.equal(vm.runInContext('editor.dirty',ctx),false);
 el('#wf-node-apply').onclick();assert.equal(vm.runInContext('a.agent.instructions',ctx),'暂存职责');assert.equal(vm.runInContext('editor.dirty',ctx),true);
 el('#wf-agent-instructions').value='应取消';el('#wf-agent-instructions').oninput();el('#wf-node-cancel').onclick();
 assert.equal(vm.runInContext('a.agent.instructions',ctx),'暂存职责');
});
test('execution mode and result source changes restore independent drafts',()=>{
 const {ctx,el}=editorPage();vm.runInContext("a.kind='execution';delete a.agent;a.execution={mode:'command',command:'make verify',output:{source:'exit_code'}};a.execution_timeout_seconds=60;graph.value.environment='opensandbox';a.rule_handoff=workflowRulePolicy('command');renderWorkflowInspector(editor)",ctx);
 el('#wf-execution-mode').value='code';el('#wf-execution-mode').onchange();el('#wf-execution-code').value='print(42)';el('#wf-execution-code').oninput();
 el('#wf-result-source').value='file';el('#wf-result-source').onchange({target:el('#wf-result-source')});el('#wf-result-path').value='reports/result.json';el('#wf-result-path').oninput({target:el('#wf-result-path')});
 el('#wf-result-source').value='stdout';el('#wf-result-source').onchange({target:el('#wf-result-source')});el('#wf-result-source').value='file';el('#wf-result-source').onchange({target:el('#wf-result-source')});assert.equal(el('#wf-result-path').value,'reports/result.json');
 el('#wf-execution-mode').value='command';el('#wf-execution-mode').onchange();assert.equal(el('#wf-execution-command').value,'make verify');
 el('#wf-execution-mode').value='code';el('#wf-execution-mode').onchange();assert.equal(el('#wf-execution-code').value,'print(42)');
 assert.equal(vm.runInContext('a.execution.mode',ctx),'command');
});

test('inactive execution modes and file source drafts survive serialized workflow reload',()=>{
 const {ctx,el}=editorPage();vm.runInContext("a.kind='execution';delete a.agent;a.execution={mode:'command',command:'make verify',output:{source:'exit_code'}};a.execution_timeout_seconds=60;graph.value.environment='opensandbox';a.rule_handoff=workflowRulePolicy('command');renderWorkflowInspector(editor)",ctx);
 el('#wf-execution-mode').value='code';el('#wf-execution-mode').onchange();el('#wf-execution-code').value='print(42)';el('#wf-execution-code').oninput();
 el('#wf-result-source').value='file';el('#wf-result-source').onchange({target:el('#wf-result-source')});el('#wf-result-path').value='reports/result.json';el('#wf-result-path').oninput({target:el('#wf-result-path')});
 el('#wf-result-source').value='stdout';el('#wf-result-source').onchange({target:el('#wf-result-source')});el('#wf-node-apply').onclick();
 vm.runInContext('editor.nodeEditor=null;editor.modeDrafts={};editor.graph=new WorkflowGraph(JSON.parse(JSON.stringify(editor.graph.value)));renderWorkflowInspector(editor)',ctx);
 el('#wf-result-source').value='file';el('#wf-result-source').onchange({target:el('#wf-result-source')});assert.equal(el('#wf-result-path').value,'reports/result.json');
 el('#wf-execution-mode').value='command';el('#wf-execution-mode').onchange();assert.equal(el('#wf-execution-command').value,'make verify');
 el('#wf-execution-mode').value='code';el('#wf-execution-mode').onchange();assert.equal(el('#wf-execution-code').value,'print(42)');
});

test('target parameter drafts stay independent and Apply rejects any invalid target atomically',()=>{
 const {ctx}=editorPage();
 vm.runInContext("graph.connect(a.id,'other',c.id,'handoff','另一目标');a.handoff_parameters={[b.id]:{enabled:true,schema:{type:'object',required:['commit']},instructions:'版本'},[c.id]:{enabled:false,schema:{type:'object',required:['ready']},instructions:'复核'}}",ctx);
 vm.runInContext("applyCompletionDraft(a,{[b.id]:{enabled:false,schema:'{\"type\":\"object\",\"required\":[\"commit\"]}',instructions:'新版说明'}})",ctx);
 assert.equal(vm.runInContext('a.handoff_parameters[c.id].instructions',ctx),'复核');
 assert.equal(vm.runInContext('a.handoff_parameters[b.id].enabled',ctx),false);
 const before=vm.runInContext('JSON.stringify(a.handoff_parameters)',ctx);
 assert.throws(()=>vm.runInContext("applyCompletionDraft(a,{[b.id]:{enabled:true,schema:'{\"type\":\"object\"}',instructions:'不应提交'},[c.id]:{enabled:true,schema:'{bad',instructions:''}})",ctx),/JSON/);
 assert.equal(vm.runInContext('JSON.stringify(a.handoff_parameters)',ctx),before);
});

test('legal target and node names never read or modify inherited object properties',()=>{
 const {ctx,el}=editorPage();
 vm.runInContext(`const named={id:'author',handoff_parameters:JSON.parse('{"constructor":{"enabled":true,"schema":{"type":"object","required":["commit"]},"instructions":"原说明"},"toString":{"enabled":true,"schema":{"type":"object"},"instructions":"另一说明"}}')};const drafts={completionDrafts:{author:{review:{enabled:false,schema:'',instructions:''}}}}`,ctx);
 assert.equal(vm.runInContext("workflowTargetParamsDraft(drafts,named,'constructor').enabled",ctx),true);
 assert.equal(vm.runInContext("workflowTargetParamsDraft(drafts,named,'toString').instructions",ctx),'另一说明');
 vm.runInContext("a.id='constructor';editor.selected=a.id;graph.value.entry=a.id;graph.value.edges[0].source=a.id;renderWorkflowInspector(editor)",ctx);
 el('#wf-handoff-params-node_2').checked=true;el('#wf-handoff-params-node_2').onchange();
 assert.equal(vm.runInContext("Object.hasOwn(editor.nodeEditor.completionDrafts,'constructor')",ctx),true);
 assert.equal(vm.runInContext("Object.hasOwn(Object,'node_2')",ctx),false);
});
