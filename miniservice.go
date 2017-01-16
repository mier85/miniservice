package miniservice

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/ftloc/exception"
	"github.com/mier85/consulter"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

const servicePrefix string = "miniservice-"

type MiniService struct {
	grpcServer *grpc.Server
	id         string
	name       string
}

func getPort(addr net.Addr) (uint16, error) {
	_, p, err := net.SplitHostPort(addr.String())
	if nil != err {
		return 0, errors.Wrapf(err, "failed determining port: %s", addr)
	}
	pI, err := strconv.Atoi(p)
	if nil != err {
		return 0, errors.Wrapf(err, "failed converting port to integer: %s", p)
	}
	return uint16(pI), nil
}

func NewService(id, name string) *MiniService {
	s := grpc.NewServer()
	return &MiniService{
		id:         id,
		name:       name,
		grpcServer: s,
	}
}

func (ms *MiniService) Register(fn, handler interface{}) *MiniService {
	v := reflect.ValueOf(fn)
	t := v.Type()
	exception.ThrowOnFalse(t.Kind() == reflect.Func, errors.New("function expected"))
	exception.ThrowOnFalse(t.NumIn() == 2, errors.New("handler function should have exactly two input parameters"))
	exception.ThrowOnFalse(t.NumOut() == 0, errors.New("handler function should have no output parameter"))
	vS := reflect.ValueOf(ms.grpcServer)
	vH := reflect.ValueOf(handler)
	exception.ThrowOnFalse(vS.Type().AssignableTo(t.In(0)), errors.Errorf("first parameter of function should be *grpc.Server but is %s", t.In(0).Kind()))
	exception.ThrowOnFalse(vH.Type().AssignableTo(t.In(1)), errors.Errorf("handler should be assignable to second parameter of function but we got %s vs %s", vH.Kind(), t.In(1).Kind()))
	v.Call([]reflect.Value{vS, vH})
	return ms
}

func (ms *MiniService) newConn() (net.Listener, uint16, error) {
	l, err := net.Listen("tcp", "0.0.0.0:0")
	if nil != err {
		return nil, 0, errors.Errorf("failed listening: %s", err.Error())
	}

	p, err := getPort(l.Addr())
	if nil != err {
		l.Close()
		return nil, 0, errors.New("failed determining port")
	}
	return l, p, nil
}

func (ms *MiniService) httpHealthChecker() (uint16, error, chan error) {
	con, p, err := ms.newConn()
	if nil != err {
		return 0, errors.Wrap(err, "failed starting server"), nil
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(rw http.ResponseWriter, req *http.Request) { rw.WriteHeader(200) })
	cErr := make(chan error)
	go func(mux *http.ServeMux, cErr chan error, con net.Listener) {
		defer con.Close()
		err := http.Serve(con, mux)
		if nil != err {
			cErr <- err
		}
	}(mux, cErr, con)
	return p, nil, cErr
}

func (ms *MiniService) consul(host string, tag string, port uint16) *consulter.Consulter {
	c := &consulter.Consulter{
		Id:   ms.id,
		Name: ms.name,
		Tags: []string{"miniservice", tag},
		Port: int(port),
		Url: consulter.UrlParts{
			Base:   host,
			Health: "/health",
		},
		Intervals: consulter.Intervals{
			Health:  "2s",
			Timeout: "2s",
		},
	}
	return c
}

func (ms *MiniService) Listen() error {
	con, p, err := ms.newConn()
	if nil != err {
		return errors.Wrap(err, "failed starting server")
	}

	lIp, err := consulter.GetHostName()
	if nil != err {
		return errors.Wrap(err, "failed determining ip address")
	}
	tag := fmt.Sprintf("%s%s:%d", servicePrefix, lIp, p)
	log.Printf("service reachable at: %s:%d (tag: %s)", lIp, p, tag)

	hPort, err, hcErrChan := ms.httpHealthChecker()
	if nil != err {
		return errors.Wrap(err, "failed getting http health checker")
	}
	errChan := make(chan error)
	go func(ms *MiniService) {
		defer con.Close()
		err := ms.grpcServer.Serve(con)
		if nil != err {

		}
	}(ms)
	consul := ms.consul(lIp, tag, hPort)

	tC := time.After(250 * time.Millisecond)
	for {
		select {
		case <-tC:
			defer consul.Connect().Register().Close()
		case err := <-hcErrChan:
			if nil != err {
				return errors.Wrap(err, "failed starting health check http service")
			}
			log.Printf("health check has ended - exiting")
			return nil
		case err := <-errChan:
			if nil != err {
				return errors.Wrap(err, "failed starting grpc service")
			}
			log.Printf("service has ended - exiting")
			return nil
		}
	}
	return nil
}
