package core

import "testing"

// Two items that score the same go to the shallower one, wherever it is listed:
// "skill" is as close to the top-level skills folder as to skill-doctor inside
// commands, which comes first.
func TestEqualScoreGoesToShallowerItem(t *testing.T) {
	items, err := LoadYAMLFromString(`
- name: commands
  description: Claude Code's own commands
  children:
    - name: skill-doctor
      description: Show which loaded skills are unused
- name: skills
  description: Claude Code's skills
  children:
    - name: done
`)
	if err != nil {
		t.Fatalf("LoadYAMLFromString: %v", err)
	}
	cfg := Config{Tiered: true}
	s, searchCols := NewState(items, cfg)
	s.TopCtx().Query = []rune("skill")
	FilterItems(s, cfg, searchCols)
	filtered := s.TopCtx().Filtered
	if len(filtered) < 2 || filtered[0].Fields[0] != "skills" || filtered[1].Fields[0] != "skill-doctor" {
		var names []string
		for _, it := range filtered {
			names = append(names, it.Fields[0])
		}
		t.Fatalf("ranked %v, want skills before skill-doctor", names)
	}
	if filtered[0].Score != filtered[1].Score {
		t.Fatalf("the two don't tie (%+v, %+v), so this tests nothing", filtered[0].Score, filtered[1].Score)
	}
}
