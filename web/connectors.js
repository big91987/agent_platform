'use strict';
const connectorKinds={'command':'本机命令','github.issue':'GitHub · 关联或创建 Issue','github.issue_create':'GitHub · 创建 Issue','github.issue_comment':'GitHub · Issue 评论','github.pull_request':'GitHub · 创建草稿 PR'};
async function connectorManager(){
 const entries=await api('/api/connectors');
 dialog(`<div class="dialog-head"><h2>Connector</h2><button data-close>✕</button></div><div class="dialog-body"><p class="hint">绑定受控外部操作。配置变更用于新运行；停用立即阻止后续调用。</p>${entries.map(v=>`<div class="wf-run-row"><span><strong>${esc(v.name)}</strong><small>${esc(connectorKinds[v.kind])} · v${v.revision} · ${v.enabled?'启用':'停用'}</small></span><button data-edit-connector="${esc(v.id)}">配置</button></div>`).join('')||'<p>还没有 Connector。</p>'}</div><div class="dialog-footer"><button data-close>关闭</button><button id="connector-new" class="primary">新增 Connector</button></div>`);
 $('#connector-new').onclick=()=>connectorForm();document.querySelectorAll('[data-edit-connector]').forEach(b=>b.onclick=()=>connectorForm(entries.find(v=>v.id===b.dataset.editConnector)));
}
async function connectorForm(existing){
 const v=existing||{name:'',kind:'command',enabled:true,authorized_users:[],timeout_seconds:60,args:[],env_refs:{}};
 const users=await api('/api/users');
 dialog(`<form id="connector-form"><div class="dialog-head"><h2>${v.id?'配置':'新增'} Connector</h2><button type="button" data-close>✕</button></div><div class="dialog-body"><div class="field"><label for="connector-name">名称</label><input id="connector-name" required value="${esc(v.name)}"></div><div class="field"><label for="connector-kind">操作</label><select id="connector-kind">${Object.entries(connectorKinds).map(([key,name])=>`<option value="${key}" ${key===v.kind?'selected':''}>${name}</option>`).join('')}</select></div><div class="field"><label for="connector-root">允许的工作区根目录</label><input id="connector-root" required value="${esc(v.workspace_root||'')}" placeholder="绝对路径"></div><div id="connector-command"><div class="field"><label for="connector-executable">可执行文件</label><input id="connector-executable" value="${esc(v.executable||'')}" placeholder="例如 /usr/bin/python3"></div><div class="field"><label for="connector-args">固定参数（JSON 数组）</label><textarea id="connector-args" rows="3">${esc(JSON.stringify(v.args||[],null,2))}</textarea></div><div class="field"><label for="connector-env">环境变量引用（JSON 对象）</label><textarea id="connector-env" rows="2">${esc(JSON.stringify(v.env_refs||{},null,2))}</textarea><p class="hint">例如 {"GH_TOKEN":"WORKFLOW_GITHUB_TOKEN"}。填写变量名，密钥在服务环境中配置。节点任务通过 stdin JSON 传入。</p></div></div><div id="connector-github"><div class="field"><label for="connector-repo">GitHub 仓库</label><input id="connector-repo" value="${esc(v.repository||'')}" placeholder="owner/repository"></div><div class="field"><label for="connector-token">GitHub 凭据名称</label><input id="connector-token" value="${esc(v.token_env||'WORKFLOW_GITHUB_TOKEN')}"></div></div><div class="field"><label for="connector-timeout">超时秒数</label><input id="connector-timeout" type="number" min="1" max="1800" required value="${v.timeout_seconds}"></div><label class="check"><input id="connector-enabled" type="checkbox" ${v.enabled?'checked':''}>启用</label><h4>授权用户</h4><p class="hint">管理员可用。图的授权不会代替 Connector 的授权。</p>${users.filter(u=>u.role!=='admin').map(u=>`<label class="check"><input name="connector-user" type="checkbox" value="${esc(u.user_id)}" ${v.authorized_users.includes(u.user_id)?'checked':''}>${esc(u.username)}</label>`).join('')}<p id="connector-check-result" class="hint" role="status"></p></div><div class="dialog-footer"><button type="button" id="connector-back">返回列表</button>${v.id?'<button type="button" id="connector-check">检查已保存配置</button>':''}<button type="submit" class="primary">保存 Connector</button></div></form>`);
 const show=()=>{const command=$('#connector-kind').value==='command';$('#connector-command').hidden=!command;$('#connector-github').hidden=command};show();$('#connector-kind').onchange=show;
 $('#connector-back').onclick=()=>connectorManager();
 if($('#connector-check'))$('#connector-check').onclick=async e=>{e.target.disabled=true;try{await api('/api/connectors/'+v.id+'/check','POST',{});$('#connector-check-result').textContent='配置检查通过：未执行命令或写入操作。'}catch(err){$('#connector-check-result').textContent=err.message}finally{e.target.disabled=false}};
 $('#connector-form').onsubmit=async e=>{e.preventDefault();e.submitter.disabled=true;try{
  const next={id:v.id||'',revision:v.revision||0,name:$('#connector-name').value,kind:$('#connector-kind').value,enabled:$('#connector-enabled').checked,workspace_root:$('#connector-root').value,timeout_seconds:Number($('#connector-timeout').value),authorized_users:[...document.querySelectorAll('[name="connector-user"]:checked')].map(el=>el.value)};
  if(next.kind==='command'){next.executable=$('#connector-executable').value;next.args=JSON.parse($('#connector-args').value);next.env_refs=JSON.parse($('#connector-env').value)}else{next.repository=$('#connector-repo').value;next.token_env=$('#connector-token').value}
  await api('/api/connectors'+(v.id?'/'+v.id:''),v.id?'PUT':'POST',next);toast('Connector 已保存');await connectorManager();
 }catch(err){toast(err.message);e.submitter.disabled=false}};
}

