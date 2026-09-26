package user

import (
	"errors"
	"net/mail"
	"strings"

	"example.com/saasovation/identityaccess/tenant"
)

// FullName is the name a person is addressed by.
type FullName struct {
	firstName string
	lastName  string
}

// ErrFullNameIncomplete is returned when a first or last name is blank.
var ErrFullNameIncomplete = errors.New("full name needs a first and a last name")

// NewFullName is the one door through which two names become a FullName.
// Invariant both-names-present.
func NewFullName(firstName, lastName string) (FullName, error) {
	firstName, lastName = strings.TrimSpace(firstName), strings.TrimSpace(lastName)
	if firstName == "" || lastName == "" {
		return FullName{}, ErrFullNameIncomplete
	}
	return FullName{firstName: firstName, lastName: lastName}, nil
}

// FirstName is the given name.
func (n FullName) FirstName() string { return n.firstName }

// LastName is the family name.
func (n FullName) LastName() string { return n.lastName }

// String renders the name as "First Last".
func (n FullName) String() string { return n.firstName + " " + n.lastName }

// EmailAddress is an address mail can be delivered to.
type EmailAddress struct {
	address string
}

// ErrEmailAddressMalformed is returned when the text is not an address.
var ErrEmailAddressMalformed = errors.New("email address is malformed")

// NewEmailAddress is the one door through which text becomes an
// EmailAddress. Invariant well-formed: RFC 5322 address with no display name.
func NewEmailAddress(text string) (EmailAddress, error) {
	parsed, err := mail.ParseAddress(strings.TrimSpace(text))
	if err != nil || parsed.Name != "" {
		return EmailAddress{}, ErrEmailAddressMalformed
	}
	return EmailAddress{address: parsed.Address}, nil
}

// Address renders the address.
func (e EmailAddress) Address() string { return e.address }

// Person is the human behind a user account: the tenant they belong to,
// their name, and how to reach them. It is a value: two persons with the
// same tenant, name, and address are the same person record.
type Person struct {
	tenantID tenant.TenantID
	name     FullName
	email    EmailAddress
}

// NewPerson composes a person record under a tenant.
func NewPerson(tenantID tenant.TenantID, name FullName, email EmailAddress) (Person, error) {
	if tenantID.IsZero() {
		return Person{}, errors.New("person requires a tenant")
	}
	return Person{tenantID: tenantID, name: name, email: email}, nil
}

// TenantID is the tenant the person belongs to.
func (p Person) TenantID() tenant.TenantID { return p.tenantID }

// Name is the person's full name.
func (p Person) Name() FullName { return p.name }

// EmailAddress is where the person receives mail.
func (p Person) EmailAddress() EmailAddress { return p.email }

// WithName returns the same person under a different name.
func (p Person) WithName(name FullName) Person {
	p.name = name
	return p
}
