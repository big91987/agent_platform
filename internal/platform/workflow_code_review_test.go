package platform

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCodeReviewTemplatesRespectExternalSandboxContract(t *testing.T) {
	source, err := filepath.Abs("../../examples/code-review")
	if err != nil {
		t.Fatal(err)
	}
	for _, engine := range []string{"cli", "delegate"} {
		t.Run(engine, func(t *testing.T) {
			script := `import json,sys
sys.path.insert(0,sys.argv[1])
from workflows import definition
refs={key:'REVIEW_'+key for key in ['OCR_LLM_URL','OCR_LLM_TOKEN','OCR_LLM_MODEL','OCR_LLM_PROTOCOL']}
print(json.dumps(definition('owner/project','GIT_REF',sys.argv[2],sys.argv[1]+'/skill',refs)))`
			raw, err := exec.Command("python3", "-c", script, source, engine).CombinedOutput()
			if err != nil {
				t.Fatalf("template: %v %s", err, raw)
			}
			var graph Workflow
			if err = json.Unmarshal(raw, &graph); err != nil {
				t.Fatal(err)
			}
			if err = validateWorkflow(graph); err != nil {
				t.Fatal(err)
			}
			for _, node := range graph.Nodes {
				if node.Agent != nil {
					if err = validateSandboxAgent(*node.Agent); err != nil {
						t.Fatalf("%s cannot start: %v", node.ID, err)
					}
				}
			}
		})
	}
}
