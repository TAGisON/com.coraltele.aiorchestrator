package control

import (
	"testing"
)

func TestBargePolicyTextCommitDefault(t *testing.T) {
	p := defaultBargePolicy()
	if !p.textCommit("hi") {
		t.Fatal("hi should commit")
	}
	if p.textCommit("a") {
		t.Fatal("single char should not commit")
	}
}
