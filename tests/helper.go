package tests

//
//import (
//	"strings"
//	"testing"
//
//	"github.com/stretchr/testify/require"
//)
//
//type responseWithHeader interface {
//	GetHeader() *mini.MessageHeader
//}
//
//type responseWithError interface {
//	GetError() *v1.Error
//}
//
//func MustSuccessResp(t testing.TB, resp any) {
//	t.Helper()
//	withHeader, ok := resp.(responseWithHeader)
//	require.True(t, ok)
//
//	header := withHeader.GetHeader()
//	require.NotNil(t, header)
//	if header.GetResponseCode() == v1.ResponseCodeSuccess {
//		return
//	}
//
//	code := header.GetResponseCode().String()
//	if withError, ok := resp.(responseWithError); ok && withError.GetError() != nil {
//		if msg := strings.TrimSpace(withError.GetError().GetMessage()); msg != "" {
//			t.Fatalf("unexpected response code=%s message=%s", code, msg)
//		}
//	}
//	t.Fatalf("unexpected response code=%s type=%T", code, resp)
//}
