// Real-world example: User settings where values may or may not be set
// Option[T] eliminates nil pointer panics by making absence explicit
package main

import "fmt"


type OptionTag uint8
const (
	OptionTagSome OptionTag = iota
	OptionTagNone
)
type OptionUser struct {
	tag  OptionTag
	some *User
}
func OptionUserSome(arg User) OptionUser {
	return OptionUser{tag: OptionTagSome, some: &arg}
}
func OptionUserNone() OptionUser {
	return OptionUser{tag: OptionTagNone}
}
func (o OptionUser) IsSome() bool {
	return o.tag == OptionTagSome
}
func (o OptionUser) IsNone() bool {
	return o.tag == OptionTagNone
}
func (o OptionUser) Unwrap() User {
	if o.tag != OptionTagSome {
		panic("called Unwrap on None")
	}
	return *o.some
}
func (o OptionUser) UnwrapOr(defaultValue User) User {
	if o.tag == OptionTagSome {
		return *o.some
	}
	return defaultValue
}
func (o OptionUser) UnwrapOrElse(fn func() User) User {
	if o.tag == OptionTagSome {
		return *o.some
	}
	return fn()
}
func (o OptionUser) Map(fn func(User) interface{}) OptionUser {
	if o.tag == OptionTagNone {
		return o
	}
	mapped := fn(*o.some)
	result := mapped.(User)
	return OptionUser{tag: OptionTagSome, some: &result}
}
func (o OptionUser) AndThen(fn func(User) OptionUser) OptionUser {
	if o.tag == OptionTagNone {
		return o
	}
	return fn(*o.some)
}
func (o OptionUser) Filter(predicate func(User) bool) OptionUser {
	if o.tag == OptionTagNone {
		return o
	}
	if predicate(*o.some) {
		return o
	}
	return OptionUser{tag: OptionTagNone}
}
type OptionString struct {
	tag  OptionTag
	some *string
}
func OptionStringSome(arg string) OptionString {
	return OptionString{tag: OptionTagSome, some: &arg}
}
func OptionStringNone() OptionString {
	return OptionString{tag: OptionTagNone}
}
func (o OptionString) IsSome() bool {
	return o.tag == OptionTagSome
}
func (o OptionString) IsNone() bool {
	return o.tag == OptionTagNone
}
func (o OptionString) Unwrap() string {
	if o.tag != OptionTagSome {
		panic("called Unwrap on None")
	}
	return *o.some
}
func (o OptionString) UnwrapOr(defaultValue string) string {
	if o.tag == OptionTagSome {
		return *o.some
	}
	return defaultValue
}
func (o OptionString) UnwrapOrElse(fn func() string) string {
	if o.tag == OptionTagSome {
		return *o.some
	}
	return fn()
}
func (o OptionString) Map(fn func(string) interface{}) OptionString {
	if o.tag == OptionTagNone {
		return o
	}
	mapped := fn(*o.some)
	result := mapped.(string)
	return OptionString{tag: OptionTagSome, some: &result}
}
func (o OptionString) AndThen(fn func(string) OptionString) OptionString {
	if o.tag == OptionTagNone {
		return o
	}
	return fn(*o.some)
}
func (o OptionString) Filter(predicate func(string) bool) OptionString {
	if o.tag == OptionTagNone {
		return o
	}
	if predicate(*o.some) {
		return o
	}
	return OptionString{tag: OptionTagNone}
}
type OptionInt struct {
	tag  OptionTag
	some *int
}
func OptionIntSome(arg int) OptionInt {
	return OptionInt{tag: OptionTagSome, some: &arg}
}
func OptionIntNone() OptionInt {
	return OptionInt{tag: OptionTagNone}
}
func (o OptionInt) IsSome() bool {
	return o.tag == OptionTagSome
}
func (o OptionInt) IsNone() bool {
	return o.tag == OptionTagNone
}
func (o OptionInt) Unwrap() int {
	if o.tag != OptionTagSome {
		panic("called Unwrap on None")
	}
	return *o.some
}
func (o OptionInt) UnwrapOr(defaultValue int) int {
	if o.tag == OptionTagSome {
		return *o.some
	}
	return defaultValue
}
func (o OptionInt) UnwrapOrElse(fn func() int) int {
	if o.tag == OptionTagSome {
		return *o.some
	}
	return fn()
}
func (o OptionInt) Map(fn func(int) interface{}) OptionInt {
	if o.tag == OptionTagNone {
		return o
	}
	mapped := fn(*o.some)
	result := mapped.(int)
	return OptionInt{tag: OptionTagSome, some: &result}
}
func (o OptionInt) AndThen(fn func(int) OptionInt) OptionInt {
	if o.tag == OptionTagNone {
		return o
	}
	return fn(*o.some)
}
func (o OptionInt) Filter(predicate func(int) bool) OptionInt {
	if o.tag == OptionTagNone {
		return o
	}
	if predicate(*o.some) {
		return o
	}
	return OptionInt{tag: OptionTagNone}
}
type OptionBool struct {
	tag  OptionTag
	some *bool
}
func OptionBoolSome(arg bool) OptionBool {
	return OptionBool{tag: OptionTagSome, some: &arg}
}
func OptionBoolNone() OptionBool {
	return OptionBool{tag: OptionTagNone}
}
func (o OptionBool) IsSome() bool {
	return o.tag == OptionTagSome
}
func (o OptionBool) IsNone() bool {
	return o.tag == OptionTagNone
}
func (o OptionBool) Unwrap() bool {
	if o.tag != OptionTagSome {
		panic("called Unwrap on None")
	}
	return *o.some
}
func (o OptionBool) UnwrapOr(defaultValue bool) bool {
	if o.tag == OptionTagSome {
		return *o.some
	}
	return defaultValue
}
func (o OptionBool) UnwrapOrElse(fn func() bool) bool {
	if o.tag == OptionTagSome {
		return *o.some
	}
	return fn()
}
func (o OptionBool) Map(fn func(bool) interface{}) OptionBool {
	if o.tag == OptionTagNone {
		return o
	}
	mapped := fn(*o.some)
	result := mapped.(bool)
	return OptionBool{tag: OptionTagSome, some: &result}
}
func (o OptionBool) AndThen(fn func(bool) OptionBool) OptionBool {
	if o.tag == OptionTagNone {
		return o
	}
	return fn(*o.some)
}
func (o OptionBool) Filter(predicate func(bool) bool) OptionBool {
	if o.tag == OptionTagNone {
		return o
	}
	if predicate(*o.some) {
		return o
	}
	return OptionBool{tag: OptionTagNone}
}

