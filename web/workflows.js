'use strict';
let workflowEditor = null;
function leaveWorkflowEditor(){workflowEditor=null;if(typeof leaveWorkflowRun==='function')leaveWorkflowRun();}
window.addEventListener('beforeunload',event=>{if(workflowEditor?.dirty){event.preventDefault();event.returnValue='';}});

async function workflowsView(){
 const items=await api('/api/workflows');
 $('#content').innerHTML=heading('智能体编排','把 Agent、外部动作与人工反馈连接起来。',state.me.admin?'<div class="wf-actions"><button id="wf-import">导入</button><button id="wf-connectors">管理 Connector</button><button id="wf-new" class="primary">＋ 创建智能体编排</button></div>':'')+
 `<div class="agent-grid">${items.map(w=>`<article class="card"><div class="agent-head"><span class="agent-icon">⌘</span><div><h2>${esc(w.name)}</h2><span class="muted small">版本 ${w.revision} · ${w.enabled?'已启用':'已停用'}</span></div></div><p class="muted">${w.nodes.length} 个节点 · ${w.edges.length} 条路径</p><div class="agent-foot"><a class="wf-link" data-nav href="/workflows/${esc(w.id)}">${state.me.admin?'打开编排':'查看流程'} ↗</a><a class="wf-link" data-nav href="/workflows/${esc(w.id)}/runs">运行记录 →</a></div></article>`).join('')||'<div class="empty"><strong>将协作过程变成可复用的流程</strong>创建智能体编排，从节点开始编排。</div>'}</div>`;
 if(state.me.admin){$('#wf-connectors').onclick=()=>connectorManager();$('#wf-new').onclick=()=>go('/workflows/new');$('#wf-import').onclick=()=>importWorkflow();}
}
function importWorkflow(){
 dialog(`<form id="wf-import-form"><div class="dialog-head"><h2>导入智能体编排</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><div class="field"><label for="wf-import-json">智能体编排 JSON</label><textarea id="wf-import-json" rows="14" required placeholder="粘贴导出的智能体编排配置"></textarea></div><button type="button" id="wf-load-file">选择 JSON 文件</button><p class="hint">导入为新副本；不会复制原智能体编排的用户授权。</p></div><div class="dialog-footer"><button type="button" data-close>取消</button><button class="primary">导入副本</button></div></form>`);
 $('#wf-load-file').onclick=()=>{
  const input=document.createElement('input');input.type='file';input.accept='.json,application/json';
  input.onchange=async()=>{try{const file=input.files?.[0];if(!file)return;if(file.size>512*1024)throw new Error('文件超过 512 KB');$('#wf-import-json').value=await file.text()}catch(e){toast(e.message)}};input.click();
 };
 $('#wf-import-form').onsubmit=async e=>{e.preventDefault();try{const graph=WorkflowGraph.import($('#wf-import-json').value);$('#dialog').close();history.replaceState({},'','/workflows/new');await openWorkflowEditor(graph);toast('已导入副本。请核对 Agent 和授权后保存。')}catch(error){toast(error.message)}};
}
function exportWorkflow(editor){
 const copy=JSON.parse(JSON.stringify(editor.graph.value));copy.id='';copy.revision=0;copy.authorized_users=[];delete copy.updated_at;
 const text=JSON.stringify(copy,null,2);
 dialog(`<div class="dialog-head"><h2>导出智能体编排</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><p class="hint">可以复制到其他平台实例或保存到仓库。用户授权不随配置导出。</p><div class="field"><label for="wf-export-json">智能体编排 JSON</label><textarea id="wf-export-json" rows="16" readonly>${esc(text)}</textarea></div></div><div class="dialog-footer"><button data-close>关闭</button><button id="wf-download" class="primary">下载 JSON</button></div>`);
 $('#wf-export-json').select();$('#wf-download').onclick=()=>{const url=URL.createObjectURL(new Blob([text],{type:'application/json'}));const a=document.createElement('a');a.href=url;a.download='workflow.json';document.body.append(a);a.click();a.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);};
}
async function workflowView(id){
 const graph=id==='new'?new WorkflowGraph():new WorkflowGraph(await api('/api/workflows/'+id));
 if(id==='new'&&!state.me.admin)throw new Error('需要管理员权限');
 await openWorkflowEditor(graph);
}
async function openWorkflowEditor(graph){
 // Import can be opened from the list without writing any server state.
 if(!$('#wf-stage')){shell('workflows','智能体编排');}
 const users=state.me.admin?await api('/api/users'):[];
 const connectors=await api('/api/connectors');
 const editor=workflowEditor={graph,selected:graph.value.entry,dirty:!graph.value.id,readOnly:!state.me.admin,users,connectors,connectSource:null,showAllEdges:false,zoom:1};
 const w=graph.value;
 $('#content').classList.add('wf-content');
 $('#content').innerHTML=`<div class="wf-heading"><div><a href="/workflows" data-nav class="muted small">← 智能体编排</a><h1 id="wf-title">${esc(w.name)}</h1><span class="muted small" id="wf-save-state"></span></div><div class="wf-actions">${w.id?`<a class="wf-link" data-nav href="/workflows/${esc(w.id)}/runs">运行记录 →</a>`:''}<button id="wf-start" class="primary">运行</button><button id="wf-export">导出</button><button id="wf-hooks">通知 Hook</button>${editor.readOnly?'':'<button id="wf-import">导入副本</button><button id="wf-settings">流程设置</button><button id="wf-save" class="primary">保存</button>'}</div></div>
 <div class="wf-layout"><aside class="wf-palette"><span class="eyebrow">添加节点</span>${['agent','connector','approval','end'].map(kind=>`<button draggable="${!editor.readOnly}" data-kind="${kind}" ${editor.readOnly?'disabled':''}><span>${{agent:'◇',connector:'↗',approval:'◎',end:'◉'}[kind]}</span>${WorkflowGraph.kinds[kind]}</button>`).join('')}<p class="hint">拖到画布，或点击添加。<br>从节点右侧圆点拖向目标左侧圆点连线。</p><button id="wf-overview">回到入口</button><button id="wf-fit">适应宽度</button><button id="wf-actual">100%</button><button id="wf-show-edges" aria-pressed="false">显示全部连线</button><div class="wf-legend"><span class="wf-legend-handoff">┄ Agent 自主交接</span><span>━ 固定规则流转</span><span>◎ 人工选择</span></div></aside>
 <div class="wf-workarea"><div id="wf-hint" class="wf-hint" role="status">选择节点查看交接目标 · 点击连线配置决策方式</div><div id="wf-viewport" tabindex="0" aria-label="智能体编排画布"><div id="wf-stage"><svg id="wf-lines" aria-label="节点连接"><defs><marker id="wf-arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8" fill="#8293b1"/></marker></defs><g id="wf-edge-group"></g><path id="wf-draft-line"/></svg><div id="wf-nodes"></div></div></div><div class="wf-canvas-footer">虚线：Agent 决策 · 实线：固定规则（◎ 人工选择） <span id="wf-count"></span></div></div>
 <aside id="wf-inspector" class="wf-inspector"></aside></div>`;
 $('#wf-start').onclick=()=>{if(editor.dirty){toast('请先保存编排改动');return}startWorkflowDialog(editor.graph.value).catch(e=>toast(e.message))};
 $('#wf-hooks').onclick=()=>workflowHooksDialog(editor);
 $('#wf-export').onclick=()=>exportWorkflow(editor);
 if(!editor.readOnly){
  $('#wf-import').onclick=()=>{if(!editor.dirty||confirm('放弃当前未保存的改动并导入副本？'))importWorkflow()};
  $('#wf-settings').onclick=()=>workflowSettings(editor);
  $('#wf-save').onclick=()=>saveWorkflow(editor);
 }
 $('#wf-show-edges').onclick=()=>{editor.showAllEdges=!editor.showAllEdges;$('#wf-show-edges').textContent=editor.showAllEdges?'聚焦当前节点':'显示全部连线';$('#wf-show-edges').setAttribute('aria-pressed',String(editor.showAllEdges));renderWorkflowEdges(editor)};
 $('#wf-fit').onclick=()=>{const width=Math.max(400,...graph.value.nodes.map(n=>n.x+240));setWorkflowZoom(editor,Math.min(1,($('#wf-viewport').clientWidth-20)/width))};$('#wf-actual').onclick=()=>setWorkflowZoom(editor,1);
 $('#wf-overview').onclick=()=>{const n=graph.node(graph.value.entry);if(n)$('#wf-viewport').scrollTo({left:Math.max(0,(n.x-60)*editor.zoom),top:Math.max(0,(n.y-80)*editor.zoom),behavior:'smooth'})};
 document.querySelectorAll('[data-kind]').forEach(button=>{
  button.onclick=()=>addWorkflowNode(editor,button.dataset.kind,40+$('#wf-viewport').scrollLeft/editor.zoom,60+$('#wf-viewport').scrollTop/editor.zoom);
  button.ondragstart=e=>e.dataTransfer.setData('application/x-platform-node',button.dataset.kind);
 });
 const stage=$('#wf-stage');stage.ondragover=e=>{if(!editor.readOnly)e.preventDefault()};
 stage.ondrop=e=>{if(editor.readOnly)return;e.preventDefault();const kind=e.dataTransfer.getData('application/x-platform-node');if(!Object.hasOwn(WorkflowGraph.kinds,kind))return;const p=workflowPoint(e);addWorkflowNode(editor,kind,p.x,p.y)};
 stage.onpointerdown=e=>workflowPointerDown(e,editor);
 stage.onkeydown=e=>{const node=e.target.closest('[data-node]');if(!node)return;if(['Enter',' '].includes(e.key)){e.preventDefault();editor.selected=node.dataset.node;renderWorkflowInspector(editor);renderWorkflowNodes(editor)}};
 renderWorkflowNodes(editor);renderWorkflowInspector(editor);workflowSaveStatus(editor);
}
function workflowSaveStatus(editor){
 if(workflowEditor!==editor)return;
 $('#wf-save-state').textContent=editor.dirty?'有未保存的改动':`已保存 · 版本 ${editor.graph.value.revision}`;
 $('#wf-title').textContent=editor.graph.value.name;
 const edges=editor.graph.value.edges;const visible=editor.showAllEdges||!editor.selected?edges:edges.filter(e=>e.source===editor.selected||e.target===editor.selected);$('#wf-count').textContent=`${editor.graph.value.nodes.length} 节点 / ${visible.length} 条可见连线（共 ${edges.length} 条）`;
}
function workflowChanged(editor){editor.dirty=true;workflowSaveStatus(editor);}
function setWorkflowZoom(editor,zoom){editor.zoom=Math.max(.25,Math.min(1.5,zoom));$('#wf-stage').style.zoom=editor.zoom;$('#wf-viewport').scrollTo(0,0);$('#wf-actual').textContent=Math.round(editor.zoom*100)+'% · 原尺寸';}
function workflowPoint(event){const rect=$('#wf-stage').getBoundingClientRect(),scale=workflowEditor?.zoom||1;return {x:(event.clientX-rect.left)/scale,y:(event.clientY-rect.top)/scale};}
function addWorkflowNode(editor,kind,x,y){if(editor.readOnly)return;const n=editor.graph.add(kind,x,y);editor.selected=n.id;workflowChanged(editor);renderWorkflowNodes(editor);renderWorkflowInspector(editor);}
function renderWorkflowNodes(editor){
 if(workflowEditor!==editor)return;
 const w=editor.graph.value;
 $('#wf-nodes').innerHTML=w.nodes.map(n=>`<div class="wf-node wf-${esc(n.kind)} ${n.id===editor.selected?'selected':''}" data-node="${esc(n.id)}"><button class="wf-port wf-in" data-port="in" title="连接到 ${esc(n.name)}" aria-label="连接到 ${esc(n.name)}" ${editor.readOnly?'disabled':''}></button><div class="wf-node-body" role="button" tabindex="0" aria-label="${esc(n.name)} 节点"><span class="wf-node-type">${esc(WorkflowGraph.kinds[n.kind])}${n.id===w.entry?' · 入口':''}</span><strong>${esc(n.name)}</strong><small>${n.kind==='agent'?esc(state.agents.find(a=>a.id===n.agent_id)?.name||'选择 Agent'):n.kind==='end'?'完成运行':n.kind==='connector'?'外部操作':'等待用户决策'}</small></div>${n.kind==='end'?'':`<button class="wf-port wf-out" data-port="out" title="从 ${esc(n.name)} 连线" aria-label="从 ${esc(n.name)} 连线" ${editor.readOnly?'disabled':''}></button>`}</div>`).join('')||'<div class="wf-empty">从左侧拖入第一个节点<br><small>例如：Agent → 人工确认 → 结束</small></div>';
 $('#wf-nodes').querySelectorAll('[data-node]').forEach(el=>{const n=editor.graph.node(el.dataset.node);el.style.left=n.x+'px';el.style.top=n.y+'px'});
 $('#wf-nodes').querySelectorAll('[data-port]').forEach(port=>port.onclick=e=>{if(editor.readOnly)return;e.stopPropagation();const id=port.closest('[data-node]').dataset.node;if(port.dataset.port==='out'){editor.connectSource=id;$('#wf-hint').textContent='选择目标节点左侧圆点，或拖拽完成连线'}else if(editor.connectSource){const source=editor.connectSource;editor.connectSource=null;workflowConnectDialog(editor,source,id)}});
 const width=Math.max(1200,...w.nodes.map(n=>n.x+280)),height=Math.max(780,...w.nodes.map(n=>n.y+200));
 $('#wf-stage').style.width=width+'px';$('#wf-stage').style.height=height+'px';
 renderWorkflowEdges(editor);
}
function workflowCurve(a,b){const bend=Math.max(65,Math.abs(b.x-a.x)*.45);return `M ${a.x} ${a.y} C ${a.x+bend} ${a.y}, ${b.x-bend} ${b.y}, ${b.x} ${b.y}`;}
function renderWorkflowEdges(editor){
 const w=editor.graph.value;
 const visible=w.edges.filter(e=>editor.showAllEdges||!editor.selected||e.source===editor.selected||e.target===editor.selected);
 $('#wf-edge-group').innerHTML=w.edges.map((e,i)=>{
  if(!visible.includes(e))return '';
  const a=editor.graph.node(e.source),b=editor.graph.node(e.target);if(!a||!b)return '';
  const mode=editor.graph.edgeMode(e),from={x:a.x+184,y:a.y+48},to={x:b.x,y:b.y+48};
  // Backward edges travel around the nodes rather than across their content.
  const backwards=b.x<=a.x;
  const lane=Math.max(a.y,b.y)+135+(i%3)*22;
  const curve=backwards?`M ${from.x} ${from.y} C ${from.x+65} ${from.y}, ${from.x+65} ${lane}, ${from.x} ${lane} L ${to.x-35} ${lane} Q ${to.x-55} ${lane}, ${to.x-55} ${lane-20} L ${to.x-55} ${to.y+25} Q ${to.x-55} ${to.y}, ${to.x} ${to.y}`:workflowCurve(from,to);
  const labelX=(from.x+to.x)/2,labelY=backwards?lane-8:(from.y+to.y)/2-10;
  const type=a.kind==='approval'?'人工选择':mode==='handoff'?'Agent 自主交接':'固定规则';
  return `<g class="wf-edge wf-edge-${mode}" data-edge="${i}" role="button" tabindex="0" aria-label="${esc(a.name)} → ${esc(b.name)}，${type}，${esc(e.route)}"><title>${esc(e.description||type)}</title><path d="${curve}" marker-end="url(#wf-arrow)"/><text x="${labelX}" y="${labelY}">${esc(e.description||e.route)}</text></g>`;
 }).join('');
 $('#wf-count').textContent=`${w.nodes.length} 节点 · ${visible.length}/${w.edges.length} 连线可见`;
 $('#wf-edge-group').querySelectorAll('[data-edge]').forEach(el=>{el.onclick=()=>workflowEdgeDialog(editor,Number(el.dataset.edge));el.onkeydown=e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();el.onclick()}}});
}
function workflowEdgeDialog(editor,index){
 const edge=editor.graph.value.edges[index];if(!edge)return;
 workflowConnectDialog(editor,edge.source,edge.target,index);
}
function workflowPointerDown(e,editor){
 if(e.button!==0)return;
 const nodeEl=e.target.closest('[data-node]');if(!nodeEl)return;
 const id=nodeEl.dataset.node,n=editor.graph.node(id),port=e.target.closest('[data-port]');
 if(port?.dataset.port==='in')return;
 if(editor.readOnly){editor.selected=id;renderWorkflowInspector(editor);return;}
 const stage=$('#wf-stage'),initial=workflowPoint(e),origin={x:n.x,y:n.y};let moved=false;
 editor.selected=id;
 const move=event=>{
  const p=workflowPoint(event);if(Math.abs(p.x-initial.x)+Math.abs(p.y-initial.y)>4)moved=true;
  if(port){$('#wf-draft-line').setAttribute('d',workflowCurve({x:n.x+184,y:n.y+48},p));return}
  editor.graph.move(id,origin.x+p.x-initial.x,origin.y+p.y-initial.y);nodeEl.style.left=n.x+'px';nodeEl.style.top=n.y+'px';renderWorkflowEdges(editor);
 };
 const finish=event=>{
  stage.removeEventListener('pointermove',move);stage.removeEventListener('pointerup',finish);stage.removeEventListener('pointercancel',cancel);$('#wf-draft-line').setAttribute('d','');
  if(stage.hasPointerCapture(e.pointerId))stage.releasePointerCapture(e.pointerId);
  if(event.type==='pointercancel'){renderWorkflowNodes(editor);return}
  if(port){if(!moved){editor.connectSource=id;$('#wf-hint').textContent='选择目标节点左侧圆点完成连线'}else{const target=document.elementFromPoint(event.clientX,event.clientY)?.closest('[data-node]');if(target){editor.connectSource=null;workflowConnectDialog(editor,id,target.dataset.node)}}}
  else{if(moved)workflowChanged(editor);renderWorkflowNodes(editor);renderWorkflowInspector(editor)}
 };
 const cancel=event=>{if(!port)editor.graph.move(id,origin.x,origin.y);finish(event)};
 stage.setPointerCapture(e.pointerId);stage.addEventListener('pointermove',move);stage.addEventListener('pointerup',finish);stage.addEventListener('pointercancel',cancel);
}
function workflowConnectDialog(editor,source,target,index=null){
 if(workflowEditor!==editor)return;
 const old=index==null?null:editor.graph.value.edges[index],n=editor.graph.node(source),mode=old?editor.graph.edgeMode(old):(n.kind==='agent'?'handoff':'automatic');
 dialog(`<form id="wf-connect"><div class="dialog-head"><h2>${old?'连线设置':'连接节点'}</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><p>${esc(n?.name)} → ${esc(editor.graph.node(target)?.name)}</p><fieldset ${editor.readOnly?'disabled':''}><div class="field"><label for="wf-edge-mode">谁决定流转？</label><select id="wf-edge-mode">${n.kind==='agent'?`<option value="handoff" ${mode==='handoff'?'selected':''}>Agent 自主决策 · 虚线</option>`:''}<option value="automatic" ${mode==='automatic'?'selected':''}>${n.kind==='approval'?'用户选择后的固定流转':'固定规则 · 实线'}</option></select><p class="hint">Agent 自己理解意图、选择虚线目标；实线由平台按明确的完成结果推进。普通聊天回复不会结束节点。</p></div><div class="field"><label for="wf-route">路线标识</label><input id="wf-route" required pattern="[a-zA-Z][a-zA-Z0-9_-]{0,63}" value="${esc(old?.route||'next')}"><span class="hint">同一节点不能重复。命令节点：next 为成功，failed 为失败。</span></div><div class="field"><label for="wf-edge-description">何时走这条线</label><textarea id="wf-edge-description" rows="3" maxlength="1000" placeholder="例如：发现设计遗漏时交回修订">${esc(old?.description||'')}</textarea><p class="hint">自主路线的说明会交给 Agent 参考；它不是平台执行的条件表达式。</p></div></fieldset></div><div class="dialog-footer"><button type="button" data-close>关闭</button>${editor.readOnly?'':`${old?'<button type="button" id="wf-edge-delete" class="danger">删除连线</button>':''}<button class="primary">${old?'应用设置':'添加连线'}</button>`}</div></form>`);
 if(editor.readOnly)return;
 $('#wf-connect').onsubmit=e=>{e.preventDefault();const edges=editor.graph.value.edges,backup=edges.slice();try{
  if(old)edges.splice(index,1);
  editor.graph.connect(source,$('#wf-route').value.trim(),target,$('#wf-edge-mode').value,$('#wf-edge-description').value.trim());
  if(old){const updated=edges.pop();edges.splice(index,0,updated)}
  workflowChanged(editor);$('#dialog').close();editor.selected=source;renderWorkflowNodes(editor);renderWorkflowInspector(editor);
 }catch(error){editor.graph.value.edges=backup;toast(error.message)}};
 if(old)$('#wf-edge-delete').onclick=()=>{editor.graph.value.edges.splice(index,1);workflowChanged(editor);$('#dialog').close();renderWorkflowNodes(editor);renderWorkflowInspector(editor)};
}
function renderWorkflowInspector(editor){
 if(workflowEditor!==editor)return;
 const n=editor.graph.node(editor.selected),w=editor.graph.value;
 if(!n){$('#wf-inspector').innerHTML='<h3>节点设置</h3><p class="muted">在画布中选择一个节点。</p>';return}
 const outgoing=w.edges.map((e,i)=>({...e,index:i})).filter(e=>e.source===n.id);
 $('#wf-inspector').innerHTML=`<span class="eyebrow">${esc(WorkflowGraph.kinds[n.kind])}</span><h3>节点设置</h3><form id="wf-node-form"><fieldset ${editor.readOnly?'disabled':''}><div class="field"><label for="wf-node-name">名称</label><input id="wf-node-name" required maxlength="100" value="${esc(n.name)}"></div>${n.kind==='agent'?`<div class="field"><label for="wf-node-agent">使用 Agent</label><select id="wf-node-agent"><option value="">请选择</option>${state.agents.map(a=>`<option value="${esc(a.id)}" ${a.id===n.agent_id?'selected':''}>${esc(a.name)}${a.enabled?'':'（已停用）'}</option>`).join('')}</select></div>`:''}${n.kind==='connector'?`<div class="field"><label for="wf-node-connector">使用 Connector</label><select id="wf-node-connector"><option value="">请选择</option>${editor.connectors.map(c=>`<option value="${esc(c.id)}" ${c.id===n.connector_id?'selected':''}>${esc(c.name)} · ${esc(connectorKinds[c.kind])}</option>`).join('')}</select></div>${connectorNodeFields(editor,n)}`:''}${['agent','approval'].includes(n.kind)?`<div class="field"><label for="wf-node-prompt">${n.kind==='agent'?'节点任务':'确认内容'}</label><textarea id="wf-node-prompt" rows="5" maxlength="32000">${esc(n.prompt||'')}</textarea></div>`:''}<div class="wf-coordinates"><div class="field"><label for="wf-node-x">横坐标</label><input id="wf-node-x" type="number" min="0" max="10000" value="${n.x}"></div><div class="field"><label for="wf-node-y">纵坐标</label><input id="wf-node-y" type="number" min="0" max="10000" value="${n.y}"></div></div><button type="submit">应用设置</button></fieldset></form><h4>交接与流转</h4><div class="wf-route-list">${outgoing.map(e=>`<div><span data-edit-edge="${e.index}" role="button" tabindex="0"><strong>${editor.graph.edgeMode(e)==='handoff'?'┄':'━'} ${esc(e.route)}</strong><small>→ ${esc(editor.graph.node(e.target)?.name)}</small></span>${editor.readOnly?'':`<button data-remove-edge="${e.index}" aria-label="删除 ${esc(e.route)} 连线">×</button>`}</div>`).join('')||'<p class="hint">'+(n.kind==='end'?'此节点结束运行。':'从右侧圆点连接下一节点。')+'</p>'}</div>${editor.readOnly?'':`<div class="wf-inspector-actions">${n.id===w.entry?'<span class="hint">当前入口节点</span>':'<button id="wf-set-entry">设为入口</button>'}<button id="wf-delete-node" class="danger">删除节点</button></div>`}`;
 document.querySelectorAll('[data-edit-edge]').forEach(el=>{el.onclick=()=>workflowEdgeDialog(editor,Number(el.dataset.editEdge));el.onkeydown=e=>{if(e.key==='Enter')el.onclick()}});
 $('#wf-node-form').onsubmit=e=>{e.preventDefault();if(editor.readOnly)return;try{editor.graph.move(n.id,Number($('#wf-node-x').value),Number($('#wf-node-y').value));n.name=$('#wf-node-name').value.trim();if(n.kind==='agent')n.agent_id=$('#wf-node-agent').value;if(n.kind==='connector'){n.connector_id=$('#wf-node-connector').value;n.connector_input=readConnectorNodeFields()};if($('#wf-node-prompt'))n.prompt=$('#wf-node-prompt').value;workflowChanged(editor);renderWorkflowNodes(editor)}catch(error){toast(error.message)}};
 if(!editor.readOnly){
  if($('#wf-node-connector'))$('#wf-node-connector').onchange=()=>{n.name=$('#wf-node-name').value.trim();n.connector_id=$('#wf-node-connector').value;n.connector_input={};workflowChanged(editor);renderWorkflowInspector(editor);renderWorkflowNodes(editor)};
  $('#wf-delete-node').onclick=()=>{editor.graph.remove(n.id);editor.selected=null;workflowChanged(editor);renderWorkflowNodes(editor);renderWorkflowInspector(editor)};
  if($('#wf-set-entry'))$('#wf-set-entry').onclick=()=>{w.entry=n.id;workflowChanged(editor);renderWorkflowNodes(editor);renderWorkflowInspector(editor)};
  document.querySelectorAll('[data-remove-edge]').forEach(b=>b.onclick=()=>{w.edges.splice(Number(b.dataset.removeEdge),1);workflowChanged(editor);renderWorkflowEdges(editor);renderWorkflowInspector(editor)});
 }
}
function workflowSettings(editor){
 const w=editor.graph.value;
 dialog(`<form id="wf-settings-form"><div class="dialog-head"><h2>流程设置</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><div class="field"><label for="wf-name">智能体编排名称</label><input id="wf-name" required value="${esc(w.name)}"></div><div class="field"><label for="wf-limit">最多执行节点次数（含循环）</label><input id="wf-limit" type="number" min="1" max="1000" required value="${w.max_steps}"></div><label class="check"><input id="wf-enabled" type="checkbox" ${w.enabled?'checked':''}>允许启动新运行</label><h4>授权用户</h4><p class="hint">管理员始终有权访问；用户还需获得节点 Agent 的授权。</p>${editor.users.filter(u=>u.role!=='admin').map(u=>`<label class="check"><input type="checkbox" name="wf-user" value="${esc(u.user_id)}" ${w.authorized_users.includes(u.user_id)?'checked':''}>${esc(u.username||u.user_id)}</label>`).join('')}<h4>允许直接开始的节点</h4><p class="hint">默认从入口开始。其他入口只对之后的新运行生效。</p>${w.nodes.filter(n=>n.id!==w.entry).map(n=>`<label class="check"><input type="checkbox" name="wf-start" value="${esc(n.id)}" ${w.start_nodes.includes(n.id)?'checked':''}>${esc(n.name)}</label>`).join('')}</div><div class="dialog-footer"><button type="button" data-close>取消</button><button class="primary">应用设置</button></div></form>`);
 $('#wf-settings-form').onsubmit=e=>{e.preventDefault();w.name=$('#wf-name').value.trim();w.max_steps=Number($('#wf-limit').value);w.enabled=$('#wf-enabled').checked;w.authorized_users=[...document.querySelectorAll('[name="wf-user"]:checked')].map(el=>el.value);w.start_nodes=[...document.querySelectorAll('[name="wf-start"]:checked')].map(el=>el.value);workflowChanged(editor);$('#dialog').close()};
}
async function saveWorkflow(editor){
 const button=$('#wf-save');button.disabled=true;
 try{
  const w=editor.graph.value;
  // Apply the visible node form before saving, including text not yet blurred.
  if($('#wf-node-form')){if(!$('#wf-node-form').reportValidity())return;$('#wf-node-form').requestSubmit();}
  $('#content').inert=true;
  const saved=await api('/api/workflows'+(w.id?'/'+w.id:''),w.id?'PUT':'POST',w);
  if(workflowEditor!==editor)return;
  editor.graph.value=saved;editor.dirty=false;renderWorkflowNodes(editor);renderWorkflowInspector(editor);$('#wf-hint').textContent='选择节点查看交接目标 · 点击连线配置决策方式';history.replaceState({},'','/workflows/'+saved.id);workflowSaveStatus(editor);toast('智能体编排已保存');
 }catch(e){toast(e.status===409?'智能体编排已被其他页面更新。请导出当前改动，再重新打开合并。':e.message)}finally{button.disabled=false;if(workflowEditor===editor)$('#content').inert=false}
}

