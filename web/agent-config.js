'use strict';
// Common execution settings; identity and authorization belong to each entry page.
function agentExecutionLabel(a={}){return `${a.executor==='codex'?'Codex':a.executor||'未选择执行器'} · ${a.model||'默认模型'}`;}
function agentConfigFields(a,catalog=[],prefix='agent',envDraft,runWorkspace=false){
 const text=(key,label,value,rows)=>`<div class="field"><label for="${prefix}-${key}">${label}</label>${rows?`<textarea id="${prefix}-${key}" rows="${rows}">${esc(value||'')}</textarea>`:`<input id="${prefix}-${key}" value="${esc(value||'')}">`}</div>`;
 const check=(key,label,value)=>`<label class="check"><input id="${prefix}-${key}" type="checkbox" ${value?'checked':''}>${label}</label>`;
 const bindings=a.tool_servers||[];
 const servers=[...catalog,...bindings.filter(b=>!catalog.some(s=>s.id===b.server_id)).map(b=>({id:b.server_id,name:b.server_id+'（不可用）',tools:[]}))];
 const tools=servers.map((s,i)=>{
  const binding=bindings.find(b=>b.server_id===s.id),known=s.tools||[];
  const entries=[...known,...(binding?.tools||[]).filter(name=>!known.some(t=>t.name===name)).map(name=>({name,description:'当前目录中不可用'}))];
  return `<fieldset><legend>${esc(s.name)}${s.enabled===false?'（已停用）':''}</legend>${entries.map((t,j)=>`<div class="tool-choice"><label class="check"><input type="checkbox" id="${prefix}-tool-${i}-${j}" data-agent-tool data-server="${esc(s.id)}" value="${esc(t.name)}" ${binding?.tools.includes(t.name)?'checked':''}>${esc(t.name)}</label><p class="hint">${esc(t.description||'')}</p><label for="${prefix}-tool-approval-${i}-${j}">调用审批</label><select id="${prefix}-tool-approval-${i}-${j}" data-agent-approval data-server="${esc(s.id)}" data-tool="${esc(t.name)}"><option value="auto">自动审批</option><option value="confirm" ${binding?.approvals?.[t.name]==='confirm'?'selected':''}>每次确认</option></select></div>`).join('')||'<p class="hint">尚未发现工具。</p>'}</fieldset>`;
 }).join('');
 return `<div class="field"><label for="${prefix}-executor">执行器</label><select id="${prefix}-executor"><option value="codex">Codex</option></select></div>${text('model','模型（留空使用默认值）',a.model)}${text('instructions','角色指令',a.instructions,6)}<details><summary>执行权限</summary><div class="field"><label for="${prefix}-sandbox">文件操作范围</label><select id="${prefix}-sandbox"><option value="workspace-write" ${a.sandbox!=='read-only'?'selected':''}>允许写入工作区</option><option value="read-only" ${a.sandbox==='read-only'?'selected':''}>只读</option></select></div>${check('network','允许联网',a.network_access)}${check('elevation','权限不足时允许申请管理员审批',a.allow_elevation)}</details><details><summary>Skills 与外部工具</summary>${text('skills','Skill 目录（每行一个）',(a.skills||[]).join('\n'),4)}<p class="hint">目录须包含 SKILL.md。工具连接在外部工具页面注册和发现。</p><div id="${prefix}-tools">${tools||'<p class="hint">尚未注册外部工具。</p>'}</div></details><details><summary>高级：原生配置、Hook 与环境</summary>${runWorkspace?'<p class="hint">工作区在开始运行时统一指定，所有节点使用同一目录。</p>':text('seed','工作区模板目录（可选）',a.seed_dir)+'<p class="hint">未指定已有工作区时，用于初始化目录；已有工作区直接使用其文件和 AGENTS.md。</p>'}${text('native','原生 TOML 配置',a.native_config,6)}${check('trust','信任配置中的本机 Hook 命令',a.trust_hooks)}${check('inherit','继承本机环境',a.inherit_env)}${text('env','环境覆盖 / 清除（JSON）',envDraft??JSON.stringify(a.env||{},null,2),5)}<p class="hint">字符串覆盖环境变量，null 清除变量。</p></details>`;
}
function readAgentConfig(source,prefix='agent',validate=true,runWorkspace=false){
 const a=JSON.parse(JSON.stringify(source));
 for(const key of ['executor','model','instructions'])a[key]=$('#'+prefix+'-'+key).value;
 a.sandbox=$('#'+prefix+'-sandbox').value;if(!runWorkspace)a.seed_dir=$('#'+prefix+'-seed').value;a.native_config=$('#'+prefix+'-native').value;
 a.skills=$('#'+prefix+'-skills').value.split('\n').map(s=>s.trim()).filter(Boolean);
 for(const [key,id]of [['network_access','network'],['allow_elevation','elevation'],['trust_hooks','trust'],['inherit_env','inherit']])a[key]=$('#'+prefix+'-'+id).checked;
 const raw=$('#'+prefix+'-env').value;
 try{a.env=agentEnvironment(raw)}catch(error){if(validate)throw error;}
 const container=$('#'+prefix+'-tools'),modes=[...container.querySelectorAll('[data-agent-approval]')],bindings=new Map();
 container.querySelectorAll('[data-agent-tool]:checked').forEach(el=>{
  const server=el.dataset.server;if(!bindings.has(server))bindings.set(server,{server_id:server,tools:[],approvals:{}});
  const b=bindings.get(server);b.tools.push(el.value);b.approvals[el.value]=modes.find(m=>m.dataset.server===server&&m.dataset.tool===el.value)?.value||'auto';
 });
 a.tool_servers=[...bindings.values()];return a;
}
function agentEnvironment(raw){
 try{
  const env=JSON.parse(raw||'{}');
  if(!env||Array.isArray(env)||typeof env!=='object'||Object.values(env).some(v=>v!==null&&typeof v!=='string'))throw new Error();
  return env;
 }catch(error){throw new Error('环境配置必须是 JSON 对象，变量值使用字符串或 null');}
}
