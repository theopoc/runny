package core

// ChangeSummary is the resource result for one Target execution. Known
// distinguishes an explicit zero from missing or incomplete data.
type ChangeSummary struct {
	Detected bool   `json:"detected,omitempty"`
	Known    bool   `json:"known,omitempty"`
	Phase    string `json:"phase,omitempty"`
	Add      int64  `json:"add,omitempty"`
	Change   int64  `json:"change,omitempty"`
	Destroy  int64  `json:"destroy,omitempty"`
}
