package control

import (
	"testing"
)

func TestListenFinalCommit(t *testing.T) {
	if !listenFinalCommit("hi") {
		t.Fatal("hi should commit")
	}
	if listenFinalCommit("a") {
		t.Fatal("single char should not commit")
	}
	if listenFinalCommit("  ") {
		t.Fatal("blank should not commit")
	}
}
