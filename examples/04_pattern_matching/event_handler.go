// Real-world example: Event handler with pattern matching
// Pattern matching makes complex conditionals readable and exhaustive
package main

import (
	"fmt"
)

// Event sum type - all possible events in the system

type EventTag uint8

const (
	EventTagUserCreated EventTag = iota
	EventTagUserDeleted
	EventTagOrderPlaced
	EventTagOrderShipped
	EventTagPaymentReceived
	EventTagPaymentFailed
)

type Event struct {
	tag                         EventTag
	orderplaced_amount          *float64
	orderplaced_orderID         *string
	orderplaced_userID          *int
	ordershipped_orderID        *string
	ordershipped_trackingNumber *string
	paymentfailed_orderID       *string
	paymentfailed_reason        *string
	paymentreceived_amount      *float64
	paymentreceived_orderID     *string
	usercreated_email           *string
	usercreated_userID          *int
	userdeleted_userID          *int
}

func EventUserCreated(userID int, email string) Event {
	return Event{tag: EventTagUserCreated, usercreated_userID: &userID, usercreated_email: &email}
}
func EventUserDeleted(userID int) Event {
	return Event{tag: EventTagUserDeleted, userdeleted_userID: &userID}
}
func EventOrderPlaced(orderID string, amount float64, userID int) Event {
	return Event{tag: EventTagOrderPlaced, orderplaced_orderID: &orderID, orderplaced_amount: &amount, orderplaced_userID: &userID}
}
func EventOrderShipped(orderID string, trackingNumber string) Event {
	return Event{tag: EventTagOrderShipped, ordershipped_orderID: &orderID, ordershipped_trackingNumber: &trackingNumber}
}
func EventPaymentReceived(orderID string, amount float64) Event {
	return Event{tag: EventTagPaymentReceived, paymentreceived_orderID: &orderID, paymentreceived_amount: &amount}
}
func EventPaymentFailed(orderID string, reason string) Event {
	return Event{tag: EventTagPaymentFailed, paymentfailed_orderID: &orderID, paymentfailed_reason: &reason}
}
func (e Event) IsUserCreated() bool {
	return e.tag == EventTagUserCreated
}
func (e Event) IsUserDeleted() bool {
	return e.tag == EventTagUserDeleted
}
func (e Event) IsOrderPlaced() bool {
	return e.tag == EventTagOrderPlaced
}
func (e Event) IsOrderShipped() bool {
	return e.tag == EventTagOrderShipped
}
func (e Event) IsPaymentReceived() bool {
	return e.tag == EventTagPaymentReceived
}
func (e Event) IsPaymentFailed() bool {
	return e.tag == EventTagPaymentFailed
}

// ProcessEvent handles all event types with exhaustive matching
// The compiler ensures every Event variant is handled
func ProcessEvent(event Event) string {
	tmp := event
	switch tmp.tag {
	case EventTagUserCreated:
		userID := *tmp.usercreated_userID
		email := *tmp.usercreated_email
		return fmt.Sprintf("Welcome email sent to %s (user #%d)", email, userID) // dingo:M:1
	case EventTagUserDeleted:
		userID := *tmp.userdeleted_userID
		return fmt.Sprintf("User #%d data archived", userID) // dingo:M:1
	case EventTagOrderPlaced:
		orderID := *tmp.orderplaced_orderID
		amount := *tmp.orderplaced_amount
		userID := *tmp.orderplaced_userID
		if amount > 1000 {
			return fmt.Sprintf("HIGH VALUE: Order %s ($%.2f) flagged for review", orderID, amount) // dingo:M:1
		} else {
			return fmt.Sprintf("Order %s confirmed for user #%d", orderID, userID) // dingo:M:1
		}
	case EventTagOrderShipped:
		orderID := *tmp.ordershipped_orderID
		trackingNumber := *tmp.ordershipped_trackingNumber
		return fmt.Sprintf("Order %s shipped: %s", orderID, trackingNumber) // dingo:M:1
	case EventTagPaymentReceived:
		orderID := *tmp.paymentreceived_orderID
		amount := *tmp.paymentreceived_amount
		return fmt.Sprintf("Payment $%.2f received for order %s", amount, orderID) // dingo:M:1
	case EventTagPaymentFailed:
		orderID := *tmp.paymentfailed_orderID
		reason := *tmp.paymentfailed_reason
		return fmt.Sprintf("ALERT: Payment failed for %s - %s", orderID, reason) // dingo:M:1
	}
	panic("non-exhaustive match")

}

// GetEventPriority uses guards for complex conditions
func GetEventPriority(event Event) int {
	tmp := event
	switch tmp.tag {
	case EventTagPaymentFailed:
		return 1 // dingo:M:1
	case EventTagOrderPlaced:
		amount := *tmp.orderplaced_amount
		if amount > 500 {
			return 2 // dingo:M:1
		}
	case EventTagUserCreated:
		return 3 // dingo:M:1
	default:
		return 4 // dingo:M:1
	}
	panic("non-exhaustive match")

}

// FilterUserEvents extracts only user-related events
func FilterUserEvents(events []Event) []Event {
	var userEvents []Event
	for _, event := range events {
		isUserEvent = func() bool {
			tmp := event
			switch tmp.tag {
			case EventTagUserCreated:
				return true // dingo:M:1
			case EventTagUserDeleted:
				return true // dingo:M:1
			default:
				return false // dingo:M:1
			}
		}()
		if isUserEvent {
			userEvents = append(userEvents, event)
		}
	}
	return userEvents
}
func main() {
	events := []Event{
		Event.UserCreated(1, "alice@example.com"),
		Event.OrderPlaced("ORD-001", 1500.00, 1),
		Event.PaymentReceived("ORD-001", 1500.00),
		Event.OrderShipped("ORD-001", "TRK-12345"),
		Event.PaymentFailed("ORD-002", "insufficient funds"),
	}

	fmt.Println("=== Processing Events ===")
	for _, event := range events {
		result := ProcessEvent(event)
		priority := GetEventPriority(event)
		fmt.Printf("[P%d] %s\n", priority, result)
	}

	fmt.Println("\n=== User Events Only ===")
	for _, event := range FilterUserEvents(events) {
		fmt.Println(ProcessEvent(event))
	}
}
