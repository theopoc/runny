package changes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCapturedNativeSummaries(t *testing.T) {
	var fixtures []struct {
		Name, Command, Phase string
		Counts               [3]int64
		Exit                 int
	}
	raw, err := os.ReadFile("testdata/native/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata/native", f.Name+".txt"))
			if err != nil {
				t.Fatal(err)
			}
			for _, size := range []int{1, 73, len(raw)} {
				p := New(f.Command)
				for offset := 0; offset < len(raw); offset += size {
					p.Write(raw[offset:min(len(raw), offset+size)])
				}
				got := p.Finish()
				if got.Known != (f.Phase != "") || got.Phase != f.Phase || got.Add != f.Counts[0] || got.Change != f.Counts[1] || got.Destroy != f.Counts[2] {
					t.Fatalf("chunk=%d: %+v", size, got)
				}
				if f.Exit == 2 && p.DetailedPlanSucceeded(got) != (f.Phase == "plan") {
					t.Fatal("incorrect code 2 classification")
				}
			}
		})
	}
}
