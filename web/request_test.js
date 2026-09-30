const test = require('node:test');
const assert = require('node:assert/strict');
const {pendingInputs} = require('./request.js');
test('unchanged requests reuse identity after response loss and page reload',()=>{
 const saved=new Map();const storage={getItem:k=>saved.get(k),setItem:(k,v)=>saved.set(k,v),removeItem:k=>saved.delete(k)};
 let sequence=0;const id=()=>String(++sequence);
 let requests=pendingInputs(storage,id);const payload={message:'work',user_id:'alice'};
 const first=requests.body('chat',payload);
 assert.deepEqual(requests.body('chat',payload),first);
 requests=pendingInputs(storage,id);assert.deepEqual(requests.body('chat',payload),first);
 assert.notEqual(requests.body('chat',{...payload,message:'changed'}).request_id,first.request_id);
 requests.accepted('chat');assert.notEqual(requests.body('chat',payload).request_id,first.request_id);
});
