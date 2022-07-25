//go:build !msalmock

package aad

import "github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"

const flavor = "real"

func newPublicClient(clientID string, options ...public.Option) (publicClient, error) {
	return public.New(clientID, options...)
}
