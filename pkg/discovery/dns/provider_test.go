// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package dns

import (
	"context"
	"sort"
	"testing"

	"github.com/go-kit/kit/log"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/thanos-io/thanos/pkg/testutil"
)

func TestProvider(t *testing.T) {
	ips := []string{
		"127.0.0.1:19091",
		"127.0.0.2:19092",
		"127.0.0.3:19093",
		"127.0.0.4:19094",
		"127.0.0.5:19095",
	}

	prv := NewProvider(log.NewNopLogger(), nil, "")
	prv.resolver = &mockResolver{
		res: map[string][]string{
			"a": ips[:2],
			"b": ips[2:4],
			"c": {ips[4]},
		},
	}
	ctx := context.TODO()

	prv.Resolve(ctx, []string{"any+x"})
	result := prv.Addresses()
	sort.Strings(result)
	testutil.Equals(t, []string(nil), result)
	testutil.Equals(t, 1, promtestutil.CollectAndCount(prv.resolverAddrs))
	testutil.Equals(t, float64(0), promtestutil.ToFloat64(prv.resolverAddrs.WithLabelValues("any+x")))

	prv.Resolve(ctx, []string{"any+a", "any+b", "any+c"})
	result = prv.Addresses()
	sort.Strings(result)
	testutil.Equals(t, ips, result)
	testutil.Equals(t, 3, promtestutil.CollectAndCount(prv.resolverAddrs))
	testutil.Equals(t, float64(2), promtestutil.ToFloat64(prv.resolverAddrs.WithLabelValues("any+a")))
	testutil.Equals(t, float64(2), promtestutil.ToFloat64(prv.resolverAddrs.WithLabelValues("any+b")))
	testutil.Equals(t, float64(1), promtestutil.ToFloat64(prv.resolverAddrs.WithLabelValues("any+c")))

	prv.Resolve(ctx, []string{"any+b", "any+c"})
	result = prv.Addresses()
	sort.Strings(result)
	testutil.Equals(t, ips[2:], result)
	testutil.Equals(t, 2, promtestutil.CollectAndCount(prv.resolverAddrs))
	testutil.Equals(t, float64(2), promtestutil.ToFloat64(prv.resolverAddrs.WithLabelValues("any+b")))
	testutil.Equals(t, float64(1), promtestutil.ToFloat64(prv.resolverAddrs.WithLabelValues("any+c")))

	prv.Resolve(ctx, []string{"any+x"})
	result = prv.Addresses()
	sort.Strings(result)
	testutil.Equals(t, []string(nil), result)
	testutil.Equals(t, 1, promtestutil.CollectAndCount(prv.resolverAddrs))
	testutil.Equals(t, float64(0), promtestutil.ToFloat64(prv.resolverAddrs.WithLabelValues("any+x")))

	prv.Resolve(ctx, []string{"any+a", "any+b", "any+c"})
	result = prv.Addresses()
	sort.Strings(result)
	testutil.Equals(t, ips, result)
	testutil.Equals(t, 3, promtestutil.CollectAndCount(prv.resolverAddrs))
	testutil.Equals(t, float64(2), promtestutil.ToFloat64(prv.resolverAddrs.WithLabelValues("any+a")))
	testutil.Equals(t, float64(2), promtestutil.ToFloat64(prv.resolverAddrs.WithLabelValues("any+b")))
	testutil.Equals(t, float64(1), promtestutil.ToFloat64(prv.resolverAddrs.WithLabelValues("any+c")))

	prv.Resolve(ctx, []string{"any+b", "example.com:90", "any+c"})
	result = prv.Addresses()
	sort.Strings(result)
	testutil.Equals(t, append(ips[2:], "example.com:90"), result)
	testutil.Equals(t, 3, promtestutil.CollectAndCount(prv.resolverAddrs))
	testutil.Equals(t, float64(2), promtestutil.ToFloat64(prv.resolverAddrs.WithLabelValues("any+b")))
	testutil.Equals(t, float64(1), promtestutil.ToFloat64(prv.resolverAddrs.WithLabelValues("example.com:90")))
	testutil.Equals(t, float64(1), promtestutil.ToFloat64(prv.resolverAddrs.WithLabelValues("any+c")))

	prv.Resolve(ctx, []string{"any+b", "any+c"})
	result = prv.Addresses()
	sort.Strings(result)
	testutil.Equals(t, ips[2:], result)
	testutil.Equals(t, 2, promtestutil.CollectAndCount(prv.resolverAddrs))
	testutil.Equals(t, float64(2), promtestutil.ToFloat64(prv.resolverAddrs.WithLabelValues("any+b")))
	testutil.Equals(t, float64(1), promtestutil.ToFloat64(prv.resolverAddrs.WithLabelValues("any+c")))

}

type mockResolver struct {
	res map[string][]string
	err error
}

func (d *mockResolver) Resolve(_ context.Context, name string, _ QType) ([]string, error) {
	if d.err != nil {
		return nil, d.err
	}
	return d.res[name], nil
}