function connectorNodeFields(editor,n){
 const c=editor.connectors.find(c=>c.id===n.connector_id),p=n.connector_input||{};
 if(!c)return '<p class="hint">先选择一个 Connector。</p>';
 if(c.kind==='command')return '<p class="hint">使用管理员配置的固定命令。任务和前序节点结果通过标准输入传入；输出路径为 next 或 failed。</p>';
 const text=(key,label,placeholder='')=>`<div class="field"><label for="wf-c-${key}">${label}</label><input id="wf-c-${key}" value="${esc(p[key]||'')}" placeholder="${esc(placeholder)}"></div>`;
 let fields=c.kind==='github.issue_comment'?'':text('title','标题','留空使用任务首行');
 fields+=text('body_file','正文文件','可选：工作区相对路径；留空使用任务原文');
 if(c.kind==='github.pull_request')fields+=text('head','来源分支','workflow/{{run_id}}')+text('base','目标分支','main');
 if(c.kind==='github.issue')fields+=text('issue_parameter','已有 Issue 编号来自运行参数','例如 issue_number；参数未提供时创建 Issue');
 if(!['github.issue_create','github.issue'].includes(c.kind))fields+=`<div class="field"><label for="wf-c-issue_node">使用流程关联的 Issue</label><select id="wf-c-issue_node"><option value="">不引用节点</option>${editor.graph.value.nodes.filter(x=>x.id!==n.id&&['github.issue','github.issue_create'].includes(editor.connectors.find(v=>v.id===x.connector_id)?.kind)).map(x=>`<option value="${esc(x.id)}" ${x.id===p.issue_node?'selected':''}>${esc(x.name)}</option>`).join('')}</select></div>${text('issue_number','或指定 Issue 编号','例如 1')}`;
 return fields+'<p class="hint">标题和分支支持 {{input}}、{{run_id}}；PR 始终创建为草稿，不自动合并。</p>';
}
function readConnectorNodeFields(){
 const value={};for(const k of ['title','body_file','head','base','issue_node','issue_number','issue_parameter']){const el=$('#wf-c-'+k);if(el?.value)value[k]=k==='issue_number'?Number(el.value):el.value}
 return value;
}
