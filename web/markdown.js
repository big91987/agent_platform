'use strict';
// Deliberately small renderer: escaped text only, no HTML execution or remote embeds.
function messageMarkup(value) {
  const escape = s => String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const inline = s => escape(s).replace(/`([^`]+)`/g,'<code>$1</code>').replace(/\*\*([^*]+)\*\*/g,'<strong>$1</strong>');
  let code=false, buffer=[], out=[];
  for (const line of String(value).split('\n')) {
    if (/^\s*```/.test(line)) {
      if(code){out.push('<pre><code>'+escape(buffer.join('\n'))+'</code></pre>');buffer=[]}
      code=!code;continue;
    }
    if(code){buffer.push(line);continue}
    const heading=line.match(/^(#{1,4})\s+(.+)$/);
    if(heading){out.push('<h'+(heading[1].length+1)+'>'+inline(heading[2])+'</h'+(heading[1].length+1)+'>');continue}
    if(!line.trim()){out.push('<div class="paragraph-gap"></div>');continue}
    out.push('<div>'+inline(line)+'</div>');
  }
  if(code)out.push('<pre><code>'+escape(buffer.join('\n'))+'</code></pre>');
  return out.join('');
}
if(typeof module!=='undefined')module.exports={messageMarkup};
