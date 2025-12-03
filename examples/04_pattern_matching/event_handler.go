// Generated Go code from event_handler.dingo
// Pattern matching becomes type switches with struct field extraction
package main

import (
	"fmt"
)

// Event is the interface for the sum type
type Event interface {
	isEvent()
}

// Event variants as structs
type UserCreated struct {
	UserID int
	Email  string
}

func (UserCreated) isEvent() {}

type UserDeleted struct {
	UserID int
}

func (UserDeleted) isEvent() {}

type OrderPlaced struct {
	OrderID string
	Amount  float64
	UserID  int
}

func (OrderPlaced) isEvent() {}

type OrderShipped struct {
	OrderID        string
	TrackingNumber string
}

func (OrderShipped) isEvent() {}

type PaymentReceived struct {
	OrderID string
	Amount  float64
}

func (PaymentReceived) isEvent() {}

type PaymentFailed struct {
	OrderID string
	Reason  string
}

func (PaymentFailed) isEvent() {}

// ProcessEvent handles all event types with exhaustive matching
// The compiler ensures every Event variant is handled
func ProcessEvent(event Event) string {
	switch e := event.(type) {
	case UserCreated:
		userID := e.UserID
		email := e.Email
		return fmt.Sprintf("Welcome email sent to %s (user #%d)", email, userID)

	case UserDeleted:
		userID := e.UserID
		return fmt.Sprintf("User #%d data archived", userID)

	case OrderPlaced:
		orderID := e.OrderID
		amount := e.Amount
		userID := e.UserID
		if amount > 1000 {
			return fmt.Sprintf("HIGH VALUE: Order %s ($%.2f) flagged for review", orderID, amount)
		}
		return fmt.Sprintf("Order %s confirmed for user #%d", orderID, userID)

	case OrderShipped:
		orderID := e.OrderID
		trackingNumber := e.TrackingNumber
		return fmt.Sprintf("Order %s shipped: %s", orderID, trackingNumber)

	case PaymentReceived:
		orderID := e.OrderID
		amount := e.Amount
		return fmt.Sprintf("Payment $%.2f received for order %s", amount, orderID)

	case PaymentFailed:
		orderID := e.OrderID
		reason := e.Reason
		return fmt.Sprintf("ALERT: Payment failed for %s - %s", orderID, reason)

	default:
		panic("non-exhaustive match")
	}
}

// GetEventPriority uses guards for complex conditions
func GetEventPriority(event Event) int {
	switch e := event.(type) {
	case PaymentFailed:
		return 1 // Highest priority
	case OrderPlaced:
		if e.Amount > 500 {
			return 2
		}
		return 4
	case UserCreated:
		return 3
	default:
		return 4 // Everything else
	}
}

// FilterUserEvents extracts only user-related events
func FilterUserEvents(events []Event) []Event {
	var userEvents []Event
	for _, event := range events {
		switch event.(type) {
		case UserCreated:
			userEvents = append(userEvents, event)
		case UserDeleted:
			userEvents = append(userEvents, event)
		default:
			// Ignore other events
		}
	}
	return userEvents
}

func main() {
	events := []Event{
		UserCreated{UserID: 1, Email: "alice@example.com"},
		OrderPlaced{OrderID: "ORD-001", Amount: 1500.00, UserID: 1},
		PaymentReceived{OrderID: "ORD-001", Amount: 1500.00},
		OrderShipped{OrderID: "ORD-001", TrackingNumber: "TRK-12345"},
		PaymentFailed{OrderID: "ORD-002", Reason: "insufficient funds"},
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
