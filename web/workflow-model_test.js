const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
function model() {
 const context = vm.createContext({});
 vm.runInContext(fs.readFileSync(__dirname + '/workflow-model.js', 'utf8'), context);
 return context.WorkflowGraph;
}
test('moving and deleting a node preserves other nodes and removes dangling routes',()=>{
 const G=model(),g=new G();
 const a=g.add('approval',25,40),b=g.add('end',380,40);
 g.connect(a.id,'approved',b.id);g.move(a.id,150,200);
 let saved=JSON.parse(JSON.stringify(g.value));
 assert.deepEqual([saved.nodes[0].x,saved.nodes[0].y],[150,200]);
 assert.equal(saved.edges[0].target,b.id);
 g.remove(b.id);assert.equal(g.value.edges.length,0);assert.equal(g.value.nodes.length,1);
});
test('editor rejects duplicate routes without erasing existing connection',()=>{
 const G=model(),g=new G(),a=g.add('approval',20,20),b=g.add('end',300,20),c=g.add('end',300,180);
 g.connect(a.id,'approved',b.id);
 assert.throws(()=>g.connect(a.id,'approved',c.id),/路由/);
 assert.equal(g.value.edges.length,1);assert.equal(g.value.edges[0].target,b.id);
 assert.throws(()=>g.connect(b.id,'next',a.id),/结束/);
});
test('import creates an independent unsaved copy and rejects malformed graph input',()=>{
 const G=model(),g=new G(),a=g.add('approval',0,0),b=g.add('end',300,0);g.connect(a.id,'next',b.id);
 g.value.id='a'.repeat(32);g.value.revision=9;g.value.authorized_users=['private-user'];
 const copy=G.import(JSON.stringify(g.value));
 assert.equal(copy.value.id,'');assert.equal(copy.value.revision,0);assert.equal(copy.value.authorized_users.length,0);
 copy.move(a.id,200,200);assert.equal(g.value.nodes[0].x,0);
 assert.throws(()=>G.import('{"nodes":"oops"}'));
 assert.throws(()=>G.import('{"nodes":[{"id":"a","kind":"script"}],"edges":[]}'));
});
test('adding nodes at the same palette position keeps the new node selectable',()=>{
 const G=model(),g=new G();const a=g.add('approval',40,60),b=g.add('end',40,60);
 assert.ok(Math.abs(a.x-b.x)>=184 || Math.abs(a.y-b.y)>=96,'new node must not cover the existing node');
});
