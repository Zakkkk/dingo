// Real-world example: User service with Result types and implicit wrapping
// Shows how Result[T, E] enables clean, explicit error handling
package main

import (
	"database/sql"
	"fmt"
)


type ResultTag uint8
const (
	ResultTagOk ResultTag = iota
	ResultTagErr
)
type ResultUserServiceError struct {
	tag ResultTag
	ok  *User
	err *ServiceError
}
func ResultUserServiceErrorOk(arg0 User) ResultUserServiceError {
	return ResultUserServiceError{tag: ResultTagOk, ok: &arg0}
}
func ResultUserServiceErrorErr(arg0 ServiceError) ResultUserServiceError {
	return ResultUserServiceError{tag: ResultTagErr, err: &arg0}
}
func (r ResultUserServiceError) IsOk() bool {
	return r.tag == ResultTagOk
}
func (r ResultUserServiceError) IsErr() bool {
	return r.tag == ResultTagErr
}
func (r ResultUserServiceError) Unwrap() User {
	if r.tag != ResultTagOk {
		panic("called Unwrap on Err")
	}
	if r.ok == nil {
		panic("Result contains nil Ok value")
	}
	return *r.ok
}
func (r ResultUserServiceError) UnwrapOr(defaultValue User) User {
	if r.tag == ResultTagOk {
		return *r.ok
	}
	return defaultValue
}
func (r ResultUserServiceError) UnwrapErr() ServiceError {
	if r.tag != ResultTagErr {
		panic("called UnwrapErr on Ok")
	}
	if r.err == nil {
		panic("Result contains nil Err value")
	}
	return *r.err
}
func (r ResultUserServiceError) UnwrapOrElse(fn func(ServiceError) User) User {
	if r.tag == ResultTagOk && r.ok != nil {
		return *r.ok
	}
	if r.err != nil {
		return fn(*r.err)
	}
	panic("Result in invalid state")
}
func (r ResultUserServiceError) Map(fn func(User) interface{}) interface{} {
	if r.tag == ResultTagOk && r.ok != nil {
		u := fn(*r.ok)
		return struct {
			tag ResultTag
			ok  *interface{}
			err *ServiceError
		}{tag: ResultTagOk, ok: &u}
	}
	return struct {
		tag ResultTag
		ok  *interface{}
		err *ServiceError
	}{tag: r.tag, ok: nil, err: r.err}
}
func (r ResultUserServiceError) MapErr(fn func(ServiceError) interface{}) interface{} {
	if r.tag == ResultTagErr && r.err != nil {
		f := fn(*r.err)
		return struct {
			tag ResultTag
			ok  *User
			err *interface{}
		}{tag: ResultTagErr, ok: nil, err: &f}
	}
	return struct {
		tag ResultTag
		ok  *User
		err *interface{}
	}{tag: r.tag, ok: r.ok, err: nil}
}
func (r ResultUserServiceError) Filter(predicate func(User) bool) ResultUserServiceError {
	if r.tag == ResultTagOk && predicate(*r.ok) {
		return r
	}
	return r
}
func (r ResultUserServiceError) AndThen(fn func(User) interface{}) interface{} {
	if r.tag == ResultTagOk && r.ok != nil {
		return fn(*r.ok)
	}
	return struct {
		tag ResultTag
		ok  *interface{}
		err *ServiceError
	}{tag: r.tag, ok: nil, err: r.err}
}
func (r ResultUserServiceError) OrElse(fn func(ServiceError) interface{}) interface{} {
	if r.tag == ResultTagErr && r.err != nil {
		return fn(*r.err)
	}
	return struct {
		tag ResultTag
		ok  *User
		err *interface{}
	}{tag: r.tag, ok: r.ok, err: nil}
}
func (r ResultUserServiceError) And(other interface{}) interface{} {
	if r.tag == ResultTagOk {
		return other
	}
	return r
}
func (r ResultUserServiceError) Or(other ResultUserServiceError) ResultUserServiceError {
	if r.tag == ResultTagOk {
		return r
	}
	return other
}
type ResultSliceOrderServiceError struct {
	tag ResultTag
	ok  *[]Order
	err *ServiceError
}
func ResultSliceOrderServiceErrorOk(arg0 []Order) ResultSliceOrderServiceError {
	return ResultSliceOrderServiceError{tag: ResultTagOk, ok: &arg0}
}
func ResultSliceOrderServiceErrorErr(arg0 ServiceError) ResultSliceOrderServiceError {
	return ResultSliceOrderServiceError{tag: ResultTagErr, err: &arg0}
}
func (r ResultSliceOrderServiceError) IsOk() bool {
	return r.tag == ResultTagOk
}
func (r ResultSliceOrderServiceError) IsErr() bool {
	return r.tag == ResultTagErr
}
func (r ResultSliceOrderServiceError) Unwrap() []Order {
	if r.tag != ResultTagOk {
		panic("called Unwrap on Err")
	}
	if r.ok == nil {
		panic("Result contains nil Ok value")
	}
	return *r.ok
}
func (r ResultSliceOrderServiceError) UnwrapOr(defaultValue []Order) []Order {
	if r.tag == ResultTagOk {
		return *r.ok
	}
	return defaultValue
}
func (r ResultSliceOrderServiceError) UnwrapErr() ServiceError {
	if r.tag != ResultTagErr {
		panic("called UnwrapErr on Ok")
	}
	if r.err == nil {
		panic("Result contains nil Err value")
	}
	return *r.err
}
func (r ResultSliceOrderServiceError) UnwrapOrElse(fn func(ServiceError) []Order) []Order {
	if r.tag == ResultTagOk && r.ok != nil {
		return *r.ok
	}
	if r.err != nil {
		return fn(*r.err)
	}
	panic("Result in invalid state")
}
func (r ResultSliceOrderServiceError) Map(fn func([]Order) interface{}) interface{} {
	if r.tag == ResultTagOk && r.ok != nil {
		u := fn(*r.ok)
		return struct {
			tag ResultTag
			ok  *interface{}
			err *ServiceError
		}{tag: ResultTagOk, ok: &u}
	}
	return struct {
		tag ResultTag
		ok  *interface{}
		err *ServiceError
	}{tag: r.tag, ok: nil, err: r.err}
}
func (r ResultSliceOrderServiceError) MapErr(fn func(ServiceError) interface{}) interface{} {
	if r.tag == ResultTagErr && r.err != nil {
		f := fn(*r.err)
		return struct {
			tag ResultTag
			ok  *[]Order
			err *interface{}
		}{tag: ResultTagErr, ok: nil, err: &f}
	}
	return struct {
		tag ResultTag
		ok  *[]Order
		err *interface{}
	}{tag: r.tag, ok: r.ok, err: nil}
}
func (r ResultSliceOrderServiceError) Filter(predicate func([]Order) bool) ResultSliceOrderServiceError {
	if r.tag == ResultTagOk && predicate(*r.ok) {
		return r
	}
	return r
}
func (r ResultSliceOrderServiceError) AndThen(fn func([]Order) interface{}) interface{} {
	if r.tag == ResultTagOk && r.ok != nil {
		return fn(*r.ok)
	}
	return struct {
		tag ResultTag
		ok  *interface{}
		err *ServiceError
	}{tag: r.tag, ok: nil, err: r.err}
}
func (r ResultSliceOrderServiceError) OrElse(fn func(ServiceError) interface{}) interface{} {
	if r.tag == ResultTagErr && r.err != nil {
		return fn(*r.err)
	}
	return struct {
		tag ResultTag
		ok  *[]Order
		err *interface{}
	}{tag: r.tag, ok: r.ok, err: nil}
}
func (r ResultSliceOrderServiceError) And(other interface{}) interface{} {
	if r.tag == ResultTagOk {
		return other
	}
	return r
}
func (r ResultSliceOrderServiceError) Or(other ResultSliceOrderServiceError) ResultSliceOrderServiceError {
	if r.tag == ResultTagOk {
		return r
	}
	return other
}
type ResultFloat64ServiceError struct {
	tag ResultTag
	ok  *float64
	err *ServiceError
}
func ResultFloat64ServiceErrorOk(arg0 float64) ResultFloat64ServiceError {
	return ResultFloat64ServiceError{tag: ResultTagOk, ok: &arg0}
}
func ResultFloat64ServiceErrorErr(arg0 ServiceError) ResultFloat64ServiceError {
	return ResultFloat64ServiceError{tag: ResultTagErr, err: &arg0}
}
func (r ResultFloat64ServiceError) IsOk() bool {
	return r.tag == ResultTagOk
}
func (r ResultFloat64ServiceError) IsErr() bool {
	return r.tag == ResultTagErr
}
func (r ResultFloat64ServiceError) Unwrap() float64 {
	if r.tag != ResultTagOk {
		panic("called Unwrap on Err")
	}
	if r.ok == nil {
		panic("Result contains nil Ok value")
	}
	return *r.ok
}
func (r ResultFloat64ServiceError) UnwrapOr(defaultValue float64) float64 {
	if r.tag == ResultTagOk {
		return *r.ok
	}
	return defaultValue
}
func (r ResultFloat64ServiceError) UnwrapErr() ServiceError {
	if r.tag != ResultTagErr {
		panic("called UnwrapErr on Ok")
	}
	if r.err == nil {
		panic("Result contains nil Err value")
	}
	return *r.err
}
func (r ResultFloat64ServiceError) UnwrapOrElse(fn func(ServiceError) float64) float64 {
	if r.tag == ResultTagOk && r.ok != nil {
		return *r.ok
	}
	if r.err != nil {
		return fn(*r.err)
	}
	panic("Result in invalid state")
}
func (r ResultFloat64ServiceError) Map(fn func(float64) interface{}) interface{} {
	if r.tag == ResultTagOk && r.ok != nil {
		u := fn(*r.ok)
		return struct {
			tag ResultTag
			ok  *interface{}
			err *ServiceError
		}{tag: ResultTagOk, ok: &u}
	}
	return struct {
		tag ResultTag
		ok  *interface{}
		err *ServiceError
	}{tag: r.tag, ok: nil, err: r.err}
}
func (r ResultFloat64ServiceError) MapErr(fn func(ServiceError) interface{}) interface{} {
	if r.tag == ResultTagErr && r.err != nil {
		f := fn(*r.err)
		return struct {
			tag ResultTag
			ok  *float64
			err *interface{}
		}{tag: ResultTagErr, ok: nil, err: &f}
	}
	return struct {
		tag ResultTag
		ok  *float64
		err *interface{}
	}{tag: r.tag, ok: r.ok, err: nil}
}
func (r ResultFloat64ServiceError) Filter(predicate func(float64) bool) ResultFloat64ServiceError {
	if r.tag == ResultTagOk && predicate(*r.ok) {
		return r
	}
	return r
}
func (r ResultFloat64ServiceError) AndThen(fn func(float64) interface{}) interface{} {
	if r.tag == ResultTagOk && r.ok != nil {
		return fn(*r.ok)
	}
	return struct {
		tag ResultTag
		ok  *interface{}
		err *ServiceError
	}{tag: r.tag, ok: nil, err: r.err}
}
func (r ResultFloat64ServiceError) OrElse(fn func(ServiceError) interface{}) interface{} {
	if r.tag == ResultTagErr && r.err != nil {
		return fn(*r.err)
	}
	return struct {
		tag ResultTag
		ok  *float64
		err *interface{}
	}{tag: r.tag, ok: r.ok, err: nil}
}
func (r ResultFloat64ServiceError) And(other interface{}) interface{} {
	if r.tag == ResultTagOk {
		return other
	}
	return r
}
func (r ResultFloat64ServiceError) Or(other ResultFloat64ServiceError) ResultFloat64ServiceError {
	if r.tag == ResultTagOk {
		return r
	}
	return other
}
type ResultBoolServiceError struct {
	tag ResultTag
	ok  *bool
	err *ServiceError
}
func ResultBoolServiceErrorOk(arg0 bool) ResultBoolServiceError {
	return ResultBoolServiceError{tag: ResultTagOk, ok: &arg0}
}
func ResultBoolServiceErrorErr(arg0 ServiceError) ResultBoolServiceError {
	return ResultBoolServiceError{tag: ResultTagErr, err: &arg0}
}
func (r ResultBoolServiceError) IsOk() bool {
	return r.tag == ResultTagOk
}
func (r ResultBoolServiceError) IsErr() bool {
	return r.tag == ResultTagErr
}
func (r ResultBoolServiceError) Unwrap() bool {
	if r.tag != ResultTagOk {
		panic("called Unwrap on Err")
	}
	if r.ok == nil {
		panic("Result contains nil Ok value")
	}
	return *r.ok
}
func (r ResultBoolServiceError) UnwrapOr(defaultValue bool) bool {
	if r.tag == ResultTagOk {
		return *r.ok
	}
	return defaultValue
}
func (r ResultBoolServiceError) UnwrapErr() ServiceError {
	if r.tag != ResultTagErr {
		panic("called UnwrapErr on Ok")
	}
	if r.err == nil {
		panic("Result contains nil Err value")
	}
	return *r.err
}
func (r ResultBoolServiceError) UnwrapOrElse(fn func(ServiceError) bool) bool {
	if r.tag == ResultTagOk && r.ok != nil {
		return *r.ok
	}
	if r.err != nil {
		return fn(*r.err)
	}
	panic("Result in invalid state")
}
func (r ResultBoolServiceError) Map(fn func(bool) interface{}) interface{} {
	if r.tag == ResultTagOk && r.ok != nil {
		u := fn(*r.ok)
		return struct {
			tag ResultTag
			ok  *interface{}
			err *ServiceError
		}{tag: ResultTagOk, ok: &u}
	}
	return struct {
		tag ResultTag
		ok  *interface{}
		err *ServiceError
	}{tag: r.tag, ok: nil, err: r.err}
}
func (r ResultBoolServiceError) MapErr(fn func(ServiceError) interface{}) interface{} {
	if r.tag == ResultTagErr && r.err != nil {
		f := fn(*r.err)
		return struct {
			tag ResultTag
			ok  *bool
			err *interface{}
		}{tag: ResultTagErr, ok: nil, err: &f}
	}
	return struct {
		tag ResultTag
		ok  *bool
		err *interface{}
	}{tag: r.tag, ok: r.ok, err: nil}
}
func (r ResultBoolServiceError) Filter(predicate func(bool) bool) ResultBoolServiceError {
	if r.tag == ResultTagOk && predicate(*r.ok) {
		return r
	}
	return r
}
func (r ResultBoolServiceError) AndThen(fn func(bool) interface{}) interface{} {
	if r.tag == ResultTagOk && r.ok != nil {
		return fn(*r.ok)
	}
	return struct {
		tag ResultTag
		ok  *interface{}
		err *ServiceError
	}{tag: r.tag, ok: nil, err: r.err}
}
func (r ResultBoolServiceError) OrElse(fn func(ServiceError) interface{}) interface{} {
	if r.tag == ResultTagErr && r.err != nil {
		return fn(*r.err)
	}
	return struct {
		tag ResultTag
		ok  *bool
		err *interface{}
	}{tag: r.tag, ok: r.ok, err: nil}
}
func (r ResultBoolServiceError) And(other interface{}) interface{} {
	if r.tag == ResultTagOk {
		return other
	}
	return r
}
func (r ResultBoolServiceError) Or(other ResultBoolServiceError) ResultBoolServiceError {
	if r.tag == ResultTagOk {
		return r
	}
	return other
}

