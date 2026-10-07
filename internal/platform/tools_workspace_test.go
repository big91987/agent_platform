package platform

import "testing"

func TestConversationToolWorkspaceBinding(t *testing.T) {
	a := Agent{ResolvedTools: map[string]ResolvedToolServer{"browser": {Connection: MCPConnection{Command: "python", Args: []string{"browser.py", "--workspace-root", "{{workspace}}"}}}}}
	bound := bindToolWorkspace(a, "/authorized/project")
	c := bound.ResolvedTools["browser"].Connection
	if c.CWD != "/authorized/project" || c.Args[2] != "/authorized/project" {
		t.Fatalf("workspace not bound: %+v", c)
	}
	if a.ResolvedTools["browser"].Connection.Args[2] != "{{workspace}}" {
		t.Fatal("shared snapshot was mutated")
	}
}
