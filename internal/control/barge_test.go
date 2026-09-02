package control

import (
	"testing"
)

func TestBargePolicyTextCommitDefault(t *testing.T) {
	p := defaultBargePolicy()
	// Default MinBargeChars is 3; the energy-VAD path handles shorter interrupts.
	if !p.textCommit("stop") {
		t.Fatal("a real word should commit")
	}
	if p.textCommit("hi") {
		t.Fatal("two chars should not commit at min 3")
	}
}
