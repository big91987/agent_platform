const {test}=require('node:test');
const assert=require('node:assert/strict');
const {messageMarkup}=require('./markdown');
test('native replies format headings and code while escaping executable HTML',()=>{
 const text=messageMarkup('# Result\n**Confirmed** `JSON`\n```js\n<script>alert(1)</script>\n```\n<img src=x onerror=alert(1)>');
 assert.match(text,/<h2>Result<\/h2>/);assert.match(text,/<strong>Confirmed<\/strong>/);assert.match(text,/<code>JSON<\/code>/);assert.doesNotMatch(text,/<script>|<img/);assert.match(text,/&lt;script&gt;/);
});