type UserSettings struct {
	Theme       OptionString
	FontSize    OptionInt
	Language    OptionString
	NotifyEmail OptionBool
}

type User struct {
	ID       int
	Name     string
	Settings UserSettings
}

// GetTheme returns the user's theme or system default
// No nil checks needed - Option forces explicit handling
//
// Alternative approaches for Option[T]:
//
//	opt.MustSome()       - Go style, panics if None (recommended)
//	opt.Unwrap()         - Rust style alias for MustSome() (deprecated)
//	opt.SomeOr(default)  - Returns default if None
//	opt.SomeOrElse(fn)   - Computes default lazily via fn() if None
func GetTheme(user User) string {
	return user.Settings.Theme.SomeOr("system")
}

// GetFontSize applies validation and returns CSS value
func GetFontSize(user User) string {
	// Check if font size is set and apply validation
	if user.Settings.FontSize.IsSome() {
		size := user.Settings.FontSize.MustSome()
		if size < 10 {
			return "10px"
		}
		if size > 32 {
			return "32px"
		}
		return fmt.Sprintf("%dpx", size)
	}
	return "16px"
}

// ShouldSendNotification checks multiple optional settings
func ShouldSendNotification(user User, notificationType string) bool {
	// Check if email notifications are enabled
	if user.Settings.NotifyEmail.IsSome() {
		return user.Settings.NotifyEmail.MustSome()
	}
	// Default behavior based on notification type
	return notificationType == "critical"
}

// FindUserByLanguage returns first user with matching language preference
func FindUserByLanguage(users []User, lang string) OptionUser {
	for _, user := range users {
		if user.Settings.Language.IsSome() && user.Settings.Language.MustSome() == lang {
			return OptionUserSome(user)
		}
	}
	return OptionUserNone()
}
func main() {
	// User with some settings configured
	alice := User{
		ID:   1,
		Name: "Alice",
		Settings: UserSettings{
			Theme:    OptionStringSome("dark"),
			FontSize: OptionIntSome(18),
			Language: OptionStringNone(), // Not set - will use system language
		},
	}

	// User with minimal settings
	bob := User{
		ID:   2,
		Name: "Bob",
		Settings: UserSettings{
			// All settings use defaults
			Theme:    OptionStringNone(),
			FontSize: OptionIntNone(),
			Language: OptionStringSome("es"),
		},
	}

	fmt.Printf("Alice's theme: %s\n", GetTheme(alice))   // "dark"
	fmt.Printf("Bob's theme: %s\n", GetTheme(bob))       // "system"
	fmt.Printf("Alice's font: %s\n", GetFontSize(alice)) // "18px"
	fmt.Printf("Bob's font: %s\n", GetFontSize(bob))     // "16px"

	users := []User{alice, bob}
	if spanish := FindUserByLanguage(users, "es"); spanish.IsSome() {
		fmt.Printf("Spanish user: %s\n", spanish.MustSome().Name) // "Bob"
	}
}
