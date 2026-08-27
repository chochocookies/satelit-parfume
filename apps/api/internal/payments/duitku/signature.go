package duitku

import (
	"crypto/md5" //nolint:gosec // required by Duitku's API itself, not a choice made here — see the package doc comment
	"crypto/subtle"
	"encoding/hex"
	"fmt"
)

// Duitku's classic "Request Transaction" API uses MD5-based signatures —
// and critically, TWO DIFFERENT FIELD ORDERS for what look like the same
// four inputs, depending on which endpoint you're signing for. Mixing
// these up is a well-known, easy mistake with this exact integration:
// this project's own author has debugged this precise confusion before,
// on a different application (a Laravel site using the same gateway).
// The two functions below are kept deliberately separate — never merged
// into one "generic" signer with a parameter for field order — so the
// distinction can't quietly disappear in a future refactor.

// createInvoiceSignature: MD5(merchantCode + merchantOrderId + paymentAmount + merchantKey).
// Used when requesting a new payment (CreatePayment).
func createInvoiceSignature(merchantCode, merchantOrderID string, amount int64, merchantKey string) string {
	raw := fmt.Sprintf("%s%s%d%s", merchantCode, merchantOrderID, amount, merchantKey)
	return md5Hex(raw)
}

// callbackSignature: MD5(merchantCode + amount + merchantOrderId + merchantKey).
// Note amount comes SECOND here, not third like createInvoiceSignature —
// the same four values, deliberately reordered by Duitku's own API
// design between these two operations. Used to verify an incoming
// webhook (HandleWebhook).
func callbackSignature(merchantCode string, amount int64, merchantOrderID string, merchantKey string) string {
	raw := fmt.Sprintf("%s%d%s%s", merchantCode, amount, merchantOrderID, merchantKey)
	return md5Hex(raw)
}

// statusCheckSignature: MD5(merchantCode + merchantOrderId + merchantKey) —
// a THIRD shape again: no amount at all. Used by VerifyPayment.
func statusCheckSignature(merchantCode, merchantOrderID, merchantKey string) string {
	raw := fmt.Sprintf("%s%s%s", merchantCode, merchantOrderID, merchantKey)
	return md5Hex(raw)
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s)) //nolint:gosec // Duitku's required scheme, not used for anything security-load-bearing beyond matching their API
	return hex.EncodeToString(sum[:])
}

// signaturesMatch compares in constant time — signature verification is
// exactly the kind of comparison that shouldn't leak timing information
// about how much of the expected value matched, even though MD5 itself
// (Duitku's choice, not this project's) is not a strong MAC to begin with.
func signaturesMatch(received, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(received), []byte(expected)) == 1
}
