package changes

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/theopoc/runny/internal/core"
)

func TestSummaries(t *testing.T) {
	plan := "Plan: 3 to add, 2 to change, 1 to destroy."
	apply := "Apply complete! Resources: 3 added, 2 changed, 1 destroyed."
	for _, tt := range []struct {
		name, command, output, phase string
		known                        bool
		add, change, destroy         int64
	}{
		{"terraform plan", "terraform plan", plan, "plan", true, 3, 2, 1},
		{"quoted terragrunt value", "terraform plan -var='description=terragrunt run --all'", plan, "plan", true, 3, 2, 1},
		{"nested raw multi unknown", "sh -c 'terragrunt run --all -- plan'", plan + "\n" + plan, "", false, 0, 0, 0},
		{"apply saved", "terraform apply saved.plan", apply, "apply", true, 3, 2, 1},
		{"saved named plan", "terraform apply plan", apply, "apply", true, 3, 2, 1},
		{"zero", "tofu plan", "No changes. Your infrastructure matches the configuration.", "plan", true, 0, 0, 0},
		{"imports forget", "tofu plan", "Plan: 9 to import, 3 to add, 2 to change, 1 to destroy, 4 to forget.", "plan", true, 3, 2, 1},
		{"applied forget", "tofu apply", "Apply complete! Resources: 9 imported, 3 added, 2 changed, 1 destroyed, 4 forgotten.", "apply", true, 3, 2, 1},
		{"actions", "terraform plan", plan + " Actions: 2 to invoke.", "plan", true, 3, 2, 1},
		{"last operation", "terraform plan && terraform apply -auto-approve", plan + "\n" + apply, "apply", true, 3, 2, 1},
		{"no applied result", "terraform apply", plan, "", false, 0, 0, 0},
		{"masked error", "terraform plan; true", plan + "\n│ Error: backend failed", "", false, 0, 0, 0},
		{"unknown line", "terraform plan", plan + "\nPlan: unsupported format", "", false, 0, 0, 0},
		{"oversize number", "terraform plan", "Plan: 999999999999999999999 to add, 2 to change, 1 to destroy.", "", false, 0, 0, 0},
		{"JSON tofu", "tofu plan", `{"type":"version","@module":"tofu.ui","ui":"1.2","tofu":"1.12.6"}
{"type":"change_summary","@module":"tofu.ui","changes":{"operation":"plan","add":3,"change":2,"remove":1,"forget":9}}`, "plan", true, 3, 2, 1},
		{"unknown JSON major", "tofu plan", `{"type":"version","@module":"tofu.ui","ui":"2.0"}
{"type":"change_summary","@module":"tofu.ui","changes":{"operation":"plan","add":3,"change":2,"remove":1}}`, "", false, 0, 0, 0},
		{"missing JSON field", "terraform plan", `{"type":"change_summary","@module":"terraform.ui","changes":{"operation":"plan","add":3,"remove":1}}`, "", false, 0, 0, 0},
		{"negative JSON field", "terraform plan", `{"type":"change_summary","@module":"terraform.ui","changes":{"operation":"plan","add":-3,"change":0,"remove":1}}`, "", false, 0, 0, 0},
		{"raw multi unknown", "terragrunt run --all -- plan", plan + "\n" + plan, "", false, 0, 0, 0},
		{"partial phases", "terragrunt run --all -- apply", "12:00:00 STDOUT [vpc] tofu: " + apply + "\n12:00:00 STDOUT [db] tofu: " + plan, "", false, 0, 0, 0},
		{"hook error", "terragrunt plan -detailed-exitcode", "12:00:00 STDOUT tofu: " + plan + "\n12:00:01 ERROR after_hook failed", "", false, 0, 0, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, step := range []int{1, 7, len(tt.output)} {
				p := New(tt.command)
				for i := 0; i < len(tt.output); i += step {
					p.Write([]byte(tt.output[i:min(len(tt.output), i+step)]))
				}
				got := p.Finish()
				if got.Known != tt.known || got.Phase != tt.phase || got.Add != tt.add || got.Change != tt.change || got.Destroy != tt.destroy {
					t.Fatalf("chunk=%d got %+v", step, got)
				}
				if !got.Detected {
					t.Fatal("summary not detected")
				}
			}
		})
	}
}

