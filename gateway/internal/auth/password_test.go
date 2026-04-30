package auth

import "testing"

func TestPasswordRoundtrip(t *testing.T) {
	hash, err := HashPassword("correcthorsebattery")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("correcthorsebattery", hash) {
		t.Fatal("verify must succeed for correct password")
	}
	if VerifyPassword("wrong", hash) {
		t.Fatal("verify must fail for wrong password")
	}
}

func TestPasswordRejectsShort(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("expected error for short password")
	}
}

func TestPasswordVerifyRejectsBadFormat(t *testing.T) {
	if VerifyPassword("anything", "not-a-phc-string") {
		t.Fatal("expected false for malformed stored hash")
	}
}
