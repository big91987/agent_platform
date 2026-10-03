const {test}=require('node:test');
const assert=require('node:assert/strict');
const {messageMarkup,artifactReference}=require('./markdown');
test('native replies format headings and code while escaping executable HTML',()=>{
 const text=messageMarkup('# Result\n**Confirmed** `JSON`\n```js\n<script>alert(1)</script>\n```\n<img src=x onerror=alert(1)>');
 assert.match(text,/<h2>Result<\/h2>/);assert.match(text,/<strong>Confirmed<\/strong>/);assert.match(text,/<code>JSON<\/code>/);assert.doesNotMatch(text,/<script>|<img/);assert.match(text,/&lt;script&gt;/);
});

test('workspace references become previews without granting arbitrary file or script access',()=>{
 const files=[{path:'docs/design.md'},{path:'prototype/preview.html'},{path:'images/mobile.png'}];
 const resolve=ref=>artifactReference(ref,files,'/workspace');
 assert.deepEqual(resolve('/workspace/docs/design.md:12'),{path:'docs/design.md',line:12});
 assert.deepEqual(artifactReference('../images/mobile.png',files,'/workspace','docs/design.md'),{path:'images/mobile.png',line:0});
 for(const invalid of ['../private.md','/other/docs/design.md','file:///workspace/docs/design.md','javascript:alert(1)','//other/docs/design.md','missing.md'])assert.equal(resolve(invalid),null);
 const html=messageMarkup('[原型](prototype/preview.html) `docs/design.md:12` ![手机](images/mobile.png) [网站](https://example.com) [危险](javascript:alert)\n```\n[原型](prototype/preview.html)\n```',resolve);
 assert.match(html,/data-file-path="prototype\/preview.html"/);
 assert.match(html,/data-file-line="12"/);
 assert.match(html,/data-file-path="images\/mobile.png"/);
 assert.match(html,/href="https:\/\/example.com"/);
 assert.doesNotMatch(html,/href="javascript:|<img/);
 assert.match(html,/<pre><code>\[原型\]/,'code examples remain literal');
 assert.match(messageMarkup('| 文件 | 说明 |\n|---|---|\n| [设计](docs/design.md) | <script> |',resolve),/<table>[\s\S]*data-file-path="docs\/design.md"[\s\S]*&lt;script&gt;/);
});
