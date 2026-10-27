// Package smoke drives a local-only native WebView integration check. Its routes
// are installed only with --smoke-test and an explicit fresh data directory.
package smoke

import (
	"encoding/json"
	"net/http"
	"sync"
)

type Report struct {
	OK     bool     `json:"ok"`
	Checks []string `json:"checks"`
	Error  string   `json:"error,omitempty"`
}
type Session struct {
	OpenCallback  func()
	OpenObserver  func()
	CheckObserver func()
	Complete      func(Report)
	mu            sync.Mutex
	observerReady bool
	leaked        bool
	checked       bool
}

func (s *Session) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/__desktop_smoke__" {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(page))
			return
		}
		if r.URL.Path != "/__desktop_smoke__/control" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Query().Get("action") {
		case "observer":
			s.OpenObserver()
		case "observer-ready":
			s.mu.Lock()
			s.observerReady = true
			s.mu.Unlock()
		case "observer-status":
			s.mu.Lock()
			s.leaked = r.URL.Query().Get("leaked") == "true"
			s.checked = true
			s.mu.Unlock()
		case "callback":
			s.OpenCallback()
		case "check-observer":
			s.CheckObserver()
		case "status":
			s.mu.Lock()
			defer s.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]bool{"ready": s.observerReady, "checked": s.checked, "leaked": s.leaked})
			return
		case "result":
			var report Report
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&report); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			go s.Complete(report)
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

const page = `<!doctype html><html><body><h1>Fasten Community desktop smoke test</h1><pre id="status">Starting…</pre><script type="module">
import * as runtime from '/wails/runtime.js';
import {createDesktopBridge} from '/assets/desktop/bridge.mjs';
const bridge=createDesktopBridge(runtime), checks=[];
const role=new URL(location.href).searchParams.get('role');
const post=(action,data)=>fetch('/__desktop_smoke__/control?action='+action,{method:'POST',body:data&&JSON.stringify(data)});
const check=(value,name)=>{if(!value)throw new Error(name);checks.push(name);document.querySelector('#status').textContent=checks.join('\n');};
const wait=async(predicate)=>{for(let i=0;i<100;i++){if(await predicate())return;await new Promise(r=>setTimeout(r,100));}throw new Error('native smoke handshake timed out');};
const status=()=>post('status').then(r=>r.json());
if(role==='observer') {
 let leaked=false;
 runtime.Events.On('fasten:source-auth-response',()=>{leaked=true;});
 window.reportSmokeStatus=()=>post('observer-status&leaked='+leaked);
 post('observer-ready');
} else if(role==='callback') {
 bridge.completeAuth({state:'smoke-one',code:'synthetic-code'}).catch(console.error);
} else {
 try {
  const info=await bridge.getInfo();check(info.application==='Fasten Community','native binding round trip');
  const health=await fetch('/api/health').then(r=>r.json());check(health.data.first_run_wizard&&!health.data.standby_mode,'fresh database API');
  const account={username:'desktop-smoke',password:'synthetic-desktop-password',join_mailing_list:false};
  const send=path=>fetch('/api/auth/'+path,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(account)}).then(r=>r.json());
  check((await send('signup')).success,'local account creation');
  check((await send('signin')).success,'local sign in');
  let rejected=false;try{await bridge.openExternal('file:///etc/passwd','smoke-one');}catch{rejected=true;}
  check(rejected,'native URL validation');
  const missing=await fetch('/api/not-a-route');check(missing.status===404&&missing.headers.get('content-type').includes('json'),'API errors stay separate from SPA');
  await post('observer');await wait(async()=> (await status()).ready);
  const callback=new Promise((resolve,reject)=>{const stop=bridge.onAuthResponse('smoke-one',resolve);setTimeout(()=>{stop();reject(new Error('native callback timed out'));},10000);});
  await post('callback');const data=await callback;check(data.code==='synthetic-code','owned popup callback round trip');
  await post('check-observer');await wait(async()=> (await status()).checked);
  check(!(await status()).leaked,'callback isolated from unrelated window');
  await post('result',{ok:true,checks});
 }catch(error){document.querySelector('#status').textContent=String(error);await post('result',{ok:false,checks,error:String(error)});}
}
</script></body></html>`
