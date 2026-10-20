'use strict';
let workflowEditor = null;
function leaveWorkflowEditor(){workflowEditor?.cancelZoom?.();workflowEditor?.cancelPan?.();workflowEditor?.cancelNodeDrag?.();workflowEditor?.resizeObserver?.disconnect();workflowEditor=null;if(typeof leaveWorkflowRun==='function')leaveWorkflowRun();}
window.addEventListener('beforeunload',event=>{if(workflowEditor?.dirty){event.preventDefault();event.returnValue='';}});

async function workflowsView(){
 const [items,runGroups]=await Promise.all([api('/api/workflows'),api('/api/workflow-run-groups')]);
 const activeIDs=new Set(items.map(w=>w.id));
 const history=runGroups.filter(w=>!activeIDs.has(w.id));
 $('#content').innerHTML=heading('智能体编排','把 Agent、外部动作与人工反馈连接起来。',state.me.admin?'<div class="wf-actions"><button id="wf-import">导入</button><button id="wf-connectors">管理 Connector</button><button id="wf-new" class="primary">＋ 创建智能体编排</button></div>':'')+
 `<div class="agent-grid">${items.map(w=>`<article class="card"><div class="agent-head"><span class="agent-icon">⌘</span><div><h2>${esc(w.name)}</h2><span class="muted small">版本 ${w.revision} · ${w.enabled?'已启用':'已停用'}</span></div></div><p class="muted">${w.nodes.length} 个节点 · ${w.edges.length} 条路径</p><div class="agent-foot"><a class="wf-link" data-nav href="/workflows/${esc(w.id)}">${state.me.admin?'打开编排':'查看流程'} ↗</a><a class="wf-link" data-nav href="/workflows/${esc(w.id)}/runs">运行记录 →</a>${state.me.admin?`<button class="quiet danger" data-delete-workflow="${esc(w.id)}">删除</button>`:''}</div></article>`).join('')||'<div class="empty"><strong>将协作过程变成可复用的流程</strong>创建智能体编排，从节点开始编排。</div>'}</div>${history.length?`<details class="wf-runs-list"><summary>历史运行（${history.length} 个编排）</summary><div class="agent-grid">${history.map(w=>`<article class="card"><h3>${esc(w.name)}</h3><a class="wf-link" data-nav href="/workflows/${esc(w.id)}/runs">运行记录 →</a></article>`).join('')}</div></details>`:''}`;
 if(state.me.admin){$('#wf-connectors').onclick=()=>connectorManager();$('#wf-new').onclick=()=>go('/workflows/new');$('#wf-import').onclick=()=>importWorkflow();}
 if(state.me.admin)document.querySelectorAll('[data-delete-workflow]').forEach(button=>button.onclick=async()=>{
  const item=items.find(w=>w.id===button.dataset.deleteWorkflow);
  if(!item||!confirm(`删除“${item.name}”？删除后不能再启动或恢复运行，已有运行记录和产物仍保留。`))return;
  button.disabled=true;
  try{await api('/api/workflows/'+item.id,'DELETE',{revision:item.revision});await workflowsView();toast('工作流已删除，历史运行已保留');}
  catch(error){button.disabled=false;toast(error.message);}
 });
}
function importWorkflow(){
 dialog(`<form id="wf-import-form"><div class="dialog-head"><h2>导入智能体编排</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><div class="field"><label for="wf-import-json">智能体编排 JSON</label><textarea id="wf-import-json" rows="14" required placeholder="粘贴导出的智能体编排配置"></textarea></div><button type="button" id="wf-load-file">选择 JSON 文件</button><p class="hint">导入为新副本；不会复制原智能体编排的用户授权。</p></div><div class="dialog-footer"><button type="button" data-close>取消</button><button class="primary">导入副本</button></div></form>`);
 $('#wf-load-file').onclick=()=>{
  const input=document.createElement('input');input.type='file';input.accept='.json,application/json';
  input.onchange=async()=>{try{const file=input.files?.[0];if(!file)return;if(file.size>512*1024)throw new Error('文件超过 512 KB');$('#wf-import-json').value=await file.text()}catch(e){toast(e.message)}};input.click();
 };
 $('#wf-import-form').onsubmit=async e=>{e.preventDefault();try{const graph=WorkflowGraph.import($('#wf-import-json').value,state.agents);$('#dialog').close();history.replaceState({},'','/workflows/new');await openWorkflowEditor(graph);toast('已导入副本。请核对 Agent 和授权后保存。')}catch(error){toast(error.message)}};
}
function exportWorkflow(editor){
 let copy;try{const selected=editor.graph.node(editor.selected);copy=workflowPayload(editor.graph.value,selected?workflowNodeDraft(editor,selected):null,editor.agentEnvDrafts,state.agents,editor.completionDrafts,editor.executionDrafts,editor.bindingDrafts,editor.executionEnvDrafts,editor.ruleSchemaDrafts)}catch(error){toast(error.message);return}
 copy.id='';copy.revision=0;copy.authorized_users=[];delete copy.updated_at;
 for(const node of copy.nodes)if(node.agent){node.agent.env={};node.agent.native_config='';}
 const text=JSON.stringify(copy,null,2);
 dialog(`<div class="dialog-head"><h2>导出智能体编排</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><p class="hint">分享导出已省略用户授权、环境变量和原生配置。导入后请重新配置环境与原生设置，并核对本机路径和工具连接。</p><div class="field"><label for="wf-export-json">智能体编排 JSON</label><textarea id="wf-export-json" rows="16" readonly>${esc(text)}</textarea></div></div><div class="dialog-footer"><button data-close>关闭</button><button id="wf-download" class="primary">下载 JSON</button></div>`);
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
 workflowEditor?.cancelZoom?.();workflowEditor?.cancelPan?.();workflowEditor?.cancelNodeDrag?.();workflowEditor?.resizeObserver?.disconnect();
 const upgraded=graph.upgradeForEditing(state.agents);
 const users=state.me.admin?await api('/api/users'):[];
 const [connectors,toolServers]=await Promise.all([api('/api/connectors'),state.me.admin?api('/api/tool-servers'):Promise.resolve([])]);
 const editor=workflowEditor={graph,selected:null,panelTab:'config',dirty:!graph.value.id||upgraded,readOnly:!state.me.admin,users,connectors,toolServers,connectSource:null,showAllEdges:false,zoom:1};
 const w=graph.value;
 if(w.nodes.length>1&&w.nodes.every(n=>(n.x||0)===(w.nodes[0].x||0)&&(n.y||0)===(w.nodes[0].y||0)))editor.positions=arrangedWorkflowPositions(graph);
 $('#content').classList.add('wf-content');
 $('#content').innerHTML=`<div class="wf-heading"><div><a href="/workflows" data-nav class="muted small">← 智能体编排</a><h1 id="wf-title">${esc(w.name)}</h1><span class="muted small" id="wf-save-state"></span></div><div class="wf-actions">${w.id?`<a class="wf-link" data-nav href="/workflows/${esc(w.id)}/runs">运行记录 →</a>`:''}<button id="wf-start" class="primary">运行</button><button id="wf-export">导出</button><button id="wf-hooks">通知 Hook</button>${editor.readOnly?'':'<button id="wf-import">导入副本</button><button id="wf-settings">流程设置</button><button id="wf-save" class="primary">保存</button>'}</div></div>
 <div class="wf-layout"><aside class="wf-palette"><span class="eyebrow">添加节点</span>${['agent','execution','approval','end'].map(kind=>`<button draggable="${!editor.readOnly}" data-kind="${kind}" ${editor.readOnly?'disabled':''}><span>${{agent:'◇',execution:'↗',approval:'◎',end:'◉'}[kind]}</span>${WorkflowGraph.kinds[kind]}</button>`).join('')}<p class="hint">拖到画布，或点击添加。<br>在节点内配置交接策略。</p></aside>
 <div class="wf-workarea"><div class="wf-canvas-tools" aria-label="画布工具"><div><button id="wf-overview">回到入口</button><button id="wf-fit">显示全图</button>${editor.readOnly?'':'<button id="wf-arrange">整理布局</button>'}<button id="wf-show-edges" aria-pressed="false">全部连线</button></div><div><button id="wf-zoom-out" aria-label="缩小画布">−</button><button id="wf-actual" aria-label="选择缩放比例" aria-haspopup="dialog" title="选择缩放比例">100%</button><button id="wf-zoom-in" aria-label="放大画布">＋</button></div></div><div id="wf-hint" class="wf-hint" role="status">Ctrl/Cmd＋滚轮缩放 · 点击画布后，空格＋拖动平移</div><div id="wf-viewport" tabindex="0" aria-label="智能体编排画布"><div id="wf-stage"><svg id="wf-lines" aria-label="节点连接"><defs><marker id="wf-arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8" fill="#8293b1"/></marker></defs><g id="wf-edge-group" transform="translate(80 80)"></g><path id="wf-draft-line" transform="translate(80 80)"/></svg><div id="wf-nodes"></div></div></div><div class="wf-canvas-footer">虚线：Agent 决策 · 实线：固定规则（◎ 人工选择） <span id="wf-count"></span></div></div>
 <aside id="wf-inspector" class="wf-inspector wf-detail-panel" aria-label="节点详情" hidden></aside></div>`;
 $('#wf-start').onclick=()=>{if(editor.dirty){toast('请先保存编排改动');return}startWorkflowDialog(editor.graph.value).catch(e=>toast(e.message))};
 $('#wf-hooks').onclick=()=>workflowHooksDialog(editor);
 $('#wf-export').onclick=()=>exportWorkflow(editor);
 if(!editor.readOnly){
  $('#wf-import').onclick=()=>{if(!editor.dirty||confirm('放弃当前未保存的改动并导入副本？'))importWorkflow()};
  $('#wf-settings').onclick=()=>workflowSettings(editor);
  $('#wf-save').onclick=()=>saveWorkflow(editor);
 }
 $('#wf-show-edges').onclick=()=>{editor.showAllEdges=!editor.showAllEdges;$('#wf-show-edges').textContent=editor.showAllEdges?'仅相关连线':'全部连线';$('#wf-show-edges').setAttribute('aria-pressed',String(editor.showAllEdges));renderWorkflowEdges(editor)};
 $('#wf-fit').onclick=()=>{closeWorkflowInspector(editor);fitWorkflowCanvas(editor)};
 $('#wf-actual').onclick=()=>workflowZoomDialog(editor);
 $('#wf-zoom-out').onclick=()=>{editor.fit=false;setWorkflowZoom(editor,workflowZoomLimit(editor.zoom)/1.25)};
 $('#wf-zoom-in').onclick=()=>{editor.fit=false;setWorkflowZoom(editor,workflowZoomLimit(editor.zoom)*1.25)};
 if(!editor.readOnly)$('#wf-arrange').onclick=()=>{rememberWorkflowNodeDraft(editor);editor.positions=arrangedWorkflowPositions(graph);materializeWorkflowPositions(editor);workflowChanged(editor);renderWorkflowNodes(editor);renderWorkflowInspector(editor);fitWorkflowCanvas(editor)};
 $('#wf-overview').onclick=()=>{const n=graph.node(graph.value.entry);if(n){editor.fit=false;editor.canvasOffset={x:80,y:80};setWorkflowZoom(editor,1);const p=workflowNodePosition(editor,n);$('#wf-viewport').scrollTo({left:Math.max(0,p.x),top:Math.max(0,p.y),behavior:'smooth'})}};
 document.querySelectorAll('[data-kind]').forEach(button=>{
  button.onclick=()=>{const p=workflowCanvasOffset(editor);addWorkflowNode(editor,button.dataset.kind,Math.max(0,80+$('#wf-viewport').scrollLeft/editor.zoom-p.x),Math.max(0,80+$('#wf-viewport').scrollTop/editor.zoom-p.y))};
  button.ondragstart=e=>e.dataTransfer.setData('application/x-platform-node',button.dataset.kind);
 });
 bindWorkflowNavigation(editor);
 const stage=$('#wf-stage');stage.ondragover=e=>{if(!editor.readOnly)e.preventDefault()};
 stage.ondrop=e=>{if(editor.readOnly)return;e.preventDefault();const kind=e.dataTransfer.getData('application/x-platform-node');if(!Object.hasOwn(WorkflowGraph.kinds,kind))return;const p=workflowPoint(e);addWorkflowNode(editor,kind,p.x,p.y)};
 stage.onpointerdown=e=>workflowPointerDown(e,editor);
 stage.onkeydown=e=>{const node=e.target.closest('[data-node]');if(!node)return;if(['Enter',' '].includes(e.key)){e.preventDefault();editor.panelTab='config';editor.selected=node.dataset.node;editor.previewRequest=null;renderWorkflowInspector(editor);renderWorkflowNodes(editor)}};
 renderWorkflowNodes(editor);renderWorkflowInspector(editor);workflowSaveStatus(editor);
 fitWorkflowCanvas(editor);
 editor.resizeObserver=new ResizeObserver(()=>{if(workflowEditor===editor&&editor.fit)fitWorkflowCanvas(editor)});editor.resizeObserver.observe($('#wf-viewport'));
}
function workflowSaveStatus(editor){
 if(workflowEditor!==editor)return;
 $('#wf-save-state').textContent=editor.dirty?'有未保存的改动':`已保存 · 版本 ${editor.graph.value.revision}`;
 $('#wf-title').textContent=editor.graph.value.name;
 const edges=editor.graph.value.edges;const visible=editor.showAllEdges||!editor.selected?edges:edges.filter(e=>e.source===editor.selected||e.target===editor.selected);$('#wf-count').textContent=`${editor.graph.value.nodes.length} 节点 / ${visible.length} 条可见连线（共 ${edges.length} 条）`;
}
function workflowChanged(editor){editor.dirty=true;editor.previewRequest=null;workflowSaveStatus(editor);}
function rememberWorkflowNodeDraft(editor){
 const node=editor.graph.node(editor.selected);if(!node||editor.readOnly||!$('#wf-node-form'))return;
 if(node.agent_id&&!editor.nodeFormEdited)return;
 const next=workflowNodeDraft(editor,node,false);
 if(JSON.stringify(next)!==editor.nodeFormBaseline&&JSON.stringify(next)!==JSON.stringify(node)){Object.assign(node,next);if(next.agent)delete node.agent_id;workflowChanged(editor)}
}
function arrangedWorkflowPositions(graph){
 const order=[],seen=new Set(),queue=[];
 for(const root of [graph.value.entry,...graph.value.nodes.map(n=>n.id)]){
  queue.push(root);
  while(queue.length){const id=queue.shift();if(seen.has(id)||!graph.node(id))continue;seen.add(id);order.push(id);for(const edge of graph.value.edges)if(edge.source===id)queue.push(edge.target)}
 }
 return Object.fromEntries(order.map((id,i)=>[id,{x:(i%4)*280,y:Math.floor(i/4)*200}]));
}
function workflowNodePosition(editor,node){return editor.positions?.[node.id]||{x:node.x||0,y:node.y||0};}
function materializeWorkflowPositions(editor){if(!editor.positions)return;for(const n of editor.graph.value.nodes)Object.assign(n,workflowNodePosition(editor,n));editor.positions=null;}
function workflowCanvasBounds(editor){
 const points=editor.graph.value.nodes.map(n=>workflowNodePosition(editor,n));
 return points.length?{left:Math.min(...points.map(p=>p.x))-80,top:Math.min(...points.map(p=>p.y))-100,right:Math.max(...points.map(p=>p.x+264)),bottom:Math.max(...points.map(p=>p.y+196))}:{left:0,top:0,right:240,bottom:160};
}
function workflowCanvasOffset(editor){return editor.canvasOffset||{x:80,y:80};}
function sizeWorkflowStage(editor){
 const b=workflowCanvasBounds(editor),v=$('#wf-viewport'),p=workflowCanvasOffset(editor);
 $('#wf-stage').style.width=Math.max(b.right+p.x,v.clientWidth/editor.zoom,(editor.viewExtent?.width||0)/editor.zoom)+'px';$('#wf-stage').style.height=Math.max(b.bottom+p.y,v.clientHeight/editor.zoom,(editor.viewExtent?.height||0)/editor.zoom)+'px';
 $('#wf-nodes').style.left=p.x+'px';$('#wf-nodes').style.top=p.y+'px';
 for(const selector of ['#wf-edge-group','#wf-draft-line'])$(selector).setAttribute('transform',`translate(${p.x} ${p.y})`);
}
// Scroll extents and padding are view state only. Expand them at an edge so
// cursor-anchored zoom and panning work even when the whole graph fits on screen.
function moveWorkflowView(editor,left,top){
 const view=$('#wf-viewport'),p={...workflowCanvasOffset(editor)};
 if(left<0){p.x-=left/editor.zoom;left=0}if(top<0){p.y-=top/editor.zoom;top=0}
 editor.canvasOffset=p;editor.viewExtent={width:left+view.clientWidth,height:top+view.clientHeight};
 sizeWorkflowStage(editor);view.scrollTo(left,top);
}
function workflowZoomLimit(zoom){return Math.max(.025,Math.min(2,zoom));}
// Dampen only outward movement beyond a limit, in logarithmic scale. The
// bounded response composes across input events; reversing has no stored debt.
function workflowGestureZoom(zoom,factor){
 const from=Math.log(zoom),delta=Math.log(factor),to=from+delta,low=Math.log(.025),high=Math.log(2);
 if(to>=low&&to<=high)return Math.exp(to);
 const direction=to>high?1:-1,bound=direction>0?high:low;
 if(delta*direction<=0)return Math.exp(to);
 const limit=.08,previous=Math.max(0,(from-bound)*direction),distance=(to-bound)*direction-previous;
 return Math.exp(bound+direction*(limit-(limit-previous)*Math.exp(-distance/limit)));
}
function setWorkflowZoom(editor,zoom,anchor){
 editor.cancelZoom?.();applyWorkflowZoom(editor,zoom,anchor);
}
function applyWorkflowZoom(editor,zoom,anchor,world,elastic=false){
 if(editor.canvasGesture||!Number.isFinite(zoom))return;
 const view=$('#wf-viewport'),old=editor.zoom||1,p=workflowCanvasOffset(editor);
 anchor||={x:view.clientWidth/2,y:view.clientHeight/2};
 world||={x:(view.scrollLeft+anchor.x)/old-p.x,y:(view.scrollTop+anchor.y)/old-p.y};
 editor.zoom=elastic?Math.max(.025*Math.exp(-.08),Math.min(2*Math.exp(.08),zoom)):workflowZoomLimit(zoom);$('#wf-stage').style.zoom=editor.zoom;
 moveWorkflowView(editor,(world.x+p.x)*editor.zoom-anchor.x,(world.y+p.y)*editor.zoom-anchor.y);
 $('#wf-actual').textContent=Math.round(workflowZoomLimit(editor.zoom)*1000)/10+'%';
}
function workflowZoomDialog(editor){
 editor.cancelZoom?.();
 dialog(`<form id="wf-zoom-form"><div class="dialog-head"><h2>缩放画布</h2><button type="button" data-close aria-label="关闭缩放设置">关闭</button></div><div class="dialog-body"><div class="wf-zoom-presets">${[25,50,75,100,125,150,200].map(value=>`<button type="button" data-zoom="${value}">${value}%</button>`).join('')}</div><div class="field"><label for="wf-zoom-value">缩放比例（%）</label><input id="wf-zoom-value" type="number" min="2.5" max="200" step="0.1" required value="${Math.round(editor.zoom*1000)/10}"><p class="hint">2.5%–200%。100% 恢复原尺寸。</p></div></div><div class="dialog-footer"><button class="primary">应用</button></div></form>`);
 const apply=value=>{if(workflowEditor!==editor)return;editor.fit=false;setWorkflowZoom(editor,value/100);$('#dialog').close();};
 $('#wf-zoom-form').onsubmit=e=>{e.preventDefault();if($('#wf-zoom-form').reportValidity())apply(Number($('#wf-zoom-value').value));};
 $('#dialog').querySelectorAll('[data-zoom]').forEach(button=>button.onclick=()=>apply(Number(button.dataset.zoom)));
}
function bindWorkflowNavigation(editor){
 const view=$('#wf-viewport');
 // Keep fractional world coordinates across paints: scrollLeft/Top are rounded
 // by the browser, so deriving the anchor again on every delta accumulates drift.
 let pending=null,frame=0,endTimer=0,settling=false;
 const reducedMotion=()=>window.matchMedia('(prefers-reduced-motion: reduce)').matches;
 const renderPending=()=>{
  applyWorkflowZoom(editor,pending.zoom,pending.anchor,pending.world,true);
  pending.left=view.scrollLeft;pending.top=view.scrollTop;
 };
 editor.cancelZoom=()=>{
  clearTimeout(endTimer);if(frame)cancelAnimationFrame(frame);frame=0;settling=false;
  const last=pending;pending=null;editor.pinch=null;
  if(editor.zoom!==workflowZoomLimit(editor.zoom))applyWorkflowZoom(editor,workflowZoomLimit(editor.zoom),last?.anchor,last?.world);
 };
 const finishZoom=()=>{
  clearTimeout(endTimer);if(frame)cancelAnimationFrame(frame);frame=0;
  if(!pending||workflowEditor!==editor)return;
  renderPending();const from=pending.zoom,target=workflowZoomLimit(from);
  if(from===target||reducedMotion()){applyWorkflowZoom(editor,target,pending.anchor,pending.world);pending=null;return;}
  settling=true;const start=performance.now();
  const settle=now=>{
   frame=0;if(!pending||workflowEditor!==editor)return;
   const t=Math.min(1,(now-start)/160),remaining=(1-t)**3;
   pending.zoom=t===1?target:Math.exp(Math.log(target)+(Math.log(from)-Math.log(target))*remaining);
   renderPending();
   if(t<1)frame=requestAnimationFrame(settle);else{pending=null;settling=false;}
  };
  frame=requestAnimationFrame(settle);
 };
 const queueZoom=(e,factor)=>{
  if(!Number.isFinite(factor)||factor<=0)return;
  clearTimeout(endTimer);
  if(settling){if(frame)cancelAnimationFrame(frame);frame=0;settling=false;}
  const r=view.getBoundingClientRect(),anchor={x:e.clientX-r.left-view.clientLeft,y:e.clientY-r.top-view.clientTop};
  if(!pending||anchor.x!==pending.anchor.x||anchor.y!==pending.anchor.y||view.scrollLeft!==pending.left||view.scrollTop!==pending.top){
   const p=workflowCanvasOffset(editor);
   pending={zoom:pending?.zoom||editor.zoom,anchor,world:{x:(view.scrollLeft+anchor.x)/editor.zoom-p.x,y:(view.scrollTop+anchor.y)/editor.zoom-p.y},left:view.scrollLeft,top:view.scrollTop};
  }
  pending.zoom=reducedMotion()?workflowZoomLimit(pending.zoom*factor):workflowGestureZoom(pending.zoom,factor);
  editor.fit=false;
  if(!frame)frame=requestAnimationFrame(()=>{
   frame=0;if(!pending||workflowEditor!==editor||editor.canvasGesture)return;
   renderPending();
  });
  // WheelEvent has no pinch-end phase. A short idle period ends Chrome's
  // gesture; Safari provides an explicit gestureend. Normal zoom never eases.
  if(!editor.pinch)endTimer=setTimeout(finishZoom,140);
 };
 view.addEventListener('wheel',e=>{
  if(!e.ctrlKey&&!e.metaKey){editor.cancelZoom();return;}
  e.preventDefault();if(editor.canvasGesture||editor.pinch)return;
  const pixels=e.deltaY*(e.deltaMode===1?16:e.deltaMode===2?view.clientHeight:1);
  // Chromium encodes pinch scale as -100 * log(scale) in DOM wheel deltaY.
  queueZoom(e,Math.exp(-Math.max(-500,Math.min(500,pixels))*.01));
 },{passive:false});
 view.addEventListener('gesturestart',e=>{e.preventDefault();clearTimeout(endTimer);if(frame)cancelAnimationFrame(frame);frame=0;settling=false;if(!editor.canvasGesture)editor.pinch={scale:1};},{passive:false});
 view.addEventListener('gesturechange',e=>{e.preventDefault();if(editor.pinch&&Number.isFinite(e.scale)&&e.scale>0){const factor=e.scale/editor.pinch.scale;editor.pinch.scale=e.scale;queueZoom(e,factor);}},{passive:false});
 view.addEventListener('gestureend',e=>{e.preventDefault();editor.pinch=null;finishZoom();},{passive:false});
 view.addEventListener('pointerdown',e=>{
  editor.cancelZoom();editor.panClick=false;
  if(e.button!==0||!editor.spacePan||editor.canvasGesture)return;
  e.preventDefault();e.stopImmediatePropagation();view.focus({preventScroll:true});
  const saved={x:e.clientX,y:e.clientY,left:view.scrollLeft,top:view.scrollTop,offset:{...workflowCanvasOffset(editor)},extent:editor.viewExtent,fit:editor.fit};
  editor.canvasGesture=true;editor.panClick=true;view.classList.add('wf-panning');
  const move=event=>{if(event.pointerId!==e.pointerId)return;editor.fit=false;editor.canvasOffset={...saved.offset};moveWorkflowView(editor,saved.left-(event.clientX-saved.x),saved.top-(event.clientY-saved.y));};
  const finish=event=>{
   if(event&&event.pointerId!==e.pointerId)return;
   view.removeEventListener('pointermove',move);view.removeEventListener('pointerup',finish);view.removeEventListener('pointercancel',cancel);view.removeEventListener('lostpointercapture',cancel);
   editor.canvasGesture=false;editor.cancelPan=null;view.classList.remove('wf-panning');
   if(view.hasPointerCapture(e.pointerId))view.releasePointerCapture(e.pointerId);
  };
  const cancel=event=>{if(event&&event.pointerId!==e.pointerId)return;editor.canvasOffset=saved.offset;editor.viewExtent=saved.extent;editor.fit=saved.fit;sizeWorkflowStage(editor);view.scrollTo(saved.left,saved.top);finish(event);};
  editor.cancelPan=cancel;
  view.setPointerCapture(e.pointerId);view.addEventListener('pointermove',move);view.addEventListener('pointerup',finish);view.addEventListener('pointercancel',cancel);view.addEventListener('lostpointercapture',cancel);
 },true);
 view.addEventListener('click',e=>{if(editor.panClick){e.preventDefault();e.stopImmediatePropagation();editor.panClick=false;}},true);
}
window.addEventListener('keydown',event=>{
 const editor=workflowEditor,view=$('#wf-viewport');
 if(event.code!=='Space'||!editor||!view||$('#dialog')?.open||event.target.closest('input,textarea,select,button,a,[contenteditable]:not([contenteditable="false"]),[role="button"]'))return;
 if(!view.contains(event.target)&&!view.matches(':hover'))return;
 event.preventDefault();editor.spacePan=true;view.classList.add('wf-pan-ready');
},true);
function releaseWorkflowPan(){if(workflowEditor){workflowEditor.spacePan=false;workflowEditor.pinch=null;$('#wf-viewport')?.classList.remove('wf-pan-ready');}}
window.addEventListener('keyup',event=>{if(event.code==='Space')releaseWorkflowPan();});
window.addEventListener('blur',()=>{workflowEditor?.cancelZoom?.();workflowEditor?.cancelPan?.();workflowEditor?.cancelNodeDrag?.();releaseWorkflowPan();});
function fitWorkflowCanvas(editor){
 editor.cancelZoom?.();
 const b=workflowCanvasBounds(editor),v=$('#wf-viewport'),width=b.right-b.left,height=b.bottom-b.top;
 const zoom=Math.max(.025,Math.min(1,(v.clientWidth-16)/width,(v.clientHeight-16)/height));
 editor.fit=true;editor.canvasOffset={x:(v.clientWidth/zoom-width)/2-b.left,y:(v.clientHeight/zoom-height)/2-b.top};
 editor.zoom=zoom;editor.viewExtent=null;$('#wf-stage').style.zoom=zoom;sizeWorkflowStage(editor);v.scrollTo(0,0);$('#wf-actual').textContent=Math.round(zoom*1000)/10+'%';
}
function workflowPoint(event){const rect=$('#wf-stage').getBoundingClientRect(),scale=workflowEditor?.zoom||1,p=workflowCanvasOffset(workflowEditor||{});return {x:(event.clientX-rect.left)/scale-p.x,y:(event.clientY-rect.top)/scale-p.y};}
function addWorkflowNode(editor,kind,x,y){if(editor.readOnly)return;materializeWorkflowPositions(editor);const n=editor.graph.add(kind,x,y);if(kind==='execution')n.rule_handoff=workflowRulePolicy();editor.panelTab='config';editor.selected=n.id;editor.previewRequest=null;workflowChanged(editor);renderWorkflowNodes(editor);renderWorkflowInspector(editor);}
function renderWorkflowNodes(editor){
 if(workflowEditor!==editor)return;
 const w=editor.graph.value;
 $('#wf-nodes').innerHTML=w.nodes.map(n=>`<div class="wf-node wf-${esc(n.kind)} ${n.id===editor.selected?'selected':''}" data-node="${esc(n.id)}"><button class="wf-port wf-in" data-port="in" title="连接到 ${esc(n.name)}" aria-label="连接到 ${esc(n.name)}" ${editor.readOnly?'disabled':''}></button><div class="wf-node-body" role="button" tabindex="0" aria-label="${esc(n.name)} 节点"><span class="wf-node-type">${esc(WorkflowGraph.kinds[n.kind])}${n.id===w.entry?' · 入口':''}</span><strong>${esc(n.name)}</strong><small>${n.kind==='agent'?esc(agentExecutionLabel(n.agent||state.agents.find(a=>a.id===n.agent_id))):n.kind==='end'?'完成运行':n.kind==='execution'?(esc(n.execution?.mode==='code'?({python:'Python 代码',typescript:'TypeScript 代码'}[n.execution.language]||'选择语言'):n.execution?.mode==='command'?'项目命令':n.execution?.tool||'选择执行工具')):n.kind==='connector'?'外部操作':'等待用户决策'}</small></div>${n.kind==='end'?'':`<button class="wf-port wf-out" data-port="out" title="从 ${esc(n.name)} 连线" aria-label="从 ${esc(n.name)} 连线" ${editor.readOnly?'disabled':''}></button>`}</div>`).join('')||'<div class="wf-empty">从左侧拖入第一个节点<br><small>例如：Agent → 人工确认 → 结束</small></div>';
 $('#wf-nodes').querySelectorAll('[data-node]').forEach(el=>{const n=workflowNodePosition(editor,editor.graph.node(el.dataset.node));el.style.left=n.x+'px';el.style.top=n.y+'px'});
 $('#wf-nodes').querySelectorAll('[data-port]').forEach(port=>port.onclick=e=>{if(editor.readOnly)return;e.stopPropagation();const id=port.closest('[data-node]').dataset.node;if(port.dataset.port==='out'){editor.connectSource=id;$('#wf-hint').textContent='选择目标节点左侧圆点，或拖拽完成连线'}else if(editor.connectSource){const source=editor.connectSource;editor.connectSource=null;workflowConnectDialog(editor,source,id)}});
 sizeWorkflowStage(editor);
 renderWorkflowEdges(editor);
}
function workflowCurve(a,b){const bend=Math.max(65,Math.abs(b.x-a.x)*.45);return `M ${a.x} ${a.y} C ${a.x+bend} ${a.y}, ${b.x-bend} ${b.y}, ${b.x} ${b.y}`;}
function renderWorkflowEdges(editor){
 const w=editor.graph.value;
 const visible=w.edges.filter(e=>editor.showAllEdges||!editor.selected||e.source===editor.selected||e.target===editor.selected);
 $('#wf-edge-group').innerHTML=w.edges.map((e,i)=>{
  if(!visible.includes(e))return '';
  const a=editor.graph.node(e.source),b=editor.graph.node(e.target);if(!a||!b)return '';
  const ap=workflowNodePosition(editor,a),bp=workflowNodePosition(editor,b);
  const mode=editor.graph.edgeMode(e),from={x:ap.x+184,y:ap.y+48},to={x:bp.x,y:bp.y+48};
  // Backward edges travel around the nodes rather than across their content.
  const backwards=bp.x<=ap.x;
  const siblings=w.edges.filter(x=>x.source===e.source&&x.target===e.target),slot=siblings.indexOf(e),spread=siblings.length>1?(slot-(siblings.length-1)/2)*32:0;
  const lane=Math.max(ap.y,bp.y)+135+(siblings.length>1?slot:i%3)*32;
  const curve=backwards?`M ${from.x} ${from.y} C ${from.x+65} ${from.y}, ${from.x+65} ${lane}, ${from.x} ${lane} L ${to.x-35} ${lane} Q ${to.x-55} ${lane}, ${to.x-55} ${lane-20} L ${to.x-55} ${to.y+25} Q ${to.x-55} ${to.y}, ${to.x} ${to.y}`:spread?`M ${from.x} ${from.y} C ${from.x+(to.x-from.x)*.45} ${from.y+spread*1.34}, ${to.x-(to.x-from.x)*.45} ${to.y+spread*1.34}, ${to.x} ${to.y}`:workflowCurve(from,to);
  const labelX=(from.x+to.x)/2,labelY=backwards?lane-8:(from.y+to.y)/2+spread-10;
  const rule=typeof workflowRuleForEdge==='function'?workflowRuleForEdge(a,e):null;
  const tip=[e.description,rule?.when?'条件：'+rule.when:rule?.else?'else':''].filter(Boolean).join('\n');
  const type=a.kind==='approval'?'人工选择':mode==='handoff'?'Agent 自主交接':'固定规则';
  return `<g class="wf-edge wf-edge-${mode} ${rule&&['rule_invalid','rule_expression'].includes(e.route)?'wf-edge-error':''}" data-edge="${i}" role="button" tabindex="0" aria-label="${esc(a.name)} → ${esc(b.name)}，${type}，${esc(b.name)}"><title>${esc(tip.length>200?tip.slice(0,197)+'…':tip||type)}</title><path d="${curve}" marker-end="url(#wf-arrow)"/><text x="${labelX}" y="${labelY}">${esc(e.title||e.description||(a.kind==='agent'&&w.context_version>=1?b.name:e.route))}</text></g>`;
 }).join('');
 $('#wf-count').textContent=`${w.nodes.length} 节点 · ${visible.length}/${w.edges.length} 连线可见`;
 $('#wf-edge-group').querySelectorAll('[data-edge]').forEach(el=>{el.onclick=()=>workflowEdgeDialog(editor,Number(el.dataset.edge));el.onkeydown=e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();el.onclick()}}});
}
function workflowEdgeDialog(editor,index){
 const edge=editor.graph.value.edges[index];if(!edge)return;
 if(editor.graph.node(edge.source)?.kind==='execution'){rememberWorkflowNodeDraft(editor);editor.selected=edge.source;editor.panelTab='handoff';renderWorkflowInspector(editor);renderWorkflowNodes(editor);return;}
 workflowConnectDialog(editor,edge.source,edge.target,index);
}
function workflowPointerDown(e,editor){
 if(e.button!==0)return;
 const nodeEl=e.target.closest('[data-node]');if(!nodeEl){if(!e.target.closest('[data-edge]')){closeWorkflowInspector(editor);$('#wf-viewport').focus({preventScroll:true})}return;}
 const id=nodeEl.dataset.node,n=editor.graph.node(id),port=e.target.closest('[data-port]');
 if(port?.dataset.port==='in')return;
 if(editor.selected!==id)rememberWorkflowNodeDraft(editor);
 if(editor.readOnly){editor.panelTab='config';editor.selected=id;renderWorkflowInspector(editor);renderWorkflowNodes(editor);return;}
 editor.canvasGesture=true;
 const stage=$('#wf-stage'),initial=workflowPoint(e),origin={...workflowNodePosition(editor,n)},previousPositions=editor.positions,previousCoordinates=editor.graph.value.nodes.map(n=>({id:n.id,x:n.x,y:n.y}));let moved=false;
 if(editor.selected!==id){editor.panelTab='config';editor.previewRequest=null;}editor.selected=id;
 const move=event=>{
  const p=workflowPoint(event);if(Math.abs(p.x-initial.x)+Math.abs(p.y-initial.y)>4)moved=true;
  if(port){$('#wf-draft-line').setAttribute('d',workflowCurve({x:origin.x+184,y:origin.y+48},p));return}
  if(!moved)return;materializeWorkflowPositions(editor);editor.fit=false;
  editor.graph.move(id,origin.x+p.x-initial.x,origin.y+p.y-initial.y);nodeEl.style.left=n.x+'px';nodeEl.style.top=n.y+'px';renderWorkflowEdges(editor);
 };
 const finish=event=>{
  editor.canvasGesture=false;editor.cancelNodeDrag=null;
  stage.removeEventListener('pointermove',move);stage.removeEventListener('pointerup',finish);stage.removeEventListener('pointercancel',cancel);stage.removeEventListener('lostpointercapture',cancel);$('#wf-draft-line').setAttribute('d','');
  if(stage.hasPointerCapture(e.pointerId))stage.releasePointerCapture(e.pointerId);
  if(event.type==='pointercancel'||event.type==='lostpointercapture'){renderWorkflowNodes(editor);return}
  if(port){if(!moved){editor.connectSource=id;$('#wf-hint').textContent='选择目标节点左侧圆点完成连线'}else{const target=document.elementFromPoint(event.clientX,event.clientY)?.closest('[data-node]');if(target){editor.connectSource=null;workflowConnectDialog(editor,id,target.dataset.node)}}}
  else{if(moved)workflowChanged(editor);renderWorkflowNodes(editor);renderWorkflowInspector(editor)}
 };
 const cancel=event=>{if(!port){for(const p of previousCoordinates)Object.assign(editor.graph.node(p.id),p);editor.positions=previousPositions}finish(event)};
 editor.cancelNodeDrag=()=>cancel({type:'pointercancel'});
 stage.setPointerCapture(e.pointerId);stage.addEventListener('pointermove',move);stage.addEventListener('pointerup',finish);stage.addEventListener('pointercancel',cancel);stage.addEventListener('lostpointercapture',cancel);
}
function workflowConnectDialog(editor,source,target,index=null){
 if(workflowEditor!==editor)return;
 if(editor.graph.node(source)?.kind==='execution'){rememberWorkflowNodeDraft(editor);editor.selected=source;editor.panelTab='handoff';renderWorkflowInspector(editor);return;}
 const old=index==null?null:editor.graph.value.edges[index],n=editor.graph.node(source),mode=old?editor.graph.edgeMode(old):(n.kind==='agent'&&n.exit_mode!=='complete'?'handoff':'automatic');
 const route=old?.route||(n.kind==='agent'?editor.graph.nextRoute(source):'next');
 dialog(`<form id="wf-connect"><div class="dialog-head"><h2>${old?'目标与策略':'连接节点'}</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><p>从 ${esc(n?.name)} 继续</p><fieldset ${editor.readOnly?'disabled':''}><div class="field"><label for="wf-edge-target">目标节点</label><select id="wf-edge-target" required>${editor.graph.value.nodes.map(t=>`<option value="${esc(t.id)}" ${t.id===target?'selected':''}>${esc(t.name)} · ${esc(t.id)}</option>`).join('')}</select></div><div class="field"><label for="wf-edge-mode">交接方式</label><select id="wf-edge-mode" ${n.exit_mode?'disabled':''}>${n.kind==='agent'?`<option value="handoff" ${mode==='handoff'?'selected':''}>Agent 自主交接 · 虚线</option>`:''}<option value="automatic" ${mode==='automatic'?'selected':''}>${n.kind==='approval'?'用户选择后的固定流转':'固定完成 · 实线'}</option></select><p class="hint">自主交接由 Agent 按策略选择目标；固定完成由平台推进。普通聊天回复不会结束节点。</p></div>${['connector','execution'].includes(n.kind)?`<div class="field"><label for="wf-route">执行结果</label><select id="wf-route"><option value="next" ${route==='next'?'selected':''}>成功</option><option value="failed" ${route==='failed'?'selected':''}>失败</option>${!['next','failed'].includes(route)?`<option value="${esc(route)}" selected>兼容路径：${esc(route)}</option>`:''}</select></div>`:n.kind==='approval'?`<div class="field"><label for="wf-route">用户选择标识</label><input id="wf-route" required pattern="[a-zA-Z][a-zA-Z0-9_-]{0,63}" value="${esc(route)}"></div>`:''}<div class="field"><label for="wf-edge-description">交接策略</label><textarea id="wf-edge-description" rows="3" maxlength="1000" placeholder="例如：检查通过后交给验收；发现设计遗漏时交回修订">${esc(old?.description||'')}</textarea><p class="hint">Agent 根据交接策略选择目标。固定路径在节点完成后继续。</p></div></fieldset></div><div class="dialog-footer"><button type="button" data-close>关闭</button>${editor.readOnly?'':`${old?'<button type="button" id="wf-edge-delete" class="danger">删除连线</button>':''}<button class="primary">${old?'应用设置':'添加目标'}</button>`}</div></form>`);
 if(editor.readOnly)return;
 $('#wf-connect').onsubmit=e=>{e.preventDefault();try{
  const next={route:['connector','execution','approval'].includes(n.kind)?$('#wf-route').value.trim():route,target:$('#wf-edge-target').value,mode:$('#wf-edge-mode').value,description:$('#wf-edge-description').value.trim()};
  if(old)editor.graph.updateEdge(index,next);else editor.graph.connect(source,next.route,next.target,next.mode,next.description);
  workflowChanged(editor);$('#dialog').close();editor.selected=source;editor.panelTab='handoff';editor.previewRequest=null;renderWorkflowNodes(editor);renderWorkflowInspector(editor);
 }catch(error){toast(error.message)}};
 if(old)$('#wf-edge-delete').onclick=()=>{editor.graph.removeEdge(index);workflowChanged(editor);$('#dialog').close();renderWorkflowNodes(editor);renderWorkflowInspector(editor)};
}
function workflowAgentConfig(n,agents=state.agents){
 const source=n.agent||agents.find(a=>a.id===n.agent_id);
 if(!source)throw new Error('节点「'+n.name+'」的原智能体配置不可用，请恢复配置后保存');
 return WorkflowGraph.agentConfig(source);
}
function workflowAgentFields(editor,n){
 if(editor.readOnly)return '<p class="hint">节点执行配置由管理员管理。</p>';
 try{return agentConfigFields(workflowAgentConfig(n),editor.toolServers||[],'wf-agent',editor.agentEnvDrafts?.[n.id],true,editor.graph.value.environment)}
 catch(error){return '<p class="callout">'+esc(error.message)+'</p>'}
}
function readWorkflowAgent(editor,n,validate){
 (editor.agentEnvDrafts||={})[n.id]=$('#wf-agent-env').value;
 return readAgentConfig(workflowAgentConfig(n),'wf-agent',validate,true);
}
// Build an independent request snapshot; reading or validating never edits the graph.
function workflowPayload(value,nodeDraft,envDrafts,agents,completionDrafts,executionDrafts,bindingDrafts,executionEnvDrafts,ruleSchemaDrafts){
 const graph=new WorkflowGraph(value);
 if(nodeDraft){const node=graph.node(nodeDraft.id);Object.assign(node,JSON.parse(JSON.stringify(nodeDraft)));if(nodeDraft.agent)delete node.agent_id;}
 graph.upgradeForEditing(agents);
 if(typeof applyRuleSchemaDrafts==='function')applyRuleSchemaDrafts(graph,ruleSchemaDrafts);
 for(const node of graph.value.nodes){
  if(Object.hasOwn(completionDrafts||{},node.id))applyCompletionDraft(node,completionDrafts[node.id]);
  if(Object.hasOwn(executionDrafts||{},node.id))applyExecutionDraft(node,executionDrafts[node.id]);
  if(Object.hasOwn(bindingDrafts||{},node.id))applyExecutionBindings(node,bindingDrafts[node.id]);
  if(Object.hasOwn(executionEnvDrafts||{},node.id))applyExecutionEnvRefs(node,executionEnvDrafts[node.id]);
  graph.validateAgent(node);
  if(node.kind==='agent'){
   node.agent=workflowAgentConfig(node,agents);delete node.agent_id;
   if(Object.hasOwn(envDrafts||{},node.id))node.agent.env=agentEnvironment(envDrafts[node.id]);
   graph.validateAgent(node);
  }
 }
 return graph.value;
}
function workflowNodeDraft(editor,n,validate=true){
 const next={...n};if(editor.readOnly||editor.selected!==n.id||!$('#wf-node-form'))return next;
 if(n.kind==='agent'&&true){const draft=editor.completionDrafts?.[n.id];if(draft){if(validate)applyCompletionDraft(next,draft);else{try{applyCompletionDraft(next,draft)}catch{}}}}
 next.name=$('#wf-node-name').value.trim();next.x=Number($('#wf-node-x').value);next.y=Number($('#wf-node-y').value);
 if(editor.positions){const p=workflowNodePosition(editor,n);if(next.x===p.x&&next.y===p.y){next.x=n.x;next.y=n.y}else{materializeWorkflowPositions(editor)}}
 if(n.kind==='agent'){
  next.agent=readWorkflowAgent(editor,n,validate);delete next.agent_id;
  {next.allow_user_input=$('#wf-allow-user-input').checked;next.continuation_limit=Number($('#wf-continuation').value);next.execution_timeout_seconds=Number($('#wf-timeout').value)}
 }
 if(n.kind==='connector'){next.connector_id=$('#wf-node-connector').value;next.connector_input=readConnectorNodeFields()}
 if(n.kind==='execution'){
  const mode=n.execution?.mode||'mcp';
  next.execution={mode,...(n.execution?.output?{output:n.execution.output}:{})};next.execution_timeout_seconds=Number($('#wf-execution-timeout').value);
  if(mode==='mcp'){
   next.execution.server_id=$('#wf-execution-server').value;next.execution.tool=$('#wf-execution-tool').value;
   const raw=$('#wf-execution-arguments').value;(editor.executionDrafts||={})[n.id]=raw;
   const refs=$('#wf-execution-bindings').value;(editor.bindingDrafts||={})[n.id]=refs;
   if(validate){applyExecutionDraft(next,raw);applyExecutionBindings(next,refs)}else{try{applyExecutionDraft(next,raw)}catch{}try{applyExecutionBindings(next,refs)}catch{}}
  }else if(mode==='code'){next.execution.language=$('#wf-execution-language').value;next.execution.code=$('#wf-execution-code').value}
  else{next.execution.command=$('#wf-execution-command').value}
  if(mode!=='mcp'){const raw=$('#wf-execution-env').value;(editor.executionEnvDrafts||={})[n.id]=raw;if(validate)applyExecutionEnvRefs(next,raw);else{try{applyExecutionEnvRefs(next,raw)}catch{}}}
 }
 if(n.kind==='approval')next.prompt=$('#wf-node-prompt').value;
 if(n.kind==='agent')delete next.prompt;
 if(validate)editor.graph.validateAgent(next);return next;
}
function applyWorkflowNode(editor,n){
 const next=workflowNodeDraft(editor,n);editor.graph.move(n.id,next.x,next.y);Object.assign(n,next);if(next.agent)delete n.agent_id;workflowChanged(editor);renderWorkflowNodes(editor);
}
function closeWorkflowInspector(editor){
 if(workflowEditor!==editor)return;
 rememberWorkflowNodeDraft(editor);
 const id=editor.selected;editor.selected=null;editor.previewRequest=null;renderWorkflowInspector(editor);renderWorkflowNodes(editor);
 document.querySelector?.('[data-node="'+id+'"] .wf-node-body')?.focus();
}
window.addEventListener('keydown',event=>{
 if(event.key==='Escape'&&!$('#dialog')?.open&&workflowEditor?.selected){event.preventDefault();closeWorkflowInspector(workflowEditor)}
});
function workflowContextHelp(editor){
 const legacy=(editor.graph.value.context_version||0)===0;
 dialog(`<div class="dialog-head"><h2 id="wf-context-help-title">上下文如何组装</h2><button type="button" data-close aria-label="关闭上下文说明">关闭</button></div><div class="dialog-body wf-context-explanation">
 <h3>角色指令：规定职责</h3><p>节点中配置的角色指令与任务正文分开传递。使用 Codex 时，角色指令与原生配置中已有的开发者指令合并，通过 <code>developerInstructions</code> 传入；平台还会补充回复方式等执行约束。它不是这里展示的工作流提示词，也不是模型的系统提示词。</p>
 <h3>首次进入节点：组装任务消息</h3>${legacy?'<p>此工作流使用旧式输入：第一条用户消息是运行任务；节点说明、流转规则和前序结果随角色指令传递。请在运行记录中核对该节点实际保存的角色指令和任务输入。</p>':`<p>平台按以下顺序拼成一条用户消息；不存在的材料不追加：</p><ol><li>当前节点的工作流规则：交接目标、完成条件、等待方式，以及本次执行来源。</li><li>启动运行时提交的原始任务。</li><li>本次运行中已处理的用户补充和修正，按消息顺序排列；随后是人工回退及当前恢复反馈。</li><li>最近一次 Agent 交接的结论、输入和产物引用。</li><li>各人工确认节点最近一次有效确认，以及各执行节点最近一次回执。完整日志和产物正文按需读取，不在此处全部展开。</li></ol>`}
 <h3>后续补充：继续原会话</h3><p>给当前节点追加的用户输入，作为新消息送入同一原生会话，按运行状态引导当前回合或排队接续；不会每次重新拼接上述整段消息。原生执行器负责维护会话历史。</p>
 <h3>交接：创建下一节点的会话</h3><p>下一节点使用自己的角色配置和重新组装的任务消息。同一个 Run 共享工作区，交接传递结论和产物引用，不复制上一个 Agent 的完整聊天记录。</p>
 <h3>预览能看到什么</h3><p>这里预览的是当前草稿生成的工作流提示词和任务占位说明，不是完整任务消息或模型的全部上下文。实际保存的输入可在运行记录的节点详情中查看。项目 <code>AGENTS.md</code>、Skills、工具结果及历史上下文由原生执行器加载和管理；工具与执行权限通过独立配置生效，任务文字不授予权限。</p></div>`);
 $('#dialog').setAttribute('aria-labelledby','wf-context-help-title');
 $('#dialog').addEventListener('close',()=>$('#dialog').removeAttribute('aria-labelledby'),{once:true});
}
async function previewWorkflowNode(editor,n){
 const request={};editor.previewRequest=request;
 try{
  const draft=new WorkflowGraph(editor.graph.value),next=workflowNodeDraft(editor,n);Object.assign(draft.node(n.id),next);if(next.agent)delete draft.node(n.id).agent_id;
  editor.panelTab='preview';
  for(const key of ['config','handoff','preview']){const pane=$('#wf-pane-'+key),tab=$('#wf-tab-'+key);if(pane)pane.hidden=key!=='preview';if(tab){tab.setAttribute('aria-selected',String(key==='preview'));tab.tabIndex=key==='preview'?0:-1;}}
  $('#wf-preview-content').innerHTML='<p class="hint" role="status">正在展开当前草稿…</p>';
  const result=await api('/api/workflows/preview','POST',{workflow:draft.value,node_id:n.id});
  if(workflowEditor!==editor||editor.selected!==n.id||editor.previewRequest!==request)return;
  $('#wf-preview-content').innerHTML=`<p class="hint">根据当前节点配置生成，包含交接、完成和等待规则。具体任务和上游结果在运行时追加。</p><button id="wf-refresh-preview" class="quiet">刷新预览</button><div class="field"><label for="wf-preview-input">工作流提示词（Prompt）</label><textarea id="wf-preview-input" rows="14" readonly>${esc(result.input)}</textarea></div>`;
  $('#wf-refresh-preview').onclick=()=>previewWorkflowNode(editor,n);
 }catch(error){if(workflowEditor===editor&&editor.selected===n.id&&editor.previewRequest===request)$('#wf-preview-content').innerHTML='<p class="callout" role="alert">'+esc(error.message)+'</p>';}
}
function renderWorkflowInspector(editor){
 if(workflowEditor!==editor)return;
 editor.nodeFormEdited=false;
 editor.ruleValidation=null;
 editor.previewRequest=null;
 const n=editor.graph.node(editor.selected),w=editor.graph.value;
 if(!n){$('#wf-inspector').hidden=true;$('#wf-inspector').innerHTML='';return}
 $('#wf-inspector').hidden=false;
 const outgoing=w.edges.map((e,i)=>({...e,index:i})).filter(e=>e.source===n.id);
 const content=`<form id="wf-node-form"><fieldset ${editor.readOnly?'disabled':''}><div class="field"><label for="wf-node-name">名称</label><input id="wf-node-name" required maxlength="100" value="${esc(n.name)}"></div>${n.kind==='agent'?workflowAgentFields(editor,n):''}${n.kind==='execution'?workflowExecutionFields(editor,n):''}${n.kind==='connector'?`<div class="field"><label for="wf-node-connector">使用 Connector</label><select id="wf-node-connector"><option value="">请选择</option>${editor.connectors.map(c=>`<option value="${esc(c.id)}" ${c.id===n.connector_id?'selected':''}>${esc(c.name)} · ${esc(connectorKinds[c.kind])}</option>`).join('')}</select></div>${connectorNodeFields(editor,n)}`:''}${n.kind==='approval'?`<div class="field"><label for="wf-node-prompt">确认内容</label><textarea id="wf-node-prompt" rows="7" maxlength="32000">${esc(n.prompt||'')}</textarea></div>`:''}${n.kind==='agent'?`<div class="field"><label><input id="wf-allow-user-input" type="checkbox" ${n.allow_user_input!==false?'checked':''}> 允许请求用户输入</label><p class="hint">允许智能体提出问题，并在当前会话等待答复。</p></div><div class="field"><label for="wf-continuation">自动继续次数</label><input id="wf-continuation" type="number" min="0" max="10" required value="${n.continuation_limit??0}"><p class="hint">未完成且未明确等待时在原会话继续。0 关闭，最多 10 次。</p></div><div class="field"><label for="wf-timeout">持续推进期限（秒）</label><input id="wf-timeout" type="number" min="0" max="86400" required value="${n.execution_timeout_seconds??0}"><p class="hint">从本次节点开始计时，期限到达后不再自动续跑。0 关闭；最多 86400 秒。</p></div>`:''}<details><summary>画布位置</summary><div class="wf-coordinates"><div class="field"><label for="wf-node-x">横坐标</label><input id="wf-node-x" type="number" min="0" max="10000" value="${workflowNodePosition(editor,n).x}"></div><div class="field"><label for="wf-node-y">纵坐标</label><input id="wf-node-y" type="number" min="0" max="10000" value="${workflowNodePosition(editor,n).y}"></div></div></details><button type="submit">应用设置</button></fieldset></form>${n.kind==='agent'?'<button id="wf-preview" class="quiet">预览会话输入</button>':''}<h4>交接目标与策略</h4>${n.kind==='agent'?workflowExitFields(editor,n):''}<div class="wf-route-list">${outgoing.map(e=>`<div><span data-edit-edge="${e.index}" role="button" tabindex="0"><strong>${editor.graph.edgeMode(e)==='handoff'?'┄':'━'} ${esc(editor.graph.node(e.target)?.name)} · ${esc(e.target)}</strong><small>${editor.graph.edgeMode(e)==='handoff'?'自主交接':'固定完成'} · ${esc(e.description||'尚未填写策略')}${n.kind!=='agent'?' · '+esc(e.route):''}</small></span>${editor.readOnly?'':`<button data-remove-edge="${e.index}" aria-label="删除到 ${esc(e.target)} 的连线">×</button>`}</div>`).join('')||'<p class="hint">'+(n.kind==='end'?'此节点结束运行。':'从右侧圆点连接目标，或添加交接目标。')+'</p>'}</div>${editor.readOnly||n.kind==='end'?'':'<button id="wf-add-target">添加交接目标</button>'}${editor.readOnly?'':`<div class="wf-inspector-actions">${n.id===w.entry?'<span class="hint">当前入口节点</span>':'<button id="wf-set-entry">设为入口</button>'}<button id="wf-delete-node" class="danger">删除节点</button></div>`}`;
 const split=content.indexOf('<h4>交接目标与策略</h4>');
 const tabs=[{key:'config',label:'配置',body:content.slice(0,split)},{key:'handoff',label:n.kind==='execution'?'交接策略（基于规则）':'交接',body:n.kind==='execution'?workflowRulesFields(editor,n)+(editor.readOnly?'':content.slice(content.indexOf('<div class="wf-inspector-actions">'))):content.slice(split)}];
 if(n.kind==='agent')tabs.push({key:'preview',label:'输入预览',body:'<div id="wf-preview-content"><p class="hint">查看当前节点的工作流提示词，包含交接、完成和等待规则。</p><button id="wf-load-preview" class="primary">生成输入预览</button></div>'});
 const active=tabs.some(t=>t.key===editor.panelTab)?editor.panelTab:'config';editor.panelTab=active;
 $('#wf-inspector').innerHTML=workflowPanelMarkup('wf',n.name,n.kind==='agent'?agentExecutionLabel(n.agent||state.agents.find(a=>a.id===n.agent_id)):WorkflowGraph.kinds[n.kind],tabs,active,n.kind==='agent');
 bindWorkflowPanel('wf',tabs,active,key=>{editor.panelTab=key;if(key==='preview')previewWorkflowNode(editor,n)},()=>closeWorkflowInspector(editor));
 if(n.kind==='execution')bindWorkflowRules(editor,n);
 if(n.kind==='agent'){bindWorkflowExit(editor,n);$('#wf-context-help').onclick=()=>workflowContextHelp(editor);}
 if(n.kind==='agent')$('#wf-load-preview').onclick=()=>previewWorkflowNode(editor,n);
 document.querySelectorAll('[data-edit-edge]').forEach(el=>{el.onclick=()=>workflowEdgeDialog(editor,Number(el.dataset.editEdge));el.onkeydown=e=>{if(e.key==='Enter')el.onclick()}});
 $('#wf-node-form').onsubmit=e=>{e.preventDefault();if(editor.readOnly)return;try{applyWorkflowNode(editor,n)}catch(error){toast(error.message)}};
 if(n.kind==='agent')$('#wf-preview').onclick=()=>previewWorkflowNode(editor,n);
 if(!editor.readOnly){
  const syncDraft=()=>{try{editor.nodeFormEdited=true;Object.assign(n,workflowNodeDraft(editor,n,false));if(n.agent)delete n.agent_id;$('#wf-panel-title').textContent=n.name;const meta=$('#wf-inspector .wf-panel-meta');if(meta&&n.kind==='agent')meta.textContent=agentExecutionLabel(n.agent);workflowChanged(editor);renderWorkflowNodes(editor)}catch(error){toast(error.message)}};
  if(n.kind==='execution'){
   $('#wf-execution-timeout').oninput=syncDraft;
   $('#wf-execution-mode').onchange=()=>{const mode=$('#wf-execution-mode').value;rememberWorkflowNodeDraft(editor);n.execution=mode==='mcp'?{mode,server_id:'',tool:'',arguments:{}}:mode==='code'?{mode,language:'python',code:''}:{mode,command:''};if(editor.executionDrafts)delete editor.executionDrafts[n.id];if(editor.bindingDrafts)delete editor.bindingDrafts[n.id];if(editor.executionEnvDrafts)delete editor.executionEnvDrafts[n.id];workflowChanged(editor);renderWorkflowInspector(editor);renderWorkflowNodes(editor)};
   if(n.execution.mode==='mcp'){
    $('#wf-execution-arguments').oninput=syncDraft;$('#wf-execution-bindings').oninput=syncDraft;
    $('#wf-execution-tool').onchange=()=>{syncDraft();renderWorkflowInspector(editor)};
    $('#wf-execution-server').onchange=()=>{syncDraft();n.execution.tool='';workflowChanged(editor);renderWorkflowInspector(editor)};
   }else if(n.execution.mode==='code'){$('#wf-execution-language').onchange=syncDraft;$('#wf-execution-code').oninput=syncDraft}
   else{$('#wf-execution-command').oninput=syncDraft}
   if(n.execution.mode!=='mcp')$('#wf-execution-env').oninput=syncDraft;
  }
  if(n.kind==='connector')$('#wf-node-form').querySelectorAll('input,textarea,select').forEach(field=>field.oninput=syncDraft);
  for(const selector of ['#wf-node-name','#wf-node-x','#wf-node-y',...(n.kind==='approval'?['#wf-node-prompt']:[]),...(n.kind==='agent'?['#wf-allow-user-input','#wf-continuation','#wf-timeout']:[])])$(selector).oninput=syncDraft;
  if(n.kind==='agent'){for(const id of ['executor','model','instructions','sandbox','network','elevation','seed','skills','native','trust','inherit','env'])if($('#wf-agent-'+id))$('#wf-agent-'+id).oninput=syncDraft;$('#wf-agent-tools')?.querySelectorAll('input,select').forEach(field=>field.onchange=syncDraft);}
  if($('#wf-node-connector')&&n.kind==='connector')$('#wf-node-connector').onchange=()=>{n.name=$('#wf-node-name').value.trim();n.connector_id=$('#wf-node-connector').value;n.connector_input={};workflowChanged(editor);renderWorkflowInspector(editor);renderWorkflowNodes(editor)};
  if(n.kind!=='end'&&n.kind!=='execution')$('#wf-add-target').onclick=()=>{const target=w.nodes.find(t=>t.id!==n.id);if(!target){toast('请先添加目标节点');return}workflowConnectDialog(editor,n.id,target.id)};
  $('#wf-delete-node').onclick=()=>{editor.graph.remove(n.id);if(editor.agentEnvDrafts)delete editor.agentEnvDrafts[n.id];if(editor.completionDrafts)delete editor.completionDrafts[n.id];if(editor.executionDrafts)delete editor.executionDrafts[n.id];if(editor.bindingDrafts)delete editor.bindingDrafts[n.id];if(editor.executionEnvDrafts)delete editor.executionEnvDrafts[n.id];if(editor.ruleSchemaDrafts)delete editor.ruleSchemaDrafts[n.id];if(editor.ruleSamples)delete editor.ruleSamples[n.id];editor.selected=null;workflowChanged(editor);renderWorkflowNodes(editor);renderWorkflowInspector(editor)};
  if(n.id!==w.entry)$('#wf-set-entry').onclick=()=>{w.entry=n.id;workflowChanged(editor);renderWorkflowNodes(editor);renderWorkflowInspector(editor)};
  document.querySelectorAll('[data-remove-edge]').forEach(b=>b.onclick=()=>{editor.graph.removeEdge(Number(b.dataset.removeEdge));workflowChanged(editor);renderWorkflowEdges(editor);renderWorkflowInspector(editor)});
  try{editor.nodeFormBaseline=JSON.stringify(workflowNodeDraft(editor,n,false))}catch{editor.nodeFormBaseline=null}
 }
}
// Raw JSON lives in the editor until valid, so closing a panel cannot discard a typo.
function applyCompletionDraft(node,draft){
 let schema;try{schema=draft.schema.trim()?WorkflowGraph.parseJSON(draft.schema):undefined}catch(error){if(error.message.includes('数值'))throw error;throw new Error('节点「'+node.name+'」的输出格式不是有效 JSON')}
 if(schema!==undefined&&(!schema||typeof schema!=='object'||Array.isArray(schema)||schema.type!=='object'))throw new Error('输出格式须为 type=object 的 JSON Schema');
 if(schema===undefined)delete node.completion_schema;else node.completion_schema=schema;
 node.completion_instructions=draft.instructions;
}
function workflowExitFields(editor,n){
 const mode=n.exit_mode||(editor.graph.agentMode(n.id)==='automatic'?'complete':editor.graph.agentMode(n.id)),draft=editor.completionDrafts?.[n.id]||{schema:n.completion_schema?JSON.stringify(n.completion_schema,null,2):'',instructions:n.completion_instructions||''};
 return `<fieldset ${editor.readOnly?'disabled':''}><div class="wf-exit-modes" role="group" aria-label="交接方式"><button type="button" id="wf-exit-handoff" aria-pressed="${mode==='handoff'}">自主交接 · handoff</button><button type="button" id="wf-exit-complete" aria-pressed="${mode==='complete'}">固定流转 · complete_node</button></div><p class="hint">${mode==='handoff'?'Agent 根据各目标的交接策略选择下一步，并提交交接内容。':mode==='complete'?'Agent 按输出格式提交结果，校验通过后沿唯一连线继续。':'请选择一种交接方式。'}</p><div ${mode!=='complete'?'hidden':''}><div class="field"><label for="wf-completion-schema">输出格式（JSON Schema）</label><textarea id="wf-completion-schema" rows="10" maxlength="32000" spellcheck="false" placeholder='{"type":"object","properties":{"passed":{"type":"boolean"}},"required":["passed"],"additionalProperties":false}'>${esc(draft.schema)}</textarea><p class="hint">约束 complete_node 的 inputs。填写字段名、类型、必填项与字段描述；留空时使用默认的字符串键值对象。</p></div><div class="field"><label for="wf-completion-instructions">输出填写说明</label><textarea id="wf-completion-instructions" rows="5" maxlength="16000" placeholder="说明各字段从哪里取值、如何判断以及何时提交，可附示例。">${esc(draft.instructions)}</textarea></div><p class="hint">summary 记录完成结论，artifacts 列出实际产物路径。下游程序从 previous_results 中读取本节点的 result.inputs。</p></div></fieldset>`;
}
function bindWorkflowExit(editor,n){
 if(editor.readOnly)return;
 const remember=()=>{
  const draft={schema:$('#wf-completion-schema').value,instructions:$('#wf-completion-instructions').value};
  (editor.completionDrafts||={})[n.id]=draft;
  // A previously implicit mode becomes explicit only after an edit.
  if(!n.exit_mode&&editor.graph.agentMode(n.id)!=='mixed')editor.graph.setExitMode(n.id,editor.graph.agentMode(n.id)==='automatic'?'complete':'handoff');
  try{applyCompletionDraft(n,draft)}catch{}
  workflowChanged(editor);
 };
 $('#wf-completion-schema').oninput=remember;$('#wf-completion-instructions').oninput=remember;
 const apply=(mode,route)=>{try{editor.graph.setExitMode(n.id,mode,route);workflowChanged(editor);renderWorkflowNodes(editor);renderWorkflowInspector(editor)}catch(error){toast(error.message)}};
 $('#wf-exit-handoff').onclick=()=>apply('handoff');
 $('#wf-exit-complete').onclick=()=>{
  const edges=editor.graph.value.edges.filter(e=>e.source===n.id);
  if(edges.length<=1){apply('complete');return}
  dialog(`<form id="wf-exit-select"><div class="dialog-head"><h2>选择固定流转目标</h2></div><div class="dialog-body"><p>切换后只保留所选连线，其余出站连线将移除。</p><div class="field"><label for="wf-exit-target">保留的目标</label><select id="wf-exit-target">${edges.map(e=>`<option value="${esc(e.route)}">${esc(editor.graph.node(e.target)?.name)} · ${esc(e.description||e.route)}</option>`).join('')}</select></div></div><div class="dialog-footer"><button type="button" data-close>取消</button><button class="primary">切换并保留目标</button></div></form>`);
  $('#wf-exit-select').onsubmit=e=>{e.preventDefault();const route=$('#wf-exit-target').value;$('#dialog').close();apply('complete',route)};
 };
}
function workflowSettings(editor){
 const w=editor.graph.value;
 dialog(`<form id="wf-settings-form"><div class="dialog-head"><h2>流程设置</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><div class="field"><label for="wf-name">智能体编排名称</label><input id="wf-name" required value="${esc(w.name)}"></div><div class="field"><label for="wf-environment">执行环境</label><select id="wf-environment"><option value="" ${!w.environment?'selected':''}>本机已有工作区</option><option value="opensandbox" ${w.environment==='opensandbox'?'selected':''}>OpenSandbox · 每次运行独立环境</option></select></div><div class="field"><label for="wf-limit">最多执行节点次数（含循环）</label><input id="wf-limit" type="number" min="1" max="1000" required value="${w.max_steps}"></div><label class="check"><input id="wf-enabled" type="checkbox" ${w.enabled?'checked':''}>允许启动新运行</label><h4>授权用户</h4><p class="hint">管理员始终有权访问；节点会话使用本流程的授权。</p>${editor.users.filter(u=>u.role!=='admin').map(u=>`<label class="check"><input type="checkbox" name="wf-user" value="${esc(u.user_id)}" ${w.authorized_users.includes(u.user_id)?'checked':''}>${esc(u.username||u.user_id)}</label>`).join('')}<h4>允许直接开始的节点</h4><p class="hint">默认从入口开始。其他入口只对之后的新运行生效。</p>${w.nodes.filter(n=>n.id!==w.entry).map(n=>`<label class="check"><input type="checkbox" name="wf-start" value="${esc(n.id)}" ${w.start_nodes.includes(n.id)?'checked':''}>${esc(n.name)}</label>`).join('')}</div><div class="dialog-footer"><button type="button" data-close>取消</button><button class="primary">应用设置</button></div></form>`);
 $('#wf-settings-form').onsubmit=e=>{e.preventDefault();w.name=$('#wf-name').value.trim();w.environment=$('#wf-environment').value;w.max_steps=Number($('#wf-limit').value);w.enabled=$('#wf-enabled').checked;w.authorized_users=[...document.querySelectorAll('[name="wf-user"]:checked')].map(el=>el.value);w.start_nodes=[...document.querySelectorAll('[name="wf-start"]:checked')].map(el=>el.value);workflowChanged(editor);$('#dialog').close()};
}
async function saveWorkflow(editor){
 const button=$('#wf-save');button.disabled=true;
 try{
  // Apply the visible node form before saving, including text not yet blurred.
  if($('#wf-node-form')&&!$('#wf-node-form').checkValidity()){$('#wf-tab-config')?.click();$('#wf-node-form').reportValidity();return}
  const selected=editor.graph.node(editor.selected),w=workflowPayload(editor.graph.value,selected?workflowNodeDraft(editor,selected):null,editor.agentEnvDrafts,state.agents,editor.completionDrafts,editor.executionDrafts,editor.bindingDrafts,editor.executionEnvDrafts,editor.ruleSchemaDrafts);
  $('#content').inert=true;
  const saved=await api('/api/workflows'+(w.id?'/'+w.id:''),w.id?'PUT':'POST',w);
  if(workflowEditor!==editor)return;
  editor.graph.value=saved;editor.dirty=false;renderWorkflowNodes(editor);renderWorkflowInspector(editor);$('#wf-hint').textContent='选择节点配置交接策略 · 悬浮连线查看条件';history.replaceState({},'','/workflows/'+saved.id);workflowSaveStatus(editor);toast('智能体编排已保存');
 }catch(e){toast(e.status===409?'智能体编排已被其他页面更新。请导出当前改动，再重新打开合并。':e.message)}finally{button.disabled=false;if(workflowEditor===editor)$('#content').inert=false}
}

