'use strict';
// Presentation follows Codex's native item lifecycle: started -> deltas -> completed.
// Reference: codex-rs/tui/src/chatwidget/tool_lifecycle.rs and history_cell/mcp.rs.
// No tool result is interpreted as a business decision or an Agent message.
const platformTranscript = (() => {
  const escape = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const text = value => value == null ? '' : typeof value === 'string' ? value : JSON.stringify(value, null, 2);
  const kind = value => String(value || '').replace(/[A-Z]/g, c => '_' + c.toLowerCase());
  const isTool = type => ['command_execution','mcp_tool_call','dynamic_tool_call','file_change','web_search','image_view','image_generation','collab_agent_tool_call'].includes(type) || type.endsWith('_tool_call');
  const key = (parent, id) => `${parent}:${id}`;

  function entries(data, events) {
    const items = new Map(), starts = new Map(), seen = new Set(), ended = new Set();
    for (const event of [...events].sort((a,b) => a.id-b.id)) {
      if (seen.has(event.id)) continue;
      seen.add(event.id);
      if (['turn.completed','turn.failed','platform.execution.completed','platform.execution.failed','platform.execution.stopped'].includes(event.type)) ended.add(event.message_id);
      const raw = event.raw || {}, native = raw.native || {}, params = native.params || {};
      const item = raw.item || params.item;
      const id = item?.id || params.itemId;
      if (!id) continue;
      const idKey = key(event.message_id, id);
      if (!starts.has(idKey)) starts.set(idKey, {at:event.created_at, order:event.id});
      const type = kind(item?.type);
      if (item && (isTool(type) || type === 'reasoning')) {
        const record = items.get(idKey) || {key:idKey, parent:event.message_id, item:{}, ...starts.get(idKey), output:'', progress:''};
        const values = Object.fromEntries(Object.entries(item).filter(([,v]) => v != null));
        record.item = {...record.item, ...values, type};
        if (Array.isArray(item.summary)) record.item.summary = [...item.summary];
        const output = item.aggregatedOutput ?? item.aggregated_output;
        if (output != null) record.output = output;
        record.finished ||= event.type === 'item.completed' || native.method === 'item/completed';
        record.raw = raw;
        items.set(idKey, record);
      } else if (native.method === 'item/reasoning/summaryTextDelta') {
        const record = items.get(idKey) || {key:idKey, parent:event.message_id, item:{type:'reasoning',summary:[]}, ...starts.get(idKey)};
        const index = params.summaryIndex ?? 0;
        const summary = record.item.summary || (record.item.summary = []);
        summary[index] = (summary[index] || '') + (params.delta || '');
        items.set(idKey, record);
      } else if (items.has(idKey)) {
        const record = items.get(idKey);
        if (['item/commandExecution/outputDelta','item/fileChange/outputDelta'].includes(native.method)) record.output += params.delta || '';
        if (native.method === 'item/mcpToolCall/progress') record.progress = params.message || '';
      }
    }
    const rows = data.messages.map(message => {
      const first = starts.get(key(message.parent_id, message.native_item));
      return {message, at:first?.at || message.created_at, order:first?.order || 0};
    });
    for (const record of items.values()) {
      const parent = data.messages.find(m => m.id === record.parent);
      record.turnActive = !ended.has(record.parent) && !['completed','failed','stopped'].includes(parent?.status);
      record.interrupted = !record.finished && !record.turnActive;
      // Display only the public native reasoning summary, never opaque/private payloads.
      if (record.item.type === 'reasoning' && !(record.item.summary || []).some(Boolean) && (record.finished || !record.turnActive)) continue;
      rows.push({[record.item.type === 'reasoning' ? 'reasoning' : 'tool']:record, at:record.at, order:record.order});
    }
    return rows.sort((a,b) => (Date.parse(a.at)||0) - (Date.parse(b.at)||0) || a.order-b.order);
  }

  function reasoningMarkup(record, expanded = new Set()) {
    const summary = (record.item.summary || []).join('\n\n');
    const open = expanded.has(`thinking:${record.key}`);
    const active = !record.finished && record.turnActive;
    return `<section class="thinking"><details data-disclosure="thinking:${escape(record.key)}"${open?' open':''}><summary class="thinking-heading"><span class="${active?'activity-spinner':'tool-indicator'}" aria-hidden="true">${active?'':'✓'}</span><span>Thinking</span><span class="tool-more" aria-hidden="true">···</span><span class="tool-chevron" aria-hidden="true">▸</span></summary>${open?`<div class="bubble thinking-text">${escape(summary || (active ? 'Thinking…' : ''))}</div>`:''}</details></section>`;
  }

  function toolMarkup(record, expanded = new Set()) {
    const item = record.item, result = item.result;
    const failed = ['failed','declined','error'].includes(item.status) || item.error != null || result?.isError === true || result?.is_error === true || (item.exitCode ?? item.exit_code ?? 0) !== 0 || item.success === false;
    const active = !record.finished && !record.interrupted;
    const status = record.interrupted ? '已中断 · 未收到结果' : active ? '执行中' : failed ? '失败' : '已完成';
    const title = item.type === 'command_execution' ? '命令' : item.type === 'file_change' ? '文件变更' : [item.server, item.tool || item.type].filter(Boolean).join(' · ');
    const command = item.command || item.query || item.path || '';
    const duration = item.durationMs ?? item.duration_ms;
    const exit = item.exitCode ?? item.exit_code;
    const meta = [duration == null ? '' : `${(duration/1000).toFixed(1)}s`, exit == null ? '' : `退出码 ${exit}`].filter(Boolean).join(' · ');
    let output = record.output;
    if (item.error != null) output = text(item.error) + (output ? '\n' + output : '');
    if (result != null) output += (output ? '\n' : '') + (Array.isArray(result.content) ? result.content.map(block => block.type === 'text' ? block.text : `[${block.type || 'content'}]`).join('\n') : text(result));
    if (item.changes) output += (output ? '\n' : '') + text(item.changes);
    if (item.contentItems) output += (output ? '\n' : '') + text(item.contentItems);
    const description = text(item.title || command || '').replace(/\s+/g, ' ').trim();
    const shortDescription = description.length > 100 ? description.slice(0, 100) + '…' : description;
    const fullResult = result == null ? output : text(result);
    const payload = item.arguments ?? (command ? {command, cwd:item.cwd} : item);
    const open = expanded.has('tool:' + record.key);
    const rawOpen = expanded.has('raw:' + record.key);
    return `<article class="tool-event ${active?'running':failed?'failed':record.interrupted?'interrupted':'completed'}" data-tool-key="${escape(record.key)}"><details data-disclosure="tool:${escape(record.key)}"${open?' open':''}><summary class="tool-heading"><span class="${active?'activity-spinner':'tool-indicator'}" aria-hidden="true">${active?'':failed?'×':record.interrupted?'○':'✓'}</span><strong>${escape(title)}</strong>${shortDescription?`<span class="tool-description">${escape(shortDescription)}</span>`:''}<span class="tool-status">${escape(status)}</span><span class="tool-more" aria-hidden="true">···</span><span class="tool-chevron" aria-hidden="true">▸</span></summary>${open?`<div class="tool-details">${meta?`<div class="tool-duration">${escape(meta)}</div>`:''}<div class="tool-label">调用参数</div><pre>${escape(text(payload))}</pre><div class="tool-label">${active?'当前输出':'调用结果'}</div><pre>${escape(fullResult || record.progress || (active?'尚无输出':'未返回内容'))}</pre><details data-disclosure="raw:${escape(record.key)}"${rawOpen?' open':''}><summary>原始事件</summary>${rawOpen?`<pre>${escape(text(record.raw))}</pre>`:''}</details></div>`:''}</details></article>`;
  }
  return {entries, toolMarkup, reasoningMarkup};
})();
if (typeof module !== 'undefined') module.exports = platformTranscript;
