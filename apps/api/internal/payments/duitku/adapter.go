// Package duitku implements payments.Provider for Duitku
// (https://duitku.com), an Indonesian payment aggregator supporting
// QRIS and bank virtual accounts among other methods.
//
// This targets Duitku's classic "Request Transaction" API (MD5
// signatures, documented consistently across their official docs and
// community SDKs) — not their newer "POP" product line, which appears to
// use HMAC signatures and a different request shape. Confirm which your
// merchant credentials are actually provisioned for before this goes
// live; the two aren't interchangeable, and this project can't verify
// that without a real Duitku account.
package duitku

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"satelit-parfume-api/internal/payments"
)

const (
	// DefaultSandboxBaseURL is the safe default: a missing or blank
	// DUITKU_BASE_URL should never accidentally point at production.
	// Confirm the real production hostname from your live Duitku
	// merchant dashboard when you're ready — Duitku's docs show
	// different hosts for different product lines, so this project
	// deliberately doesn't guess at one.
	DefaultSandboxBaseURL = "https://sandbox.duitku.com"

	createInvoicePath = "/webapi/api/merchant/v2/inquiry"
	checkStatusPath   = "/webapi/api/merchant/transactionStatus"
)

type Adapter struct {
	merchantCode string
	merchantKey  string
	baseURL      string
	httpClient   *http.Client
}

func NewAdapter(merchantCode, merchantKey, baseURL string) *Adapter {
	if baseURL == "" {
		baseURL = DefaultSandboxBaseURL
	}
	return &Adapter{
		merchantCode: merchantCode,
		merchantKey:  merchantKey,
		baseURL:      baseURL,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (a *Adapter) Name() string { return "duitku" }

type createInvoiceRequest struct {
	MerchantCode    string `json:"merchantCode"`
	PaymentAmount   int64  `json:"paymentAmount"`
	PaymentMethod   string `json:"paymentMethod"`
	MerchantOrderID string `json:"merchantOrderId"`
	ProductDetails  string `json:"productDetails"`
	Email           string `json:"email"`
	PhoneNumber     string `json:"phoneNumber,omitempty"`
	CustomerVaName  string `json:"customerVaName"`
	CallbackURL     string `json:"callbackUrl"`
	ReturnURL       string `json:"returnUrl"`
	ExpiryPeriod    int    `json:"expiryPeriod"`
	Signature       string `json:"signature"`
}

type createInvoiceResponse struct {
	Reference     string `json:"reference"`
	PaymentURL    string `json:"paymentUrl"`
	VANumber      string `json:"vaNumber"`
	QrString      string `json:"qrString"`
	StatusCode    string `json:"statusCode"`
	StatusMessage string `json:"statusMessage"`
}

func (a *Adapter) CreatePayment(ctx context.Context, req payments.CreatePaymentRequest) (*payments.CreatePaymentResult, error) {
	if req.CustomerEmail == "" {
		// Duitku's classic API requires an email. Guest pickup checkout
		// (Phase 7) only collects name + phone, so this is a real gap:
		// online payment needs an email prompt added wherever it's
		// triggered from, not a fabricated placeholder address here.
		return nil, fmt.Errorf("duitku: email is required to create a payment")
	}

	body := createInvoiceRequest{
		MerchantCode:    a.merchantCode,
		PaymentAmount:   req.Amount,
		PaymentMethod:   req.PaymentMethod,
		MerchantOrderID: req.OrderNumber,
		ProductDetails:  req.ProductDetail,
		Email:           req.CustomerEmail,
		PhoneNumber:     req.CustomerPhone,
		CustomerVaName:  req.CustomerName,
		CallbackURL:     req.CallbackURL,
		ReturnURL:       req.ReturnURL,
		ExpiryPeriod:    req.ExpiryMinutes,
		Signature:       createInvoiceSignature(a.merchantCode, req.OrderNumber, req.Amount, a.merchantKey),
	}

	raw, err := a.postJSON(ctx, createInvoicePath, body)
	if err != nil {
		return nil, err
	}

	var resp createInvoiceResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("duitku: parse response: %w", err)
	}
	if resp.StatusCode != "" && resp.StatusCode != "00" {
		return nil, fmt.Errorf("duitku: %s (status %s)", resp.StatusMessage, resp.StatusCode)
	}
	if resp.Reference == "" {
		return nil, fmt.Errorf("duitku: response had no reference: %s", string(raw))
	}

	return &payments.CreatePaymentResult{
		Reference:   resp.Reference,
		PaymentURL:  resp.PaymentURL,
		QRString:    resp.QrString,
		VANumber:    resp.VANumber,
		RawResponse: string(raw),
	}, nil
}

type statusCheckRequest struct {
	MerchantCode    string `json:"merchantCode"`
	MerchantOrderID string `json:"merchantOrderId"`
	Signature       string `json:"signature"`
}

type statusCheckResponse struct {
	Reference     string `json:"reference"`
	Amount        string `json:"amount"`
	StatusCode    string `json:"statusCode"`
	StatusMessage string `json:"statusMessage"`
}

// VerifyPayment uses statusCheckSignature — the third of Duitku's three
// distinct signature formulas (see signature.go): no amount involved at
// all here, unlike the other two operations.
func (a *Adapter) VerifyPayment(ctx context.Context, orderNumber string) (*payments.VerifyPaymentResult, error) {
	body := statusCheckRequest{
		MerchantCode:    a.merchantCode,
		MerchantOrderID: orderNumber,
		Signature:       statusCheckSignature(a.merchantCode, orderNumber, a.merchantKey),
	}

	raw, err := a.postJSON(ctx, checkStatusPath, body)
	if err != nil {
		return nil, err
	}

	var resp statusCheckResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("duitku: parse response: %w", err)
	}

	amount, err := strconv.ParseInt(resp.Amount, 10, 64)
	if err != nil && resp.Amount != "" {
		return nil, fmt.Errorf("duitku: unparseable amount %q: %w", resp.Amount, err)
	}

	// statusCode "00" means paid/settled on this endpoint too — same
	// convention Duitku uses for the invoice-creation response.
	return &payments.VerifyPaymentResult{
		Reference: resp.Reference,
		Amount:    amount,
		Paid:      resp.StatusCode == "00",
	}, nil
}

