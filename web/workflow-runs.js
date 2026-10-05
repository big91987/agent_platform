'use strict';
let workflowRunTimer=null;
let workflowRunPage=null;
function leaveWorkflowRun(){if(workflowRunTimer)clearInterval(workflowRunTimer);workflowRunTimer=null;workflowRunPage=null;}
const workflowStatus={pending:'待开始',running:'执行中',waiting:'等待输入',stopping:'正在停止',stopped:'已停止',failed:'需处理',completed:'已完成',cancelled:'已回退'};
async function startWorkflowDialog(w){
 if(!w.id)throw new Error('请先保存智能体编排');
 const entries=[...new Set([w.entry,...w.start_nodes])];
 dialog(`<form id="wf-start-form"><div class="dialog-head"><h2>运行 ${esc(w.name)}</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><div class="field"><label for="wf-task">你要完成什么？</label><textarea id="wf-task" rows="4" required placeholder="用一句话描述任务"></textarea></div><div class="field"><label for="wf-workspace">工作区目录</label><input id="wf-workspace" required placeholder="本机已有项目的绝对路径"><p class="hint">所有节点使用这份工作区。请为独立任务使用独立目录。</p></div>${entries.length>1?`<div class="field"><label for="wf-entry">从哪开始</label><select id="wf-entry">${entries.map(id=>`<option value="${esc(id)}">${esc(w.nodes.find(n=>n.id===id)?.name)}</option>`).join('')}</select></div>`:''}<p class="hint">使用已保存的版本 ${w.revision}。之后修改编排不会改变本次运行。</p></div><div class="dialog-footer"><button type="button" data-close>取消</button><button class="primary">开始运行</button></div></form>`);
 const inputKey="workflow:"+w.id;
 $('#wf-start-form').onsubmit=async e=>{e.preventDefault();e.submitter.disabled=true;try{const run=await api('/api/workflow-runs','POST',platformInputs.body(inputKey,{workflow_id:w.id,input:$('#wf-task').value,workspace_path:$('#wf-workspace').value,start_node:$('#wf-entry')?.value||w.entry}));platformInputs.accepted(inputKey);$('#dialog').close();go('/workflow-runs/'+run.id)}catch(err){toast(err.message);e.submitter.disabled=false}};
}
async function workflowRunsList(workflowID){
 const runs=await api('/api/workflow-runs?workflow_id='+encodeURIComponent(workflowID));
 return `<section class="wf-runs-list"><h2>运行记录</h2><p class="hint">点击任务查看节点进度、执行记录和当前 Agent 会话。</p>${runs.length?runs.map(r=>`<a data-nav class="wf-run-row" href="/workflow-runs/${esc(r.id)}"><span><strong>${esc(r.definition.name)}</strong><small>${esc(r.input.slice(0,100))}</small></span><span class="wf-run-status status-${esc(r.status)}">${esc(workflowStatus[r.status]||r.status)} · 查看进度 →</span></a>`).join(''):'<p class="muted">此工作流还没有运行。打开编排，输入任务即可开始。</p>'}</section>`;
}
async function workflowRunsView(id){
 const workflow=await api('/api/workflows/'+id);
 $('#content').innerHTML=heading(workflow.name+' · 运行记录','点击一次运行查看节点进度，再进入对应 Agent 会话。',`<a data-nav class="wf-link" href="/workflows/${esc(id)}">打开编排 ↗</a>`)+await workflowRunsList(id);
}
async function workflowRunView(id){
 workflowRunPage={id,signature:''};
 $('#content').innerHTML='<div id="wf-run-header"></div><div class="wf-run-layout"><section id="wf-run-history"></section><aside id="wf-run-controls" class="card"></aside></div>';
 await refreshWorkflowRun(id);
 workflowRunTimer=setInterval(()=>{if(workflowRunPage?.id===id&&!document.hidden)refreshWorkflowRun(id).catch(e=>toast(e.message))},1500);
}
async function refreshWorkflowRun(id){
 const r=await api('/api/workflow-runs/'+id);if(workflowRunPage?.id!==id)return;
 const notices=await api('/api/workflow-runs/'+id+'/notifications');if(workflowRunPage?.id!==id)return;
 const signature=JSON.stringify([r,notices]);if(workflowRunPage.signature===signature)return;workflowRunPage.signature=signature;
 const step=r.steps.at(-1),node=r.definition.nodes.find(n=>n.id===step.node_id),active=['running','waiting','stopping'].includes(r.status);
 $('#wf-run-header').innerHTML=`<div class="wf-heading"><div><a data-nav href="/workflows/${esc(r.workflow_id)}/runs" class="muted small">← 运行记录</a><h1>${esc(r.definition.name)} <span class="wf-run-status status-${esc(r.status)}">${esc(workflowStatus[r.status]||r.status)}</span></h1><span class="muted small">版本 ${r.definition.revision} · 第 ${r.seq} 次节点执行</span></div><a data-nav class="wf-link" href="/workflows/${esc(r.workflow_id)}">打开编排 ↗</a></div><p class="wf-run-input">${esc(r.input)}</p>${r.error?`<div class="callout" role="alert">${esc(r.error)}</div>`:''}`;
 $('#wf-run-history').innerHTML=r.steps.map(s=>{const n=r.definition.nodes.find(n=>n.id===s.node_id);return `<article class="card wf-run-step ${s.seq===r.seq?'current':''}"><div class="wf-step-heading"><span class="wf-step-number">${s.seq}</span><h3>${esc(n?.name||s.node_id)}</h3><span class="wf-run-status status-${esc(s.status)}">${esc(workflowStatus[s.status]||s.status)}</span></div>${s.result?`<p>${esc(s.result.summary)}</p><div class="hint">交接路径：${esc(s.result.route)}</div>${s.result.artifacts?.length?`<ul>${s.result.artifacts.map(a=>`<li>${/^https:\/\//.test(a)?`<a href="${esc(a)}" target="_blank" rel="noopener noreferrer">${esc(a)}</a>`:esc(a)}</li>`).join('')}</ul>`:''}`:''}${s.connector_receipt?`<details><summary>调用回执${s.connector_receipt.exit_code!=null?' · 退出码 '+s.connector_receipt.exit_code:''}${s.connector_receipt.recovered?' · 已核对外部结果':''}</summary><pre>${esc(JSON.stringify(s.connector_receipt,null,2))}</pre></details>`:''}${s.error?`<p class="hint">${esc(s.error)}</p>`:''}${s.conversation_id?`<a class="wf-link" data-nav href="/conversations/${esc(s.conversation_id)}">${s.seq===r.seq&&active?'进入 Agent 会话':'查看会话记录'} ↗</a>`:''}</article>`}).join('');
 if(notices.length){$('#wf-run-history').insertAdjacentHTML('beforeend',`<section class="card wf-run-step"><h3>通知记录</h3><p class="hint">通知失败独立处理，不会重跑 Agent 或交接。结果未知时只查询外部回执。</p>${notices.map(n=>`<div class="wf-hook-row"><strong>${esc(workflowHookEvents[n.event]||n.event)} · ${esc(r.definition.nodes.find(v=>v.id===n.node)?.name||n.node)}</strong><small>${esc({pending:'待发送',sending:'发送中',succeeded:'已发送',failed:'准备失败',unknown:'结果未知，需核对'}[n.status]||n.status)}</small>${n.error?`<p class="hint">${esc(n.error)}</p>`:''}${n.receipt?.url?`<a class="wf-link" href="${esc(n.receipt.url)}" target="_blank" rel="noopener noreferrer">查看通知 ↗</a>`:''}${['failed','unknown'].includes(n.status)?`<button data-notice-retry="${esc(n.id)}">${n.status==='unknown'?'核对外部结果':'重试通知'}</button>`:''}</div>`).join('')}</section>`);document.querySelectorAll('[data-notice-retry]').forEach(b=>b.onclick=async()=>{b.disabled=true;try{await api('/api/workflow-runs/'+id+'/notifications/'+b.dataset.noticeRetry+'/retry','POST',{});await refreshWorkflowRun(id)}catch(e){toast(e.message);b.disabled=false}})}
 // Preserve an in-progress decision while a native status update arrives.
 const old=Object.fromEntries([...$('#wf-run-controls').querySelectorAll('input,textarea,select')].map(el=>[el.id,el.value]));
 let controls=`<h3>${esc(node.name)}</h3><p class="hint">${esc(node.prompt||'')}</p>`;
 if(node.kind==='approval'&&['running','waiting'].includes(r.status)&&!step.result){controls+=`<form id="wf-decision-form"><div class="field"><label for="wf-decision-note">说明或修改意见</label><textarea id="wf-decision-note" rows="4"></textarea></div><div class="wf-actions">${r.definition.edges.filter(e=>e.source===node.id).map(e=>`<button type="submit" name="route" value="${esc(e.route)}">${esc(e.route)} → ${esc(r.definition.nodes.find(n=>n.id===e.target)?.name)}</button>`).join('')}</div></form>`;}
 else if(step.result&&active){controls+=`<p class="hint">${node.kind==='agent'?'交接已保存，等待当前 Agent 结束后继续。':'决定已保存，正在推进下一节点。'}</p>`}
 else if(node.kind==='connector'&&active){controls+='<p class="hint">外部操作执行中，完成后保存回执并推进。</p>'}
 else if(node.kind==='agent'&&active){controls+=`<p class="hint">${r.status==='waiting'?'Agent 正在等待输入。进入会话继续澄清；它提交交接后会自动推进。':'Agent 正在处理。可以进入会话查看消息和工具调用。'}</p>`}
 if(active&&r.status!=='stopping')controls+='<button id="wf-stop" class="quiet">停止当前运行</button>';
 if(['stopped','failed'].includes(r.status)){
  if(r.status==='failed')controls+='<button id="wf-stop" class="quiet">停止并选择回退节点</button>';
  if(node.kind==='connector'&&step.connector_dispatched)controls+='<p class="hint">该动作已经发出。GitHub 恢复只核对已有资源；命令如需重新执行，请先检查效果，再停止并回到该节点。</p>';
  if(!step.result&&!(node.kind==='connector'&&step.connector_dispatched&&r.connectors[node.connector_id]?.kind==='command'))controls+='<form id="wf-resume-form"><div class="field"><label for="wf-resume-message">继续说明</label><textarea id="wf-resume-message" rows="3" placeholder="说明如何接着处理；已执行的动作不会自动重放"></textarea></div><button type="submit">继续原节点</button></form>';
  if(r.status==='stopped')controls+=`<form id="wf-return-form"><div class="field"><label for="wf-return-target">回到节点</label><select id="wf-return-target">${r.definition.nodes.map(n=>`<option value="${esc(n.id)}">${esc(n.name)}</option>`).join('')}</select></div><div class="field"><label for="wf-return-reason">回退原因</label><textarea id="wf-return-reason" rows="3" required></textarea></div><button type="submit">从该节点继续</button></form>`;
 }
 if(r.status==='completed')controls+='<p class="hint">已到达结束节点。全部执行记录与会话保留。</p>';
 controls+=`<details><summary>运行信息</summary><p class="hint">运行 ID：${esc(r.id)}</p><p class="hint">工作区：${esc(r.workspace_path)}</p></details>`;
 $('#wf-run-controls').innerHTML=controls;for(const [key,value]of Object.entries(old)){const el=$('#'+key);if(el)el.value=value}
 const command=async(action,data={})=>{const panel=$('#wf-run-controls');panel.inert=true;try{await api('/api/workflow-runs/'+id+'/'+action,'POST',{seq:r.seq,...data});if(workflowRunPage?.id===id){workflowRunPage.signature='';await refreshWorkflowRun(id)}}catch(e){toast(e.message)}finally{panel.inert=false}};
 if($('#wf-stop'))$('#wf-stop').onclick=()=>command('stop');
 if($('#wf-decision-form'))$('#wf-decision-form').onsubmit=e=>{e.preventDefault();command('decision',{route:e.submitter.value,summary:$('#wf-decision-note').value})};
 if($('#wf-return-form'))$('#wf-return-form').onsubmit=e=>{e.preventDefault();command('return',{target:$('#wf-return-target').value,summary:$('#wf-return-reason').value})};
 if($('#wf-resume-form'))$('#wf-resume-form').onsubmit=e=>{e.preventDefault();command('resume',{message:$('#wf-resume-message').value})};
}