const workflowHookEvents={'node.started':'Agent 节点启动','agent.reply.completed':'Agent 最终回复完成','handoff.after':'交接完成','run.completed':'整次运行完成'};
function workflowHooksDialog(editor){
 const hooks=JSON.parse(JSON.stringify(editor.graph.value.hooks||[]));
 const draw=()=>{
 dialog(`<div class="dialog-head"><h2>事件通知 Hook</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><p class="hint">事件决定时机，动作决定做什么，正文模板映射数据。通知不阻塞节点，也不判断回复是否在请求审批。</p><div id="wf-hook-list">${hooks.map((h,i)=>`<div class="wf-hook-row"><strong>${esc(workflowHookEvents[h.event])}</strong><small>${esc(h.node||'全部节点')} → ${esc(editor.connectors.find(c=>c.id===h.connector_id)?.name||h.connector_id)}</small><button data-hook-edit="${i}">编辑</button><button data-hook-remove="${i}" ${editor.readOnly?'disabled':''}>删除</button></div>`).join('')||'<p>没有配置通知。平台不会自动向外发送消息。</p>'}</div>${editor.readOnly?'':'<button id="wf-hook-add">添加通知</button>'}</div><div class="dialog-footer"><button data-close>取消</button>${editor.readOnly?'':'<button id="wf-hooks-apply" class="primary">应用配置</button>'}</div>`);
 if(!editor.readOnly){$('#wf-hook-add').onclick=()=>edit(null);$('#wf-hooks-apply').onclick=()=>{editor.graph.value.hooks=hooks;workflowChanged(editor);$('#dialog').close()};document.querySelectorAll('[data-hook-remove]').forEach(b=>b.onclick=()=>{hooks.splice(Number(b.dataset.hookRemove),1);draw()})}
 document.querySelectorAll('[data-hook-edit]').forEach(b=>b.onclick=()=>edit(Number(b.dataset.hookEdit)));
 };
 const edit=index=>{
 const h=index==null?{event:'agent.reply.completed',node:'',input:{},body:'{{text}}\n\n[在平台继续]({{conversation_url}})'}:hooks[index];
 dialog(`<form id="wf-hook-form"><div class="dialog-head"><h2>通知规则</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><fieldset ${editor.readOnly?'disabled':''}><div class="field"><label for="hook-event">触发时机</label><select id="hook-event">${Object.entries(workflowHookEvents).map(([id,label])=>`<option value="${id}" ${id===h.event?'selected':''}>${label}</option>`).join('')}</select></div><div class="field"><label for="hook-node">作用节点</label><select id="hook-node"><option value="">全部节点</option>${editor.graph.value.nodes.map(n=>`<option value="${esc(n.id)}" ${h.node===n.id?'selected':''}>${esc(n.name)}</option>`).join('')}</select></div><div class="field"><label for="hook-action">通知动作</label><select id="hook-action" required><option value="">选择已配置的动作</option>${editor.connectors.filter(c=>c.kind==='github.issue_comment').map(c=>`<option value="${esc(c.id)}" ${c.id===h.connector_id?'selected':''}>${esc(c.name)}</option>`).join('')}</select><p class="hint">当前已支持 GitHub Issue 评论动作。凭证由 Connector 管理，不放进正文。</p></div><div class="field"><label for="hook-issue-node">Issue 来自哪个节点的创建结果</label><select id="hook-issue-node"><option value="">使用固定 Issue 编号</option>${editor.graph.value.nodes.filter(n=>n.kind==='connector'&&editor.connectors.find(c=>c.id===n.connector_id)?.kind==='github.issue_create').map(n=>`<option value="${esc(n.id)}" ${n.id===h.input.issue_node?'selected':''}>${esc(n.name)}</option>`).join('')}</select></div><div class="field"><label for="hook-issue">固定 Issue 编号</label><input id="hook-issue" type="number" min="1" value="${h.input.issue_number||''}"></div><div class="field"><label for="hook-body">通知正文模板</label><textarea id="hook-body" rows="6" required maxlength="16000">${esc(h.body)}</textarea><p class="hint">字段：{{text}} 原文、{{node_name}} 节点名称、{{summary}} 交接说明、{{target}} 目标、{{artifacts}} 产物、{{conversation_url}} 会话链接、{{run_url}} 运行链接。只做取值替换，不执行表达式。</p></div></fieldset></div><div class="dialog-footer"><button type="button" id="hook-back">返回规则列表</button>${editor.readOnly?'':'<button class="primary">保存此规则</button>'}</div></form>`);
 $('#hook-back').onclick=draw;
 $('#wf-hook-form').onsubmit=e=>{e.preventDefault();if(editor.readOnly)return;const issueNode=$('#hook-issue-node').value,issueNumber=Number($('#hook-issue').value);if(!issueNode&&!issueNumber){toast('请选择 Issue 来源或填写编号');return}const next={event:$('#hook-event').value,node:$('#hook-node').value,connector_id:$('#hook-action').value,input:issueNode?{issue_node:issueNode}:{issue_number:issueNumber},body:$('#hook-body').value};if(index==null)hooks.push(next);else hooks[index]=next;draw()};
 };
 draw();
}
