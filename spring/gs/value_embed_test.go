package gs

import "testing"

type valueEmbedPod struct {
	Name string `value:"${pod.name:=}"`
}

type valueEmbedSvc struct {
	valueEmbedPod
}

// A struct with `value` tags, embedded anonymously in a bean, still gets its
// fields bound from configuration — no separate bean registration needed.
func TestValueTagEmbedBinds(t *testing.T) {
	t.Setenv("GS_POD_NAME", "pod-xyz")
	RunTest(t, func(s *valueEmbedSvc) {
		if s.Name != "pod-xyz" {
			t.Fatalf("embed binding failed: %+v", s.valueEmbedPod)
		}
	})
}
