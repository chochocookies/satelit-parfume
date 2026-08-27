package duitku

import "testing"

// These lock in the exact field order for each of Duitku's three MD5
// signature formulas — see signature.go's package comment for why that
// matters. Expected values computed independently (Python's hashlib),
// not by reading the implementation back to itself, so a transposed
// field order in the code would actually fail these, not just agree
// with its own mistake.
func TestCreateInvoiceSignature(t *testing.T) {
	got := createInvoiceSignature("D1234", "SP-20260814-0001", 35000, "testkey")
	want := "9e9b2dab70bbdfb50148375cf1ec324a"
	if got != want {
		t.Errorf("createInvoiceSignature() = %q, want %q", got, want)
	}
}

func TestCallbackSignature(t *testing.T) {
	got := callbackSignature("D1234", 35000, "SP-20260814-0001", "testkey")
	want := "ee3f6f2f1e658e8f10bea31f79594683"
	if got != want {
		t.Errorf("callbackSignature() = %q, want %q", got, want)
	}
}

func TestStatusCheckSignature(t *testing.T) {
	got := statusCheckSignature("D1234", "SP-20260814-0001", "testkey")
	want := "edd90c8afbc0ee13c97486532a11be7a"
	if got != want {
		t.Errorf("statusCheckSignature() = %q, want %q", got, want)
	}
}

// The whole point of keeping these as three separate functions: the same
// four inputs must NOT produce the same signature across operations,
// since Duitku expects a different field order for each. If these ever
// collide, something has been refactored into "one generic signer" and
// the distinction this package exists to preserve has been lost.
func TestSignaturesDifferByOperation(t *testing.T) {
	invoice := createInvoiceSignature("D1234", "ORDER1", 10000, "key")
	callback := callbackSignature("D1234", 10000, "ORDER1", "key")
	status := statusCheckSignature("D1234", "ORDER1", "key")

	if invoice == callback {
		t.Error("createInvoiceSignature and callbackSignature produced the same hash for the same inputs — the field-order distinction between them has been lost")
	}
	if invoice == status {
		t.Error("createInvoiceSignature and statusCheckSignature produced the same hash for the same inputs")
	}
	if callback == status {
		t.Error("callbackSignature and statusCheckSignature produced the same hash for the same inputs")
	}
}

func TestSignaturesMatch(t *testing.T) {
	sig := createInvoiceSignature("D1234", "ORDER1", 10000, "key")

	if !signaturesMatch(sig, sig) {
		t.Error("signaturesMatch() = false comparing a signature to itself, want true")
	}
	if signaturesMatch(sig, "0000000000000000000000000000000") {
		t.Error("signaturesMatch() = true for a clearly different signature, want false")
	}
	if signaturesMatch("", "") == false {
		// Two empty strings are equal strings; this isn't a meaningful
		// signature check, just confirming ConstantTimeCompare doesn't
		// do anything surprising on empty input.
		t.Error("signaturesMatch(\"\", \"\") = false, want true (equal, if degenerate, input)")
	}
}
