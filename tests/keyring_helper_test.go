package deploy_test

import (
	"webtyp.com/deploy"
	"webtyp.com/keyring"
	keyringtests "webtyp.com/keyring/tests"
)

// newTestKeyring returns a keyring backed by an in-memory provider, scoped to
// the same service deploy uses.
//
// It replaces zalando/go-keyring's MockInit, which installed a process-wide
// mock: with the backend injected instead, nothing touches the developer's real
// credential store and tests can run in parallel without clobbering each other.
func newTestKeyring() *keyring.Keyring {
	return keyring.OpenKeyring(deploy.KeyringServiceName, keyringtests.NewMemProvider())
}

// newTestSecureStore wraps base in a SecureStore over an in-memory keyring, and
// returns the keyring too so a test can assert what actually landed in it.
func newTestSecureStore(base deploy.Store) (*deploy.SecureStore, *keyring.Keyring) {
	kr := newTestKeyring()
	return deploy.NewSecureStoreWithKeyring(base, kr), kr
}
