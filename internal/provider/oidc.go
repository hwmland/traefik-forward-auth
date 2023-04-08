package provider

import (
	"context"
	"errors"
	"github.com/coreos/go-oidc"
	"golang.org/x/oauth2"
	"os"
	"io"
	"log"
	str "strings"
)

// OIDC provider
type OIDC struct {
	IssuerURL    string `long:"issuer-url" env:"ISSUER_URL" description:"Issuer URL"`
	ClientID     string `long:"client-id" env:"CLIENT_ID" description:"Client ID"`
	ClientSecret string `long:"client-secret" env:"CLIENT_SECRET" description:"Client Secret" json:"-"`

	OAuthProvider

	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier

	MyLog *log.Logger
}

// Name returns the name of the provider
func (o *OIDC) Name() string {
	return "oidc"
}

// Setup performs validation and setup
func (o *OIDC) Setup() error {
	// Check parms
	if o.IssuerURL == "" || o.ClientID == "" || o.ClientSecret == "" {
		return errors.New("providers.oidc.issuer-url, providers.oidc.client-id, providers.oidc.client-secret must be set")
	}

	var err error
	o.ctx = context.Background()

	// Try to initiate provider
	o.provider, err = oidc.NewProvider(o.ctx, o.IssuerURL)
	if err != nil {
		return err
	}

	myLog := log.New(io.Discard, "ByMarek: ", log.Lmsgprefix|log.Ldate|log.Ltime)
	myLog.SetOutput(os.Stdout)
	myLog.Println("----------> OIDC.Setup")
	o.MyLog = myLog

	// Create oauth2 config
	o.Config = &oauth2.Config{
		ClientID:     o.ClientID,
		ClientSecret: o.ClientSecret,
		Endpoint:     o.provider.Endpoint(),
	
		// "openid" is a required scope for OpenID Connect flows.
		Scopes: []string{oidc.ScopeOpenID, "profile", "email"},
	}

	// Create OIDC verifier
	o.verifier = o.provider.Verifier(&oidc.Config{
		ClientID: o.ClientID,
	})

	return nil
}

// GetLoginURL provides the login url for the given redirect uri and state
func (o *OIDC) GetLoginURL(redirectURI, state string) string {
	return o.OAuthGetLoginURL(redirectURI, state)
}

// ExchangeCode exchanges the given redirect uri and code for a token
func (o *OIDC) ExchangeCode(redirectURI, code string) (string, error) {
	token, err := o.OAuthExchangeCode(redirectURI, code)
	if err != nil {
		return "", err
	}

	// Extract ID token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return "", errors.New("Missing id_token")
	}

	o.MyLog.Println("----------> OIDC.ExchangeCode, rawIDToken:", rawIDToken)

	return rawIDToken, nil
}

// Keys returns the keys of the map m.
// The keys will be in an indeterminate order.
func mapKeys[M ~map[K]V, K comparable, V any](m M) []K {
	r := make([]K, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	return r
}

// GetUser uses the given token and returns a complete provider.User object
func (o *OIDC) GetUser(token, _ string) (*User, error) {
	// Parse & Verify ID Token
	idToken, err := o.verifier.Verify(o.ctx, token)
	if err != nil {
		return nil, err
	}

	// Extract custom claims
	var user struct {
		Email string `json:"email"`
		Groups []string `json:"groups"`
	}
	if err := idToken.Claims(&user); err != nil {
		return nil, err
	}
	o.MyLog.Println("----------> OIDC.GetUser, user:", user)

	groupMap := make(map[string]bool)
	for _, groupFull := range user.Groups {
		for _, group := range str.Split(groupFull, "/") {
			if group != "" {
				o.MyLog.Println("----------> OIDC.GetUser, group:", group)
				groupMap[group] = true
			}
		}
	}
	uniqueGrops := mapKeys(groupMap)
	return &User{User: user.Email, Groups: uniqueGrops, }, nil
}
