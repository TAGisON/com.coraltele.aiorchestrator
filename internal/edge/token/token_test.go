package token_test

import (
	"testing"
	"time"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/edge/token"
)

func TestIssueValidate(t *testing.T) {
	secret := []byte("lab-secret")
	tok, err := token.Issue(secret, token.Claims{
		TenantID:  "t1",
		SessionID: "s1",
	}, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	c, err := token.Validate(secret, tok, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if c.SessionID != "s1" || c.TenantID != "t1" {
		t.Fatalf("claims %+v", c)
	}
}

func TestValidate_BadSig(t *testing.T) {
	secret := []byte("lab-secret")
	tok, err := token.Issue(secret, token.Claims{SessionID: "s1"}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	_, err = token.Validate([]byte("other"), tok, time.Now())
	if err != token.ErrInvalid {
		t.Fatalf("want ErrInvalid got %v", err)
	}
}

func TestValidate_Expired(t *testing.T) {
	secret := []byte("lab-secret")
	tok, err := token.Issue(secret, token.Claims{
		SessionID: "s1",
		Exp:       time.Now().Add(-time.Minute).Unix(),
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = token.Validate(secret, tok, time.Now())
	if err != token.ErrExpired {
		t.Fatalf("want ErrExpired got %v", err)
	}
}
