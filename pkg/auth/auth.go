// Package auth provides the building blocks of provider sign-in flows: PKCE,
// device codes, loopback redirects and token requests.
//
// The contracts (catwalk.Authenticator, catwalk.Credential and friends) live
// in package catwalk, and each provider package registers its implementation
// there. A provider's authenticator is reached with catwalk.Provider.Auth.
package auth
