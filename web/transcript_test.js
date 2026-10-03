const {test} = require('node:test');
const assert = require('node:assert/strict');
const transcript = require('./transcript.js');

const event = (id, parent, type, item, second=id) => ({id,message_id:parent,type,created_at:`2026-10-02T00:00:${String(second).padStart(2,'0')}Z`,raw:{item}});
const data = {messages:[{id:1,role:'user',status:'running',created_at:'2026-10-02T00:00:00Z'}, {id:2,parent_id:1,role:'agent',native_item:'message',content:'reply',created_at:'2026-10-02T00:00:09Z'}]};

test('native command/MCP lifecycle streams once, preserves ordering and replays across turns',()=>{
 const start=event(1,1,'item.started',{id:'call',type:'command_execution',command:'printf output',status:'inProgress'});
 const delta={id:2,message_id:1,type:'native.item/commandExecution/outputDelta',created_at:'2026-10-02T00:00:02Z',raw:{native:{method:'item/commandExecution/outputDelta',params:{itemId:'call',delta:'partial\n'}}}};
 const reply=event(3,1,'item.started',{id:'message',type:'agent_message'},3);
 let events=[start,delta,reply];
 let rows=transcript.entries(data,events);
 assert.deepEqual(rows.map(r=>r.tool?'tool':r.message.id),[1,'tool',2]);
 assert.match(transcript.toolMarkup(rows[1].tool),/执行中/);
 assert.doesNotMatch(transcript.toolMarkup(rows[1].tool),/partial/);
 assert.match(transcript.toolMarkup(rows[1].tool,new Set(['tool:1:call'])),/partial/);
 const done=event(4,1,'item.completed',{id:'call',type:'command_execution',status:'completed',aggregatedOutput:'partial\nfinished',exitCode:0,durationMs:900});
 const mcp=event(5,1,'item.completed',{id:'mcp',type:'mcp_tool_call',server:'external',tool:'save',arguments:{path:'<input>'},status:'completed',result:{isError:true,content:[{type:'text',text:'denied <script>bad</script>'}]}});
 const other=event(6,9,'item.started',{id:'call',type:'command_execution',command:'other turn'});
 events.push(done,mcp,other,done);
 rows=transcript.entries(data,events);
 assert.equal(rows.filter(r=>r.tool).length,3);
 assert.equal(rows[1].tool.output,'partial\nfinished');
 assert.doesNotMatch(transcript.toolMarkup(rows[1].tool),/finished|退出码/);
 assert.match(transcript.toolMarkup(rows[1].tool,new Set(['tool:1:call'])),/退出码 0/);
 const failed=transcript.toolMarkup(rows.find(r=>r.tool?.key==='1:mcp').tool,new Set(['tool:1:mcp']));
 assert.match(failed,/失败/);assert.match(failed,/denied &lt;script&gt;/);assert.doesNotMatch(failed,/<script>/);
 assert.deepEqual(transcript.entries(data,JSON.parse(JSON.stringify(events))),rows,'reloaded persisted events must produce the same transcript');
 const stopped=transcript.entries({...data,messages:[...data.messages,{id:9,status:'stopped'}]},events).find(r=>r.tool?.key==='9:call');
 assert.match(transcript.toolMarkup(stopped.tool),/已中断/);
});

test('turn completion preserves chronological thinking, messages and expandable tool output', () => {
 const snapshot = {messages:[
  {id:1,role:'user',status:'running',created_at:'2026-10-02T00:00:00Z'},
  {id:2,parent_id:1,role:'agent',kind:'progress',created_at:'2026-10-02T00:00:02Z'},
  {id:3,role:'user',status:'queued',created_at:'2026-10-02T00:00:04Z'},
  {id:4,parent_id:1,role:'agent',kind:'reply',created_at:'2026-10-02T00:00:08Z'},
 ]};
 const events=[event(10,1,'item.started',{id:'thought',type:'reasoning',summary:[]},1),
  {id:11,message_id:1,type:'native.item/reasoning/summaryTextDelta',created_at:'2026-10-02T00:00:01Z',raw:{native:{method:'item/reasoning/summaryTextDelta',params:{itemId:'thought',summaryIndex:0,delta:'Checking <input>'}}}},
  event(12,1,'item.completed',{id:'before',type:'command_execution',aggregatedOutput:'visible-output'},3),
  event(13,1,'item.started',{id:'after',type:'command_execution',command:'second'},5),
  event(14,1,'item.completed',{id:'thought2',type:'reasoning',summary:['Review result']},6)];
 const labels=rows=>rows.map(r=>r.activity?'process':r.reasoning?'thinking':r.tool?'tool':r.message.id);
 let rows=transcript.entries(snapshot,events);
 assert.deepEqual(labels(rows),[1,'thinking',2,'tool',3,'tool','thinking',4]);
 assert.equal(rows[1].reasoning.item.summary[0],'Checking <input>');
 assert.doesNotMatch(transcript.reasoningMarkup(rows[1].reasoning),/Checking &lt;input&gt;/,'thinking defaults to collapsed during streaming');
 assert.match(transcript.reasoningMarkup(rows[1].reasoning,new Set(['thinking:1:thought'])),/Checking &lt;input&gt;/,'public summary remains available on expansion');
 assert.doesNotMatch(transcript.toolMarkup(rows[3].tool),/visible-output/,'completed tools default to compact disclosures');
 assert.match(transcript.toolMarkup(rows[3].tool,new Set(['tool:1:before'])),/visible-output/,'full output is available when expanded');
 events.push(event(15,1,'item.completed',{id:'thought',type:'reasoning',summary:['Checking <input>']},7));
 assert.equal(transcript.entries(snapshot,events).filter(r=>r.reasoning).length,2,'completion replaces streamed content without duplication');
 snapshot.messages[0].status='completed';
 rows=transcript.entries(snapshot,events);
 assert.deepEqual(labels(rows),[1,'thinking',2,'tool',3,'tool','thinking',4]);
 assert.doesNotMatch(transcript.reasoningMarkup(rows[1].reasoning),/Checking &lt;input&gt;/,'completed thinking remains collapsed by default');
 assert.match(transcript.reasoningMarkup(rows[1].reasoning,new Set(['thinking:1:thought'])),/Checking &lt;input&gt;/,'manual expansion survives completion');
 assert.match(transcript.toolMarkup(rows[3].tool,new Set(['tool:1:before'])),/visible-output/,'turn completion preserves expanded output');
 assert.deepEqual(transcript.entries(snapshot,JSON.parse(JSON.stringify(events))),rows,'refresh retains the same flat timeline');
 const empty=event(16,1,'item.completed',{id:'empty',type:'reasoning',summary:[]},9);
 assert.deepEqual(transcript.entries(snapshot,[...events,empty]),rows,'no empty thinking card when native executor provides no summary');
});
