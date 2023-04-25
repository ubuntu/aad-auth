package aad

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	msalErrors "github.com/AzureAD/microsoft-authentication-library-for-go/apps/errors"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
)

// NewWithMockClient returns a mock AAD client that can be controlled through input for tests.
func NewWithMockClient() AAD {
	return AAD{
		newPublicClient: publicNewMockClient,
	}
}

func publicNewMockClient(clientID string, _ ...public.Option) (publicClient, error) {
	var forceOffline bool
	var publicClientDisallowed bool
	var noTenantWideConsent bool

	switch clientID {
	case "connection failed":
		return publicClientMock{}, errors.New("connection failed")
	case "force offline":
		forceOffline = true
	case "public client disallowed":
		publicClientDisallowed = true
	case "no tenant-wide consent":
		noTenantWideConsent = true
	}

	return publicClientMock{
		forceOffline:           forceOffline,
		publicClientDisallowed: publicClientDisallowed,
		noTenantWideConsent:    noTenantWideConsent,
	}, nil
}

type publicClientMock struct {
	forceOffline           bool
	publicClientDisallowed bool
	noTenantWideConsent    bool
}

func (m publicClientMock) AcquireTokenByUsernamePassword(_ context.Context, _ []string, username string, _ string, _ ...public.AcquireByUsernamePasswordOption) (public.AuthResult, error) {
	r := public.AuthResult{}
	callErr := msalErrors.CallErr{
		Resp: &http.Response{},
	}

	if m.forceOffline {
		return r, fmt.Errorf("Offline")
	}

	if m.publicClientDisallowed {
		callErr.Resp.Body = io.NopCloser(strings.NewReader(fmt.Sprintf("{\"error_codes\": [%d]}", noClientSecretCode)))
		return r, callErr
	}

	if m.noTenantWideConsent {
		callErr.Resp.Body = io.NopCloser(strings.NewReader(fmt.Sprintf("{\"error_codes\": [%d]}", noConsentCode)))
		return r, callErr
	}

	switch username {
	case "success@domain.com":
	case "success@otherdomain.com":
	case "requireMFA@domain.com":
		callErr.Resp.Body = io.NopCloser(strings.NewReader(fmt.Sprintf("{\"error_codes\": [%d]}", requiresMFACode)))
		return r, callErr
	case "unreadable server response":
		callErr.Resp.Body = io.NopCloser(errorReader{})
		return r, callErr
	case "invalid server response":
		callErr.Resp.Body = io.NopCloser(strings.NewReader("Not json"))
		return r, callErr
	case "invalid credentials":
		callErr.Resp.Body = io.NopCloser(strings.NewReader(fmt.Sprintf("{\"error_codes\": [%d]}", invalidCredCode)))
		return r, callErr
	case "no such user":
		callErr.Resp.Body = io.NopCloser(strings.NewReader(fmt.Sprintf("{\"error_codes\": [%d]}", noSuchUserCode)))
		return r, callErr
	case "unknown error code":
		callErr.Resp.Body = io.NopCloser(strings.NewReader("{\"error_codes\": [4242]}"))
		return r, callErr
	case "unknown error type":
		return r, errors.New("not a msal error")
	case "multiple errors, first known is mfa":
		callErr.Resp.Body = io.NopCloser(strings.NewReader(fmt.Sprintf("{\"error_codes\": [4242, %d, 4243, %d]}", requiresMFACode, invalidCredCode)))
		return r, callErr
	case "multiple errors, first known is invalid credential":
		callErr.Resp.Body = io.NopCloser(strings.NewReader(fmt.Sprintf("{\"error_codes\": [4242, %d, 4243, %d]}", invalidCredCode, requiresMFACode)))
		return r, callErr
	default:
		// default is unknown user
		callErr.Resp.Body = io.NopCloser(strings.NewReader(fmt.Sprintf("{\"error_codes\": [%d]}", noSuchUserCode)))
		return r, callErr
	}

	return r, nil
}

type errorReader struct{}

func (errorReader) Read(_ []byte) (n int, err error) {
	return 0, errors.New("invalid READ")
}