// Domain types
type User struct {
	ID    int
	Name  string
	Email string
}

type Order struct {
	ID     int
	UserID int
	Total  float64
}

// Custom error type with error codes
type ServiceError struct {
	Code    string
	Message string
}
func (e ServiceError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Repository functions returning Result types
// Implicit wrapping: just return the value or error directly
func FindUser(db *sql.DB, id int) ResultUserServiceError {
	row := db.QueryRow("SELECT id, name, email FROM users WHERE id = ?", id)

	var user User
	err := row.Scan(&user.ID, &user.Name, &user.Email)
	if err == sql.ErrNoRows {
		return ResultUserServiceErrorErr(ServiceError{Code: "NOT_FOUND", Message: "user not found"})
	}
	if err != nil {
		return ResultUserServiceErrorErr(ServiceError{Code: "DB_ERROR", Message: err.Error()})
	}

	return ResultUserServiceErrorOk(user)
}
func FindOrdersByUser(db *sql.DB, userID int) ResultSliceOrderServiceError {
	tmp, err := db.Query("SELECT id, user_id, total FROM orders WHERE user_id = ?", userID)

	if err != nil {
		return ResultSliceOrderServiceErrorErr(ServiceError{Code: "DB_ERROR", Message: err.Error()})
	}
	var rows = tmp
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		// Note: Scan returns only error, not (T, error), so we use explicit error handling
		if err := rows.Scan(&order.ID, &order.UserID, &order.Total); err != nil {
			return ResultSliceOrderServiceErrorErr(ServiceError{Code: "SCAN_ERROR", Message: err.Error()})
		}
		orders = append(orders, order)
	}
	return ResultSliceOrderServiceErrorOk(orders)
}

