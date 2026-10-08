'use strict';
// The editable graph is separate from DOM state; API validation remains authoritative.
globalThis.WorkflowGraph = class WorkflowGraph {
  static kinds = {agent:'Agent', approval:'人工确认', connector:'Connector', end:'结束'};
  static agentConfig(source) {
    const fields=['executor','model','instructions','skills','tool_servers','sandbox','network_access','allow_elevation','native_config','trust_hooks','inherit_env','env','seed_dir'];
    return JSON.parse(JSON.stringify(Object.fromEntries(fields.filter(key=>Object.hasOwn(source,key)).map(key=>[key,source[key]]))));
  }
  constructor(value) {
    this.value = value ? JSON.parse(JSON.stringify(value)) : {id:'',name:'新智能体编排',revision:0,context_version:2,enabled:true,authorized_users:[],entry:'',start_nodes:[],max_steps:100,nodes:[],edges:[]};
    this.value.authorized_users ||= [];
    this.value.start_nodes ||= [];
  }
  upgradeForEditing(agents=[]) {
    const next=new WorkflowGraph(this.value);
    next.value.context_version=2;
    for(const n of next.value.nodes){
      if(n.kind!=='agent')continue;
      if(n.agent!=null&&n.agent_id)throw new Error('节点不能同时使用独立配置和共享 Agent 引用');
      if(n.agent!=null&&(typeof n.agent!=='object'||Array.isArray(n.agent)))throw new Error('节点执行配置必须是对象');
      const source=n.agent||agents.find(a=>a.id===n.agent_id);
      if(!source)throw new Error('节点「'+n.name+'」的智能体配置不可用');
      n.agent=WorkflowGraph.agentConfig(source);delete n.agent_id;delete n.agent.seed_dir;
      const guidance=(n.prompt||'').replaceAll('{{handoff}}','').trim();
      if(guidance)n.agent.instructions=[n.agent.instructions,guidance].filter(Boolean).join('\n\n');
      delete n.prompt;
      const outgoing=next.value.edges.filter(e=>e.source===n.id);
      const modes=new Set(outgoing.map(e=>next.edgeMode(e)));
      if(!n.exit_mode){
        n.exit_mode=modes.size===1&&modes.has('automatic')?'complete':'handoff';
        if(modes.size>1){
          for(const e of outgoing)e.mode='handoff';
          n.agent.instructions=(n.agent.instructions||'').replaceAll('complete_node','handoff');
        }
      }
    }
    const before=new WorkflowGraph(this.value).value;
    for(const n of before.nodes)if(n.agent){n.agent=WorkflowGraph.agentConfig(n.agent);if(!n.agent.seed_dir)delete n.agent.seed_dir;}
    const changed=JSON.stringify(next.value)!==JSON.stringify(before);
    this.value=next.value;
    return changed;
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
    if(kind==='agent'){if(this.value.context_version===2)n.exit_mode='handoff';n.agent={executor:'codex',model:'',instructions:'',skills:[],tool_servers:[],sandbox:'workspace-write',network_access:false,allow_elevation:false,native_config:'',trust_hooks:false,inherit_env:true,env:{},seed_dir:''};n.allow_user_input=true;if(this.value.context_version!==2)n.prompt='{{handoff}}';n.continuation_limit=3;n.execution_timeout_seconds=14400;}
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
    mode ||= n.kind==='agent'&&n.exit_mode!=='complete'?'handoff':'automatic';
    if(n.exit_mode&&mode!==(n.exit_mode==='complete'?'automatic':'handoff'))throw new Error('连线方式须与节点交接方式一致');
    if(!['handoff','automatic'].includes(mode)||mode==='handoff'&&n.kind!=='agent')throw new Error('连线决策方式无效');
    if(this.value.context_version>=1&&mode==='handoff'&&this.value.edges.some(e=>e.source===source&&e.target===target&&this.edgeMode(e)==='handoff'))throw new Error('同一自主交接目标只能配置一条策略，请合并策略内容');
    if(mode==='automatic'&&n.kind==='agent'&&this.value.edges.some(e=>e.source===source&&this.edgeMode(e)==='automatic'))throw new Error('Agent 只能有一条固定完成线');
    this.value.edges.push({source,route,target,mode,description});
  }
  setExitMode(id,mode,route) {
    const n=this.node(id);if(n?.kind!=='agent'||this.value.context_version!==2||!['handoff','complete'].includes(mode))throw new Error('交接方式无效');
    const edges=this.value.edges.filter(e=>e.source===id);
    if(mode==='complete'&&edges.length>1&&!edges.some(e=>e.route===route))throw new Error('请选择固定流转保留的目标');
    if(mode==='handoff'&&new Set(edges.map(e=>e.target)).size!==edges.length)throw new Error('同一目标存在多条连线，请先合并策略');
    this.value.edges=this.value.edges.filter(e=>e.source!==id||mode!=='complete'||edges.length<=1||e.route===route);
    for(const e of this.value.edges.filter(e=>e.source===id))e.mode=mode==='complete'?'automatic':'handoff';
    n.exit_mode=mode;
  }
  agentMode(id) {
    if(this.node(id)?.exit_mode)return this.node(id).exit_mode==='complete'?'automatic':'handoff';
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
    if(node.kind==='agent'&&node.agent&&node.agent_id)throw new Error('节点不能同时使用独立配置和共享 Agent 引用');
    if(node.kind==='agent'&&node.agent!=null){
      const a=node.agent,object=value=>value!==null&&typeof value==='object'&&!Array.isArray(value),strings=value=>Array.isArray(value)&&value.every(v=>typeof v==='string');
      if(!object(a))throw new Error('节点执行配置必须是对象');
      if(a.skills!=null&&!strings(a.skills))throw new Error('Skill 配置必须是目录字符串数组');
      if(a.tool_servers!=null&&(!Array.isArray(a.tool_servers)||a.tool_servers.some(b=>!object(b)||typeof b.server_id!=='string'||!strings(b.tools)||b.approvals!=null&&(!object(b.approvals)||Object.values(b.approvals).some(v=>!['auto','confirm'].includes(v))))))throw new Error('工具配置必须是包含服务、工具名称数组与审批设置的数组');
      if(a.env!=null&&(!object(a.env)||Object.values(a.env).some(v=>v!==null&&typeof v!=='string')))throw new Error('环境配置必须是对象，变量值使用字符串或 null');
      for(const key of ['executor','model','instructions','sandbox','native_config','seed_dir'])if(a[key]!=null&&typeof a[key]!=='string')throw new Error('节点配置 '+key+' 必须是字符串');
      for(const key of ['network_access','allow_elevation','trust_hooks','inherit_env'])if(a[key]!=null&&typeof a[key]!=='boolean')throw new Error('节点配置 '+key+' 必须是布尔值');
    }
    if(node.exit_mode){
      if(node.kind!=='agent'||this.value.context_version!==2||!['handoff','complete'].includes(node.exit_mode))throw new Error('节点交接方式无效');
      const edges=this.value.edges.filter(e=>e.source===node.id);
      if(edges.some(e=>this.edgeMode(e)!==(node.exit_mode==='complete'?'automatic':'handoff')))throw new Error('连线方式须与节点交接方式一致');
      if(node.exit_mode==='complete'&&edges.length!==1)throw new Error('固定流转须配置一个目标');
    }
    if(node.completion_schema!=null&&(typeof node.completion_schema!=='object'||Array.isArray(node.completion_schema)||node.completion_schema.type!=='object'))throw new Error('输出格式须为 type=object 的 JSON Schema');
    if(!this.value.context_version||node.kind!=='agent')return;
    if(this.value.context_version===2&&node.agent?.seed_dir)throw new Error('工作区在开始运行时统一指定，节点不单独设置工作区模板');
    if(this.value.context_version===2&&node.prompt)throw new Error('此编排不再使用独立工作说明；请将长期职责放入角色指令，具体任务在运行时输入');
    if(this.value.context_version===1){
    const prompt=node.prompt||'',tokens=prompt.match(/{{[\s\S]*?}}/g)||[];
    if(tokens.some(t=>t!=='{{handoff}}'))throw new Error('Session Prompt 包含未知占位符；仅支持 {{handoff}}');
    const handoff=this.value.edges.some(e=>e.source===node.id&&this.edgeMode(e)==='handoff');
    if(tokens.length>1||handoff&&tokens.length!==1)throw new Error('自主交接的 Session Prompt 必须恰好包含一个 {{handoff}}');
    }
    if(node.allow_user_input!=null&&typeof node.allow_user_input!=='boolean')throw new Error('用户输入开关必须是布尔值');
    if(!Number.isInteger(node.continuation_limit??0)||(node.continuation_limit??0)<0||(node.continuation_limit??0)>10)throw new Error('自动继续次数必须是 0–10 的整数');
    if(!Number.isInteger(node.execution_timeout_seconds??0)||(node.execution_timeout_seconds??0)<0||(node.execution_timeout_seconds??0)>86400)throw new Error('持续推进期限必须是 0–86400 秒的整数');
  }
  static parseJSON(text) {
    const value=JSON.parse(text);
    const decimal=raw=>{
      const [mantissa,exponent='0']=raw.toLowerCase().split('e'),negative=mantissa.startsWith('-');
      let digits=mantissa.replace(/[-.]/g,'').replace(/^0+/,'');
      let scale=Number(exponent)-(mantissa.split('.')[1]?.length||0);
      if(!digits)return '0';
      const zeros=digits.match(/0+$/)?.[0].length||0;digits=digits.slice(0,digits.length-zeros);scale+=zeros;
      return (negative?'-':'')+digits+'e'+scale;
    };
    // Skip complete string tokens; compare every numeric token before JS can round it.
    for(const match of text.matchAll(/"(?:[^"\\]|\\.)*"|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?/g)){
      const raw=match[0];if(raw.startsWith('"'))continue;
      const number=Number(raw);if(!Number.isFinite(number)||decimal(raw)!==decimal(String(number)))throw new Error('数值无法无损保存，请将高精度数值或长编号配置为字符串');
    }
    return value;
  }
  static import(text,agents=[]) {
    if(text.length>512*1024)throw new Error('文件超过 512 KB');
    const value=WorkflowGraph.parseJSON(text);
    if(!value||!Array.isArray(value.nodes)||!Array.isArray(value.edges)||value.nodes.length>128||value.edges.length>512)throw new Error('无效的工作流文件');
    const ids=new Set();
    for(const n of value.nodes){
      if(!n||!Object.hasOwn(WorkflowGraph.kinds,n.kind)||!(/^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/).test(n.id)||ids.has(n.id)||!Number.isFinite(n.x)||!Number.isFinite(n.y))throw new Error('文件包含无效的节点');
      ids.add(n.id);
    }
    for(const e of value.edges)if(!e||!ids.has(e.source)||!ids.has(e.target)||typeof e.route!=='string'||e.mode&&!['handoff','automatic'].includes(e.mode))throw new Error('文件包含无效的连线');
    // Imported definitions are new copies; source identity and user grants do not transfer.
    const graph=new WorkflowGraph({...value,id:'',revision:0,authorized_users:[],updated_at:''});
    graph.upgradeForEditing(agents);
    for(const node of graph.value.nodes)graph.validateAgent(node);
    return graph;
  }
};
