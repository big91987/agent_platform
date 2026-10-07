'use strict';
// The editable graph is separate from DOM state; API validation remains authoritative.
globalThis.WorkflowGraph = class WorkflowGraph {
  static kinds = {agent:'Agent', approval:'人工确认', connector:'Connector', end:'结束'};
  constructor(value) {
    this.value = value ? JSON.parse(JSON.stringify(value)) : {id:'',name:'新智能体编排',revision:0,context_version:1,enabled:true,authorized_users:[],entry:'',start_nodes:[],max_steps:100,nodes:[],edges:[]};
    this.value.authorized_users ||= [];
    this.value.start_nodes ||= [];
  }
  node(id) { return this.value.nodes.find(n=>n.id===id); }
  add(kind,x,y) {
    if(!Number.isFinite(x)||!Number.isFinite(y))throw new Error('节点位置无效');
    if(this.value.nodes.length>=128)throw new Error('最多添加 128 个节点');
    if(!Object.hasOwn(WorkflowGraph.kinds,kind))throw new Error('不支持的节点类型');
    let i=1;while(this.node('node_'+i))i++;
    x=Math.max(0,Math.min(10000,Math.round(x)));y=Math.max(0,Math.min(10000,Math.round(y)));
    while(this.value.nodes.some(n=>Math.abs(n.x-x)<210&&Math.abs(n.y-y)<120)){y+=140;if(y>9800){y=40;x=(x+240)%9800;}}
    const n={id:'node_'+i,name:WorkflowGraph.kinds[kind],kind,x:0,y:0};
    if(kind==='agent'){n.prompt='{{handoff}}';n.continuation_limit=3;n.execution_timeout_seconds=14400;}
    this.value.nodes.push(n);this.move(n.id,x,y);
    if(!this.value.entry)this.value.entry=n.id;
    return n;
  }
  move(id,x,y) {
    const n=this.node(id);if(!n||!Number.isFinite(x)||!Number.isFinite(y))throw new Error('节点或位置无效');
    n.x=Math.max(0,Math.min(10000,Math.round(x)));n.y=Math.max(0,Math.min(10000,Math.round(y)));
  }
  remove(id) {
    this.value.nodes=this.value.nodes.filter(n=>n.id!==id);
    this.value.edges=this.value.edges.filter(e=>e.source!==id&&e.target!==id);
    this.value.start_nodes=this.value.start_nodes.filter(n=>n!==id);
    if(this.value.entry===id)this.value.entry=this.value.nodes[0]?.id||'';
  }
  edgeMode(edge) { return edge.mode || (this.node(edge.source)?.kind==='agent'?'handoff':'automatic'); }
  connect(source,route,target,mode,description='') {
    const n=this.node(source);if(!n||!this.node(target))throw new Error('连线节点不存在');
    if(n.kind==='end')throw new Error('结束节点不能继续连线');
    if(!/^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/.test(route))throw new Error('路由使用字母开头的英文、数字、下划线或短横线');
    if(this.value.edges.some(e=>e.source===source&&e.route===route))throw new Error('这个节点已经有同名路由');
    mode ||= n.kind==='agent'?'handoff':'automatic';
    if(!['handoff','automatic'].includes(mode)||mode==='handoff'&&n.kind!=='agent')throw new Error('连线决策方式无效');
    if(this.value.context_version===1&&mode==='handoff'&&this.value.edges.some(e=>e.source===source&&e.target===target&&this.edgeMode(e)==='handoff'))throw new Error('同一自主交接目标只能配置一条策略，请合并策略内容');
    if(mode==='automatic'&&n.kind==='agent'&&this.value.edges.some(e=>e.source===source&&this.edgeMode(e)==='automatic'))throw new Error('Agent 只能有一条固定完成线');
    this.value.edges.push({source,route,target,mode,description});
  }
  agentMode(id) {
    const modes=new Set(this.value.edges.filter(e=>e.source===id).map(e=>this.edgeMode(e)));
    return modes.size>1?'mixed':modes.has('handoff')?'handoff':'automatic';
  }
  nextRoute(source) { let i=1;while(this.value.edges.some(e=>e.source===source&&e.route==='target_'+i))i++;return 'target_'+i; }
  removeEdge(index) { this.value.edges.splice(index,1); }
  updateEdge(index,changes) {
    const old=this.value.edges[index];if(!old)throw new Error('连线不存在');
    const next={...old,...changes},backup=this.value.edges.slice();
    try {
      this.value.edges.splice(index,1);
      this.connect(next.source,next.route,next.target,next.mode,next.description);
      const updated=this.value.edges.pop();this.value.edges.splice(index,0,updated);
    } catch(error) {this.value.edges=backup;throw error;}
  }
  validateAgent(node) {
    if(this.value.context_version!==1||node.kind!=='agent')return;
    const prompt=node.prompt||'',tokens=prompt.match(/{{[\s\S]*?}}/g)||[];
    if(tokens.some(t=>t!=='{{handoff}}'))throw new Error('Session Prompt 包含未知占位符；仅支持 {{handoff}}');
    const handoff=this.value.edges.some(e=>e.source===node.id&&this.edgeMode(e)==='handoff');
    if(tokens.length>1||handoff&&tokens.length!==1)throw new Error('自主交接的 Session Prompt 必须恰好包含一个 {{handoff}}');
    if(!Number.isInteger(node.continuation_limit??0)||(node.continuation_limit??0)<0||(node.continuation_limit??0)>10)throw new Error('自动继续次数必须是 0–10 的整数');
    if(!Number.isInteger(node.execution_timeout_seconds??0)||(node.execution_timeout_seconds??0)<0||(node.execution_timeout_seconds??0)>86400)throw new Error('持续推进期限必须是 0–86400 秒的整数');
  }
  static import(text) {
    if(text.length>512*1024)throw new Error('文件超过 512 KB');
    const value=JSON.parse(text);
    if(!value||!Array.isArray(value.nodes)||!Array.isArray(value.edges)||value.nodes.length>128||value.edges.length>512)throw new Error('无效的工作流文件');
    const ids=new Set();
    for(const n of value.nodes){
      if(!n||!Object.hasOwn(WorkflowGraph.kinds,n.kind)||!(/^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/).test(n.id)||ids.has(n.id)||!Number.isFinite(n.x)||!Number.isFinite(n.y))throw new Error('文件包含无效的节点');
      ids.add(n.id);
    }
    for(const e of value.edges)if(!e||!ids.has(e.source)||!ids.has(e.target)||typeof e.route!=='string'||e.mode&&!['handoff','automatic'].includes(e.mode))throw new Error('文件包含无效的连线');
    // Imported definitions are new copies; source identity and user grants do not transfer.
    const graph=new WorkflowGraph({...value,id:'',revision:0,authorized_users:[],updated_at:''});
    return graph;
  }
};
