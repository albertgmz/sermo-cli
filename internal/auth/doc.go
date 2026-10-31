// Package auth holds the full device login contract with a Sermo server.
// It contains the device authorization flow, the token store, the credential resolution and the sign-out call.
// The sign-out call is the auth.signOutDevice operation. It ends only the device login of the client.
// A change to the contract must touch only this package.
package auth
