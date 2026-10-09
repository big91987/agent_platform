'use strict';
let workflowRunTimer=null;
let workflowRunPage=null;
function leaveWorkflowRun(){if(workflowRunTimer)clearInterval(workflowRunTimer);workflowRunTimer=null;workflowRunPage=null;}
const workflowStatus={pending:'待开始',running:'执行中',waiting:'等待处理',stopping:'正在停止',stopped:'已停止',failed:'需处理',completed:'已完成',cancelled:'已取消'};
async function startWorkflowDialog(w){
 if(!w.id)throw new Error('请先保存智能体编排');
 const entries=[...new Set([w.entry,...w.start_nodes])];
 dialog(`<form id="wf-start-form"><div class="dialog-head"><h2>运行 ${esc(w.name)}</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><div class="field"><label for="wf-task">你要完成什么？</label><textarea id="wf-task" rows="4" required placeholder="用一句话描述任务"></textarea></div><div class="field"><label for="wf-workspace">工作区目录</label><input id="wf-workspace" required placeholder="本机已有项目的绝对路径"><p class="hint">所有节点使用这份工作区。请为独立任务使用独立目录。</p></div>${entries.length>1?`<div class="field"><label for="wf-entry">从哪开始</label><select id="wf-entry">${entries.map(id=>`<option value="${esc(id)}">${esc(w.nodes.find(n=>n.id===id)?.name)}</option>`).join('')}</select></div>`:''}<p class="hint">使用已保存的版本 ${w.revision}。之后修改编排不会改变本次运行。</p></div><div class="dialog-footer"><button type="button" data-close>取消</button><button class="primary">开始运行</button></div></form>`);
 const inputKey="workflow:"+w.id;
 $('#wf-start-form').onsubmit=async e=>{e.preventDefault();e.submitter.disabled=true;try{const run=await api('/api/workflow-runs','POST',platformInputs.body(inputKey,{workflow_id:w.id,input:$('#wf-task').value,workspace_path:$('#wf-workspace').value,start_node:$('#wf-entry')?.value||w.entry}));platformInputs.accepted(inputKey);$('#dialog').close();go('/workflow-runs/'+run.id)}catch(err){toast(err.message);e.submitter.disabled=false}};
}
async function workflowRunsList(workflowID,runs){
 runs=runs||await api('/api/workflow-runs?workflow_id='+encodeURIComponent(workflowID));
 return `<section class="wf-runs-list"><h2>运行记录</h2><p class="hint">点击任务查看节点进度、执行记录和当前 Agent 会话。</p>${runs.length?runs.map(r=>`<a data-nav class="wf-run-row" href="/workflow-runs/${esc(r.id)}"><span><strong>${esc(r.definition.name)}</strong><small>${esc(r.input.slice(0,100))}</small></span><span class="wf-run-status status-${esc(r.status)}">${esc(workflowStateLabel(r.status,r.steps?.at(-1)?.wait_kind))} · 查看进度 →</span></a>`).join(''):'<p class="muted">此工作流还没有运行。打开编排，输入任务即可开始。</p>'}</section>`;
}
async function workflowRunsView(id){
 const runs=await api('/api/workflow-runs?workflow_id='+encodeURIComponent(id));
 let workflow;
 try{workflow=await api('/api/workflows/'+id)}catch(error){if(![403,404].includes(error.status)||!runs.length)throw error}
 const name=workflow?.name||runs[0].definition.name;
 const action=workflow?`<a data-nav class="wf-link" href="/workflows/${esc(id)}">打开编排 ↗</a>`:'';
 $('#content').innerHTML=heading(name+' · 运行记录','点击一次运行查看节点进度，再进入对应 Agent 会话。',action)+await workflowRunsList(id,runs);
}
async function workflowRunView(id){
 workflowRunPage={id,signature:'',delivery:null,deliveryRequested:false};
 $('#content').innerHTML='<div id="wf-run-header"></div><div class="wf-run-layout"><section id="wf-run-history"></section><aside id="wf-run-controls" class="card"></aside></div><aside id="wf-step-details" class="wf-detail-panel wf-step-details" aria-label="节点执行详情" hidden></aside>';
 await refreshWorkflowRun(id);
 workflowRunTimer=setInterval(()=>{if(workflowRunPage?.id===id&&!document.hidden)refreshWorkflowRun(id).catch(e=>toast(e.message))},1500);
}
function workflowExternalLink(url,label){
 try{const parsed=new URL(url);if(!['http:','https:'].includes(parsed.protocol))return esc(label)}catch{return esc(label)}
 return `<a class="wf-link" href="${esc(url)}" target="_blank" rel="noopener noreferrer">${esc(label)} ↗</a>`;
}
function workflowDeliveryHTML(d){
 const pr=workflowExternalLink(d.pull_request_url,d.merged?'查看已合并 PR':'查看草稿 PR / 审查合并');
 const actions=workflowExternalLink(d.actions_url,'查看 GitHub Actions');
 const changed=d.run_head_sha&&d.current_head_sha&&d.run_head_sha!==d.current_head_sha?'<p class="callout">PR 在本次 Run 的 QA 后更新；原 QA 结论不覆盖当前 PR 提交。</p>':'';
 if(!d.merged)return `<p>${pr} · ${actions}</p><p class="hint">PR ${esc(d.state)}${d.draft?' · Draft':''}，尚未合并；没有部署版本。</p>${changed}`;
 const deployments=d.deployments.length?d.deployments.map((item,index)=>`<div class="wf-hook-row"><strong>${esc(item.environment)} · ${esc(item.state)}</strong><small>版本 ${esc(item.version)}</small>${index===0&&item.state==='success'&&item.url?`<p>${workflowExternalLink(item.url,'打开效果地址')}</p>`:''}${item.log_url?`<p>${workflowExternalLink(item.log_url,'查看部署记录')}</p>`:''}</div>`).join(''):'<p class="hint">尚无包含此次合并提交的部署记录；到 GitHub Actions 查看准备或失败状态。</p>';
 return `<p>${pr} · ${actions}</p><p>已合并提交：<code>${esc(d.main_sha)}</code></p>${changed}${deployments}`;
}
function renderWorkflowDelivery(){
 const panel=$('#wf-delivery');if(!panel||!workflowRunPage)return;
 const state=workflowRunPage.delivery;
 panel.innerHTML=`<h3>合并与部署</h3><p class="hint">以下状态从 GitHub 只读查询，部署由仓库流程执行。</p><button id="wf-delivery-refresh" class="quiet">刷新部署状态</button>${state?.value?workflowDeliveryHTML(state.value):`<p class="hint">${esc(state?.error||'正在查询 GitHub…')}</p>`}`;
 $('#wf-delivery-refresh').onclick=()=>loadWorkflowDelivery(workflowRunPage.id);
}
async function loadWorkflowDelivery(id){
 if(workflowRunPage?.id!==id)return;
 const receiptKey=workflowRunPage.deliveryKey;
 workflowRunPage.delivery={};renderWorkflowDelivery();
 try{const value=await api('/api/workflow-runs/'+id+'/delivery');if(workflowRunPage?.id===id&&workflowRunPage.deliveryKey===receiptKey){workflowRunPage.delivery={value};renderWorkflowDelivery()}}
 catch(error){if(workflowRunPage?.id===id&&workflowRunPage.deliveryKey===receiptKey){workflowRunPage.delivery={error:error.message};renderWorkflowDelivery()}}
}
async function refreshWorkflowRun(id){
 const r=await api('/api/workflow-runs/'+id);if(workflowRunPage?.id!==id)return;
 const notices=await api('/api/workflow-runs/'+id+'/notifications');if(workflowRunPage?.id!==id)return;
 const signature=JSON.stringify([r,notices]);if(workflowRunPage.signature===signature)return;workflowRunPage.signature=signature;
 workflowRunPage.run=r;
 const step=r.steps.at(-1),node=r.definition.nodes.find(n=>n.id===step.node_id),active=['running','waiting','stopping'].includes(r.status);
 const executionLimit=r.max_steps||r.definition.max_steps,limitReached=r.seq>=executionLimit;
 const limitFailure=r.status==='failed'&&limitReached&&step.result&&r.error==='maximum node executions reached; inspect loop before starting another run';
 $('#wf-run-header').innerHTML=`<div class="wf-heading"><div><a data-nav href="/workflows/${esc(r.workflow_id)}/runs" class="muted small">← 运行记录</a><h1>${esc(r.definition.name)} <span class="wf-run-status status-${esc(r.status)}">${esc(workflowStateLabel(r.status,r.steps?.at(-1)?.wait_kind))}</span></h1><span class="muted small">版本 ${r.definition.revision} · 第 ${r.seq} 次节点执行</span></div></div><p class="wf-run-input">${esc(r.input)}</p>${r.error?`<div class="callout" role="alert">${esc(limitFailure?'已达到运行次数上限；请检查结果后选择继续节点。':r.error)}</div>`:''}`;
 $('#wf-run-history').innerHTML=r.steps.map(s=>{const n=r.definition.nodes.find(n=>n.id===s.node_id);return `<article class="card wf-run-step ${s.seq===r.seq?'current':''}"><div class="wf-step-heading"><span class="wf-step-number">${s.seq}</span><h3><button class="wf-step-select" data-step-details="${s.seq}" aria-label="${esc(n?.name||s.node_id)} · 查看节点详情">${esc(n?.name||s.node_id)} <span>↗</span></button></h3><span class="wf-run-status status-${esc(s.status)}">${esc(workflowStateLabel(s.status,s.wait_kind))}</span></div>${s.result?`<p class="wf-step-summary">${esc(s.result.summary.slice(0,120))}${s.result.summary.length>120?'…':''}</p><div class="hint">交接路径：${esc(s.result.route)}</div>${s.result.artifacts?.length?`<ul>${s.result.artifacts.map(a=>`<li>${/^https:\/\//.test(a)?`<a href="${esc(a)}" target="_blank" rel="noopener noreferrer">${esc(a)}</a>`:esc(a)}</li>`).join('')}</ul>`:''}`:''}${s.connector_receipt?`<p class="hint">外部回执${s.connector_receipt.exit_code!=null?' · 退出码 '+s.connector_receipt.exit_code:''}${s.connector_receipt.recovered?' · 已核对外部结果':''}</p>`:''}${s.connector_receipt?.log?`<a class="wf-link" href="/api/workflow-runs/${encodeURIComponent(r.id)}/steps/${s.seq}/output?format=text" target="_blank" rel="noopener noreferrer">查看命令日志${s.connector_receipt.log.truncated?"（已达保留上限）":""} ↗</a>`:''}${s.wait_reason?`<p class="callout">${s.result||['completed','cancelled'].includes(s.status)?'历史等待记录 · ':''}${esc(workflowWaitLabel(s.wait_kind))}：${esc(s.wait_reason)}</p>`:''}${n?.kind==='agent'?`<p class="hint">自动继续 ${s.continuations||0} 次</p><button class="quiet" data-step-context="${s.seq}">查看实际会话输入</button>`:''}${s.error?`<p class="hint">${esc(s.error)}</p>`:''}${s.conversation_id?`<a class="wf-link" data-nav href="/conversations/${esc(s.conversation_id)}">${s.seq===r.seq&&active?'进入 Agent 会话':'查看会话记录'} ↗</a>`:''}</article>`}).join('');
 document.querySelectorAll('[data-step-details]').forEach(button=>button.onclick=()=>workflowStepDetails(id,Number(button.dataset.stepDetails),'result'));
 document.querySelectorAll('[data-step-context]').forEach(button=>button.onclick=()=>workflowStepContextDialog(id,Number(button.dataset.stepContext)));
 if(notices.length){$('#wf-run-history').insertAdjacentHTML('beforeend',`<section class="card wf-run-step"><h3>通知记录</h3><p class="hint">通知失败独立处理，不会重跑 Agent 或交接。结果未知时只查询外部回执。</p>${notices.map(n=>`<div class="wf-hook-row"><strong>${esc(workflowHookEvents[n.event]||n.event)} · ${esc(r.definition.nodes.find(v=>v.id===n.node)?.name||n.node)}</strong><small>${esc({pending:'待发送',sending:'发送中',succeeded:'已发送',skipped:'已跳过（审批已处理）',failed:'准备失败',unknown:'结果未知，需核对'}[n.status]||n.status)}</small>${n.error?`<p class="hint">${esc(n.error)}</p>`:''}${n.receipt?.url?`<a class="wf-link" href="${esc(n.receipt.url)}" target="_blank" rel="noopener noreferrer">查看通知 ↗</a>`:''}${['failed','unknown'].includes(n.status)?`<button data-notice-retry="${esc(n.id)}">${n.status==='unknown'?'核对外部结果':'重试通知'}</button>`:''}</div>`).join('')}</section>`);document.querySelectorAll('[data-notice-retry]').forEach(b=>b.onclick=async()=>{b.disabled=true;try{await api('/api/workflow-runs/'+id+'/notifications/'+b.dataset.noticeRetry+'/retry','POST',{});await refreshWorkflowRun(id)}catch(e){toast(e.message);b.disabled=false}})}
 const latestPR=r.steps.filter(s=>s.connector_receipt?.kind==='github.pull_request').at(-1);
 if(latestPR){
  const receiptKey=JSON.stringify([latestPR.seq,latestPR.connector_receipt]);
  if(workflowRunPage.deliveryKey!==receiptKey){workflowRunPage.deliveryKey=receiptKey;workflowRunPage.deliveryRequested=false;workflowRunPage.delivery=null}
  $('#wf-run-history').insertAdjacentHTML('beforeend','<section id="wf-delivery" class="card wf-run-step"></section>');
  renderWorkflowDelivery();
  if(!workflowRunPage.deliveryRequested){workflowRunPage.deliveryRequested=true;loadWorkflowDelivery(id)}
 }
 // Preserve an in-progress decision while a native status update arrives.
 const old=Object.fromEntries([...$('#wf-run-controls').querySelectorAll('input,textarea,select')].map(el=>[el.id,el.value]));
 let controls=`<h3>${esc(node.name)}</h3><p class="hint">${esc(node.prompt||'')}</p>${step.wait_reason?`<p class="callout">${esc(workflowWaitLabel(step.wait_kind))}：${esc(step.wait_reason)}</p>`:''}`;
 if(node.kind==='approval'&&['running','waiting'].includes(r.status)&&!step.result){controls+=`<form id="wf-decision-form"><div class="field"><label for="wf-decision-note">说明或修改意见</label><textarea id="wf-decision-note" rows="4"></textarea></div><div class="wf-actions">${r.definition.edges.filter(e=>e.source===node.id).map(e=>`<button type="submit" name="route" value="${esc(e.route)}">${esc(e.route)} → ${esc(r.definition.nodes.find(n=>n.id===e.target)?.name)}</button>`).join('')}</div></form>`;}
 else if(step.result&&active){controls+=`<p class="hint">${node.kind==='agent'?'交接已保存，等待当前 Agent 结束后继续。':'决定已保存，正在推进下一节点。'}</p>`}
 else if(node.kind==='connector'&&active){controls+='<p class="hint">外部操作执行中，完成后保存回执并推进。</p>'}
 else if(node.kind==='agent'&&active){controls+=`<p class="hint">${r.status==='waiting'?workflowWaitHelp(step.wait_kind):'Agent 正在处理。可以进入会话查看消息和工具调用。'}</p>`}
 if(active&&r.status!=='stopping')controls+='<button id="wf-stop" class="quiet">停止当前运行</button>';
 if(['stopped','failed'].includes(r.status)){
  if(r.status==='failed'&&!limitFailure)controls+='<button id="wf-stop" class="quiet">停止并选择回退节点</button>';
  if(node.kind==='connector'&&step.connector_dispatched&&!limitFailure)controls+='<p class="hint">该动作已经发出。GitHub 恢复只核对已有资源；命令如需重新执行，请先检查效果，再停止并回到该节点。</p>';
  if(!step.result&&!(node.kind==='connector'&&step.connector_dispatched&&r.connectors[node.connector_id]?.kind==='command'))controls+='<form id="wf-resume-form"><div class="field"><label for="wf-resume-message">继续说明</label><textarea id="wf-resume-message" rows="3" placeholder="说明如何接着处理；已执行的动作不会自动重放"></textarea></div><button type="submit">继续原节点</button></form>';
 }
 if(['stopped','failed'].includes(r.status))controls+='<p class="hint">不再继续这项任务时，可以取消运行。历史和文件保留，工作区可用于新任务。</p><button id="wf-cancel" class="danger">取消运行</button>';
 if(r.status==='cancelled')controls+='<p class="hint">本次运行已取消，历史和文件保留。新增工作请开始一次新运行。</p>';
 if(r.status==='completed')controls+='<p class="hint">已到达结束节点。需要修订时可填写原因并回到指定节点；已有记录保留，后续节点和外部动作将按原图重新执行。工作区被其他运行占用时不能继续。</p>';
  if((['stopped','completed'].includes(r.status)||limitFailure)&&(!limitReached||r.seq<1000)){
   const suggested=r.definition.edges.find(edge=>edge.source===node.id&&edge.route===step.result?.route)?.target;
   controls+=`<form id="wf-return-form">${limitReached?`<p class="callout">已达到 ${executionLimit} 次节点执行上限。检查已有结果与返工原因后，可设置新的有限上限并选择继续节点。</p><div class="field"><label for="wf-return-max-steps">本次运行的新执行上限</label><input id="wf-return-max-steps" type="number" min="${r.seq+1}" max="1000" value="${Math.min(r.seq+20,1000)}" required></div>`:''}<div class="field"><label for="wf-return-target">回到节点</label><select id="wf-return-target">${r.definition.nodes.map(n=>`<option value="${esc(n.id)}"${limitFailure&&n.id===suggested?' selected':''}>${esc(n.name)}</option>`).join('')}</select></div><div class="field"><label for="wf-return-reason">回退原因</label><textarea id="wf-return-reason" rows="3" required></textarea></div><button type="submit">从该节点继续</button></form>`;
  }
 controls+=`<details><summary>运行信息</summary><p class="hint">运行 ID：${esc(r.id)}</p><p class="hint">工作区：${esc(r.workspace_path)}</p></details>`;
 $('#wf-run-controls').innerHTML=controls;for(const [key,value]of Object.entries(old)){const el=$('#'+key);if(el)el.value=value}
 const command=async(action,data={})=>{const panel=$('#wf-run-controls');panel.inert=true;try{await api('/api/workflow-runs/'+id+'/'+action,'POST',{seq:r.seq,...data});if(workflowRunPage?.id===id){workflowRunPage.signature='';await refreshWorkflowRun(id)}}catch(e){toast(e.message)}finally{panel.inert=false}};
 if($('#wf-stop'))$('#wf-stop').onclick=()=>command('stop');
 if($('#wf-cancel'))$('#wf-cancel').onclick=()=>{if(confirm('取消后无法继续本次运行，文件和历史会保留。确认取消？'))return command('cancel')};
 if($('#wf-decision-form'))$('#wf-decision-form').onsubmit=e=>{e.preventDefault();command('decision',{route:e.submitter.value,summary:$('#wf-decision-note').value})};
 if($('#wf-return-form'))$('#wf-return-form').onsubmit=e=>{e.preventDefault();return command('return',{target:$('#wf-return-target').value,summary:$('#wf-return-reason').value,...(limitReached?{max_steps:Number($('#wf-return-max-steps').value)}:{})})};
 if($('#wf-resume-form'))$('#wf-resume-form').onsubmit=e=>{e.preventDefault();command('resume',{message:$('#wf-resume-message').value})};
 if(workflowRunPage.selectedStep){
  const selected=r.steps.find(s=>s.seq===workflowRunPage.selectedStep);
  if(selected){renderWorkflowStepDetails(workflowRunPage,selected,r.definition.nodes.find(n=>n.id===selected.node_id));if(workflowRunPage.stepTab==='input')loadWorkflowStepInput(workflowRunPage,selected)}
 }
}

function workflowStateLabel(status,kind){return status==='waiting'&&kind?workflowWaitLabel(kind):(workflowStatus[status]||status)}
function workflowWaitHelp(kind){
 if(kind==='clarification')return '进入会话回答已列出的具体问题；回复沿原会话继续，提交交接后自动推进。';
 if(kind==='blocked')return '先核对并解除已列出的外部阻塞，再进入会话说明恢复依据。节点尚未完成。';
 if(kind==='limit'||kind==='disabled')return '自动继续已暂停，节点尚未完成。检查现场后进入会话给出继续说明，或停止后回退；这不是需要回答的新问题。';
 return '节点尚未交付。进入会话核对执行记录与可用下一步；自然语言回复不代表节点完成。';
}
function workflowWaitLabel(kind){return {clarification:'等待澄清',blocked:'等待解除阻塞',limit:'已达到自动继续上限',disabled:'自动继续已关闭'}[kind]||'等待输入'}
async function workflowStepContextDialog(id,seq){
 return workflowStepDetails(id,seq,'input');
}
function closeWorkflowStepDetails(){
 if(!workflowRunPage)return;
 const seq=workflowRunPage.selectedStep;workflowRunPage.selectedStep=null;
 $('#wf-step-details').hidden=true;
 document.querySelector?.('[data-step-details="'+seq+'"]')?.focus();
}
if(typeof window!=='undefined')window.addEventListener('keydown',event=>{
 if(event.key==='Escape'&&!$('#dialog')?.open&&workflowRunPage?.selectedStep){event.preventDefault();closeWorkflowStepDetails()}
});
function workflowStepInputHTML(context){
 return `<p class="hint">运行时保存的版本 ${esc(context.workflow_revision)}</p><details><summary>角色指令</summary><textarea id="wf-actual-role" rows="8" readonly>${esc(context.role_instructions||'')}</textarea></details><details><summary>项目规则与可用工具</summary><h3>项目规则来源</h3><ul>${(context.project_instructions||[]).map(source=>`<li>${esc(source)}</li>`).join('')||'<li>未记录来源</li>'}</ul><p class="hint">由原生执行器发现工作区规则。</p><h3>可用工具</h3><ul>${(context.tools||[]).map(tool=>`<li>${esc(tool)}</li>`).join('')||'<li>无已记录工具</li>'}</ul></details><div class="field"><label for="wf-actual-input">首轮 User Input</label><textarea id="wf-actual-input" rows="22" readonly>${esc(context.input||'')}</textarea></div>`;
}
async function workflowStepDetails(id,seq,active='result'){
 const page=workflowRunPage;if(!page||page.id!==id)return;
 const run=page.run,step=run?.steps.find(s=>s.seq===seq);if(!step)return;
 const node=run.definition.nodes.find(n=>n.id===step.node_id);
 page.selectedStep=seq;page.stepTab=active;
 renderWorkflowStepDetails(page,step,node);
 if(active==='input'&&node?.kind==='agent')await loadWorkflowStepInput(page,step);
}
function renderWorkflowStepDetails(page,step,node){
 if(workflowRunPage!==page||page.selectedStep!==step.seq)return;
 const panel=$('#wf-step-details'),scroll=panel.querySelector?.('.wf-panel-body')?.scrollTop||0;
 const focus=panel.contains?.(document.activeElement)?document.activeElement?.id:null;
 const expanded=Array.from(panel.querySelectorAll?.('details[open]')||[],el=>el.querySelector('summary')?.textContent);
 const textareas=Array.from(panel.querySelectorAll?.('textarea[id]')||[],el=>({id:el.id,scroll:el.scrollTop,start:el.selectionStart,end:el.selectionEnd}));
 const input=node?.kind==='agent'?`<div id="wf-step-input-content">${page.contexts?.[step.seq]?workflowStepInputHTML(page.contexts[step.seq]):'<p class="hint" role="status">正在读取冻结会话输入…</p>'}</div>`:`<h3>冻结节点说明</h3><p>${esc(node?.prompt||'无额外说明')}</p><h3>当前任务</h3><p>${esc(page.run.input||'')}</p>`;
 const result=`<p><span class="wf-run-status status-${esc(step.status)}">${esc(workflowStateLabel(step.status,step.wait_kind))}</span></p>${step.wait_reason?`<p class="callout">${step.result||['completed','cancelled'].includes(step.status)?'历史等待记录 · ':''}${esc(workflowWaitLabel(step.wait_kind))}：${esc(step.wait_reason)}</p>`:''}${step.error?`<p class="callout">${esc(step.error)}</p>`:''}${step.result?`<h3>交付结论</h3><p class="wf-detail-text">${esc(step.result.summary)}</p><h3>交接与产物</h3><dl class="wf-detail-facts"><dt>路径</dt><dd>${esc(step.result.route||'固定完成')}</dd>${Object.entries(step.result.inputs||{}).map(([key,value])=>`<dt>${esc(key)}</dt><dd>${esc(typeof value==='string'?value:JSON.stringify(value))}</dd>`).join('')}</dl>${step.result.artifacts?.length?`<ul class="wf-artifact-list">${step.result.artifacts.map(path=>`<li>${/^https:\/\//.test(path)?workflowExternalLink(path,path):`<code>${esc(path)}</code>`}</li>`).join('')}</ul>`:'<p class="hint">未提交产物引用</p>'}<details><summary>原始交接回执</summary><pre>${esc(JSON.stringify(step.result,null,2))}</pre></details>`:(node?.kind==='end'&&step.status==='completed'?'<p class="hint">已到达结束节点，本次运行完成。</p>':'<p class="hint">本节点尚无已接受的完成/交接结果。</p>')}${step.connector_receipt?`<details><summary>外部调用回执${step.connector_receipt.exit_code!=null?' · 退出码 '+step.connector_receipt.exit_code:''}</summary><pre>${esc(JSON.stringify(step.connector_receipt,null,2))}</pre></details>`:''}`;
 const logs=`<p class="hint">原生消息、工具调用与审批保留在原会话；固定命令日志使用本次执行留存的记录。</p>${step.conversation_id?`<p><a data-nav class="wf-link" href="/conversations/${esc(step.conversation_id)}">打开原会话与工具记录 ↗</a></p>`:''}${step.connector_receipt?.log?`<p><a class="wf-link" href="/api/workflow-runs/${encodeURIComponent(page.id)}/steps/${step.seq}/output?format=text" target="_blank" rel="noopener noreferrer">查看命令日志${step.connector_receipt.log.truncated?'（已达保留上限）':''} ↗</a></p>`:'<p class="hint">没有本次固定命令日志；缺失日志不代表验证通过。</p>'}<dl class="wf-detail-facts"><dt>节点</dt><dd>${esc(step.node_id)}</dd><dt>执行</dt><dd>${step.seq}</dd><dt>开始</dt><dd>${esc(step.created_at||'未记录')}</dd><dt>更新</dt><dd>${esc(step.updated_at||'未记录')}</dd></dl>`;
 const tabs=[{key:'input',label:'输入',body:input},{key:'result',label:'结果',body:result},{key:'logs',label:'日志与记录',body:logs}];
 const markup=workflowPanelMarkup('wf-step',node?.name||step.node_id,'节点执行 '+step.seq,tabs,page.stepTab);
 panel.hidden=false;if(page.stepDetailsMarkup===markup)return;
 page.stepDetailsMarkup=markup;panel.innerHTML=markup;
 bindWorkflowPanel('wf-step',tabs,page.stepTab,key=>{page.stepTab=key;if(key==='input'&&node?.kind==='agent')loadWorkflowStepInput(page,step)},closeWorkflowStepDetails);
 const body=panel.querySelector?.('.wf-panel-body');if(body)body.scrollTop=scroll;
 for(const el of panel.querySelectorAll?.('details')||[])el.open=expanded.includes(el.querySelector('summary')?.textContent);
 for(const saved of textareas){const el=panel.querySelector?.('#'+saved.id);if(el){el.scrollTop=saved.scroll;el.setSelectionRange(saved.start,saved.end);}}
 if(focus)$('#'+focus)?.focus();
}
async function loadWorkflowStepInput(page,step){
 if(page.contexts?.[step.seq]||page.contextRequest?.seq===step.seq)return;
 const request={seq:step.seq};page.contextRequest=request;
 try{
  const context=await api('/api/workflow-runs/'+encodeURIComponent(page.id)+'/steps/'+step.seq+'/context');
  if(workflowRunPage!==page||page.selectedStep!==step.seq||page.contextRequest!==request)return;
  page.contexts||={};page.contexts[step.seq]=context;
  $('#wf-step-input-content').innerHTML=workflowStepInputHTML(context);
 }catch(error){if(workflowRunPage===page&&page.selectedStep===step.seq&&page.contextRequest===request)$('#wf-step-input-content').innerHTML='<p class="callout" role="alert">'+esc(error.message)+'</p>'}
 finally{if(page.contextRequest===request)page.contextRequest=null}
}