// Service function chaining multiple Result-returning functions
// Error propagation pattern: check IsErr, return error, continue with value
func GetUserOrderTotal(db *sql.DB, userID int) ResultFloat64ServiceError {
	// Get user - propagate error if failed
	userResult := FindUser(db, userID)
	if userResult.IsErr() {
		return ResultFloat64ServiceErrorErr(userResult.UnwrapErr()) // Implicit wrapping to Result[float64, ServiceError]
	}
	user := userResult.Unwrap()

	// Get orders - propagate error if failed
	ordersResult := FindOrdersByUser(db, user.ID)
	if ordersResult.IsErr() {
		return ResultFloat64ServiceErrorErr(ordersResult.UnwrapErr())
	}
	orders := ordersResult.Unwrap()

	// Calculate total
	var total float64
	for _, order := range orders {
		total += order.Total
	}

	return ResultFloat64ServiceErrorOk(total) // Implicit wrapping to Ok
}

// Another example: transferring funds between users
// Uses the explicit IsErr/UnwrapErr pattern for Result error propagation
func TransferFunds(db *sql.DB, fromID int, toID int, amount float64) ResultBoolServiceError {
	// Find source user
	fromResult := FindUser(db, fromID)
	if fromResult.IsErr() {
		return ResultBoolServiceErrorErr(fromResult.UnwrapErr())
	}
	fromUser := fromResult.Unwrap()

	// Find destination user
	toResult := FindUser(db, toID)
	if toResult.IsErr() {
		return ResultBoolServiceErrorErr(toResult.UnwrapErr())
	}
	toUser := toResult.Unwrap()

	fmt.Printf("Transferring $%.2f from %s to %s\n",
		amount, fromUser.Name, toUser.Name)

	return ResultBoolServiceErrorOk(true)
}
func main() {
	var db *sql.DB // Would be initialized in real code

	// Using the service with proper error handling
	result := GetUserOrderTotal(db, 123)

	if result.IsOk() {
		total := result.Unwrap()
		fmt.Printf("Total orders: $%.2f\n", total)
	} else {
		err := result.UnwrapErr()
		fmt.Printf("Error [%s]: %s\n", err.Code, err.Message)
	}
}
