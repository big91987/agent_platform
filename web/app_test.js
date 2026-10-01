const {test} = require('node:test');
const assert = require('node:assert/strict');
const {readFileSync} = require('node:fs');
const vm = require('node:vm');

test('fixed Issue links retain the destination through login and open the original conversation', async () => {
  for (const authenticated of [true, false]) {
    let authenticatedAfterLogin = authenticated;
    const id = 'a'.repeat(32);
    const location = {pathname: '/conversations/' + id, hash: '', search: ''};
    const context = vm.createContext({
      URLSearchParams, location, clearInterval, clearTimeout, setTimeout,
      window: {addEventListener() {}},
      document: {addEventListener() {}, querySelector() {}},
      history: {replaceState(_state, _title, path) {location.pathname = path; location.hash = ''; }},
      fetch: async path => {
        if (path === '/api/me') return {ok: authenticatedAfterLogin, status: authenticatedAfterLogin ? 200 : 401, json: async () => authenticatedAfterLogin ? {admin: true} : {error: '请登录'}};
        if (path === '/api/agents') return {ok: true, json: async () => []};
        throw new Error('Unexpected API: ' + path);
      },
    });
    const source = readFileSync(__dirname + '/app.js', 'utf8').replace(/\nroute\(\)\.catch\(showRouteError\);\s*$/, '');
    vm.runInContext(source, context);
    vm.runInContext(`
      let opened = null, loginNote = null;
      login = note => { loginNote = note || ''; };
      shell = () => {};
      toast = () => {};
      conversationView = async id => { opened = id; };
    `, context);
    await vm.runInContext('route()', context);
    assert.equal(location.hash, '');
    if (authenticated) {
      assert.equal(vm.runInContext('opened', context), id, 'valid login must open the original conversation');
      assert.equal(vm.runInContext('loginNote', context), null);
    } else {
      assert.equal(vm.runInContext('opened', context), null, 'expired grant must not authorize anonymous access');
      assert.equal(vm.runInContext('loginNote', context), '');
      assert.equal(location.pathname, '/conversations/' + id);
      authenticatedAfterLogin = true;
      await vm.runInContext('route()', context);
      assert.equal(vm.runInContext('opened', context), id);
    }
  }
});
