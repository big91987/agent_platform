'use strict';
// File references resolve only against files exposed by the current workspace.
function artifactReference(reference, artifacts = [], workspace = '', basePath = '') {
  let path=String(reference).trim().replace(/^<|>$/g,'');
  try {path=decodeURIComponent(path)} catch {return null}
  let line=0;
  const suffix=path.match(/(?::(\d+)(?::\d+)?|#L(\d+)(?:C\d+)?)(?:-L?\d+)?$/);
  if(suffix){line=Number(suffix[1]||suffix[2]);path=path.slice(0,suffix.index)}
  if(/^[a-z][a-z\d+.-]*:/i.test(path)||path.startsWith('//')||/[\x00-\x1f\\?#]/.test(path))return null;
  if(path.startsWith('/')){const root=workspace.replace(/\/$/,'')+'/';if(!workspace||!path.startsWith(root))return null;path=path.slice(root.length)}
  const normalize=s=>{const parts=[];for(const part of s.split('/')){if(!part||part==='.')continue;if(part==='..'){if(!parts.length)return null;parts.pop()}else parts.push(part)}return parts.join('/')};
  const candidates=basePath?[basePath.slice(0,basePath.lastIndexOf('/')+1)+path,path]:[path];
  for(const candidate of candidates){const normalized=normalize(candidate);if(normalized&&artifacts.some(f=>f.path===normalized))return {path:normalized,line}}
  return null;
}
// Escaped text, workspace file buttons and safe external links; no raw HTML execution.
function messageMarkup(value, resolveFile = () => null) {
  const escape = s => String(s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const fileLink=(file,label)=>`<button type="button" class="file-reference" data-file-path="${escape(file.path)}" data-file-line="${file.line||0}">${escape(label)}</button>`;
  const inline = s => {
    const pattern=/`([^`]+)`|!?\[([^\]\n]+)\]\((<[^>\n]+>|[^\s)]+)\)|\*\*([^*]+)\*\*/g;
    let out='',last=0;
    for(const m of s.matchAll(pattern)){
      out+=escape(s.slice(last,m.index));last=m.index+m[0].length;
      if(m[1]!==undefined){const file=resolveFile(m[1]);out+=file?fileLink(file,m[1]):'<code>'+escape(m[1])+'</code>'}
      else if(m[2]!==undefined){const target=m[3].replace(/^<|>$/g,'');const file=resolveFile(target);if(file)out+=fileLink(file,m[2]);else if(/^https?:\/\/[^\s]+$/i.test(target))out+=`<a href="${escape(target)}" target="_blank" rel="noopener noreferrer">${escape(m[2])}</a>`;else out+=escape(m[0])}
      else out+='<strong>'+escape(m[4])+'</strong>';
    }
    return out+escape(s.slice(last));
  };
  let code=false, buffer=[], out=[];
  const lines=String(value).split('\n');
  for (let index=0;index<lines.length;index++) {
    const line=lines[index];
    if (/^\s*```/.test(line)) {
      if(code){out.push('<pre><code>'+escape(buffer.join('\n'))+'</code></pre>');buffer=[]}
      code=!code;continue;
    }
    if(code){buffer.push(line);continue}
    if(line.includes('|') && /^\s*\|?\s*:?-{3,}:?\s*\|[\s|:-]*$/.test(lines[index+1]||'')){
      const cells=row=>row.trim().replace(/^\||\|$/g,'').split('|').map(cell=>inline(cell.trim()));
      const header=cells(line);index++;
      const rows=[];while(index+1<lines.length && lines[index+1].includes('|') && lines[index+1].trim())rows.push(cells(lines[++index]));
      out.push('<div class="markdown-table"><table><thead><tr>'+header.map(cell=>'<th>'+cell+'</th>').join('')+'</tr></thead><tbody>'+rows.map(row=>'<tr>'+row.map(cell=>'<td>'+cell+'</td>').join('')+'</tr>').join('')+'</tbody></table></div>');continue;
    }
    const heading=line.match(/^(#{1,4})\s+(.+)$/);
    if(heading){out.push('<h'+(heading[1].length+1)+'>'+inline(heading[2])+'</h'+(heading[1].length+1)+'>');continue}
    if(!line.trim()){out.push('<div class="paragraph-gap"></div>');continue}
    out.push('<div>'+inline(line)+'</div>');
  }
  if(code)out.push('<pre><code>'+escape(buffer.join('\n'))+'</code></pre>');
  return out.join('');
}
if(typeof module!=='undefined')module.exports={messageMarkup,artifactReference};
