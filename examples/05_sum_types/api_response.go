// Real-world example: API response types as sum types
// Sum types model "one of several possible shapes" - perfect for APIs
package main

import "fmt"

// APIResponse models all possible responses from our payment API
// Each variant has different data - impossible to confuse them

type APIResponseTag uint8

const (
	APIResponseTagSuccess APIResponseTag = iota
	APIResponseTagValidationError
	APIResponseTagAuthError
	APIResponseTagRateLimited
	APIResponseTagServerError
)

type APIResponse struct {
	tag                     APIResponseTag
	autherror_code          *int
	autherror_reason        *string
	ratelimited_retryAfter  *int
	servererror_requestID   *string
	success_amount          *float64
	success_transactionID   *string
	validationerror_field   *string
	validationerror_message *string
}

func APIResponseSuccess(transactionID string, amount float64) APIResponse {
	return APIResponse{tag: APIResponseTagSuccess, success_transactionID: &transactionID, success_amount: &amount}
}
func APIResponseValidationError(field string, message string) APIResponse {
	return APIResponse{tag: APIResponseTagValidationError, validationerror_field: &field, validationerror_message: &message}
}
func APIResponseAuthError(code int, reason string) APIResponse {
	return APIResponse{tag: APIResponseTagAuthError, autherror_code: &code, autherror_reason: &reason}
}
func APIResponseRateLimited(retryAfter int) APIResponse {
	return APIResponse{tag: APIResponseTagRateLimited, ratelimited_retryAfter: &retryAfter}
}
func APIResponseServerError(requestID string) APIResponse {
	return APIResponse{tag: APIResponseTagServerError, servererror_requestID: &requestID}
}
func (e APIResponse) IsSuccess() bool {
	return e.tag == APIResponseTagSuccess
}
func (e APIResponse) IsValidationError() bool {
	return e.tag == APIResponseTagValidationError
}
func (e APIResponse) IsAuthError() bool {
	return e.tag == APIResponseTagAuthError
}
func (e APIResponse) IsRateLimited() bool {
	return e.tag == APIResponseTagRateLimited
}
func (e APIResponse) IsServerError() bool {
	return e.tag == APIResponseTagServerError
}

// PaymentStatus tracks the state of a payment

type PaymentStatusTag uint8

const (
	PaymentStatusTagPending PaymentStatusTag = iota
	PaymentStatusTagProcessing
	PaymentStatusTagCompleted
	PaymentStatusTagFailed
	PaymentStatusTagRefunded
)

type PaymentStatus struct {
	tag                     PaymentStatusTag
	completed_completedAt   *int64
	completed_transactionID *string
	failed_canRetry         *bool
	failed_reason           *string
	processing_processorID  *string
	refunded_amount         *float64
	refunded_refundID       *string
}
func PaymentStatusPending() PaymentStatus {
	return PaymentStatus{tag: PaymentStatusTagPending}
}
func PaymentStatusProcessing(processorID string) PaymentStatus {
	return PaymentStatus{tag: PaymentStatusTagProcessing, processing_processorID: &processorID}
}
func PaymentStatusCompleted(transactionID string, completedAt int64) PaymentStatus {
	return PaymentStatus{tag: PaymentStatusTagCompleted, completed_transactionID: &transactionID, completed_completedAt: &completedAt}
}
func PaymentStatusFailed(reason string, canRetry bool) PaymentStatus {
	return PaymentStatus{tag: PaymentStatusTagFailed, failed_reason: &reason, failed_canRetry: &canRetry}
}
func PaymentStatusRefunded(refundID string, amount float64) PaymentStatus {
	return PaymentStatus{tag: PaymentStatusTagRefunded, refunded_refundID: &refundID, refunded_amount: &amount}
}
func (e PaymentStatus) IsPending() bool {
	return e.tag == PaymentStatusTagPending
}
func (e PaymentStatus) IsProcessing() bool {
	return e.tag == PaymentStatusTagProcessing
}
func (e PaymentStatus) IsCompleted() bool {
	return e.tag == PaymentStatusTagCompleted
}
func (e PaymentStatus) IsFailed() bool {
	return e.tag == PaymentStatusTagFailed
}
func (e PaymentStatus) IsRefunded() bool {
	return e.tag == PaymentStatusTagRefunded
}

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
