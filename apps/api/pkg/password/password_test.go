package password

import "testing"

func TestHashAndVerify(t *testing.T) {
	hash, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if hash == "" {
		t.Fatal("Hash() returned empty string")
	}

	if !Verify(hash, "correct horse battery staple") {
		t.Error("Verify() = false for the correct password, want true")
	}
	if Verify(hash, "wrong password") {
		t.Error("Verify() = true for an incorrect password, want false")
	}
}

func TestHashProducesDifferentSaltsEachTime(t *testing.T) {
	h1, _ := Hash("same password")
	h2, _ := Hash("same password")
	if h1 == h2 {
		t.Error("two hashes of the same password should differ (bcrypt salts each call)")
	}
	if !Verify(h1, "same password") || !Verify(h2, "same password") {
		t.Error("both differently-salted hashes should still verify the original password")
	}
}
