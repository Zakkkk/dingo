// Generated Go code from api_response.dingo
// Sum types become interfaces with struct variants
package main

import (
	"fmt"
)

// APIResponse is the interface for the sum type
type APIResponse interface {
	isAPIResponse()
}

type Success struct {
	TransactionID string
	Amount        float64
}

func (Success) isAPIResponse() {}

type ValidationError struct {
	Field   string
	Message string
}

func (ValidationError) isAPIResponse() {}

type AuthError struct {
	Code   int
	Reason string
}

func (AuthError) isAPIResponse() {}

type RateLimited struct {
	RetryAfter int
}

func (RateLimited) isAPIResponse() {}

type ServerError struct {
	RequestID string
}

func (ServerError) isAPIResponse() {}

// PaymentStatus is the interface for the payment state sum type
type PaymentStatus interface {
	isPaymentStatus()
}

type Pending struct{}

func (Pending) isPaymentStatus() {}

type Processing struct {
	ProcessorID string
}

func (Processing) isPaymentStatus() {}

type Completed struct {
	TransactionID string
	CompletedAt   int64
}

func (Completed) isPaymentStatus() {}

type Failed struct {
	Reason   string
	CanRetry bool
}

func (Failed) isPaymentStatus() {}

type Refunded struct {
	RefundID string
	Amount   float64
}

func (Refunded) isPaymentStatus() {}

// HandleAPIResponse processes the response appropriately
func HandleAPIResponse(resp APIResponse) (bool, string) {
	switch r := resp.(type) {
	case Success:
		return true, fmt.Sprintf("Payment of $%.2f succeeded: %s", r.Amount, r.TransactionID)
	case ValidationError:
		return false, fmt.Sprintf("Invalid %s: %s", r.Field, r.Message)
	case AuthError:
		return false, fmt.Sprintf("Auth failed (%d): %s", r.Code, r.Reason)
	case RateLimited:
		return false, fmt.Sprintf("Rate limited - retry in %d seconds", r.RetryAfter)
	case ServerError:
		return false, fmt.Sprintf("Server error - reference: %s", r.RequestID)
	default:
		panic("non-exhaustive match")
	}
}

// CanRetryPayment checks if we should retry a failed payment
func CanRetryPayment(status PaymentStatus) bool {
	switch s := status.(type) {
	case Failed:
		return s.CanRetry
	case Pending:
		return true
	default:
		return false
	}
}

// GetStatusMessage generates user-friendly status message
func GetStatusMessage(status PaymentStatus) string {
	switch s := status.(type) {
	case Pending:
		return "Your payment is pending"
	case Processing:
		return fmt.Sprintf("Processing with %s", s.ProcessorID)
	case Completed:
		return fmt.Sprintf("Payment complete: %s", s.TransactionID)
	case Failed:
		if s.CanRetry {
			return fmt.Sprintf("Payment failed: %s (you can retry)", s.Reason)
		}
		return fmt.Sprintf("Payment failed: %s", s.Reason)
	case Refunded:
		return fmt.Sprintf("$%.2f has been refunded", s.Amount)
	default:
		panic("non-exhaustive match")
	}
}

func main() {
	// Simulate API responses
	responses := []APIResponse{
		Success{TransactionID: "TXN-123", Amount: 99.99},
		ValidationError{Field: "card_number", Message: "invalid format"},
		RateLimited{RetryAfter: 30},
	}

	fmt.Println("=== API Responses ===")
	for _, resp := range responses {
		ok, msg := HandleAPIResponse(resp)
		status := "FAIL"
		if ok {
			status = "OK"
		}
		fmt.Printf("[%s] %s\n", status, msg)
	}

	// Simulate payment statuses
	statuses := []PaymentStatus{
		Pending{},
		Processing{ProcessorID: "STRIPE"},
		Failed{Reason: "card declined", CanRetry: true},
		Completed{TransactionID: "TXN-456", CompletedAt: 1699900000},
	}

	fmt.Println("\n=== Payment Statuses ===")
	for _, status := range statuses {
		msg := GetStatusMessage(status)
		retry := ""
		if CanRetryPayment(status) {
			retry = " [can retry]"
		}
		fmt.Printf("%s%s\n", msg, retry)
	}
}
