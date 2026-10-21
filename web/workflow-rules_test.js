const {test}=require('node:test');const assert=require('node:assert/strict');const fs=require('node:fs');const vm=require('node:vm');
const ctx=vm.createContext({});for(const f of ['workflow-model.js','workflow-rules.js'])vm.runInContext(fs.readFileSync(__dirname+'/'+f,'utf8'),ctx);
const run=s=>JSON.parse(JSON.stringify(vm.runInContext(s,ctx)));
test('boolean and numeric enum suggestions follow the field schema without inventing success values',()=>{
 const n={execution:{mode:'command'},rule_handoff:{schema:{type:'object',properties:{passed:{type:'boolean'},status:{type:'integer',enum:[0,1]},count:{type:'integer'}}}}};ctx.n=n;
 assert.deepEqual(run('workflowRuleSuggestions(n,"output.result.passed == ",workflowRuleSchema(n)).values').map(v=>v.text),['true','false']);
 assert.deepEqual(run('workflowRuleSuggestions(n,"output.result.status == ",workflowRuleSchema(n)).values').map(v=>v.text),['0','1']);
 assert.deepEqual(run('workflowRuleSuggestions(n,"output.result.count == ",workflowRuleSchema(n)).values'),[]);
 assert.deepEqual(run('workflowRuleSuggestions(n,"output.result.passed == f",workflowRuleSchema(n)).values').map(v=>v.text),['false']);
});
test('rules project targets into solid edges and deleting a target exposes missing configuration',()=>{
 vm.runInContext("var graph=new WorkflowGraph();var n=graph.add('execution',0,0),end=graph.add('end',400,0);n.rule_handoff=workflowRulePolicy();n.rule_handoff.rules=[{title:'完成',when:'output.exit_code == 0',target:end.id}];graph.syncRuleEdges()",ctx);
 assert.equal(run('graph.value.edges')[0].mode,'automatic');assert.equal(run('graph.value.edges')[0].title,'完成');
 vm.runInContext('graph.remove(end.id)',ctx);assert.equal(run('n.rule_handoff.rules')[0].target,'');assert.equal(run('graph.value.edges').length,0);
});
test('malformed schema draft blocks saving instead of reusing previous schema',()=>{ctx.drafts={node_1:'{"type":'};vm.runInContext('n.rule_handoff.schema=workflowSelectedExample()',ctx);assert.throws(()=>vm.runInContext('applyRuleSchemaDrafts(graph,drafts)',ctx),/JSON Schema/)});
test('command handoff starts from an exit code without JSON schema or an envelope',()=>{
 const p=run('workflowRulePolicy("command")');ctx.newPolicy=p;
 assert.equal(p.schema,undefined);
 const vars=run('workflowRuleVariables({execution:{mode:"command",output:{source:"exit_code"}},rule_handoff:newPolicy})');
 assert.deepEqual(vars.map(v=>[v.path,v.type]),[['$','integer']]);
});
test('selected JSON autocomplete follows the original object or scalar',()=>{
 ctx.raw={execution:{mode:'command',output:{source:'stdout'}},rule_handoff:{schema_scope:'value',schema:{type:'object',properties:{passed:{type:'boolean'},status:{type:'integer',enum:[0,1]}}}}};
 assert.deepEqual(run('workflowRuleSuggestions(raw,"$.passed == ").values').map(v=>v.text),['true','false']);
 assert.deepEqual(run('workflowRuleSuggestions(raw,"$.status == ").values').map(v=>v.text),['0','1']);
 vm.runInContext('delete raw.rule_handoff.schema',ctx);
 assert.deepEqual(run('workflowRuleVariables(raw)').map(v=>[v.path,v.type]),[['$','string']]);
});

test('output-scope autocomplete offers only schema-declared fields',()=>{
 ctx.explicit={execution:{mode:'command'},rule_handoff:{schema_scope:'input',schema:{type:'object',properties:{output:{type:'object',properties:{result:{type:'boolean'}}}}}}};
 assert.deepEqual(run('workflowRuleVariables(explicit)').map(v=>v.path),['output','output.result']);
});
test('legacy draft displays a complete equivalent schema without changing saved source',()=>{
 vm.runInContext("var legacy={id:'legacy',execution:{mode:'command'},rule_handoff:{schema:{type:'object',properties:{score:{type:'number'}}}}}",ctx);
 const schema=run('workflowRuleSchema(legacy)');assert.equal(schema.properties.output.properties.exit_code.type,'integer');assert.equal(schema.properties.output.properties.result.properties.score.type,'number');
 assert.equal(run('legacy.rule_handoff').schema_scope,undefined);
});

test('old route projection preserves result collection and rejects unknown routes without inventing conditions',()=>{
 ctx.source={id:'prepare',execution:{mode:'code',language:'python',code:'print(0)',output:{source:'file',path:'report.json'}}};
 ctx.edges=[{source:'prepare',route:'next',target:'dev',description:'继续'}];
 const before=JSON.stringify(ctx.source);
 const view=run('workflowRuleView({graph:{value:{edges}}},source)');
 assert.deepEqual(view.execution.output,ctx.source.execution.output);
 assert.equal(view.rule_handoff.rules[0].when,'output.exit_code == 0');
 assert.deepEqual(view.rule_handoff.schema.properties.output.properties.result,{});
 assert.equal(JSON.stringify(ctx.source),before);
 ctx.edges.push({source:'prepare',route:'custom-result',target:'review'});
 assert.equal(run('workflowRuleView({graph:{value:{edges}}},source)').rule_handoff,undefined);
});
