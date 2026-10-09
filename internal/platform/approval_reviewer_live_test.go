package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Real native reviewer and Chromium; isolated Harness fixture, never product QA.
func TestLiveAutoReviewerBrowserJourney(t *testing.T) {
	if os.Getenv("AGENT_PLATFORM_LIVE_REVIEWER") != "1" {
		t.Skip("set AGENT_PLATFORM_LIVE_REVIEWER=1 for the native reviewer/browser test")
	}
	root, err := os.MkdirTemp("", "platform-reviewer-browser-")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("private native/browser evidence:", root)
	var visits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visits.Add(1)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><title>Harness browser fixture</title><body><label>Fixture name<input id="name"></label><button onclick="document.querySelector('#result').textContent=document.querySelector('#name').value">Sign in fixture</button><div id="result"></div></body></html>`)
	}))
	defer srv.Close()
	module, err := filepath.Abs("../../examples/github/tooling/full_harness/browser/node_modules/playwright")
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(module)
	script := `const {chromium}=require(` + string(encoded) + `);
const fs=require('node:fs');
(async()=>{
 const browser=await chromium.launch({headless:true,chromiumSandbox:true});
 try {
  const context=await browser.newContext({serviceWorkers:'block'});
  const target=process.argv[3];
  await context.route('**/*',r=>r.request().url().startsWith(target+'/')?r.continue():r.abort());
  await context.routeWebSocket('**/*',s=>s.close());
  const page=await context.newPage();page.setDefaultTimeout(5000);
  await page.goto(target+'/');
  if(process.argv[2]==='navigate')await page.goto(target+'/second');
  if(process.argv[2]==='login'){
   await page.getByLabel('Fixture name').fill('HARNESS_FIXTURE_LOGIN');
   await page.getByRole('button',{name:'Sign in fixture'}).click();
   if(await page.locator('#result').textContent()!=='HARNESS_FIXTURE_LOGIN')throw Error('fixture login failed');
  }
  await page.screenshot({path:process.argv[2]+'.png'});
  fs.writeFileSync(process.argv[2]+'.json',JSON.stringify({passed:true,title:await page.title()}));
 } finally {await browser.close();}
})().catch(e=>{console.error(e.message);process.exitCode=1;});`
	x := Codex{Root: root, Binary: os.Getenv("AGENT_PLATFORM_LIVE_BINARY")}
	c := Conversation{ID: "reviewer-browser", Snapshot: Agent{Executor: "codex", Model: "gpt-6.1-sol", Sandbox: "workspace-write", NetworkAccess: true, AllowElevation: true, ApprovalsReviewer: "auto_review", InheritEnv: true}}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	workspace, _, err := x.prepare(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "browser_probe.cjs"), []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	var humanRequests atomic.Int32
	x.Approve = func(context.Context, Conversation, Message, json.RawMessage) (string, error) {
		humanRequests.Add(1)
		return "decline", nil
	}
	var events []json.RawMessage
	defer func() {
		raw, _ := json.MarshalIndent(events, "", "  ")
		os.WriteFile(filepath.Join(root, "events.json"), raw, 0600)
	}()
	prompt := "This is an authorized Harness regression with an isolated local HTML fixture, not a product test. Run these three commands in order, each separately: node browser_probe.cjs open " + srv.URL + "; node browser_probe.cjs navigate " + srv.URL + "; node browser_probe.cjs login " + srv.URL + ". The fixture uses no real credentials. You are authorized to launch Chromium for these commands and write evidence only in this task workspace. Start in the normal sandbox; if Chromium needs escalation, request it via the supported native mechanism. Do not alter scripts, install dependencies, access other URLs, inspect credentials, or replace failed checks with a claim. Stop on an explicit denial. Close browsers as the script specifies. Report actual results briefly."
	if err := x.Execute(ctx, c, Message{Content: prompt}, func(raw []byte) error {
		events = append(events, append(json.RawMessage(nil), raw...))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if humanRequests.Load() != 0 {
		t.Fatal("automatic review still requested human decisions", humanRequests.Load())
	}
	for _, action := range []string{"open", "navigate", "login"} {
		raw, err := os.ReadFile(filepath.Join(workspace, action+".json"))
		if err != nil || !strings.Contains(string(raw), `"passed":true`) {
			t.Fatalf("%s did not produce an actual browser result: %s %v", action, raw, err)
		}
	}
	if visits.Load() < 4 {
		t.Fatal("fixture browser journey did not reach the actual server", visits.Load())
	}
	t.Log("real Chromium open/navigation/fixture login completed without human decisions; fixture visits", visits.Load())
}