// HandleWebhook parses Duitku's callback — sent as a standard HTML form
// POST (application/x-www-form-urlencoded), not JSON like the other two
// operations — and verifies it with callbackSignature, the second of the
// three distinct formulas.
func (a *Adapter) HandleWebhook(ctx context.Context, rawBody []byte, headers http.Header) (*payments.WebhookResult, error) {
	values, err := url.ParseQuery(string(rawBody))
	if err != nil {
		return nil, fmt.Errorf("duitku: parse callback body: %w", err)
	}

	merchantOrderID := values.Get("merchantOrderId")
	reference := values.Get("reference")
	resultCode := values.Get("resultCode")
	receivedSig := values.Get("signature")

	amount, err := strconv.ParseInt(values.Get("amount"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("duitku: invalid amount in callback: %w", err)
	}

	expectedSig := callbackSignature(a.merchantCode, amount, merchantOrderID, a.merchantKey)
	if !signaturesMatch(receivedSig, expectedSig) {
		return nil, fmt.Errorf("duitku: signature mismatch")
	}

	return &payments.WebhookResult{
		OrderNumber: merchantOrderID,
		Reference:   reference,
		Amount:      amount,
		Paid:        resultCode == "00",
	}, nil
}

// RefundPayment isn't implemented: Duitku's refund flow needs its own
// research (a different endpoint, and this project hasn't verified its
// request/signature shape the way CreatePayment/VerifyPayment/HandleWebhook
// are here) — returning a clear "not implemented" is more honest than a
// silent no-op or a guessed implementation that might move real money
// incorrectly. REFUNDED is still a settable order status (Phase 7) for
// recording that a refund happened by some other means; this is
// specifically about triggering one *through* Duitku.
func (a *Adapter) RefundPayment(ctx context.Context, reference string, amount int64) error {
	return fmt.Errorf("duitku: RefundPayment: %w", payments.ErrNotImplemented)
}

func (a *Adapter) postJSON(ctx context.Context, path string, body any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("duitku: encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("duitku: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("duitku: request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("duitku: read response: %w", err)
	}
	return raw, nil
}
