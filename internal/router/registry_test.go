package router_test

import (
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
)

func TestMemRegistryList(t *testing.T) {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	list := reg.List(port.PortListen)
	if len(list) != 1 {
		t.Fatalf("listen count=%d", len(list))
	}
}