const workflowHookEvents={'node.started':'Agent 节点启动','approval.requested':'等待执行审批','approval.resolved':'执行审批已处理','agent.reply.completed':'Agent 最终回复完成','handoff.after':'交接完成','run.completed':'整次运行完成','run.failed':'运行失败','run.execution_limit_reached':'达到节点执行次数上限'};
function workflowHooksDialog(editor){
 const hooks=JSON.parse(JSON.stringify(editor.graph.value.hooks||[]));
 const draw=()=>{
 dialog(`<div class="dialog-head"><h2>事件通知 Hook</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><p class="hint">事件决定时机，动作决定做什么，正文模板映射数据。通知不阻塞节点。已配置 Agent 回复通知时，执行审批和达到执行次数上限的停顿也发送到同一 Issue。</p><div id="wf-hook-list">${hooks.map((h,i)=>`<div class="wf-hook-row"><strong>${esc(workflowHookEvents[h.event])}</strong><small>${esc(h.node||'全部节点')} → ${esc(editor.connectors.find(c=>c.id===h.connector_id)?.name||h.connector_id)}</small><button data-hook-edit="${i}">编辑</button><button data-hook-remove="${i}" ${editor.readOnly?'disabled':''}>删除</button></div>`).join('')||'<p>没有配置通知。平台不会自动向外发送消息。</p>'}</div>${editor.readOnly?'':'<button id="wf-hook-add">添加通知</button>'}</div><div class="dialog-footer"><button data-close>取消</button>${editor.readOnly?'':'<button id="wf-hooks-apply" class="primary">应用配置</button>'}</div>`);
 if(!editor.readOnly){$('#wf-hook-add').onclick=()=>edit(null);$('#wf-hooks-apply').onclick=()=>{editor.graph.value.hooks=hooks;workflowChanged(editor);$('#dialog').close()};document.querySelectorAll('[data-hook-remove]').forEach(b=>b.onclick=()=>{hooks.splice(Number(b.dataset.hookRemove),1);draw()})}
 document.querySelectorAll('[data-hook-edit]').forEach(b=>b.onclick=()=>edit(Number(b.dataset.hookEdit)));
 };
 const edit=index=>{
 const h=index==null?{event:'agent.reply.completed',node:'',input:{},body:'{{text}}\n\n[在平台继续]({{conversation_url}})'}:hooks[index];
 dialog(`<form id="wf-hook-form"><div class="dialog-head"><h2>通知规则</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><fieldset ${editor.readOnly?'disabled':''}><div class="field"><label for="hook-event">触发时机</label><select id="hook-event">${Object.entries(workflowHookEvents).map(([id,label])=>`<option value="${id}" ${id===h.event?'selected':''}>${label}</option>`).join('')}</select></div><div class="field"><label for="hook-node">作用节点</label><select id="hook-node"><option value="">全部节点</option>${editor.graph.value.nodes.map(n=>`<option value="${esc(n.id)}" ${h.node===n.id?'selected':''}>${esc(n.name)}</option>`).join('')}</select></div><div class="field"><label for="hook-action">通知动作</label><select id="hook-action" required><option value="">选择已配置的动作</option>${editor.connectors.filter(c=>c.kind==='github.issue_comment').map(c=>`<option value="${esc(c.id)}" ${c.id===h.connector_id?'selected':''}>${esc(c.name)}</option>`).join('')}</select><p class="hint">当前已支持 GitHub Issue 评论动作。凭证由 Connector 管理，不放进正文。</p></div><div class="field"><label for="hook-issue-node">Issue 来自哪个节点的关联结果</label><select id="hook-issue-node"><option value="">使用固定 Issue 编号</option>${editor.graph.value.nodes.filter(n=>n.kind==='connector'&&['github.issue','github.issue_create'].includes(editor.connectors.find(c=>c.id===n.connector_id)?.kind)).map(n=>`<option value="${esc(n.id)}" ${n.id===h.input.issue_node?'selected':''}>${esc(n.name)}</option>`).join('')}</select></div><div class="field"><label for="hook-issue">固定 Issue 编号</label><input id="hook-issue" type="number" min="1" value="${h.input.issue_number||''}"></div><div class="field"><label for="hook-body">通知正文模板</label><textarea id="hook-body" rows="6" required maxlength="16000">${esc(h.body)}</textarea><p class="hint">字段：{{text}} 原文、{{node_name}} 节点名称、{{summary}} 交接说明、{{target}} 目标、{{artifacts}} 产物、{{approval_status}} 审批结果、{{execution_limit}} 节点执行上限、{{conversation_url}} 会话链接、{{run_url}} 运行链接。只做取值替换，不执行表达式。</p></div></fieldset></div><div class="dialog-footer"><button type="button" id="hook-back">返回规则列表</button>${editor.readOnly?'':'<button class="primary">保存此规则</button>'}</div></form>`);
 $('#hook-back').onclick=draw;
 $('#wf-hook-form').onsubmit=e=>{e.preventDefault();if(editor.readOnly)return;const issueNode=$('#hook-issue-node').value,issueNumber=Number($('#hook-issue').value);if(!issueNode&&!issueNumber){toast('请选择 Issue 来源或填写编号');return}const next={event:$('#hook-event').value,node:$('#hook-node').value,connector_id:$('#hook-action').value,input:issueNode?{issue_node:issueNode}:{issue_number:issueNumber},body:$('#hook-body').value};if(index==null)hooks.push(next);else hooks[index]=next;draw()};
 };
 draw();
}

