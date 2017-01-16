package miniservice

import (
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

func NewClientConn(id, name string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	cl, err := api.NewClient(api.DefaultConfig())
	if nil != err {
		return nil, errors.Wrap(err, "failed connecting to consul")
	}

	s, _, err := cl.Catalog().Service(name, "", nil)
	if nil != err {
		return nil, errors.Wrap(err, "failed fetching services")
	}

	var res *api.CatalogService = nil
	for i := 0; i < len(s); i++ {
		if s[i].ServiceID == id {
			res = s[i]
			break
		}
	}
	if nil == res {
		return nil, errors.Errorf("could not find service %s : %s", id, name)
	}
	var tag string = ""
	for i := 0; i < len(res.ServiceTags); i++ {
		t := res.ServiceTags[i]
		if strings.HasPrefix(t, servicePrefix) {
			tag = strings.TrimPrefix(t, servicePrefix)
			break
		}
	}
	if "" == tag {
		return nil, errors.Errorf("could not find tag with prefix %s for service (%s : %s)", servicePrefix, id, name)
	}
	return grpc.Dial(tag, opts...)
}
