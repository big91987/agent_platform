'use strict';
function pendingInputs(storage, newID) {
  const prefix='agent-platform-pending:';
  return {
    body(key,payload) {
      const value=JSON.stringify(payload),name=prefix+key;
      let saved;
      try { saved=JSON.parse(storage.getItem(name)||'null'); } catch { saved=null; }
      if (!saved || saved.value!==value) {
        saved={value,id:newID()};
        storage.setItem(name,JSON.stringify(saved));
      }
      return {...payload,request_id:saved.id};
    },
    accepted(key) { storage.removeItem(prefix+key); }
  };
}
if(typeof module!=='undefined')module.exports={pendingInputs};
else globalThis.platformInputs=pendingInputs(sessionStorage,()=>crypto.randomUUID());
