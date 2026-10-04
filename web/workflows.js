'use strict';
let workflowEditor = null;
function leaveWorkflowEditor(){workflowEditor=null;if(typeof leaveWorkflowRun==='function')leaveWorkflowRun();}
window.addEventListener('beforeunload',event=>{if(workflowEditor?.dirty){event.preventDefault();event.returnValue='';}});

async function workflowsView(){
 const items=await api('/api/workflows');
 $('#content').innerHTML=heading('工作流','把 Agent、外部动作与人工反馈连接起来。',state.me.admin?'<div class="wf-actions"><button id="wf-import">导入</button><button id="wf-new" class="primary">＋ 创建工作流</button></div>':'')+
 `<div class="agent-grid">${items.map(w=>`<article class="card"><div class="agent-head"><span class="agent-icon">⌘</span><div><h2>${esc(w.name)}</h2><span class="muted small">版本 ${w.revision} · ${w.enabled?'已启用':'已停用'}</span></div></div><p class="muted">${w.nodes.length} 个节点 · ${w.edges.length} 条路径</p><div class="agent-foot"><a class="wf-link" data-nav href="/workflows/${esc(w.id)}">${state.me.admin?'打开编排':'查看流程'} ↗</a></div></article>`).join('')||'<div class="empty"><strong>将协作过程变成可复用的流程</strong>创建工作流，从节点开始编排。</div>'}</div>`;
 $('#content').insertAdjacentHTML('beforeend',await workflowRunsList());
 if(state.me.admin){$('#wf-new').onclick=()=>go('/workflows/new');$('#wf-import').onclick=()=>importWorkflow();}
}
function importWorkflow(){
 dialog(`<form id="wf-import-form"><div class="dialog-head"><h2>导入工作流</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><div class="field"><label for="wf-import-json">工作流 JSON</label><textarea id="wf-import-json" rows="14" required placeholder="粘贴导出的工作流配置"></textarea></div><button type="button" id="wf-load-file">选择 JSON 文件</button><p class="hint">导入为新副本；不会复制原工作流的用户授权。</p></div><div class="dialog-footer"><button type="button" data-close>取消</button><button class="primary">导入副本</button></div></form>`);
 $('#wf-load-file').onclick=()=>{
  const input=document.createElement('input');input.type='file';input.accept='.json,application/json';
  input.onchange=async()=>{try{const file=input.files?.[0];if(!file)return;if(file.size>512*1024)throw new Error('文件超过 512 KB');$('#wf-import-json').value=await file.text()}catch(e){toast(e.message)}};input.click();
 };
 $('#wf-import-form').onsubmit=async e=>{e.preventDefault();try{const graph=WorkflowGraph.import($('#wf-import-json').value);$('#dialog').close();history.replaceState({},'','/workflows/new');await openWorkflowEditor(graph);toast('已导入副本。请核对 Agent 和授权后保存。')}catch(error){toast(error.message)}};
}
function exportWorkflow(editor){
 const copy=JSON.parse(JSON.stringify(editor.graph.value));copy.id='';copy.revision=0;copy.authorized_users=[];delete copy.updated_at;
 const text=JSON.stringify(copy,null,2);
 dialog(`<div class="dialog-head"><h2>导出工作流</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><p class="hint">可以复制到其他平台实例或保存到仓库。用户授权不随配置导出。</p><div class="field"><label for="wf-export-json">工作流 JSON</label><textarea id="wf-export-json" rows="16" readonly>${esc(text)}</textarea></div></div><div class="dialog-footer"><button data-close>关闭</button><button id="wf-download" class="primary">下载 JSON</button></div>`);
 $('#wf-export-json').select();$('#wf-download').onclick=()=>{const url=URL.createObjectURL(new Blob([text],{type:'application/json'}));const a=document.createElement('a');a.href=url;a.download='workflow.json';document.body.append(a);a.click();a.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);};
}
async function workflowView(id){
 const graph=id==='new'?new WorkflowGraph():new WorkflowGraph(await api('/api/workflows/'+id));
 if(id==='new'&&!state.me.admin)throw new Error('需要管理员权限');
 await openWorkflowEditor(graph);
}
async function openWorkflowEditor(graph){
 // Import can be opened from the list without writing any server state.
 if(!$('#wf-stage')){shell('workflows','工作流编排');}
 const users=state.me.admin?await api('/api/users'):[];
 const editor=workflowEditor={graph,selected:graph.value.entry,dirty:!graph.value.id,readOnly:!state.me.admin,users,connectSource:null};
 const w=graph.value;
 $('#content').classList.add('wf-content');
 $('#content').innerHTML=`<div class="wf-heading"><div><a href="/workflows" data-nav class="muted small">← 工作流</a><h1 id="wf-title">${esc(w.name)}</h1><span class="muted small" id="wf-save-state"></span></div><div class="wf-actions"><button id="wf-start" class="primary">运行</button><button id="wf-export">导出</button>${editor.readOnly?'':'<button id="wf-import">导入副本</button><button id="wf-settings">流程设置</button><button id="wf-save" class="primary">保存</button>'}</div></div>
 <div class="wf-layout"><aside class="wf-palette"><span class="eyebrow">添加节点</span>${['agent','approval','end'].map(kind=>`<button draggable="${!editor.readOnly}" data-kind="${kind}" ${editor.readOnly?'disabled':''}><span>${{agent:'◇',approval:'◎',end:'◉'}[kind]}</span>${WorkflowGraph.kinds[kind]}</button>`).join('')}<button disabled title="Connector 执行接入尚在开发">↗ Connector</button><p class="hint">拖到画布，或点击添加。<br>从节点右侧圆点拖向目标左侧圆点连线。</p><button id="wf-overview">回到入口</button><div class="wf-legend"><span>◇ Agent</span><span>◎ 等待确认</span><span>◉ 流程结束</span></div></aside>
 <div class="wf-workarea"><div id="wf-hint" class="wf-hint" role="status">选择节点配置 · 支持回退连线</div><div id="wf-viewport" tabindex="0" aria-label="工作流画布"><div id="wf-stage"><svg id="wf-lines" aria-label="节点连接"><defs><marker id="wf-arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8" fill="#8293b1"/></marker></defs><g id="wf-edge-group"></g><path id="wf-draft-line"/></svg><div id="wf-nodes"></div></div></div><div class="wf-canvas-footer">拖动节点调整布局 · 路由名称决定下一步 <span id="wf-count"></span></div></div>
 <aside id="wf-inspector" class="wf-inspector"></aside></div>`;
 $('#wf-start').onclick=()=>{if(editor.dirty){toast('请先保存编排改动');return}startWorkflowDialog(editor.graph.value).catch(e=>toast(e.message))};
 $('#wf-export').onclick=()=>exportWorkflow(editor);
 if(!editor.readOnly){
  $('#wf-import').onclick=()=>{if(!editor.dirty||confirm('放弃当前未保存的改动并导入副本？'))importWorkflow()};
  $('#wf-settings').onclick=()=>workflowSettings(editor);
  $('#wf-save').onclick=()=>saveWorkflow(editor);
 }
 $('#wf-overview').onclick=()=>{const n=graph.node(graph.value.entry);if(n)$('#wf-viewport').scrollTo({left:Math.max(0,n.x-60),top:Math.max(0,n.y-80),behavior:'smooth'})};
 document.querySelectorAll('[data-kind]').forEach(button=>{
  button.onclick=()=>addWorkflowNode(editor,button.dataset.kind,40+$('#wf-viewport').scrollLeft,60+$('#wf-viewport').scrollTop);
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
 $('#wf-count').textContent=`${editor.graph.value.nodes.length} 节点 / ${editor.graph.value.edges.length} 连线`;
}
function workflowChanged(editor){editor.dirty=true;workflowSaveStatus(editor);}
function workflowPoint(event){const rect=$('#wf-stage').getBoundingClientRect();return {x:event.clientX-rect.left,y:event.clientY-rect.top};}
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
 $('#wf-edge-group').innerHTML=w.edges.map((e,i)=>{const a=editor.graph.node(e.source),b=editor.graph.node(e.target);if(!a||!b)return '';const from={x:a.x+184,y:a.y+48},to={x:b.x,y:b.y+48};const labelX=(from.x+to.x)/2,labelY=(from.y+to.y)/2+(b.y<a.y?14:-10);return `<g class="wf-edge" data-edge="${i}" role="button" tabindex="0" aria-label="${esc(a.name)} 的 ${esc(e.route)} 到 ${esc(b.name)}"><path d="${workflowCurve(from,to)}" marker-end="url(#wf-arrow)"/><text x="${labelX}" y="${labelY}">${esc(e.route)}</text></g>`}).join('');
 $('#wf-edge-group').querySelectorAll('[data-edge]').forEach(el=>{el.onclick=()=>{editor.selected=w.edges[Number(el.dataset.edge)].source;renderWorkflowInspector(editor);renderWorkflowNodes(editor)};el.onkeydown=e=>{if(e.key==='Enter')el.onclick()}});
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
function workflowConnectDialog(editor,source,target){
 if(workflowEditor!==editor)return;
 dialog(`<form id="wf-connect"><div class="dialog-head"><h2>连接节点</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><p>${esc(editor.graph.node(source)?.name)} → ${esc(editor.graph.node(target)?.name)}</p><div class="field"><label for="wf-route">发生什么结果时走这条路径？</label><input id="wf-route" required pattern="[a-zA-Z][a-zA-Z0-9_-]{0,63}" value="next"><span class="hint">例如 approved、changes、failed；同一节点的路由名称不能重复。</span></div></div><div class="dialog-footer"><button type="button" data-close>取消</button><button class="primary">添加连线</button></div></form>`);
 $('#wf-route').select();$('#wf-connect').onsubmit=e=>{e.preventDefault();try{editor.graph.connect(source,$('#wf-route').value.trim(),target);workflowChanged(editor);$('#dialog').close();editor.selected=source;renderWorkflowNodes(editor);renderWorkflowInspector(editor);$('#wf-hint').textContent='连线已添加 · 保存后生效'}catch(error){toast(error.message)}};
}
function renderWorkflowInspector(editor){
 if(workflowEditor!==editor)return;
 const n=editor.graph.node(editor.selected),w=editor.graph.value;
 if(!n){$('#wf-inspector').innerHTML='<h3>节点设置</h3><p class="muted">在画布中选择一个节点。</p>';return}
 const outgoing=w.edges.map((e,i)=>({...e,index:i})).filter(e=>e.source===n.id);
 $('#wf-inspector').innerHTML=`<span class="eyebrow">${esc(WorkflowGraph.kinds[n.kind])}</span><h3>节点设置</h3><form id="wf-node-form"><fieldset ${editor.readOnly?'disabled':''}><div class="field"><label for="wf-node-name">名称</label><input id="wf-node-name" required maxlength="100" value="${esc(n.name)}"></div>${n.kind==='agent'?`<div class="field"><label for="wf-node-agent">使用 Agent</label><select id="wf-node-agent"><option value="">请选择</option>${state.agents.map(a=>`<option value="${esc(a.id)}" ${a.id===n.agent_id?'selected':''}>${esc(a.name)}${a.enabled?'':'（已停用）'}</option>`).join('')}</select></div>`:''}${n.kind!=='end'?`<div class="field"><label for="wf-node-prompt">${n.kind==='agent'?'节点任务':'确认内容'}</label><textarea id="wf-node-prompt" rows="5" maxlength="32000">${esc(n.prompt||'')}</textarea></div>`:''}<div class="wf-coordinates"><div class="field"><label for="wf-node-x">横坐标</label><input id="wf-node-x" type="number" min="0" max="10000" value="${n.x}"></div><div class="field"><label for="wf-node-y">纵坐标</label><input id="wf-node-y" type="number" min="0" max="10000" value="${n.y}"></div></div><button type="submit">应用设置</button></fieldset></form><h4>输出路径</h4><div class="wf-route-list">${outgoing.map(e=>`<div><span><strong>${esc(e.route)}</strong><small>→ ${esc(editor.graph.node(e.target)?.name)}</small></span>${editor.readOnly?'':`<button data-remove-edge="${e.index}" aria-label="删除 ${esc(e.route)} 连线">×</button>`}</div>`).join('')||'<p class="hint">'+(n.kind==='end'?'此节点结束运行。':'从右侧圆点连接下一节点。')+'</p>'}</div>${editor.readOnly?'':`<div class="wf-inspector-actions">${n.id===w.entry?'<span class="hint">当前入口节点</span>':'<button id="wf-set-entry">设为入口</button>'}<button id="wf-delete-node" class="danger">删除节点</button></div>`}`;
 $('#wf-node-form').onsubmit=e=>{e.preventDefault();if(editor.readOnly)return;try{editor.graph.move(n.id,Number($('#wf-node-x').value),Number($('#wf-node-y').value));n.name=$('#wf-node-name').value.trim();if(n.kind==='agent')n.agent_id=$('#wf-node-agent').value;if(n.kind!=='end')n.prompt=$('#wf-node-prompt').value;workflowChanged(editor);renderWorkflowNodes(editor)}catch(error){toast(error.message)}};
 if(!editor.readOnly){
  $('#wf-delete-node').onclick=()=>{editor.graph.remove(n.id);editor.selected=null;workflowChanged(editor);renderWorkflowNodes(editor);renderWorkflowInspector(editor)};
  if($('#wf-set-entry'))$('#wf-set-entry').onclick=()=>{w.entry=n.id;workflowChanged(editor);renderWorkflowNodes(editor);renderWorkflowInspector(editor)};
  document.querySelectorAll('[data-remove-edge]').forEach(b=>b.onclick=()=>{w.edges.splice(Number(b.dataset.removeEdge),1);workflowChanged(editor);renderWorkflowEdges(editor);renderWorkflowInspector(editor)});
 }
}
function workflowSettings(editor){
 const w=editor.graph.value;
 dialog(`<form id="wf-settings-form"><div class="dialog-head"><h2>流程设置</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><div class="field"><label for="wf-name">工作流名称</label><input id="wf-name" required value="${esc(w.name)}"></div><div class="field"><label for="wf-limit">最多执行节点次数（含循环）</label><input id="wf-limit" type="number" min="1" max="1000" required value="${w.max_steps}"></div><label class="check"><input id="wf-enabled" type="checkbox" ${w.enabled?'checked':''}>允许启动新运行</label><h4>授权用户</h4><p class="hint">管理员始终有权访问；用户还需获得节点 Agent 的授权。</p>${editor.users.filter(u=>u.role!=='admin').map(u=>`<label class="check"><input type="checkbox" name="wf-user" value="${esc(u.user_id)}" ${w.authorized_users.includes(u.user_id)?'checked':''}>${esc(u.username||u.user_id)}</label>`).join('')}<h4>允许直接开始的节点</h4><p class="hint">默认从入口开始。其他入口只对之后的新运行生效。</p>${w.nodes.filter(n=>n.id!==w.entry).map(n=>`<label class="check"><input type="checkbox" name="wf-start" value="${esc(n.id)}" ${w.start_nodes.includes(n.id)?'checked':''}>${esc(n.name)}</label>`).join('')}</div><div class="dialog-footer"><button type="button" data-close>取消</button><button class="primary">应用设置</button></div></form>`);
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
  editor.graph.value=saved;editor.dirty=false;renderWorkflowNodes(editor);renderWorkflowInspector(editor);$('#wf-hint').textContent='选择节点配置 · 支持回退连线';history.replaceState({},'','/workflows/'+saved.id);workflowSaveStatus(editor);toast('工作流已保存');
 }catch(e){toast(e.status===409?'工作流已被其他页面更新。请导出当前改动，再重新打开合并。':e.message)}finally{button.disabled=false;if(workflowEditor===editor)$('#content').inert=false}
}
