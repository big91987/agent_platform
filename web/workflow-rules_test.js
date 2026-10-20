const {test}=require('node:test');const assert=require('node:assert/strict');const fs=require('node:fs');const vm=require('node:vm');
const ctx=vm.createContext({});for(const f of ['workflow-model.js','workflow-rules.js'])vm.runInContext(fs.readFileSync(__dirname+'/'+f,'utf8'),ctx);
const run=s=>JSON.parse(JSON.stringify(vm.runInContext(s,ctx)));
test('boolean and numeric enum suggestions follow the field schema without inventing success values',()=>{
 const n={execution:{mode:'command'},rule_handoff:{schema:{type:'object',properties:{passed:{type:'boolean'},status:{type:'integer',enum:[0,1]},count:{type:'integer'}}}}};ctx.n=n;
 assert.deepEqual(run('workflowRuleSuggestions(n,"output.result.passed == ",n.rule_handoff.schema).values').map(v=>v.text),['true','false']);
 assert.deepEqual(run('workflowRuleSuggestions(n,"output.result.status == ",n.rule_handoff.schema).values').map(v=>v.text),['0','1']);
 assert.deepEqual(run('workflowRuleSuggestions(n,"output.result.count == ",n.rule_handoff.schema).values'),[]);
 assert.deepEqual(run('workflowRuleSuggestions(n,"output.result.passed == f",n.rule_handoff.schema).values').map(v=>v.text),['false']);
});
test('rules project targets into solid edges and deleting a target exposes missing configuration',()=>{
 vm.runInContext("var graph=new WorkflowGraph();var n=graph.add('execution',0,0),end=graph.add('end',400,0);n.rule_handoff=workflowRulePolicy();n.rule_handoff.rules=[{title:'完成',when:'output.exit_code == 0',target:end.id}];graph.syncRuleEdges()",ctx);
 assert.equal(run('graph.value.edges')[0].mode,'automatic');assert.equal(run('graph.value.edges')[0].title,'完成');
 vm.runInContext('graph.remove(end.id)',ctx);assert.equal(run('n.rule_handoff.rules')[0].target,'');assert.equal(run('graph.value.edges').length,0);
});
test('malformed schema draft blocks saving instead of reusing previous schema',()=>{ctx.drafts={node_1:'{"type":'};assert.throws(()=>vm.runInContext('applyRuleSchemaDrafts(graph,drafts)',ctx),/JSON Schema/)});
