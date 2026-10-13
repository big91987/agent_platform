package platform

import (
	"encoding/json"
	"errors"
	"path"
)

func sandboxNativeSkills(raw json.RawMessage, workspace string) ([]nativeSkill, error) {
	var result struct {
		Data []struct {
			Cwd    string `json:"cwd"`
			Skills []struct {
				Path    string `json:"path"`
				Enabled *bool  `json:"enabled"`
			} `json:"skills"`
			Errors []json.RawMessage `json:"errors"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &result) != nil || len(result.Data) != 1 || result.Data[0].Cwd != workspace || len(result.Data[0].Errors) != 0 {
		return nil, errors.New("native Skill discovery did not confirm the shared workspace scope")
	}
	seen := map[string]bool{}
	actual := []nativeSkill{}
	for _, skill := range result.Data[0].Skills {
		if skill.Enabled == nil || !path.IsAbs(skill.Path) || path.Clean(skill.Path) != skill.Path || seen[skill.Path] {
			return nil, errors.New("native Skill discovery returned an invalid or duplicate path")
		}
		seen[skill.Path] = true
		actual = append(actual, nativeSkill{Path: skill.Path, Enabled: *skill.Enabled})
	}
	return actual, nil
}

// Native discovery owns its filesystem paths. Do not normalize them against
// the platform host or infer an enabled scope from the transferred files.
func sandboxSkillChoices(actual []nativeSkill, selected []string, verify bool) ([]nativeSkill, error) {
	wanted, seen := map[string]bool{}, map[string]bool{}
	for _, selectedPath := range selected {
		wanted[selectedPath] = true
	}
	choices := []nativeSkill{}
	for _, skill := range actual {
		seen[skill.Path] = true
		if skill.Enabled != wanted[skill.Path] {
			if verify {
				return nil, errors.New("native Skill scope differs from the selected role; no task was sent")
			}
			choices = append(choices, nativeSkill{Path: skill.Path, Enabled: wanted[skill.Path]})
		}
	}
	for selectedPath := range wanted {
		if !seen[selectedPath] {
			return nil, errors.New("selected Skill was not discovered; no task was sent")
		}
	}
	return choices, nil
}
