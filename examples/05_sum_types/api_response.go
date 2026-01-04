// Real-world example: API response types as sum types
// Sum types model "one of several possible shapes" - perfect for APIs
//
// === Design Decision: Enum to Interface Pattern ===
//
// Dingo enums are transformed to Go interface patterns:
//
//	enum APIResponse { Success{...}, Error{...} }
//	→  type APIResponse interface { isAPIResponse() }
//
// Each variant becomes a struct implementing the interface, enabling
// exhaustive pattern matching via Go's type switch.
package main

import "fmt"

// APIResponse models all possible responses from our payment API
// Each variant has different data - impossible to confuse them
//
//line /Users/jack/mag/dingo/examples/05_sum_types/api_response.dingo:19:1
type APIResponse interface{ isAPIResponse() }

type APIResponseSuccess struct {
	transactionID string
	amount        float64
}

func (APIResponseSuccess) isAPIResponse() {}
func NewAPIResponseSuccess(transactionID string, amount float64) APIResponse {
	return APIResponseSuccess{transactionID: transactionID, amount: amount}
}

type APIResponseValidationError struct {
	field   string
	message string
}

func (APIResponseValidationError) isAPIResponse() {}
func NewAPIResponseValidationError(field string, message string) APIResponse {
	return APIResponseValidationError{field: field, message: message}
}

type APIResponseAuthError struct {
	code   int
	reason string
}

func (APIResponseAuthError) isAPIResponse() {}
func NewAPIResponseAuthError(code int, reason string) APIResponse {
	return APIResponseAuthError{code: code, reason: reason}
}

type APIResponseRateLimited struct{ retryAfter int }

func (APIResponseRateLimited) isAPIResponse() {}
func NewAPIResponseRateLimited(retryAfter int) APIResponse {
	return APIResponseRateLimited{retryAfter: retryAfter}
}

type APIResponseServerError struct{ requestID string }

func (APIResponseServerError) isAPIResponse() {}
func NewAPIResponseServerError(requestID string) APIResponse {
	return APIResponseServerError{requestID: requestID}
}

//line /Users/jack/mag/dingo/examples/05_sum_types/api_response.dingo:26:1

// PaymentStatus tracks the state of a payment
//
//line /Users/jack/mag/dingo/examples/05_sum_types/api_response.dingo:28:1
type PaymentStatus interface{ isPaymentStatus() }

type PaymentStatusPending struct{}

func (PaymentStatusPending) isPaymentStatus() {}
func NewPaymentStatusPending() PaymentStatus  { return PaymentStatusPending{} }

type PaymentStatusProcessing struct{ processorID string }

func (PaymentStatusProcessing) isPaymentStatus() {}
func NewPaymentStatusProcessing(processorID string) PaymentStatus {
	return PaymentStatusProcessing{processorID: processorID}
}

type PaymentStatusCompleted struct {
	transactionID string
	completedAt   int64
}

func (PaymentStatusCompleted) isPaymentStatus() {}
func NewPaymentStatusCompleted(transactionID string, completedAt int64) PaymentStatus {
	return PaymentStatusCompleted{transactionID: transactionID, completedAt: completedAt}
}

type PaymentStatusFailed struct {
	reason   string
	canRetry bool
}

func (PaymentStatusFailed) isPaymentStatus() {}
func NewPaymentStatusFailed(reason string, canRetry bool) PaymentStatus {
	return PaymentStatusFailed{reason: reason, canRetry: canRetry}
}

type PaymentStatusRefunded struct {
	refundID string
	amount   float64
}

func (PaymentStatusRefunded) isPaymentStatus() {}
func NewPaymentStatusRefunded(refundID string, amount float64) PaymentStatus {
	return PaymentStatusRefunded{refundID: refundID, amount: amount}
}

//line /Users/jack/mag/dingo/examples/05_sum_types/api_response.dingo:35:1

// HandleAPIResponse processes the response appropriately
// Uses type switch - idiomatic Go pattern for sum types
func HandleAPIResponse(resp APIResponse) (bool, string) {
	switch v := resp.(type) {
	case APIResponseSuccess:
		return true, fmt.Sprintf("Payment of $%.2f succeeded: %s", v.amount, v.transactionID)
	case APIResponseValidationError:
		return false, fmt.Sprintf("Invalid %s: %s", v.field, v.message)
	case APIResponseAuthError:
		return false, fmt.Sprintf("Auth failed (%d): %s", v.code, v.reason)
	case APIResponseRateLimited:
		return false, fmt.Sprintf("Rate limited - retry in %d seconds", v.retryAfter)
	case APIResponseServerError:
		return false, fmt.Sprintf("Server error - reference: %s", v.requestID)
	}
	return false, "unknown response"
}

// CanRetryPayment checks if we should retry a failed payment
func CanRetryPayment(status PaymentStatus) bool {
	switch v := status.(type) {
	case PaymentStatusFailed:
		return v.canRetry
	case PaymentStatusPending:
		return true // Not started yet
	default:
		return false // Other states can't retry
	}
}

// GetStatusMessage generates user-friendly status message
func GetStatusMessage(status PaymentStatus) string {
	switch v := status.(type) {
	case PaymentStatusPending:
		return "Your payment is pending"
	case PaymentStatusProcessing:
		return fmt.Sprintf("Processing with %s", v.processorID)
	case PaymentStatusCompleted:
		return fmt.Sprintf("Payment complete: %s", v.transactionID)
	case PaymentStatusFailed:
		if v.canRetry {
			return fmt.Sprintf("Payment failed: %s (you can retry)", v.reason)
		}
		return fmt.Sprintf("Payment failed: %s", v.reason)
	case PaymentStatusRefunded:
		return fmt.Sprintf("$%.2f has been refunded", v.amount)
	}
	return "Unknown status"
}

func main() {
	// Simulate API responses - use constructor functions (Go-idiomatic)
	responses := []APIResponse{
		NewAPIResponseSuccess("TXN-123", 99.99),
		NewAPIResponseValidationError("card_number", "invalid format"),
		NewAPIResponseRateLimited(30),
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

	// Simulate payment statuses - use constructor functions
	statuses := []PaymentStatus{
		NewPaymentStatusPending(),
		NewPaymentStatusProcessing("STRIPE"),
		NewPaymentStatusFailed("card declined", true),
		NewPaymentStatusCompleted("TXN-456", 1699900000),
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
