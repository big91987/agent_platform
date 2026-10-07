'use strict';
// Shared presentation only; editor drafts and Run facts stay with their owners.
function workflowPanelMarkup(prefix,title,meta,tabs,active){
 return `<header class="wf-panel-head"><div><p class="wf-panel-meta">${esc(meta)}</p><h2 id="${prefix}-panel-title">${esc(title)}</h2></div><button id="${prefix}-panel-close" class="wf-panel-close" aria-label="关闭节点详情" title="关闭节点详情（Esc）">×</button></header><div id="${prefix}-panel-tabs" class="wf-panel-tabs" role="tablist" aria-label="节点详情页签">${tabs.map(t=>`<button type="button" id="${prefix}-tab-${t.key}" role="tab" aria-controls="${prefix}-pane-${t.key}" aria-selected="${t.key===active}" tabindex="${t.key===active?0:-1}">${esc(t.label)}</button>`).join('')}</div><div class="wf-panel-body">${tabs.map(t=>`<section id="${prefix}-pane-${t.key}" role="tabpanel" aria-labelledby="${prefix}-tab-${t.key}" ${t.key===active?'':'hidden'}>${t.body}</section>`).join('')}</div>`;
}
function bindWorkflowPanel(prefix,tabs,active,onTab,onClose){
 const select=key=>{
  for(const t of tabs){const chosen=t.key===key;$('#'+prefix+'-tab-'+t.key).setAttribute('aria-selected',String(chosen));$('#'+prefix+'-tab-'+t.key).tabIndex=chosen?0:-1;$('#'+prefix+'-pane-'+t.key).hidden=!chosen;}
  onTab(key);
 };
 for(const t of tabs)$('#'+prefix+'-tab-'+t.key).onclick=()=>select(t.key);
 $('#'+prefix+'-panel-close').onclick=onClose;
 $('#'+prefix+'-panel-tabs').onkeydown=e=>{
  const current=tabs.findIndex(t=>e.target.id===prefix+'-tab-'+t.key);if(current<0)return;
  let next=current;if(e.key==='ArrowRight')next=(current+1)%tabs.length;else if(e.key==='ArrowLeft')next=(current+tabs.length-1)%tabs.length;else if(e.key==='Home')next=0;else if(e.key==='End')next=tabs.length-1;else return;
  e.preventDefault();select(tabs[next].key);$('#'+prefix+'-tab-'+tabs[next].key).focus();
 };
}
