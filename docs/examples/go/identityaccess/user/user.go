// Package user is the home of the User aggregate of the identityaccess
// context: an account under a tenant, the person behind it, and its
// credentials.
package user

import (
	"errors"
	"fmt"
	"time"

	"example.com/saasovation/identityaccess/event"
	"example.com/saasovation/identityaccess/tenant"
)

// User is the aggregate root.
type User struct {
	event.Recorder

	tenantID   tenant.TenantID
	username   Username
	password   EncryptedPassword
	enablement Enablement
	person     Person
	clock      func() time.Time
}

// ErrPersonOfAnotherTenant is returned when a person record carries a
// different tenant than the user.
var ErrPersonOfAnotherTenant = errors.New("the person belongs to another tenant")

// ErrCurrentPasswordMismatch is returned when the current password offered
// on a change does not match the stored one.
var ErrCurrentPasswordMismatch = errors.New("current password does not match")

// ErrPasswordUnchanged is returned when the new password repeats the current one.
var ErrPasswordUnchanged = errors.New("new password must differ from the current one")

// Register is the constructor door: a user is registered under a tenant
// with credentials, an enablement, and a person record. The encrypter is
// passed in for this one operation and never held.
func Register(tenantID tenant.TenantID, credentials Credentials, enablement Enablement, person Person, encrypter Encrypter, now func() time.Time) (*User, error) {
	if tenantID.IsZero() {
		return nil, errors.New("user requires a tenant")
	}
	encrypted, err := encrypter.Encrypt(credentials.Password())
	if err != nil {
		return nil, fmt.Errorf("register user %s: %w", credentials.Username(), err)
	}
	u := &User{tenantID: tenantID, username: credentials.Username(), password: encrypted, enablement: enablement, person: person, clock: now}
	if err := u.EnsurePersonBelongsToTheUsersTenant(); err != nil {
		return nil, err
	}
	u.Raise(UserRegistered{TenantID: tenantID, Username: u.username, Name: person.Name(), Email: person.EmailAddress(), occurredOn: now()})
	return u, nil
}

// Reconstitute rebuilds a user from persisted state; a repository adapter
// is its only caller.
func Reconstitute(tenantID tenant.TenantID, username Username, password EncryptedPassword, enablement Enablement, person Person, now func() time.Time) (*User, error) {
	u := &User{tenantID: tenantID, username: username, password: password, enablement: enablement, person: person, clock: now}
	if err := u.EnsurePersonBelongsToTheUsersTenant(); err != nil {
		return nil, err
	}
	return u, nil
}

// UserID is the aggregate's identity.
func (u *User) UserID() UserID { return NewUserID(u.tenantID, u.username) }

// TenantID is the tenant the user belongs to.
func (u *User) TenantID() tenant.TenantID { return u.tenantID }

// Username is the name the user signs in with.
func (u *User) Username() Username { return u.username }

// Password is the stored form of the user's password.
func (u *User) Password() EncryptedPassword { return u.password }

// Enablement is when the user may sign in.
func (u *User) Enablement() Enablement { return u.enablement }

// Person is the human behind the account.
func (u *User) Person() Person { return u.person }

// IsEnabled is a side-effect-free function: whether the user may sign in
// right now.
func (u *User) IsEnabled() bool { return u.enablement.IsEnablementEnabled(u.clock()) }

// ChangePassword replaces the password after the current one is proven.
func (u *User) ChangePassword(current, changed Password, encrypter Encrypter) error {
	if !encrypter.Matches(current, u.password) {
		return ErrCurrentPasswordMismatch
	}
	if err := u.AssertChangedPasswordDiffersFromCurrent(current, changed); err != nil {
		return err
	}
	credentials, err := NewCredentials(u.username, changed)
	if err != nil {
		return err
	}
	encrypted, err := encrypter.Encrypt(credentials.Password())
	if err != nil {
		return fmt.Errorf("change password of %s: %w", u.username, err)
	}
	u.password = encrypted
	if err := u.EnsurePersonBelongsToTheUsersTenant(); err != nil {
		return err
	}
	u.Raise(UserPasswordChanged{TenantID: u.tenantID, Username: u.username, occurredOn: u.clock()})
	return nil
}

// DefineEnablement sets when the user may sign in.
func (u *User) DefineEnablement(enablement Enablement) error {
	u.enablement = enablement
	if err := u.EnsurePersonBelongsToTheUsersTenant(); err != nil {
		return err
	}
	u.Raise(UserEnablementChanged{TenantID: u.tenantID, Username: u.username, Enablement: enablement, occurredOn: u.clock()})
	return nil
}

// ChangePersonalName renames the person behind the account.
func (u *User) ChangePersonalName(name FullName) error {
	u.person = u.person.WithName(name)
	return u.EnsurePersonBelongsToTheUsersTenant()
}

// EnsurePersonBelongsToTheUsersTenant enforces the aggregate invariant
// person-belongs-to-the-users-tenant: the person record a user carries
// belongs to the user's own tenant.
func (u *User) EnsurePersonBelongsToTheUsersTenant() error {
	if u.person.TenantID() != u.tenantID {
		return fmt.Errorf("user %s: %w", u.username, ErrPersonOfAnotherTenant)
	}
	return nil
}

// AssertChangedPasswordDiffersFromCurrent checks the assertion
// changed-password-differs-from-current on ChangePassword.
func (u *User) AssertChangedPasswordDiffersFromCurrent(current, changed Password) error {
	if current.Reveal() == changed.Reveal() {
		return fmt.Errorf("user %s: %w", u.username, ErrPasswordUnchanged)
	}
	return nil
}
