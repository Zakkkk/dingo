// Real-world example: Database repository with Result types
// Result[T, E] makes success/failure states explicit and type-safe
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

type ResultUserDBError struct {
	tag ResultTag
	ok  *User
	err *DBError
}

func ResultUserDBErrorOk(arg0 User) ResultUserDBError {
	return ResultUserDBError{tag: ResultTagOk,

	// FindUserByID returns a Result that explicitly models success or failure
	// Caller must handle both cases - no silent nil pointer bugs
	ok: &arg0}
}
func ResultUserDBErrorErr(arg0 DBError) ResultUserDBError {
	return ResultUserDBError{tag: ResultTagErr, err: &arg0}
}
func (r ResultUserDBError) IsOk() bool {
	return r.tag == ResultTagOk
}
func (r ResultUserDBError) IsErr() bool {
	return r.tag == ResultTagErr
}
func (r ResultUserDBError) Unwrap() User {
	if r.tag != ResultTagOk {
		panic("called Unwrap on Err")
	}
	if r.ok == nil {
		panic("Result contains nil Ok value")

		// Use generated constructor: ResultUserDBErrorErr(error)
	}
	return *r.ok
}
func (r ResultUserDBError) UnwrapOr(defaultValue User) User {
	if r.tag == ResultTagOk {
		return *r.ok
	}
	return defaultValue
}
func (r ResultUserDBError) UnwrapErr() DBError {
	if r.tag != ResultTagErr {
		panic("called UnwrapErr on Ok")
	}
	if r.err == nil {
		panic(

		// Use generated constructor: ResultUserDBErrorOk(value)
		"Result contains nil Err value")
	}
	return *r.err
}
func (r ResultUserDBError) UnwrapOrElse(fn func(

// TransferFunds shows Result chaining - each step must succeed
DBError) User) User {
	if r.tag == ResultTagOk && r.ok != nil {
		return *r.ok
	}
	if r.err != nil {
		return fn(*r.err)
	}
	panic("Result in invalid state")
}
func (r ResultUserDBError) Map(fn func

// Find source user
(User) interface{}) interface{} {
	if r.tag == ResultTagOk && r.ok != nil {
		u := fn(*r.ok)
		return struct {
			tag ResultTag
			ok  *interface{}
			err *DBError
		}{tag: ResultTagOk, ok:

		// Find destination user
		&u}
	}
	return struct {
		tag ResultTag
		ok  *interface{}
		err *DBError
	}{tag: r.tag, ok: nil, err: r.err}
}
func (r ResultUserDBError) MapErr(fn func(DBError) interface{}) interface{} {

	// In real code: begin transaction, update balances, commit
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
func

// Example usage showing explicit error handling
(r ResultUserDBError) Filter(predicate func(User) bool) ResultUserDBError {

	// Would be initialized in real code
	if r.tag == ResultTagOk && predicate(*r.ok) {
		return r
	}
	return r
}
func (r ResultUserDBError) AndThen(

// Pattern: Must explicitly check and handle both cases
fn func(User) interface{}) interface{} {
	if r.tag == ResultTagOk && r.ok != nil {
		return fn(*r.ok)
	}
	return struct {
		tag ResultTag
		ok  *interface{}
		err *DBError
	}{tag: r.tag, ok: nil, err: r.err}
}
func (r ResultUserDBError) OrElse(fn func(DBError) interface{}) interface{} {
	if r.tag == ResultTagErr && r.err != nil {
		return fn(*r.err)
	}
	return struct {
		tag ResultTag
		ok  *User
		err *interface{}
	}{tag: r.tag, ok: r.ok, err: nil}
}
func (r ResultUserDBError) And(other interface{}) interface{} {
	if r.tag == ResultTagOk {
		return other
	}
	return r
}
func (r ResultUserDBError) Or(other ResultUserDBError) ResultUserDBError {
	if r.tag == ResultTagOk {
		return r
	}
	return other
}

type ResultBoolDBError struct {
	tag ResultTag
	ok  *bool
	err *DBError
}
func ResultBoolDBErrorOk(arg0 bool) ResultBoolDBError {
	return ResultBoolDBError{tag: ResultTagOk, ok: &arg0}
}
func ResultBoolDBErrorErr(arg0 DBError) ResultBoolDBError {
	return ResultBoolDBError{tag: ResultTagErr, err: &arg0}
}
func (r ResultBoolDBError) IsOk() bool {
	return r.tag == ResultTagOk
}
func (r ResultBoolDBError) IsErr() bool {
	return r.tag == ResultTagErr
}
func (r ResultBoolDBError) Unwrap() bool {
	if r.tag != ResultTagOk {
		panic("called Unwrap on Err")
	}
	if r.ok == nil {
		panic("Result contains nil Ok value")
	}
	return *r.ok
}
func (r ResultBoolDBError) UnwrapOr(defaultValue bool) bool {
	if r.tag == ResultTagOk {
		return *r.ok
	}
	return defaultValue
}
func (r ResultBoolDBError) UnwrapErr() DBError {
	if r.tag != ResultTagErr {
		panic("called UnwrapErr on Ok")
	}
	if r.err == nil {
		panic("Result contains nil Err value")
	}
	return *r.err
}
func (r ResultBoolDBError) UnwrapOrElse(fn func(DBError) bool) bool {
	if r.tag == ResultTagOk && r.ok != nil {
		return *r.ok
	}
	if r.err != nil {
		return fn(*r.err)
	}
	panic("Result in invalid state")
}
func (r ResultBoolDBError) Map(fn func(bool) interface{}) interface{} {
	if r.tag == ResultTagOk && r.ok != nil {
		u := fn(*r.ok)
		return struct {
			tag ResultTag
			ok  *interface{}
			err *DBError
		}{tag: ResultTagOk, ok: &u}
	}
	return struct {
		tag ResultTag
		ok  *interface{}
		err *DBError
	}{tag: r.tag, ok: nil, err: r.err}
}
func (r ResultBoolDBError) MapErr(fn func(DBError) interface{}) interface{} {
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
func (r ResultBoolDBError) Filter(predicate func(bool) bool) ResultBoolDBError {
	if r.tag == ResultTagOk && predicate(*r.ok) {
		return r
	}
	return r
}
func (r ResultBoolDBError) AndThen(fn func(bool) interface{}) interface{} {
	if r.tag == ResultTagOk && r.ok != nil {
		return fn(*r.ok)
	}
	return struct {
		tag ResultTag
		ok  *interface{}
		err *DBError
	}{tag: r.tag, ok: nil, err: r.err}
}
func (r ResultBoolDBError) OrElse(fn func(DBError) interface{}) interface{} {
	if r.tag == ResultTagErr && r.err != nil {
		return fn(*r.err)
	}
	return struct {
		tag ResultTag
		ok  *bool
		err *interface{}
	}{tag: r.tag, ok: r.ok, err: nil}
}
func (r ResultBoolDBError) And(other interface{}) interface{} {
	if r.tag == ResultTagOk {
		return other
	}
	return r
}
func (r ResultBoolDBError) Or(other ResultBoolDBError) ResultBoolDBError {
	if r.tag == ResultTagOk {
		return r
	}
	return other
}

type User struct {
	ID    int
	Name  string
	Email string
}

type DBError struct {
	Code    string
	Message string
}
func (e DBError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}
func FindUserByID(db *sql.DB, id int) ResultUserDBError {
	row := db.QueryRow("SELECT id, name, email FROM users WHERE id = ?", id)

	var user User
	err := row.Scan(&user.ID, &user.Name, &user.Email)
	if err == sql.ErrNoRows {

		return ResultUserDBErrorErr(DBError{Code: "NOT_FOUND", Message: "user not found"})
	}
	if err != nil {
		return ResultUserDBErrorErr(DBError{Code: "SCAN_ERROR", Message: err.Error()})
	}

	return ResultUserDBErrorOk(user)
}
func TransferFunds(db *sql.DB, fromID int, toID int, amount float64) ResultBoolDBError {

	fromResult := FindUserByID(db, fromID)
	if fromResult.IsErr() {
		return ResultBoolDBErrorErr(fromResult.UnwrapErr())
	}

	toResult := FindUserByID(db, toID)
	if toResult.IsErr() {
		return ResultBoolDBErrorErr(toResult.UnwrapErr())
	}

	fmt.Printf("Transferring $%.2f from %s to %s\n",
		amount, fromResult.Unwrap().Name, toResult.Unwrap().Name)

	return ResultBoolDBErrorOk(true)
}
func main() {

	var db *sql.DB

	result := FindUserByID(db, 123)

	if result.IsOk() {
		user := result.Unwrap()
		fmt.Printf("Found user: %s <%s>\n", user.Name, user.Email)
	} else {
		err := result.UnwrapErr()
		fmt.Printf("Error: %s\n", err.Message)
	}
}
