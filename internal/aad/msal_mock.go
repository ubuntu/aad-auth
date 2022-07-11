//go:build msalmock

package aad

import "github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"

func newPublicClient(clientID string, options ...public.Option) (publicClient, error) {
	return public.public.Client{}, nil
}
