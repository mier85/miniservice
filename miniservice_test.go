package miniservice_test

import (
	"testing"

	"google.golang.org/grpc"

	"github.com/ftloc/exception"
	"github.com/mier85/miniservice"
)

func run(fn func()) interface{} {
	var ex interface{}
	exception.Try(func() {
		fn()
		ex = nil
	}).CatchAll(func(e interface{}) {
		ex = e
	}).Finally(func() {
	})
	return ex
}

func AssertException(t *testing.T, fn func()) {
	ex := run(fn)
	if ex == nil {
		t.Errorf("expected exception but got nil")
		t.FailNow()
	}
}

func AssertNoException(t *testing.T, fn func()) {
	ex := run(fn)
	if ex != nil {
		t.Errorf("expected no exception but got %#v", ex)
		t.FailNow()
	}
}

func TestRegister(t *testing.T) {
	ms := miniservice.NewService("", "")
	AssertException(t, func() { ms.Register("s", "s") })
	AssertException(t, func() { ms.Register(func() {}, "") })
	AssertException(t, func() { ms.Register(func(a string, b string) {}, "") })
	AssertException(t, func() { ms.Register(func(a *grpc.Server, b int) {}, "") })
	AssertException(t, func() { ms.Register(func(a *grpc.Server, b string) error { return nil }, "") })
	AssertNoException(t, func() { ms.Register(func(a *grpc.Server, b string) {}, "") })
}