function applyExecutionDraft(node,raw){
 let args;try{args=WorkflowGraph.parseJSON(raw)}catch(error){throw new Error('工具参数 JSON 无效：'+error.message)}
 if(!args||typeof args!=='object'||Array.isArray(args))throw new Error('工具参数须为 JSON 对象');
 node.execution={...node.execution,arguments:args};
}
function applyExecutionEnvRefs(node,raw){
 let refs;try{refs=WorkflowGraph.parseJSON(raw.trim()||'{}')}catch(error){throw new Error('环境变量引用 JSON 无效：'+error.message)}
 if(!refs||typeof refs!=='object'||Array.isArray(refs))throw new Error('环境变量引用须为 JSON 对象');
 node.execution={...node.execution};if(Object.keys(refs).length)node.execution.env_refs=refs;else delete node.execution.env_refs;
}
function applyExecutionBindings(node,raw){
 let bindings;try{bindings=WorkflowGraph.parseJSON(raw.trim()||'{}')}catch(error){throw new Error('参数引用 JSON 无效：'+error.message)}
 if(!bindings||typeof bindings!=='object'||Array.isArray(bindings))throw new Error('参数引用须为 JSON 对象');
 node.execution={...node.execution};if(Object.keys(bindings).length)node.execution.bindings=bindings;else delete node.execution.bindings;
}
function workflowExecutionFields(editor,node){
 const v=node.execution||{},servers=editor.toolServers||[],server=servers.find(s=>s.id===v.server_id),tools=server?.tools||[],tool=tools.find(t=>t.name===v.tool);
 const raw=editor.executionDrafts?.[node.id]??JSON.stringify(v.arguments||{},null,2),bindings=editor.bindingDrafts?.[node.id]??JSON.stringify(v.bindings||{},null,2);
 return `<div class="field"><label for="wf-execution-mode">执行方式</label><select id="wf-execution-mode">${[['code','代码'],['command','项目命令'],['mcp','MCP 工具']].map(([key,label])=>`<option value="${key}" ${v.mode===key?'selected':''}>${label}</option>`).join('')}</select></div>${v.mode==='code'?`<div class="field"><label for="wf-execution-language">语言</label><select id="wf-execution-language"><option value="python" ${v.language==='python'?'selected':''}>Python</option><option value="typescript" ${v.language==='typescript'?'selected':''}>TypeScript</option></select></div><div class="field"><label for="wf-execution-code">代码</label><textarea id="wf-execution-code" rows="14" maxlength="64000" spellcheck="false">${esc(v.code||'')}</textarea></div>`:v.mode==='command'?`<div class="field"><label for="wf-execution-command">项目命令</label><textarea id="wf-execution-command" rows="7" maxlength="64000" spellcheck="false" placeholder="例如：make verify、go test ./...、mvn verify">${esc(v.command||'')}</textarea></div>`:`<div class="field"><label for="wf-execution-server">工具服务</label><select id="wf-execution-server" required><option value="">请选择</option>${v.server_id&&!server?`<option value="${esc(v.server_id)}" selected>${esc(v.server_id)} · 不可用</option>`:''}${servers.map(s=>`<option value="${esc(s.id)}" ${s.id===v.server_id?'selected':''}>${esc(s.name)}${s.enabled?'':' · 已停用'}</option>`).join('')}</select></div><div class="field"><label for="wf-execution-tool">工具</label><select id="wf-execution-tool" required><option value="">请选择</option>${v.tool&&!tool?`<option value="${esc(v.tool)}" selected>${esc(v.tool)} · 不可用</option>`:''}${tools.map(t=>`<option value="${esc(t.name)}" ${t.name===v.tool?'selected':''}>${esc(t.title||t.name)}</option>`).join('')}</select>${tool?.description?`<p class="hint">${esc(tool.description)}</p>`:''}</div><div class="field"><label for="wf-execution-arguments">工具参数（JSON）</label><textarea id="wf-execution-arguments" rows="10" maxlength="64000" spellcheck="false">${esc(raw)}</textarea></div><details ${Object.keys(v.bindings||{}).length?'open':''}><summary>引用任务输入或节点结果</summary><div class="field"><label for="wf-execution-bindings">参数引用（JSON）</label><textarea id="wf-execution-bindings" rows="6" maxlength="64000" spellcheck="false">${esc(bindings)}</textarea><p class="hint">按工具参数名填写引用，替换对应固定值。省略 node 使用本次任务输入；填写 node 使用该节点最近一次已结束的结果。路径缺失或类型不符时停止执行。</p><pre>${esc(JSON.stringify({title:{path:'/input'},body:{node:'report',path:'/result/inputs/report'}},null,2))}</pre><p class="hint">可引用的节点：${editor.graph.value.nodes.filter(n=>n.id!==node.id).map(n=>esc(n.name)+' · '+esc(n.id)).join('、')||'尚无其他节点'}。路径使用 JSON Pointer，例如 /parameters/commit；字段名中的 / 写作 ~1，~ 写作 ~0。</p></div></details>${tool?`<details><summary>参数格式</summary><pre>${esc(JSON.stringify(tool.inputSchema,null,2))}</pre></details>`:''}`}${v.mode!=='mcp'?`<details ${Object.keys(v.env_refs||{}).length?'open':''}><summary>环境变量引用</summary><div class="field"><label for="wf-execution-env">环境变量引用（JSON）</label><textarea id="wf-execution-env" rows="5" spellcheck="false">${esc(editor.executionEnvDrafts?.[node.id]??JSON.stringify(v.env_refs||{},null,2))}</textarea><p class="hint">填写命令需要的变量名及管理员配置的引用名，例如 {"GITHUB_TOKEN":"PROJECT_GITHUB_TOKEN"}。实际值不会保存到工作流；缺少配置时停止执行。</p></div></details><p class="hint">在本次运行的工作区执行。通过 WORKFLOW_INPUT_FILE 读取 JSON 文件，包含任务输入、参数与已有结果。</p>`:''}<div class="field"><label for="wf-execution-timeout">执行超时（秒）</label><input id="wf-execution-timeout" type="number" min="1" max="86400" required value="${node.execution_timeout_seconds||60}"></div>`;
}
