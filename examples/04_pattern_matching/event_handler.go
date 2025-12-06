// Real-world example: Event handler with pattern matching
// Pattern matching makes complex conditionals readable and exhaustive
package main

import (
	"fmt"
)

// Event sum type - all possible events in the system
type Event interface {
	isEvent()
	IsUserCreated() bool
	IsUserDeleted() bool
	IsOrderPlaced() bool
	IsOrderShipped() bool
	IsPaymentReceived() bool
	IsPaymentFailed() bool
}

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

func IsEventUserCreated(v Event) bool                { _, ok := v.(EventUserCreated); return ok }
func IsEventUserDeleted(v Event) bool                { _, ok := v.(EventUserDeleted); return ok }
func IsEventOrderPlaced(v Event) bool                { _, ok := v.(EventOrderPlaced); return ok }
func IsEventOrderShipped(v Event) bool               { _, ok := v.(EventOrderShipped); return ok }
func IsEventPaymentReceived(v Event) bool            { _, ok := v.(EventPaymentReceived); return ok }
func IsEventPaymentFailed(v Event) bool              { _, ok := v.(EventPaymentFailed); return ok }
func (EventUserCreated) IsUserCreated() bool         { return true }
func (EventUserCreated) IsUserDeleted() bool         { return false }
func (EventUserCreated) IsOrderPlaced() bool         { return false }
func (EventUserCreated) IsOrderShipped() bool        { return false }
func (EventUserCreated) IsPaymentReceived() bool     { return false }
func (EventUserCreated) IsPaymentFailed() bool       { return false }
func (EventUserDeleted) IsUserCreated() bool         { return false }
func (EventUserDeleted) IsUserDeleted() bool         { return true }
func (EventUserDeleted) IsOrderPlaced() bool         { return false }
func (EventUserDeleted) IsOrderShipped() bool        { return false }
func (EventUserDeleted) IsPaymentReceived() bool     { return false }
func (EventUserDeleted) IsPaymentFailed() bool       { return false }
func (EventOrderPlaced) IsUserCreated() bool         { return false }
func (EventOrderPlaced) IsUserDeleted() bool         { return false }
func (EventOrderPlaced) IsOrderPlaced() bool         { return true }
func (EventOrderPlaced) IsOrderShipped() bool        { return false }
func (EventOrderPlaced) IsPaymentReceived() bool     { return false }
func (EventOrderPlaced) IsPaymentFailed() bool       { return false }
func (EventOrderShipped) IsUserCreated() bool        { return false }
func (EventOrderShipped) IsUserDeleted() bool        { return false }
func (EventOrderShipped) IsOrderPlaced() bool        { return false }
func (EventOrderShipped) IsOrderShipped() bool       { return true }
func (EventOrderShipped) IsPaymentReceived() bool    { return false }
func (EventOrderShipped) IsPaymentFailed() bool      { return false }
func (EventPaymentReceived) IsUserCreated() bool     { return false }
func (EventPaymentReceived) IsUserDeleted() bool     { return false }
func (EventPaymentReceived) IsOrderPlaced() bool     { return false }
func (EventPaymentReceived) IsOrderShipped() bool    { return false }
func (EventPaymentReceived) IsPaymentReceived() bool { return true }
func (EventPaymentReceived) IsPaymentFailed() bool   { return false }
func (EventPaymentFailed) IsUserCreated() bool       { return false }
func (EventPaymentFailed) IsUserDeleted() bool       { return false }
func (EventPaymentFailed) IsOrderPlaced() bool       { return false }
func (EventPaymentFailed) IsOrderShipped() bool      { return false }
func (EventPaymentFailed) IsPaymentReceived() bool   { return false }
func (EventPaymentFailed) IsPaymentFailed() bool     { return true }

// ProcessEvent handles all event types with exhaustive matching
// The compiler ensures every Event variant is handled
func ProcessEvent(event Event) string {
	return func() string {
		switch v := event.(type) {
		case UserCreated:
			userID := v.Value
			email := v.Value
			return fmt.Sprintf("Welcome email sent to %s (user #%d)", email, userID)
		case UserDeleted:
			userID := v.Value
			return fmt.Sprintf("User #%d data archived", userID)
		case OrderPlaced:
			orderID := v.Value
			amount := v.Value
			userID := v.Value
			if amount > 1000 {
				return fmt.Sprintf("HIGH VALUE: Order %s ($%.2f) flagged for review", orderID, amount)
			}
		case OrderPlaced:
			orderID := v.Value
			amount := v.Value
			userID := v.Value
			return fmt.Sprintf("Order %s confirmed for user #%d", orderID, userID)
		case OrderShipped:
			orderID := v.Value
			trackingNumber := v.Value
			return fmt.Sprintf("Order %s shipped: %s", orderID, trackingNumber)
		case PaymentReceived:
			orderID := v.Value
			amount := v.Value
			return fmt.Sprintf("Payment $%.2f received for order %s", amount, orderID)
		case PaymentFailed:
			orderID := v.Value
			reason := v.Value
			return fmt.Sprintf("ALERT: Payment failed for %s - %s", orderID, reason)
		}
	}()
}

// GetEventPriority uses guards for complex conditions
func GetEventPriority(event Event) int {
	return func() int {
		switch v := event.(type) {
		case PaymentFailed:
			return 1
		case OrderPlaced:
			amount := v.Value
			if amount > 500 {
				return 2
			}
		case UserCreated:
			return 3
		default:
			return 4
		}
	}()
}

// FilterUserEvents extracts only user-related events
func FilterUserEvents(events []Event) []Event {
	var userEvents []Event
	for _, event := range events {
		isUserEvent := func() bool {
			switch v := event.(type) {
			case UserCreated:
				return true
			case UserDeleted:
				return true
			default:
				return false
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
		EventUserCreated(1, "alice@example.com"),
		EventOrderPlaced("ORD-001", 1500.00, 1),
		EventPaymentReceived("ORD-001", 1500.00),
		EventOrderShipped("ORD-001", "TRK-12345"),
		EventPaymentFailed("ORD-002", "insufficient funds"),
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
