package platform

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceExportWritersProtectImportsAndResume(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(fmt.Sprint("sandbox=", remote), func(t *testing.T) {
			workspace := t.TempDir()
			run := strings.Repeat("b", 32)
			write := func(p string, offset int, data []byte, total int, digest string) error {
				if !remote {
					return writeWorkspaceExportChunk(workspace, p, offset, data, total, digest)
				}
				meta, _ := json.Marshal(map[string]any{"run": strings.Repeat("a", 32), "path": p, "offset": offset, "total": total, "sha256": digest})
				script := strings.Replace(sandboxWorkspaceExportWriter, "'/workspace'", fmt.Sprintf("%q", workspace), 1)
				out, err := exec.Command("python3", "-I", "-S", "-c", script, string(meta), base64.StdEncoding.EncodeToString(data)).CombinedOutput()
				if err != nil {
					return fmt.Errorf("%w: %s", err, out)
				}
				return nil
			}
			raw := []byte{'r', 'a', 'w', 0xff, 0x00, '\n'}
			digest := fmt.Sprintf("%x", sha256.Sum256(raw))
			prefix := "harness-output/imports/" + run + "/2/"
			prior := []byte("raw")
			if err := write(prefix+"collision.partial", 0, prior, len(prior), fmt.Sprintf("%x", sha256.Sum256(prior))); err != nil {
				t.Fatal(err)
			}
			if err := write(prefix+"collision", 0, raw, len(raw), digest); err != nil {
				t.Fatal("independent artifact conflicts with internal partial name", err)
			}
			if got, err := os.ReadFile(filepath.Join(workspace, prefix+"collision.partial")); err != nil || !bytes.Equal(got, prior) {
				t.Fatal("writing one artifact changed another final artifact", got, err)
			}
			p := prefix + "evidence/failure.log.artifact"
			target := filepath.Join(workspace, p)
			for _, invalid := range []string{
				"product.txt", "../escape", "/absolute", "workflow-output/" + run + "/other.log",
				"workflow-output/" + run + "/command-01.log", "workflow-output/" + run + "/command-0.log",
				"harness-output/imports/other/2/report.txt", "harness-output/imports/" + run + "/02/report.txt",
				prefix + "AGENTS.md", prefix + ".env", prefix + "dir/.private/file", prefix + "node_modules/file",
				prefix + "x/../report", prefix + "x//report", prefix + "x\\report", prefix + "x\x00report", prefix,
			} {
				if err := write(invalid, 0, raw, len(raw), digest); err == nil {
					t.Fatalf("accepted unprotected destination %q", invalid)
				}
			}
			if err := write(p, 0, raw[:3], len(raw), digest); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatal("partial import visible as final", err)
			}
			if err := write(p, 0, []byte("bad"), len(raw), digest); err == nil {
				t.Fatal("conflicting prefix accepted")
			}
			if err := write(p, 4, raw[4:], len(raw), digest); err == nil {
				t.Fatal("unwritten prefix skipped")
			}
			if err := write(p, 0, raw[:4], len(raw), digest); err != nil {
				t.Fatal("overlapping correct retry", err)
			}
			if err := write(p, 4, raw[4:], len(raw), strings.Repeat("0", 64)); err == nil {
				t.Fatal("wrong SHA accepted")
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatal("unverified import visible as final", err)
			}
			if err := write(p, 4, raw[4:], len(raw), digest); err != nil {
				t.Fatal("valid retry after checksum failure", err)
			}
			got, err := os.ReadFile(target)
			if err != nil || !bytes.Equal(got, raw) {
				t.Fatal("raw import changed", got, err)
			}
			if err := write(p, 0, raw, len(raw), digest); err != nil {
				t.Fatal("identical final not reused", err)
			}
			if err := write(p, 0, []byte("other!"), len(raw), fmt.Sprintf("%x", sha256.Sum256([]byte("other!")))); err == nil {
				t.Fatal("existing import overwritten")
			}
			got, _ = os.ReadFile(target)
			if !bytes.Equal(got, raw) {
				t.Fatal("conflict changed final")
			}
			for _, invalid := range []struct {
				offset, total int
				digest        string
			}{
				{-1, len(raw), digest}, {0, -1, digest}, {0, 8<<20 + 1, digest}, {0, len(raw), "bad"},
			} {
				if err := write(prefix+"invalid", invalid.offset, nil, invalid.total, invalid.digest); err == nil {
					t.Fatal("invalid chunk accepted", invalid)
				}
			}
			emptyDigest := fmt.Sprintf("%x", sha256.Sum256(nil))
			if err := write(prefix+"empty", 0, nil, 0, emptyDigest); err != nil {
				t.Fatal("empty artifact", err)
			}
		})
	}
}

