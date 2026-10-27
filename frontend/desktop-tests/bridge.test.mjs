import test from 'node:test';
import assert from 'node:assert/strict';
import { createDesktopBridge } from '../src/assets/desktop/bridge.mjs';

function runtimeFixture() {
  let listener;
  const calls=[];
  let removed=0;
  return {
    calls,
    runtime: {
      Call: {ByName: async (...args)=>{calls.push(args);return {version:'test'};}},
      Events: {On: (name,fn)=>{assert.equal(name,'fasten:source-auth-response');listener=fn;return ()=>{removed++;};}},
    },
    emit: data=>listener({data}),
    get removed(){return removed;},
  };
}

test('native open and close use the rebuilt service contract',async()=>{
  const f=runtimeFixture();const bridge=createDesktopBridge(f.runtime);
  await bridge.openExternal('https://example.org','state-one');
  await bridge.closeAuthWindow('state-one');
  assert.deepEqual(f.calls,[
    ['github.com/karamhouse/fasten-desktop/internal/shell.DesktopService.OpenExternal','https://example.org','state-one'],
    ['github.com/karamhouse/fasten-desktop/internal/shell.DesktopService.CloseAuthWindow','state-one'],
  ]);
});

test('an unrelated callback does not consume the expected callback',()=>{
  const f=runtimeFixture();const bridge=createDesktopBridge(f.runtime);const received=[];
  bridge.onAuthResponse('right',data=>received.push(data));
  f.emit({state:'wrong',code:'ignore'});
  assert.equal(f.removed,0);
  f.emit({state:'right',code:'synthetic'});
  assert.deepEqual(received,[{state:'right',code:'synthetic'}]);
  assert.equal(f.removed,1);
});

test('cancelling a pending authorization removes its subscription once',()=>{
  const f=runtimeFixture();const bridge=createDesktopBridge(f.runtime);
  const stop=bridge.onAuthResponse('right',()=>assert.fail('cancelled callback delivered'));
  stop();stop();f.emit({state:'right',code:'ignore'});
  assert.equal(f.removed,1);
});

test('callback completion goes through the native window/state check',async()=>{
  const f=runtimeFixture();const bridge=createDesktopBridge(f.runtime);
  await bridge.completeAuth({state:'one',code:'synthetic'});
  assert.deepEqual(f.calls,[['github.com/karamhouse/fasten-desktop/internal/shell.DesktopService.CompleteAuth',{state:'one',code:'synthetic'}]]);
});
