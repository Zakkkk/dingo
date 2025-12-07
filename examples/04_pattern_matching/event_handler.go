// Real-world example: Event handler with pattern matching
// Pattern matching makes complex conditionals readable and exhaustive
package main

import (
	"fmt"
)

// Event sum type - all possible events in the system
type Event interface{ isEvent() }

type EventUserCreated struct {
	userID int
	email  string
}

func (EventUserCreated) isEvent() {}
func NewEventUserCreated(userID int, email string) Event {
	return EventUserCreated{userID: userID, email: email}
}

type EventUserDeleted struct{ userID int }

func (EventUserDeleted) isEvent()          {}
func NewEventUserDeleted(userID int) Event { return EventUserDeleted{userID: userID} }

type EventOrderPlaced struct {
	orderID string
	amount  float64
	userID  int
}

func (EventOrderPlaced) isEvent() {}
func NewEventOrderPlaced(orderID string, amount float64, userID int) Event {
	return EventOrderPlaced{orderID: orderID, amount: amount, userID: userID}
}

type EventOrderShipped struct {
	orderID        string
	trackingNumber string
}

func (EventOrderShipped) isEvent() {}
func NewEventOrderShipped(orderID string, trackingNumber string) Event {
	return EventOrderShipped{orderID: orderID, trackingNumber: trackingNumber}
}

type EventPaymentReceived struct {
	orderID string
	amount  float64
}

func (EventPaymentReceived) isEvent() {}
func NewEventPaymentReceived(orderID string, amount float64) Event {
	return EventPaymentReceived{orderID: orderID, amount: amount}
}

type EventPaymentFailed struct {
	orderID string
	reason  string
}

func (EventPaymentFailed) isEvent() {}
func NewEventPaymentFailed(orderID string, reason string) Event {
	return EventPaymentFailed{orderID: orderID, reason: reason}
}

// ProcessEvent handles all event types with exhaustive matching
// The compiler ensures every Event variant is handled
func ProcessEvent(event Event) string {
	val3 := event
	switch v4 := val3.(type) {
	case EventUserCreated:
		userID := v4.userID
		email := v4.email
		return fmt.Sprintf("Welcome email sent to %s (user #%d)", email, userID)
	case EventUserDeleted:
		userID := v4.userID
		return fmt.Sprintf("User #%d data archived", userID)
	case EventOrderPlaced:
		orderID := v4.orderID
		amount := v4.amount
		userID := v4.userID
		if amount > 1000 {
			return fmt.Sprintf("HIGH VALUE: Order %s ($%.2f) flagged for review", orderID, amount)
		} else {
			return fmt.Sprintf("Order %s confirmed for user #%d", orderID, userID)
		}
	case EventOrderShipped:
		orderID := v4.orderID
		trackingNumber := v4.trackingNumber
		return fmt.Sprintf("Order %s shipped: %s", orderID, trackingNumber)
	case EventPaymentReceived:
		orderID := v4.orderID
		amount := v4.amount
		return fmt.Sprintf("Payment $%.2f received for order %s", amount, orderID)
	case EventPaymentFailed:
		orderID := v4.orderID
		reason := v4.reason
		return fmt.Sprintf("ALERT: Payment failed for %s - %s", orderID, reason)
	}
	panic("unreachable: exhaustive match")
}

// GetEventPriority uses guards for complex conditions
func GetEventPriority(event Event) int {
	val1 := event
	switch v2 := val1.(type) {
	case EventPaymentFailed:
		return 1
	case EventOrderPlaced:
		amount := v2.amount
		if amount > 500 {
			return 2
		}
	case EventUserCreated:
		return 3
	default:
		return 4
	}
	panic("unreachable: exhaustive match")
}

// FilterUserEvents extracts only user-related events
func FilterUserEvents(events []Event) []Event {
	var userEvents []Event
	for _, event := range events {
		var isUserEvent bool
		val := event
		switch val.(type) {
		case EventUserCreated:
			isUserEvent = true
		case EventUserDeleted:
			isUserEvent = true
		default:
			isUserEvent = false
		}
		if isUserEvent {
			userEvents = append(userEvents, event)
		}
	}
	return userEvents
}

func main() {
	events := []Event{
		NewEventUserCreated(1, "alice@example.com"),
		NewEventOrderPlaced("ORD-001", 1500.00, 1),
		NewEventPaymentReceived("ORD-001", 1500.00),
		NewEventOrderShipped("ORD-001", "TRK-12345"),
		NewEventPaymentFailed("ORD-002", "insufficient funds"),
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