func TestWorkspaceExportWritersRecoverOnlyVerifiedPublicationLinks(t *testing.T) {
	for _, remote := range []bool{false, true} {
		for _, state := range []string{"published-pair", "external-link", "conflicting-partial", "partial-symlink", "extra-link", "wrong-digest", "wrong-size"} {
			t.Run(fmt.Sprintf("sandbox=%v/%s", remote, state), func(t *testing.T) {
				workspace := t.TempDir()
				run := strings.Repeat("b", 32)
				p := "harness-output/imports/" + run + "/2/evidence.log.artifact"
				target := filepath.Join(workspace, p)
				partial := filepath.Join(filepath.Dir(target), ".evidence.log.artifact.partial")
				raw := []byte{'r', 'a', 'w', 0xff, 0x00}
				if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, raw, 0600); err != nil {
					t.Fatal(err)
				}
				link := partial
				if state == "external-link" || state == "conflicting-partial" || state == "partial-symlink" {
					link = filepath.Join(t.TempDir(), "external")
				}
				if err := os.Link(target, link); err != nil {
					t.Fatal(err)
				}
				if state == "conflicting-partial" {
					if err := os.WriteFile(partial, raw, 0600); err != nil {
						t.Fatal(err)
					}
				}
				if state == "partial-symlink" {
					if err := os.Symlink(target, partial); err != nil {
						t.Fatal(err)
					}
				}
				if state == "extra-link" {
					if err := os.Link(target, filepath.Join(t.TempDir(), "external")); err != nil {
						t.Fatal(err)
					}
				}
				beforePartial, partialErr := os.ReadFile(partial)
				digest := fmt.Sprintf("%x", sha256.Sum256(raw))
				if state == "wrong-digest" {
					digest = strings.Repeat("0", 64)
				}
				total := len(raw)
				if state == "wrong-size" {
					total++
				}
				var err error
				if !remote {
					err = writeWorkspaceExportChunk(workspace, p, 0, raw, total, digest)
				} else {
					meta, _ := json.Marshal(map[string]any{"run": run, "path": p, "offset": 0, "total": total, "sha256": digest})
					script := strings.Replace(sandboxWorkspaceExportWriter, "'/workspace'", fmt.Sprintf("%q", workspace), 1)
					out, e := exec.Command("python3", "-I", "-S", "-c", script, string(meta), base64.StdEncoding.EncodeToString(raw)).CombinedOutput()
					if e != nil {
						err = fmt.Errorf("%w: %s", e, out)
					}
				}
				if state == "published-pair" {
					if err != nil {
						t.Fatal("verified publication cannot recover after interrupted unlink", err)
					}
					if _, err := os.Lstat(partial); !os.IsNotExist(err) {
						t.Fatal("internal publication link not removed", err)
					}
				} else {
					if err == nil {
						t.Fatal("unverified or external links accepted")
					}
					afterPartial, afterErr := os.ReadFile(partial)
					if (partialErr == nil) != (afterErr == nil) || !bytes.Equal(beforePartial, afterPartial) {
						t.Fatal("rejected recovery changed conflicting partial")
					}
				}
				if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, raw) {
					t.Fatal("recovery changed completed bytes", got, err)
				}
			})
		}
	}
}