func TestTerragruntTotalsDeduplicateAndKeepPaths(t *testing.T) {
	for _, jsonLog := range []bool{false, true} {
		p := New("terragrunt run --all -- plan")
		for _, pair := range [][2]string{
			{"east/db", "Plan: 1 to add, 0 to change, 0 to destroy."},
			{"west/db", "Plan: 2 to add, 3 to change, 4 to destroy."},
			{"east/db", "Plan: 5 to add, 0 to change, 0 to destroy."},
		} {
			if jsonLog {
				// A JSON log object is a write, not necessarily a whole engine line.
				for _, part := range []string{pair[1][:12], pair[1][12:] + "\n"} {
					b, _ := json.Marshal(map[string]string{"level": "stdout", "working-dir": pair[0], "tf-path": "tofu", "msg": part})
					p.Write(append(b, '\n'))
				}
			} else {
				p.Write([]byte(fmt.Sprintf("12:00:00.123 STDOUT [%s] tofu: \x1b[32m%s\x1b[0m\r\n", pair[0], pair[1])))
			}
		}
		if got := p.Finish(); got != (core.ChangeSummary{Detected: true, Known: true, Phase: "plan", Add: 7, Change: 3, Destroy: 4}) {
			t.Fatalf("JSON=%v: %+v", jsonLog, got)
		}
	}
}

func TestIncompleteOrUnrelatedOutput(t *testing.T) {
	for _, output := range []string{
		"ordinary output\n", `{"type":"result","value":2}`,
	} {
		p := New("echo test")
		p.Write([]byte(output))
		if p.Finish().Detected {
			t.Fatal("unrelated output detected")
		}
	}
	p := New("terragrunt run --all -- plan")
	p.Write([]byte("12:00:00 STDOUT [vpc] tofu: Plan: 1 to add, 0 to change, 0 to destroy.\n12:00:01 STDOUT [db] tofu: Refreshing state...\n"))
	if got := p.Finish(); got.Known || !got.Detected {
		t.Fatalf("partial total: %+v", got)
	}
	p = New("terraform plan")
	p.Write([]byte(strings.Repeat("x", maxLineBytes*3) + "\nPlan: 1 to add, 0 to change, 0 to destroy.\n"))
	if p.Finish().Known {
		t.Fatal("lost overlong line treated as complete")
	}
	if len(p.outer.data) > maxLineBytes {
		t.Fatal("unbounded line")
	}
}

func TestDetailedExitCodeScope(t *testing.T) {
	for _, tt := range []struct {
		command string
		want    bool
	}{
		{"terraform plan -detailed-exitcode", true},
		{`terraform plan -detailed-exitcode -var 'value=$literal;*\path'`, true},
		{`terraform plan -detailed-exitcode -var "value=literal;*"`, true},
		{`terraform plan -detailed-exitcode -var 'value=it'\''s'`, true},
		{`terraform plan -detailed-exitcode -var "value=$expanded"`, false},
		{`terraform plan -detailed-exitcode -var "value=\$literal"`, true},
		{"/usr/bin/tofu -chdir='a b' plan -detailed-exitcode", true},
		{"terraform plan -detailed-exitcode=false", false},
		{"echo terraform plan -detailed-exitcode", false},
		{"terraform plan -detailed-exitcode; exit 2", false},
		{"terraform plan -detailed-exitcode && false", false},
		{"sh -c 'terraform plan -detailed-exitcode'", false},
		{"terraform apply -detailed-exitcode", false},
	} {
		p := New(tt.command)
		p.Write([]byte("Plan: 1 to add, 0 to change, 0 to destroy.\n"))
		got := p.DetailedPlanSucceeded(p.Finish())
		if got != tt.want {
			t.Errorf("%q: %v", tt.command, got)
		}
	}
}

func TestTerragruntAnnouncedUnitsMustAllComplete(t *testing.T) {
	for _, jsonLog := range []bool{false, true} {
		for _, complete := range []bool{false, true} {
			p := New("terragrunt run --all -- plan -detailed-exitcode")
			announcement := "The following units will be run, starting with dependencies and then their dependents:\n.\n├── east/db\n╰── west/db"
			if jsonLog {
				b, _ := json.Marshal(map[string]string{"level": "info", "msg": announcement})
				p.Write(append(b, '\n'))
			} else {
				p.Write([]byte("12:00:00 INFO " + announcement + "\n"))
			}
			p.Write([]byte("12:00:01 STDOUT [east/db] tofu: Plan: 1 to add, 0 to change, 0 to destroy.\n"))
			if complete {
				p.Write([]byte("12:00:02 STDOUT [west/db] tofu: Plan: 2 to add, 0 to change, 0 to destroy.\n"))
			}
			got := p.Finish()
			if got.Known != complete || p.DetailedPlanSucceeded(got) != complete {
				t.Fatalf("JSON=%v complete=%v: %+v", jsonLog, complete, got)
			}
			if complete && got.Add != 3 {
				t.Fatalf("wrong total: %+v", got)
			}
		}
	}
}
