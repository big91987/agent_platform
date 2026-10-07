'use strict';
const $ = (selector) => document.querySelector(selector);
const esc = (value) => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const state = {agents: [], conversations: [], stream: null, timer: null, events: [], id: null, toastTimer: null};
const labels = {idle:'本轮已结束 · 可继续回复',queued:'排队中',running:'执行中',stopping:'正在停止',stopped:'已停止',failed:'执行失败',closed:'已关闭'};
const time = (value) => new Date(value).toLocaleString('zh-CN',{month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'});
const badge = (status) => `<span class="badge ${esc(status)}">${esc(labels[status] || status)}</span>`;
function toast(text) { $('#toast').textContent = text; $('#toast').className = 'visible'; clearTimeout(state.toastTimer); state.toastTimer = setTimeout(() => $('#toast').className = '', 6000); }
async function api(path, method='GET', body) {
  const response = await fetch(path,{method,headers:{'Content-Type':'application/json','X-Platform-Request':'1'},body:body === undefined ? undefined : JSON.stringify(body)});
  const data = await response.json();
  if (!response.ok) { if (response.status === 401 && path !== '/api/login') { clearLive(); login(); } const error=new Error(data.error || `请求失败：${response.status}`);error.status=response.status;throw error; }
  return data;
}
function closeConversationStream(){state.stream?.close();state.stream=null;state.connection='paused';}
function pauseConversationLive(){state.live=false;closeConversationStream();clearInterval(state.timer);clearInterval(state.clock);state.timer=null;state.clock=null;}
function clearLive(){pauseConversationLive();state.id=null;}
function syncConversationStream(){
 const id=state.id;
 if(!state.live||document.hidden||state.pageHidden||!id||!['running','queued','stopping'].includes(state.view?.conversation.status)){closeConversationStream();return}
 if(state.stream)return;
 const stream=new EventSource('/api/conversations/'+id+'/events?after='+(state.historyEventID||0));
 state.stream=stream;state.connection='connecting';
 const current=()=>state.id===id&&state.stream===stream;
 let scheduled=false;
 stream.onopen=()=>{if(current()){state.connection='connected';renderExecutionStatus(state.view)}};
 stream.onmessage=e=>{
  if(!current())return;
  receiveConversationEvent(JSON.parse(e.data));
  if(!scheduled){scheduled=true;setTimeout(()=>{scheduled=false;if(current())refreshConversation(id).catch(()=>{})},100)}
 };
 stream.onerror=()=>{if(current()){state.connection='reconnecting';renderExecutionStatus(state.view)}};
}
function startConversationLive(){
 if(!state.id||document.hidden||state.pageHidden)return;
 state.live=true;
 const id=state.id;
 if(!state.timer)state.timer=setInterval(()=>{if(state.id===id&&!document.hidden)refreshConversation(id).catch(()=>{})},2000);
 if(!state.clock)state.clock=setInterval(()=>{if(state.id===id&&!document.hidden)renderExecutionStatus(state.view)},1000);
 syncConversationStream();renderExecutionStatus(state.view);
}
async function resumeConversationLive(){
 const id=state.id,owner=state.events;
 if(!id||document.hidden||state.pageHidden)return;
 try{await refreshConversation(id)}catch{} // Keep visible polling available after a transient failure.
 if(state.id===id&&state.events===owner)startConversationLive();
}
document.addEventListener('visibilitychange',()=>{if(document.hidden)pauseConversationLive();else resumeConversationLive()});
window.addEventListener('pagehide',()=>{state.pageHidden=true;pauseConversationLive()});
window.addEventListener('pageshow',()=>{state.pageHidden=false;resumeConversationLive()});
function login(note=''){clearLive();$('#app').innerHTML=`<div class="login-page"><div class="login-intro"><div class="brand"><span class="brand-icon">A</span> Agent Platform</div><h1>让 Agent 接进你的智能体编排</h1><p>配置原生能力，连接外部系统，<br>在同一段会话里持续协作。</p><div class="login-features"><span>可配置的 Agent</span><span>持久会话与实时进展</span><span>API · Webhook · 网页接续</span></div></div><form class="login-card" id="login-form"><h2>登录工作台</h2><p class="muted">管理员配置平台；用户查看并接续自己的会话。</p>${note?`<div class="callout" role="status">${esc(note)}</div>`:''}<div class="field"><label for="username">用户名</label><input id="username" autocomplete="username" required value="admin"></div><div class="field"><label for="password">密码</label><input id="password" type="password" autocomplete="current-password" required placeholder="输入登录密码"></div><button class="primary" type="submit">登录</button><p class="muted small">从业务系统打开会话链接时，登录后会自动回到原会话。</p></form></div>`;$('#login-form').onsubmit=async e=>{e.preventDefault();e.submitter.disabled=true;try{await api('/api/login','POST',{username:$('#username').value,password:$('#password').value});await route()}catch(err){toast(err.message);if($('#login-form'))$('#login-form button').disabled=false}};}
function shell(active,title){const admin=state.me.admin;const navigation=admin?[['conversations','◫','会话','/conversations/'],['agents','◇','智能体','/agents'],['workflows','⌘','智能体编排','/workflows'],['tools','⚒','外部工具','/tools'],['integrations','↗','API 接入','/integrations'],['users','◎','用户与角色','/users'],['api-docs','⌘','API 文档','/api-docs']]:[['conversations','◫','我的会话','/conversations/'],['workflows','⌘','智能体编排','/workflows']];$('#app').innerHTML=`<div class="layout ${admin?'':'caller-layout'}"><aside class="sidebar"><div class="brand"><span class="brand-icon">A</span> Agent Platform</div><div class="edition">${admin?'管理工作台':'会话工作台'}</div><nav class="nav">${navigation.map(([key,icon,label,url])=>`<a href="${url}" data-nav class="${active===key?'active':''}"><span class="nav-label">${icon}</span>${label}</a>`).join('')}</nav><div class="nav-caption">${admin?'配置 · 接入 · 持续协作':'专属会话 · 持续交流'}</div><div class="local-card"><strong><span class="dot"></span>本机平台已连接</strong>${admin?'原生 Agent 后台运行':'你的输入和 Agent 历史持续保存'}</div></aside><main class="main"><header class="topbar"><div>${esc(title)} <small> / ${admin?'管理工作台':'用户工作台'}</small></div><div class="topbar-right"><span class="role-chip">${admin?'管理员':'用户'}</span><span class="avatar">${esc((state.me.username||state.me.user_id||'访')[0])}</span><span>${esc(state.me.username||state.me.user_id)}</span><button id="logout" class="quiet small">退出</button></div></header><section class="content" id="content"></section></main></div><dialog id="dialog"></dialog>`;$('#logout').onclick=async()=>{try{await api('/api/logout','POST',{});history.replaceState({},'','/');login()}catch(e){toast(e.message)}};}
function go(path){if(typeof workflowEditor!=='undefined'&&workflowEditor?.dirty&&!confirm('离开将丢失未保存的智能体编排改动，继续？'))return;history.pushState({},'',path);route().catch(showRouteError);}
window.addEventListener('popstate',()=>route().catch(showRouteError));
document.addEventListener('click',e=>{const a=e.target.closest('a[data-nav]');if(a){e.preventDefault();go(a.getAttribute('href'))}});
async function route(){
 if(typeof leaveWorkflowEditor==='function')leaveWorkflowEditor();
 clearLive();
 // Old fragment links now use the same account login; never exchange a bearer grant.
 if(location.hash)history.replaceState({},'',location.pathname+location.search);
 try{state.me=await api('/api/me')}catch(e){if(e.status===401)return;throw e;}
 state.agents=await api('/api/agents');
 const path=location.pathname;
 if(!state.me.admin&&['/agents','/tools','/users','/integrations','/api-docs'].includes(path)){go('/conversations/');return}
 if(path==='/agents'){shell('agents','智能体');await agentsView()}
 else if(path==='/workflows'){shell('workflows','智能体编排');await workflowsView()}
 else if(/^\/workflows\/[a-f0-9]{32}\/runs$/.test(path)){shell('workflows','运行记录');await workflowRunsView(path.split('/')[2])}
 else if(/^\/workflows\/(new|[a-f0-9]{32})$/.test(path)){shell('workflows','智能体编排');await workflowView(path.split('/')[2])}
 else if(/^\/workflow-runs\/[a-f0-9]{32}$/.test(path)){shell('workflows','编排运行');await workflowRunView(path.split('/')[2])}
 else if(path==='/tools'){shell('tools','外部工具');await toolsView()}
 else if(path==='/integrations'){shell('integrations','调用方接入');await integrationsView()}
 else if(path==='/users'){shell('users','用户与角色');await usersView()}
 else if(path==='/api-docs'){shell('api-docs','API 文档');apiDocsView()}
 else{
  shell('conversations','会话');
  const match=path.match(/^\/conversations\/([a-f0-9]{32})$/);
  if(match){await conversationView(match[1])}
  else{await conversationsView()}
 }
}
function heading(title,description,action=''){return `<div class="page-heading"><div><div class="eyebrow">AGENT WORKSPACE</div><h1>${esc(title)}</h1><p class="muted">${esc(description)}</p></div>${action}</div>`;}
async function conversationsView(){state.conversations=await api('/api/conversations');const running=state.conversations.filter(c=>c.status==='running').length, queued=state.conversations.filter(c=>c.status==='queued').length;$('#content').innerHTML=heading(state.me.admin?'会话管理':'我的会话','每段会话属于一个 Agent；打开后接续这段会话的原生 Session。','<button id="new-chat" class="primary">＋ 开始会话</button>')+`<div class="stats"><div class="stat"><span class="stat-label">已保存会话</span><strong>${state.conversations.length}</strong></div><div class="stat"><span class="stat-label">正在执行</span><strong>${running}</strong></div><div class="stat"><span class="stat-label">等待执行</span><strong>${queued}</strong></div></div><div class="toolbar"><input id="search" placeholder="搜索会话内容或用户"><select id="agent-filter"><option value="">全部 Agent</option>${state.agents.map(a=>`<option value="${esc(a.id)}">${esc(a.name)}</option>`).join('')}</select><select id="status-filter"><option value="">全部状态</option>${Object.entries(labels).map(([k,v])=>`<option value="${k}">${v}</option>`).join('')}</select><button id="refresh">刷新</button></div><div class="table-card"><div class="table-head"><div>会话</div><div>Agent</div><div>当前情况</div><div>最近更新</div></div><div id="conversation-rows"></div></div>`;$('#new-chat').onclick=()=>newConversation();$('#refresh').onclick=()=>conversationsView().catch(e=>toast(e.message));$('#search').oninput=rows;$('#agent-filter').onchange=rows;$('#status-filter').onchange=rows;const requested=new URLSearchParams(location.search).get('agent');if(requested)$('#agent-filter').value=requested;rows();}
function rows(){const q=$('#search').value.toLowerCase(),a=$('#agent-filter').value,s=$('#status-filter').value;const list=state.conversations.filter(c=>(!a||c.agent_id===a)&&(!s||c.status===s)&&[c.title,c.user_id].join(' ').toLowerCase().includes(q));$('#conversation-rows').innerHTML=list.length?list.map(c=>`<div class="conversation-row" tabindex="0" role="link" data-id="${c.id}"><div><div class="row-title">${esc(c.title)}</div><div class="row-sub">${esc(c.user_id)} · ${esc(c.id.slice(0,8))}</div></div><div>${esc(c.agent_name)}</div><div>${badge(c.status)}</div><div class="muted small">${time(c.updated_at)}</div></div>`).join(''):`<div class="empty"><strong>这里还没有匹配的会话</strong>选择一个 Agent，发送第一条消息。</div>`;document.querySelectorAll('.conversation-row').forEach(row=>{row.onclick=()=>go('/conversations/'+row.dataset.id);row.onkeydown=e=>{if(e.key==='Enter')row.click()}});}
function dialog(html){$('#dialog').innerHTML=html;$('#dialog').showModal();$('#dialog').querySelectorAll('[data-close]').forEach(b=>b.onclick=()=>$('#dialog').close());}
function newConversation(selected){const enabled=state.agents.filter(a=>a.enabled);if(!enabled.length){toast(state.me.admin?'先创建并启用一个 Agent':'当前账号没有获授权的可用 Agent，请联系管理员');if(state.me.admin)go('/agents');return}dialog(`<form id="new-conversation"><div class="dialog-head"><h2>开始一段会话</h2><button type="button" class="quiet" data-close>✕</button></div><div class="dialog-body"><div class="fields"><div class="field"><label for="chat-agent">使用哪个 Agent</label><select id="chat-agent">${enabled.map(a=>`<option value="${a.id}" ${a.id===selected?'selected':''}>${esc(a.name)}</option>`).join('')}</select></div><div class="field"><label for="chat-user">用户标识</label><input id="chat-user" value="${esc(state.me.user_id||'admin')}" ${state.me.admin?'':'readonly'} required></div><div class="field full"><label for="chat-workspace">项目工作目录（可选）</label><input id="chat-workspace" placeholder="本机已有项目的绝对路径"><span class="hint">直接使用该目录；接续时固定不变。留空创建平台工作区。</span></div><div class="field full"><label for="first-message">想让它做什么？</label><textarea id="first-message" required placeholder="直接描述需求，也可以先提问题。产出形式由你决定。"></textarea><span class="hint">保存后续对话，接续同一段原生 Session。</span></div></div></div><div class="dialog-footer"><button type="button" data-close>取消</button><button class="primary" type="submit">发送并开始</button></div></form>`);$('#new-conversation').onsubmit=async e=>{e.preventDefault();const button=e.submitter;button.disabled=true;try{const r=await api('/api/invoke','POST',platformInputs.body('new',{agent_id:$('#chat-agent').value,user_id:$('#chat-user').value,message:$('#first-message').value,workspace_path:$('#chat-workspace').value.trim()}));platformInputs.accepted('new');$('#dialog').close();go('/conversations/'+r.conversation_id)}catch(err){toast(err.message);button.disabled=false}};}
async function agentsView(){state.conversations=await api('/api/conversations'); $('#content').innerHTML=heading('智能体','复用原生执行器的能力，按用途配置指令、Skill 与 Hook。','<button id="new-agent" class="primary">＋ 创建 Agent</button>')+`<div class="agent-grid">${state.agents.map(a=>`<article class="card"><div class="agent-head"><span class="agent-icon">◇</span><div><h2>${esc(a.name)}</h2><span class="muted small">${esc(agentExecutionLabel(a))} · ${a.enabled?'已启用':'已禁用'}</span></div></div><p class="muted small">${esc((a.instructions||'使用原生 Agent 的默认行为。').slice(0,110))}</p><div class="agent-meta"><span class="chip">${(a.skills||[]).length} 个 Skill</span><span class="chip">${esc(a.sandbox)}</span></div><div class="agent-foot"><button data-edit="${a.id}">配置</button><button data-check="${a.id}">检查</button><button data-sessions="${a.id}">会话 (${state.conversations.filter(c=>c.agent_id===a.id).length})</button><button data-chat="${a.id}" ${a.enabled?'':'disabled'}>试运行 ↗</button></div></article>`).join('')}</div><div class="callout">执行器配置在新会话创建时生效；用户授权立即生效。已有会话保留原生上下文和文件。</div>`;$('#new-agent').onclick=()=>editAgent();document.querySelectorAll('[data-sessions]').forEach(b=>b.onclick=()=>go('/conversations/?agent='+b.dataset.sessions));document.querySelectorAll('[data-edit]').forEach(b=>b.onclick=()=>editAgent(state.agents.find(a=>a.id===b.dataset.edit)));document.querySelectorAll('[data-chat]').forEach(b=>b.onclick=()=>newConversation(b.dataset.chat));document.querySelectorAll('[data-check]').forEach(b=>b.onclick=()=>checkAgent(b.dataset.check,b));}
async function checkAgent(id,button){button.disabled=true;const old=button.textContent;button.textContent='检查中…';try{const r=await api('/api/agents/'+id+'/check','POST',{});dialog(`<div class="dialog-head"><h2>${r.ready?'本机检查通过':'需要处理配置或环境'}</h2><button class="quiet" data-close>✕</button></div><div class="dialog-body"><p>${r.ready?'执行器、原生配置、登录与 Skill 范围已核验。':'检查没有通过；这不是模型执行失败。'}</p><pre>${esc(r.ready?JSON.stringify(r,null,2):r.error)}</pre><p class="muted small">检查通过后再试运行，才能验证模型实际执行。</p></div>`)}catch(e){toast(e.message)}finally{button.disabled=false;button.textContent=old;}}
async function editAgent(a={name:'',executor:'codex',model:'',instructions:'',seed_dir:'',skills:[],native_config:'',inherit_env:true,env:{},sandbox:'workspace-write',trust_hooks:false,enabled:true},onSaved=null){
 const [users,servers]=await Promise.all([api('/api/users'),api('/api/tool-servers')]);
 dialog(`<form id="agent-form"><div class="dialog-head"><h2>${a.id?'配置智能体':'创建智能体'}</h2><button type="button" class="quiet" data-close>✕</button></div><div class="dialog-body"><div class="field"><label for="agent-name">名称</label><input id="agent-name" required maxlength="100" value="${esc(a.name)}" placeholder="例如：项目助手"></div>${agentConfigFields(a,servers)}<details><summary>访问权限</summary>${users.filter(u=>u.role!=='admin').map(u=>`<label class="check"><input type="checkbox" name="authorized-user" value="${esc(u.user_id)}" ${(a.authorized_users||[]).includes(u.user_id)?'checked':''}>${esc(u.username)} · ${esc(u.user_id)}</label>`).join('')||'<p class="hint">在用户页面创建账号后，可以授权使用此智能体。</p>'}</details><label class="check"><input id="agent-enabled" type="checkbox" ${a.enabled?'checked':''}>允许开始新会话</label></div><div class="dialog-footer"><button type="button" data-close>取消</button><button class="primary" type="submit">保存配置</button></div></form>`);
 $('#agent-form').onsubmit=async e=>{
  e.preventDefault();
  try{
   const payload={...readAgentConfig(a),name:$('#agent-name').value.trim(),authorized_users:[...document.querySelectorAll('input[name="authorized-user"]:checked')].map(el=>el.value),enabled:$('#agent-enabled').checked};
   await api('/api/agents'+(a.id?'/'+a.id:''),a.id?'PUT':'POST',payload);
   $('#dialog').close();state.agents=await api('/api/agents');if(onSaved)await onSaved();else await agentsView();toast('配置已保存，可检查并试运行');
  }catch(err){toast(err.message)}
 };
}
function renderToolApprovals(data,id){
 const pending=(data.approvals||[]).filter(a=>!a.decision);
 const panel=$('#tool-approvals');if(!panel)return;
 const markup=pending.map(a=>`<section class="approval-card"><strong>${a.request.platform_permission_request?'等待管理员批准提权':'等待你确认工具调用'}</strong><p>${esc(a.request.platform_permission_request?(a.request.method==='item/permissions/requestApproval'?'本轮权限申请':'本次原生命令 / 文件操作'):a.request.serverName||'外部工具')}</p><pre class="approval-message">${esc(a.request.reason||a.request.message||'Agent 请求调用外部工具。')}</pre>${a.request.platform_permission_request?`<p>${a.request.method==='item/permissions/requestApproval'?'仅本轮生效。':'批准后本次操作可能在沙箱外执行，请核对完整命令及路径。'}</p><pre>${esc(a.request.command||JSON.stringify(a.request.permissions||a.request,null,2))}</pre><p class="muted">${esc(a.request.cwd||'')}${state.me.admin?'':' · 请联系平台管理员审批。'}</p>`:''}${a.request._meta?.tool_description?`<p>${esc(a.request._meta.tool_description)}</p>`:''}${a.request._meta?.tool_params?`<p>调用参数</p><pre>${esc(JSON.stringify(a.request._meta.tool_params,null,2))}</pre>`:''}<details><summary>原始审批请求</summary><pre>${esc(JSON.stringify(a.request,null,2))}</pre></details><div class="approval-actions"><button class="primary" data-approval="${esc(a.id)}" data-decision="accept" ${a.request.platform_permission_request&&!state.me.admin?'disabled':''}>批准本次调用</button><button data-approval="${esc(a.id)}" data-decision="decline" ${a.request.platform_permission_request&&!state.me.admin?'disabled':''}>拒绝本次调用</button></div></section>`).join('');
 if(panel.approvalMarkup===markup)return;
 panel.approvalMarkup=markup;
 panel.innerHTML=markup;
 panel.querySelectorAll('[data-approval]').forEach(button=>button.onclick=async()=>{
  panel.querySelectorAll('button').forEach(b=>b.disabled=true);
  try{await api('/api/conversations/'+id+'/approvals/'+button.dataset.approval,'POST',{decision:button.dataset.decision});await refreshConversation(id)}catch(error){toast(error.message);panel.approvalMarkup=null;await refreshConversation(id)}
 });
}
async function toolsView(){
 const servers=await api('/api/tool-servers');
 const assigned=new Set(state.agents.flatMap(a=>(a.tool_servers||[]).map(binding=>binding.server_id)));
 const current=servers.filter(s=>assigned.has(s.id));
 const unassigned=servers.filter(s=>!assigned.has(s.id));
 const cards=items=>items.map(s=>`<article class="card"><h2>${esc(s.name)}</h2><p class="muted">${esc(s.id)} · ${s.enabled?'已启用':'已停用'} · ${s.connection.url?'HTTP':'本机 stdio'}</p><p>${s.checked_at?'最近发现：'+time(s.checked_at):'尚未发现工具'}</p>${(s.tools||[]).map(t=>`<details><summary>${esc(t.name)}</summary><p>${esc(t.description)}</p><pre>${esc(JSON.stringify(t.inputSchema,null,2))}</pre></details>`).join('')}<div class="agent-foot"><button data-edit-server="${esc(s.id)}">配置</button><button data-discover-server="${esc(s.id)}">发现工具</button></div></article>`).join('');
 $('#content').innerHTML=heading('外部工具','连接外部 MCP 服务，发现工具后按 Agent 授权使用。','<button id="new-tool-server" class="primary">＋ 注册 MCP 服务</button>')+
 `<div class="agent-grid">${cards(current)||'<div class="empty">当前 Agent 还未分配外部工具。</div>'}</div>${unassigned.length?`<details class="card"><summary>未分配给当前 Agent 的连接（${unassigned.length}）</summary><p class="hint">保留的连接可能用于历史运行恢复；仍可在此查看和配置。</p><div class="agent-grid">${cards(unassigned)}</div></details>`:''}<div class="callout">平台只保存连接和工具选择。工具实现与业务逻辑由外部服务维护。本机命令仅限受信任的管理员注册。</div>`;
 $('#new-tool-server').onclick=()=>editToolServer();
 document.querySelectorAll('[data-edit-server]').forEach(b=>b.onclick=()=>editToolServer(servers.find(s=>s.id===b.dataset.editServer)));
 document.querySelectorAll('[data-discover-server]').forEach(b=>b.onclick=async()=>{b.disabled=true;b.textContent='发现中…';try{const r=await api('/api/tool-servers/'+b.dataset.discoverServer+'/discover','POST',{});await toolsView();toast(`已发现 ${(r.tools||[]).length} 个工具`)}catch(e){toast(e.message);b.disabled=false;b.textContent='重试发现'}});
}
function editToolServer(s={id:'',name:'',enabled:true,connection:{url:''}}){
 const c=s.connection,stdio=Boolean(c.command);
 dialog(`<form id="tool-server-form"><div class="dialog-head"><h2>注册外部 MCP 服务</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><div class="field"><label for="tool-id">标识</label><input id="tool-id" required pattern="[a-z][a-z0-9_-]{0,63}" value="${esc(s.id)}" ${s.id?'readonly':''} placeholder="例如 pipeline-tools"></div><div class="field"><label for="tool-name">名称</label><input id="tool-name" required value="${esc(s.name)}"></div><div class="field"><label for="tool-transport">连接方式</label><select id="tool-transport"><option value="http" ${stdio?'':'selected'}>HTTP MCP</option><option value="stdio" ${stdio?'selected':''}>本机命令（stdio）</option></select></div><div id="tool-http"><div class="field"><label for="tool-url">服务地址</label><input id="tool-url" value="${esc(c.url)}" placeholder="https://example.com/mcp"></div><div class="field"><label for="tool-bearer">Bearer Token 环境变量名（可选）</label><input id="tool-bearer" value="${esc(c.bearer_token_env_var)}" placeholder="只填变量名，不填密钥"></div></div><div id="tool-stdio"><div class="field"><label for="tool-command">可执行文件</label><input id="tool-command" value="${esc(c.command)}"></div><div class="field"><label for="tool-args">参数（JSON 数组）</label><textarea id="tool-args">${esc(JSON.stringify(c.args||[]))}</textarea></div><div class="field"><label for="tool-cwd">启动目录（可选）</label><input id="tool-cwd" value="${esc(c.cwd)}"></div><div class="field"><label for="tool-env">传入的环境变量名（每行一个）</label><textarea id="tool-env">${esc((c.env_vars||[]).join('\n'))}</textarea></div></div><label class="check"><input id="tool-enabled" type="checkbox" ${s.enabled?'checked':''}>允许新会话使用</label><p class="hint">保存后点击“发现工具”验证连接。修改配置仅影响新会话；撤销既有访问需在外部服务撤销凭据。</p></div><div class="dialog-footer"><button type="button" data-close>取消</button><button class="primary">保存</button></div></form>`);
 const toggle=()=>{$('#tool-http').hidden=$('#tool-transport').value!=='http';$('#tool-stdio').hidden=$('#tool-transport').value!=='stdio'};$('#tool-transport').onchange=toggle;toggle();
 $('#tool-server-form').onsubmit=async e=>{e.preventDefault();try{const connection=$('#tool-transport').value==='http'?{url:$('#tool-url').value.trim(),bearer_token_env_var:$('#tool-bearer').value.trim()}:{command:$('#tool-command').value.trim(),args:JSON.parse($('#tool-args').value||'[]'),cwd:$('#tool-cwd').value.trim(),env_vars:$('#tool-env').value.split('\n').map(x=>x.trim()).filter(Boolean)};await api('/api/tool-servers'+(s.id?'/'+s.id:''),s.id?'PUT':'POST',{id:$('#tool-id').value,name:$('#tool-name').value,enabled:$('#tool-enabled').checked,connection});$('#dialog').close();await toolsView();toast('已保存，请发现工具')}catch(e){toast(e.message)}};
}
// Display only explicit Agent reply metadata; never turn prose into workflow decisions.
function conversationLabel(data) {
  if ((data.approvals||[]).some(a=>!a.decision)) return (data.approvals||[]).some(a=>!a.decision&&a.request.platform_permission_request)?'等待管理员批准提权':'等待工具审批';
  return labels[data.conversation.status] || data.conversation.status;
}
function renderExecutionStatus(data) {
  if (!data || !$('#execution-status')) return;
  const c = data.conversation;
  const active = data.messages.find(m => m.role === 'user' && m.status === 'running');
  const queued = data.messages.filter(m => m.role === 'user' && m.status === 'queued').length;
  const events = active ? state.events.filter(e => e.message_id === active.id) : [];
  const latest = events.at(-1);
  let title = conversationLabel(data), detail = '', timing = '';
  if (c.status === 'running') {
    const started = events.find(e => e.type === 'platform.execution.started');
    if (started) timing = `本轮已运行 ${elapsed(started.created_at)}`;
    if (latest) timing += `${timing ? ' · ' : ''}最近活动 ${elapsed(latest.created_at)}前`;
  } else if (c.status === 'queued') {
    detail = '输入已保存，等待可用执行位置。';
  } else if (c.status === 'failed') {
    detail = c.error || '查看执行日志后，可补充消息并继续队列。';
  } else if (c.status === 'stopped') {
    detail = '执行已停止，历史与排队消息保留。';
  } else if (c.status === 'stopping') {
    detail = '正在终止当前执行；已完成的操作不会撤销。';
  } else if (c.status === 'idle') {
    detail = '本轮执行已结束，可继续发送消息。';
  } else if (c.status === 'closed') {
    detail = '历史与文件保留，本会话不再接收新输入。';
  }
  if ((data.approvals||[]).some(a=>!a.decision)) {title=(data.approvals||[]).some(a=>!a.decision&&a.request.platform_permission_request)?'等待管理员批准提权':'等待你确认工具调用'; detail='查看调用确认卡片，批准或拒绝后接着本轮执行。';}
  const running = ['running', 'queued', 'stopping'].includes(c.status);
  const connection = state.connection === 'connected' ? '实时连接正常' : state.connection === 'reconnecting' ? '实时连接断开，正在重连；状态仍定期同步' : '正在连接实时输出';
  const queueNote = queued ? `${queued} 条消息已排队；不会打断当前轮。` : c.status === 'running' ? '新消息默认排队，可点击消息旁的“引导”送入当前轮。' : '';
  const syncNote = state.syncFailed ? '暂时无法同步状态；后台可能仍在执行，请等待连接恢复。' : running ? connection : '';
  const compactTitle = state.syncFailed ? '连接异常' : state.connection === 'reconnecting' ? '正在重连' : c.status==='idle' ? '本轮已结束' : title;
  const markup = `<summary role="status" aria-live="polite" aria-label="执行状态：${esc(compactTitle)}"><span class="${running ? 'activity-spinner' : 'status-symbol'}" aria-hidden="true">${running?'':c.status==='failed'?'!':c.status==='idle'?'✓':'○'}</span><span>${esc(compactTitle)}${queued?` · ${queued} 条排队`:''}</span><span class="status-chevron" aria-hidden="true">⌃</span></summary><div class="status-popover">${timing?`<div>${esc(timing)}</div>`:''}${detail?`<div>${esc(detail)}</div>`:''}<div>${esc([syncNote,queueNote].filter(Boolean).join(' · '))}</div></div>`;
  const panel = $('#execution-status');
  const nextClass = `execution-status ${c.status}${state.syncFailed?' sync-failed':''}`;
  if (panel.className !== nextClass && (c.status === 'failed' || state.syncFailed)) panel.open = true;
  panel.className = nextClass;
  if (panel.innerHTML !== markup) panel.innerHTML = markup;
  $('#chat-status').innerHTML = `<span class="badge ${esc(c.status)}">${esc(state.syncFailed ? '状态待同步' : conversationLabel(data))}</span>`;
}
function elapsed(value) {
  const seconds = Math.max(0, Math.floor((Date.now() - Date.parse(value)) / 1000));
  if (!Number.isFinite(seconds)) return '0秒';
  return seconds < 60 ? `${seconds}秒` : `${Math.floor(seconds / 60)}分${seconds % 60}秒`;
}
function receiveConversationEvent(event, updateStatus = true) {
  if (state.events.some(saved => saved.id === event.id)) return;
  state.lastEventID = Math.max(state.lastEventID || 0, event.id);
  state.events.push(event);
  $('#event-count').textContent = `展开原始事件（${state.events.length}）`;
  if ($('#event-log').parentElement?.open) $('#event-log').textContent = JSON.stringify(state.events, null, 2);
  const item = event.raw?.item;
  if ($('#tool-log').parentElement?.open && item && ['command_execution', 'mcp_tool_call', 'file_change'].includes(item.type)) {
    $('#tool-log').textContent += JSON.stringify(event.raw, null, 2) + '\n\n';
  }
  if (updateStatus) renderExecutionStatus(state.view);
}
async function syncConversationEvents(id) {
 const owner=state.events;
 if(state.eventSync?.owner===owner)return state.eventSync.promise;
 const sync={owner};state.eventSync=sync;
 sync.promise=(async()=>{
  while(state.id===id && state.events===owner){
   const after=state.historyEventID||0;
   const events=await api(`/api/conversations/${id}/events?format=json&after=${after}`);
   if(state.id!==id || state.events!==owner)return;
   if(!Array.isArray(events))throw new Error("事件响应格式错误");
   for(const event of events)receiveConversationEvent(event,false);
   if(events.length)state.historyEventID=events[events.length-1].id;
   if(events.length<300)break;
  }
 })();
 try{await sync.promise}finally{if(state.eventSync===sync)state.eventSync=null}
}
function selectConversationPanel(mode='') {
 const layout=$('#conversation-layout'),panel=$('#conversation-panel');
 if(!layout||!panel)return;
 state.panel=mode;
 panel.hidden=!mode;
 layout.classList.toggle('with-panel',Boolean(mode));
 $('#panel-choice').value=mode;
 for(const name of ['files','info','logs','preview'])$('#panel-'+name).hidden=mode!==name;
 $('#panel-title').textContent=({files:'工作区文件',info:'会话信息',logs:'执行日志',preview:'文件预览'})[mode]||'';
}
function conversationFileURL(id,path,download=false) {
 return '/api/conversations/'+encodeURIComponent(id)+'/file?path='+encodeURIComponent(path)+(download?'&download=1':'');
}
function renderConversationFiles(data) {
 const filter=($('#file-filter')?.value||'').trim().toLowerCase();
 const files=data.artifacts.filter(f=>f.path.toLowerCase().includes(filter));
 const html=files.map(f=>`<button class="workspace-file" data-file-path="${esc(f.path)}" type="button"><span aria-hidden="true">${f.image?'▧':'▤'}</span><span>${esc(f.path)}</span><small>${(f.size/1024).toFixed(1)} KB</small></button>`).join('')||'<p class="muted">没有匹配的文件。</p>';
 if(state.renderedFiles!==html){state.renderedFiles=html;$('#artifacts').innerHTML=html}
}
function renderFilePreview() {
 const p=state.preview;
 if(!p)return;
 const url=conversationFileURL(state.id,p.path);
 $('#preview-path').textContent=p.path;
 $('#preview-download').href=conversationFileURL(state.id,p.path,true);
 $('#preview-mode').hidden=!/\.(md|html?)$/i.test(p.path);
 $('#preview-mode').textContent=p.source?'预览':'源码';
 const body=$('#preview-body');
 if(p.image){body.innerHTML=`<img class="preview-image" src="${url}" alt="${esc(p.path)}">`;body.querySelector('img').onerror=()=>{body.innerHTML='<p class="error-box">图片加载失败，请重新打开文件。</p>'};return}
 if(p.text===undefined){body.innerHTML='<p class="muted">此文件暂不支持在线预览，可下载查看。</p>';return}
 if(/\.html?$/i.test(p.path)&&!p.source){body.innerHTML=`<iframe title="${esc(p.path)}" class="preview-frame" sandbox="allow-scripts" src="${url}&preview=1"></iframe><p class="preview-note">隔离预览；完整运行可下载文件。</p>`;return}
 if(/\.md$/i.test(p.path)&&!p.source){body.innerHTML='<div class="preview-markdown">'+messageMarkup(p.text,ref=>artifactReference(ref,state.view?.artifacts,state.view?.conversation.workspace_path,p.path))+'</div>';return}
 body.innerHTML='<pre class="preview-source"><code>'+p.text.split('\n').map((line,index)=>`<span class="source-line${index+1===p.line?' selected-line':''}" data-line="${index+1}"><span class="line-number" aria-hidden="true">${index+1}</span>${esc(line)||' '}</span>`).join('')+'</code></pre>';
 if(p.line)body.querySelector('.selected-line')?.scrollIntoView({block:'center'});
}
async function openConversationFile(path,line=0) {
 const data=state.view,id=state.id;
 const file=data?.artifacts.find(f=>f.path===path);
 if(!file){toast('文件尚未生成或已不在工作区');return}
 selectConversationPanel('preview');
 const version=state.previewVersion=(state.previewVersion||0)+1;
 state.preview=null;
 $('#preview-path').textContent=path;
 $('#preview-download').href=conversationFileURL(id,path,true);
 $('#preview-mode').hidden=true;
 $('#preview-body').innerHTML='<p class="muted" role="status">正在读取文件…</p>';
 const current=()=>state.id===id&&state.previewVersion===version;
 try {
  let text;
  if(!file.image && file.size<=2*1024*1024 && /\.(md|txt|json|ya?ml|csv|go|py|[cm]?js|jsx|ts|tsx|css|html?|log|toml|sh|sql|xml|svg)$/i.test(path)){
   const response=await fetch(conversationFileURL(id,path));
   if(!response.ok)throw new Error('文件读取失败：'+response.status);
   text=await response.text();
  }
  if(!current())return;
  state.preview={path,line,image:file.image,text,source:Boolean(line>1 || line && !/\.md$/i.test(path))};
  renderFilePreview();
 } catch(error){if(current())$('#preview-body').innerHTML=`<p class="error-box">${esc(error.message)}</p><button data-file-path="${esc(path)}" type="button">重新读取</button>`}
}

async function conversationView(id){state.id=id;state.events=[];state.lastEventID=0;state.historyEventID=0;state.eventSync=null;state.view=null;state.connection='connecting';state.syncFailed=false;state.renderedMessages=null;state.renderedFiles=null;state.preview=null;state.previewVersion=(state.previewVersion||0)+1;$('#content').innerHTML=`<a class="back" href="/conversations/" data-nav>← ${state.me.admin?'全部会话':'我的会话'}</a><a id="conversation-workflow" class="wf-link" data-nav hidden>返回编排运行 ↗</a><div class="conversation-toolbar"><label for="panel-choice">侧栏</label><select id="panel-choice"><option value="">隐藏</option><option value="files">文件</option><option value="preview">文件预览</option><option value="info">会话信息</option><option value="logs">执行日志</option></select></div><div class="conversation-layout" id="conversation-layout"><section class="card chat-card"><div class="chat-header"><span class="agent-icon">◇</span><div><h3 id="chat-title">正在读取会话…</h3><div id="chat-status"></div></div></div><div class="timeline" id="timeline"></div><div id="tool-approvals" aria-live="polite"></div><form class="composer" id="reply-form"><textarea id="reply" required placeholder="直接回复，接着这段对话继续。"></textarea><div class="composer-footer"><details id="execution-status" class="execution-status"><summary role="status" aria-live="polite">正在连接…</summary></details><div class="composer-actions"><details id="chat-actions" class="chat-actions"><summary class="icon-button" aria-label="会话操作" title="会话操作">⋯</summary><div class="chat-actions-popover"><button id="share-chat" type="button">复制会话链接</button><button id="close-chat" type="button">关闭会话</button><button id="delete-chat" class="danger" type="button" hidden>删除会话</button></div></details><button id="continue-chat" class="icon-button" type="button" aria-label="继续队列" title="继续队列" hidden>▷</button><button id="stop-chat" class="icon-button stop-button" type="button" aria-label="停止执行" title="停止执行" hidden>■</button><button id="send-reply" class="icon-button primary" type="submit" aria-label="发送消息" title="发送消息">↑</button></div></div></form></section><aside class="conversation-panel card" id="conversation-panel" hidden><div class="panel-header"><strong id="panel-title"></strong><button id="panel-close" class="quiet" aria-label="关闭侧栏" type="button">✕</button></div><section id="panel-info" class="panel-content" hidden><div id="chat-error"></div><dl class="keyval" id="chat-info"></dl><div class="status-note" id="status-note"></div><button id="apply-agent-permissions" type="button" hidden>应用 Agent 当前联网与提权设置</button></section><section id="panel-files" class="panel-content" hidden><input id="file-filter" type="search" placeholder="搜索文件…" aria-label="搜索工作区文件"><div id="artifacts"></div></section><section id="panel-logs" class="panel-content" hidden><details><summary id="event-count">展开原始事件</summary><pre id="event-log"></pre></details><details><summary>工具与执行日志</summary><pre id="tool-log"></pre></details></section><section id="panel-preview" class="file-preview" hidden><div class="preview-toolbar"><span id="preview-path">选择对话中的文件，或从文件列表打开。</span><button id="preview-mode" class="quiet small" type="button" hidden>源码</button><a id="preview-download" download>下载</a></div><div id="preview-body"></div></section></aside></div>`;
 selectConversationPanel();
 $('#panel-choice').onchange=e=>selectConversationPanel(e.target.value);
 $('#panel-close').onclick=()=>selectConversationPanel();
 $('#file-filter').oninput=()=>{if(state.view)renderConversationFiles(state.view)};
 $('#preview-mode').onclick=()=>{if(state.preview){state.preview.source=!state.preview.source;renderFilePreview()}};
 $('#conversation-layout').addEventListener('click',e=>{const file=e.target.closest('[data-file-path]');if(file){e.preventDefault();openConversationFile(file.dataset.filePath,Number(file.dataset.fileLine)||0)}});
 $('#timeline').addEventListener('click',async e=>{const button=e.target.closest('[data-steer]');if(!button)return;button.disabled=true;try{await api('/api/conversations/'+id+'/steer','POST',{message_id:Number(button.dataset.steer)});await refreshConversation(id)}catch(error){toast(error.message);button.disabled=false}});
 $('#timeline').addEventListener('toggle',()=>renderTranscript(state.view),true);
 $('#event-log').parentElement.addEventListener('toggle',()=>{$('#event-log').textContent=JSON.stringify(state.events,null,2)});
 $('#tool-log').parentElement.addEventListener('toggle',()=>{$('#tool-log').textContent=JSON.stringify(state.events.filter(e=>e.raw?.item && e.raw.item.type !== 'agent_message'),null,2)});
 $('#reply-form').addEventListener('keydown',e=>{if(e.key==='Escape')$('#chat-actions').open=false});
 $('#conversation-layout').addEventListener('click',e=>{if(!e.target.closest('#chat-actions')||e.target.closest('#chat-actions button'))$('#chat-actions').open=false});
 $('#share-chat').onclick=async()=>{const url=location.origin+'/conversations/'+id;try{await navigator.clipboard.writeText(url);toast('会话链接已复制；打开后按账号权限访问')}catch{dialog(`<div class="dialog-head"><h2>会话链接</h2><button data-close>关闭</button></div><div class="dialog-body"><textarea readonly>${esc(url)}</textarea></div>`)}};
 $('#reply-form').onsubmit=async e=>{e.preventDefault();const text=$('#reply').value;$('#send-reply').disabled=true;try{await api('/api/conversations/'+id+'/messages','POST',platformInputs.body(id,{message:text,user_id:state.current?.user_id||'operator'}));platformInputs.accepted(id);$('#reply').value='';await refreshConversation(id)}catch(err){toast(err.message)}finally{if($('#send-reply'))$('#send-reply').disabled=false}};
 $('#delete-chat').onclick=async()=>{if(confirm('永久删除这段已关闭会话、原生记录和平台工作区？外部项目目录不会删除；删除后无法接续。')){try{await api('/api/conversations/'+id,'DELETE');go('/conversations/')}catch(e){toast(e.message)}}};$('#stop-chat').onclick=()=>conversationAction(id,'stop');$('#continue-chat').onclick=()=>conversationAction(id,'continue');$('#close-chat').onclick=()=>{if(confirm('关闭后保留历史，但不能继续执行。确定关闭？'))conversationAction(id,'close')};
 await refreshConversation(id);if(state.id!==id)return;
 startConversationLive();
}
function renderTranscript(data) {
 if (!data) return;
 const timeline=$('#timeline');
 const bottom=timeline.scrollHeight-timeline.scrollTop-timeline.clientHeight<100;
 const disclosures=[...timeline.querySelectorAll('details[data-disclosure]')];
 const expanded=new Set(disclosures.filter(el=>el.open).map(el=>el.dataset.disclosure));
 const parts=[],seenTurns=new Set();
 const entries=platformTranscript.entries(data,state.events);
 const active=data.messages.find(m=>m.role==='user' && m.status==='running');
 const hasOutput=new Set(entries.filter(e=>e.tool||e.reasoning||e.message?.role==='agent').map(e=>e.tool?.parent??e.reasoning?.parent??e.message.parent_id));
 let turn=null;
 const flush=()=>{
  if(!turn)return;
  const first=!seenTurns.has(turn.parent);seenTurns.add(turn.parent);
  parts.push(`<article class="message agent agent-turn${first?'':' continuation'}" data-turn="${esc(turn.parent)}">${first?`<span class="avatar${active?.id===turn.parent?' agent-active':''}" ${active?.id===turn.parent?'aria-label="Agent 正在执行"':''}>◇</span>`:''}<div class="message-body">${first?`<div class="message-meta"><strong>${esc(data.conversation.agent_name)}</strong><span class="message-time">${time(turn.at)}</span></div>`:''}<div class="agent-turn-content">${turn.content.join('')}</div></div></article>`);
  turn=null;
 };
 for(const entry of entries){
  const m=entry.message;
  if(m?.role==='user'){
   flush();
   parts.push(`<article class="message user ${m.kind}"><span class="avatar">我</span><div class="message-body"><div class="message-meta"><strong>用户</strong><span>${m.status==='queued'?'待执行':m.status==='steering'?'正在引导':m.status==='steered'?'已引导':m.status==='failed'?'执行失败':m.status==='stopped'?'已停止':''}</span>${m.status==='queued'&&data.conversation.status==='running'?`<button type="button" class="quiet small" data-steer="${m.id}" title="提交给当前轮，不中断模型运行" aria-label="引导当前执行">↪ 引导</button>`:''}<span class="message-time">${time(m.created_at)}</span></div><div class="bubble">${messageMarkup(m.content,ref=>artifactReference(ref,data.artifacts,data.conversation.workspace_path))}</div></div></article>`);
   if(m.id===active?.id && !hasOutput.has(m.id))turn={parent:m.id,at:entry.at,content:['<span class="awaiting-output" role="status">正在处理…</span>']};
   continue;
  }
  const parent=entry.tool?.parent??entry.reasoning?.parent??m.parent_id;
  if(turn && turn.parent!==parent)flush();
  if(!turn)turn={parent,at:entry.at,content:[]};
  turn.content.push(entry.tool?platformTranscript.toolMarkup(entry.tool,expanded):entry.reasoning?platformTranscript.reasoningMarkup(entry.reasoning,expanded):`<div class="agent-text ${esc(m.kind)}"><div class="bubble">${messageMarkup(m.content,ref=>artifactReference(ref,data.artifacts,data.conversation.workspace_path))}</div></div>`);
 }
 flush();
 const markup=parts.join('');
 if(state.renderedMessages===markup)return;
 state.renderedMessages=markup;
 timeline.innerHTML=markup||'<div class="empty">输入已保存，正在等待执行。</div>';
 timeline.querySelectorAll('details[data-disclosure]').forEach(el=>{el.open=expanded.has(el.dataset.disclosure)});
 if(bottom)timeline.scrollTop=timeline.scrollHeight;
}
async function refreshConversation(id){
 const owner=state.events;
 if(state.refresh?.owner===owner&&state.refresh.id===id)return state.refresh.promise;
 const refresh={owner,id};state.refresh=refresh;
 refresh.promise=loadConversation(id);
 try{return await refresh.promise}finally{if(state.refresh===refresh)state.refresh=null}
}
async function loadConversation(id){const revision=(state.refreshRevision||0)+1;state.refreshRevision=revision;let data;try{data=await api('/api/conversations/'+id);await syncConversationEvents(id)}catch(error){if(state.id===id){state.syncFailed=true;renderExecutionStatus(state.view)}throw error}if(state.id!==id||revision<(state.appliedRevision||0))return;state.appliedRevision=revision;state.syncFailed=false;state.view=data;if($('#conversation-workflow')){$('#conversation-workflow').hidden=!data.workflow_run_id;$('#conversation-workflow').href='/workflow-runs/'+data.workflow_run_id;}const c=data.conversation;state.current=c;$('#chat-title').textContent=c.agent_name;$('#chat-status').innerHTML=badge(c.status);$('#chat-error').innerHTML=c.error?`<div class="error-box">${esc(c.error)}</div>`:'';$('#chat-info').innerHTML=`<div><dt>会话标识</dt><dd class="mono">${esc(c.id)}</dd></div><div><dt>用户</dt><dd>${esc(c.user_id)}</dd></div><div><dt>工作目录</dt><dd class="mono">${esc(c.workspace_path||'平台会话目录')}</dd></div><div><dt>创建时间</dt><dd>${time(c.created_at)}</dd></div><div><dt>联网</dt><dd>${data.execution_permissions?.network_access?'已允许':'未预先允许'}</dd></div><div><dt>提权申请</dt><dd>${data.execution_permissions?.allow_elevation?'由管理员逐次审批':'已关闭'}</dd></div>`;$('#status-note').textContent=({running:'Agent 正在执行。新消息默认排队，可点击消息旁的“引导”送入当前轮。',queued:'输入已保存，等待本机执行位置。',idle:'本轮已结束。是否需要进一步处理，以 Agent 的回复为准。',stopping:'正在终止当前执行，已完成的操作不会被撤销。',stopped:'已停止。队列保留，明确继续后再处理。',failed:'本轮失败，已保留记录。查看原因后可补充消息并继续。',closed:'会话已关闭，历史和文件仍可查看。'}[c.status]||'');$('#apply-agent-permissions').hidden=!state.me.admin||!c.agent_id;$('#apply-agent-permissions').disabled=['running','stopping','closed'].includes(c.status);$('#apply-agent-permissions').onclick=async()=>{try{await api('/api/conversations/'+id+'/apply-agent-permissions','POST',{});toast('权限已更新，下轮生效；原会话保留');await refreshConversation(id)}catch(error){toast(error.message)}};$('#stop-chat').hidden=!['running','queued','stopping'].includes(c.status);$('#stop-chat').disabled=c.status==='stopping';$('#continue-chat').hidden=!['failed','stopped'].includes(c.status);$('#close-chat').disabled=c.status==='closed';$('#delete-chat').hidden=!state.me.admin||c.status!=='closed'||Boolean(data.workflow_run_id);$('#reply').disabled=c.status==='closed'||data.workflow_input_open===false;$('#send-reply').disabled=c.status==='closed'||data.workflow_input_open===false;
 renderTranscript(data);
 renderConversationFiles(data);
 renderToolApprovals(data,id);
 if(state.live)syncConversationStream();
 renderExecutionStatus(data);
}
async function conversationAction(id,action){try{await api('/api/conversations/'+id+'/'+action,'POST',{});await refreshConversation(id);toast(action==='continue'?'已恢复队列；失败输入不会自动重放。':action==='stop'?'已请求停止当前执行。':'会话已关闭。')}catch(e){toast(e.message)}}
async function integrationsView(){
 $('#content').innerHTML=heading('API 接入','使用用户的 API Token 下发任务，固定会话链接用于网页登录。')+`<section class="card"><h2>接入三步</h2><ol><li>在<a href="/users" data-nav>用户页面</a>创建账号并生成该用户的 API Token。</li><li>在<a href="/agents" data-nav>Agent 配置</a>中勾选授权用户。</li><li>调用 API，保存返回的 conversation_id 和 conversation_url。</li></ol><p>Token 只绑定一个用户，无需自动轮换。可以在用户页面撤销或重新生成；网页使用用户名密码登录，API Token 不放在链接里。</p><a class="button-link" href="/api-docs" data-nav>查看 API 参数与示例 →</a></section>`;
}
async function usersView(){
 const users=await api('/api/users');state.users=users;
 $('#content').innerHTML=heading('用户与角色','每个用户有固定 User ID；API 与网页登录访问同一份会话。','<button id="new-user" class="primary">＋ 新建用户</button>')+`<div class="table-card"><div class="user-row table-head"><div>用户名</div><div>角色</div><div>User ID / API Token</div><div>操作</div></div>${users.map(u=>`<div class="user-row"><div><strong>${esc(u.username)}</strong><div class="row-sub">${u.enabled?'已启用':'已停用'}</div></div><div>${u.role==='admin'?'管理员':'用户'}</div><div><span class="mono">${esc(u.user_id)}</span><div class="row-sub">${u.token_enabled?'API Token 有效':'未设置 API Token'}</div></div><div><button data-user="${esc(u.username)}">编辑</button><button data-token="${esc(u.user_id)}" ${u.enabled?'':'disabled'}>${u.token_enabled?'重置 Token':'生成 Token'}</button>${u.token_enabled?`<button class="danger" data-revoke="${esc(u.user_id)}">撤销 Token</button>`:''}</div></div>`).join('')}</div><div class="callout">允许使用哪些 Agent，在 Agent 的“授权用户”中设置。用户只能查看自己的会话。会话链接没有临时 Token，未登录时登录即可。</div>`;
 $('#new-user').onclick=()=>editUser();
 document.querySelectorAll('[data-user]').forEach(b=>b.onclick=()=>editUser(users.find(u=>u.username===b.dataset.user)));
 document.querySelectorAll('[data-token]').forEach(b=>b.onclick=async()=>{
  const u=users.find(u=>u.user_id===b.dataset.token);
  if(u.token_enabled&&!confirm('重新生成会立即使旧 API Token 失效。继续？'))return;
  try{const r=await api('/api/users/'+encodeURIComponent(u.user_id)+'/token','POST',{});await usersView();dialog(`<div class="dialog-head"><h2>${esc(u.username)} 的 API Token</h2><button data-close>关闭</button></div><div class="dialog-body"><p>仅本次显示，请保存在调用系统中。User ID：<code>${esc(r.user_id)}</code></p><textarea readonly id="one-time-token">${esc(r.token)}</textarea><button id="copy-token">复制 Token</button></div>`);$('#copy-token').onclick=async()=>{try{await navigator.clipboard.writeText(r.token);toast('已复制')}catch{$('#one-time-token').select();toast('已选中，请复制')}}}catch(e){toast(e.message)}
 });
 document.querySelectorAll('[data-revoke]').forEach(b=>b.onclick=async()=>{if(!confirm('撤销后，该 Token 不能再调用 API。网页登录和会话记录保留。'))return;try{await api('/api/users/'+encodeURIComponent(b.dataset.revoke)+'/token','DELETE');await usersView();toast('API Token 已撤销')}catch(e){toast(e.message)}});
}
function editUser(user={username:'',role:'caller',user_id:'',enabled:true}){
 dialog(`<form id="user-form"><div class="dialog-head"><h2>${user.username?'编辑用户':'新建用户'}</h2><button type="button" data-close>关闭</button></div><div class="dialog-body"><div class="fields"><div class="field"><label for="user-name">用户名</label><input autocomplete="username" id="user-name" required ${user.username?'readonly':''} value="${esc(user.username)}"></div><div class="field"><label for="user-role">角色</label><select id="user-role"><option value="caller" ${user.role==='caller'?'selected':''}>用户</option><option value="admin" ${user.role==='admin'?'selected':''}>管理员</option></select></div>${user.user_id?`<div class="field full"><label>User ID</label><code>${esc(user.user_id)}</code><span class="hint">创建后固定不变。</span></div>`:''}<div class="field full"><label for="user-password">密码</label><input id="user-password" type="password" autocomplete="new-password" ${user.username?'':'required'} placeholder="${user.username?'留空保留原密码':'设置登录密码'}"></div></div><label class="check"><input id="user-enabled" type="checkbox" ${user.enabled?'checked':''}>启用账号</label><p class="muted small">保存后该账号需要重新登录；停用账号也会阻止 API 调用。</p></div><div class="dialog-footer"><button type="button" data-close>取消</button><button class="primary">保存用户</button></div></form>`);
 $('#user-form').onsubmit=async e=>{e.preventDefault();try{await api('/api/users','POST',{username:$('#user-name').value,role:$('#user-role').value,user_id:user.user_id,enabled:$('#user-enabled').checked,password:$('#user-password').value});$('#dialog').close();await usersView();toast('用户已保存')}catch(err){toast(err.message)}};
}
function parameterTable(rows){return `<table class="param-table"><thead><tr><th>参数</th><th>类型 / 必填</th><th>含义</th></tr></thead><tbody>${rows.map(([name,type,meaning])=>`<tr><td><code>${esc(name)}</code></td><td>${esc(type)}</td><td>${esc(meaning)}</td></tr>`).join('')}</tbody></table>`;}
function apiDocsView(){
 const payload={agent_id:state.agents[0]?.id||'<agent_id>',message:'请澄清产品规则',request_id:'event-001'};
 $('#content').innerHTML=heading('API 文档','用户 Token 绑定调用身份；conversation_id 锚定原会话，网页通过账号登录。')+`<section class="card"><h2>工作流与 CI 接入</h2><p>CI 或其他入口提交任务后即可返回；平台负责后续 Agent 交接和流程推进。Python SDK 0.2.0 同时支持会话和工作流。</p><p><code>POST /api/workflow-runs</code>：workflow_id、input、workspace_path、request_id；可选 parameters（字符串映射）和 start_node。工作区必须是已准备的独立 checkout。GitHub 适配器会自动准备。</p><p><code>GET /api/workflow-runs/by-request?request_id=...</code>：按当前调用用户的原事件键找回 Run，避免响应丢失后重复接单。</p><p><code>GET /api/workflow-runs/{id}</code>：查询节点、会话、回执和状态。<code>GET /api/workflow-runs?workflow_id=...&amp;before=...</code>：分页查询。</p><p><code>POST /api/workflow-runs/{id}/messages</code>：message、request_id，可选 seq。平台在同一事务内选择当前 Agent 并保存反馈；相同事件重试返回原消息回执。409 表示当前节点不能接收，应先查看 Run 并处理恢复。</p><p><code>POST /api/workflow-runs/{id}/stop | resume | return | decision</code>：显式运行控制，必须带当前 seq。未知网络结果先查询状态，不自动重复操作。</p><p>SDK 对应：<code>start_workflow</code>、<code>workflow_by_request</code>、<code>workflow_run</code>、<code>workflow_runs</code>、<code>workflow_message</code>、<code>workflow_command</code>、<code>wait_workflow</code>。使用维护源 <code>scripts/setup-runner.sh</code> 安装或升级，说明见 <code>sdk/python/README.md</code>。</p><p>GitHub 与网页是并列入口。GitHub 入口用原 Issue ID 去重并关联同一 Run；其他渠道复用 API，不需要复制研发编排。Token 必须拥有工作流及各 Connector 的权限；仅旧引用节点仍需对应共享 Agent 权限。</p></section><section class="card"><h2>创建 / 接续会话</h2><p><code>POST /api/invoke</code> · Authorization: Bearer &lt;用户 Token&gt;</p>${parameterTable([['agent_id','string · 创建必填','选择授权给该用户的 Agent。'],['user_id','string · 可省略','平台从 Token 确定用户；如传入，普通用户必须与 Token 身份一致。'],['conversation_id','string · 接续必填','首次省略；接续时传原 ID。'],['message','string · 必填','本轮输入。运行中提交会保存排队。'],['workspace_path','string · 可选','创建会话时传本机已有目录的绝对路径；接续无需再传，不能切换目录。'],['request_id','string · 推荐','同用户范围去重；同键不同内容返回 409。']])}<pre>${esc(JSON.stringify(payload,null,2))}</pre><p>返回 202：conversation_id、message_id、status、conversation_url、duplicate。202 表示已保存输入；查询结果或订阅事件获取进展。</p><h2>查询与操作</h2><p><code>GET /api/me</code> 查看 Token 对应的 User ID。</p><p><code>GET /api/conversations</code> 查看自己的会话。</p><p><code>GET /api/conversations/{id}</code> 查看状态、消息与产物。</p><p><code>GET /api/conversations/{id}/events</code> 持久 SSE；after / Last-Event-ID 恢复游标，format=json 查询数组。</p><p><code>GET /api/conversations/{id}/file?path=prd.md&amp;download=1</code> 下载实际产物。</p><p><code>POST /api/conversations/{id}/messages</code> 提交 message 接续；也可使用 /api/webhooks/{agent_id}。</p><p><code>POST /api/conversations/{id}/stop | continue | close</code> 停止、继续队列或关闭。</p><p><code>POST /api/conversations/{id}/steer</code>，传 <code>message_id</code> 将本会话的一条排队消息引导到当前轮。202 表示正在发送；消息变为 steered 才表示原生执行器确认接收。明确拒绝时仍排队，结果未知时标为失败且不自动重发。</p><h2>网页入口</h2><p>直接展示返回的 <code>conversation_url</code>。未登录时使用用户名密码登录，随后自动回到原会话；已登录直接检查会话权限。链接没有授权 Token，不需要刷新。</p><h2>用户 Token 管理</h2><p>管理员在用户页面生成或撤销 Token；每个用户只有一个当前有效的 API Token。重置旧 Token 立即失效，撤销不删除会话、不影响正常网页登录。</p><p><code>POST /api/users/{user_id}/token</code> 生成 / 重置，原文只返回一次。<br><code>DELETE /api/users/{user_id}/token</code> 撤销。</p><h2>请求调试</h2><form id="api-try"><div class="field"><label for="api-token">用户 API Token</label><input id="api-token" type="password" autocomplete="off" required></div><div class="field"><label for="api-body">JSON 请求体</label><textarea id="api-body" class="code-input">${esc(JSON.stringify(payload,null,2))}</textarea></div><button class="primary">提交到 /api/invoke</button></form><pre id="api-result">尚未发送请求</pre></section>`;
 $('#api-try').onsubmit=async e=>{e.preventDefault();try{const r=await fetch('/api/invoke',{method:'POST',headers:{'Content-Type':'application/json','Authorization':'Bearer '+$('#api-token').value},body:JSON.stringify(JSON.parse($('#api-body').value))});$('#api-result').textContent='HTTP '+r.status+'\n'+JSON.stringify(await r.json(),null,2)}catch(err){toast(err.message)}};
}

function showRouteError(error){clearLive();if(error.status===401){login();return}if($('#content')){$('#content').innerHTML=heading(error.status===403?'无权访问这段会话':'页面暂时无法打开',error.status===403?'请使用拥有此会话的账号登录，或回到你自己的会话。':error.message)+`<a class="button-link" href="/conversations/" data-nav>返回我的会话</a>`}else{login();toast(error.message)}}
route().catch(showRouteError);
