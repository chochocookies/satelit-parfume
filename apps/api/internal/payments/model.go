package payments

import "time"

type Payment struct {
	ID            string    `json:"id"`
	OrderID       string    `json:"order_id"`
	Provider      string    `json:"provider"`
	PaymentMethod string    `json:"payment_method"`
	Reference     string    `json:"reference"`
	Amount        int64     `json:"amount"`
	Status        string    `json:"status"`
	QRString      string    `json:"qr_string,omitempty"`
	VANumber      string    `json:"va_number,omitempty"`
	PaymentURL    string    `json:"payment_url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

const (
	StatusPending = "PENDING"
	StatusPaid    = "PAID"
	StatusFailed  = "FAILED"
)

type PayRequest struct {
	PaymentMethod string `json:"payment_method" binding:"required"`
}
