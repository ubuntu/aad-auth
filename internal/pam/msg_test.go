package pam_test

import (
	"context"
	"io"
	"log"
	"testing"

	pamCom "github.com/msteinert/pam"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/aad-auth/internal/pam"
)

func TestInfo(t *testing.T) {
	var gotStyle pamCom.Style
	var gotInfoMsg string
	tx, err := pamCom.StartFunc("", "", func(s pamCom.Style, msg string) (string, error) {
		gotStyle = s
		gotInfoMsg = msg
		return "", nil
	})
	require.NoError(t, err, "Setup: pam should start a transaction with no error")
	cpam := pam.Handle(tx.Handle)

	ctx := pam.CtxWithPamh(context.Background(), cpam)
	pam.Info(ctx, "My %s message", "info")

	require.Equal(t, pamCom.Style(pamCom.TextInfo), gotStyle, "Send info style header")
	require.Equal(t, "My info message", gotInfoMsg, "Send expected info message")
}

func TestInfoWithNoPamInContext(t *testing.T) {
	var contentLog []byte
	done := make(chan struct{})

	r, w := io.Pipe()
	origOut := log.Writer()
	log.SetOutput(w)
	defer log.SetOutput(origOut)
	go func() {
		defer close(done)
		var err error
		contentLog, err = io.ReadAll(r)
		require.NoError(t, err, "read from redirected output should not fail")
	}()

	pam.Info(context.Background(), "My %s message", "info")

	w.Close()
	<-done

	require.Contains(t, string(contentLog), "WARNING: ", "Should print on stderr info output with warning")
	require.Contains(t, string(contentLog), "My info message", "Should print log message on stderr")
}
