package main

import (
	"fmt"
	"os"

	"github.com/coraltele/com.coraltele.aiorchestrator/internal/gateway/fake"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/port"
	"github.com/coraltele/com.coraltele.aiorchestrator/internal/router"
)

func main() {
	reg := router.NewMemRegistry()
	if err := fake.RegisterAll(reg); err != nil {
		fmt.Fprintf(os.Stderr, "register: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("aiorchestrator phase-a smoke: fake gateways registered")
	kinds := []port.PortKind{
		port.PortListen, port.PortSpeak, port.PortThink,
		port.PortTranslate, port.PortKnowledge, port.PortSkill,
	}
	for _, k := range kinds {
		fmt.Printf("  port %s: %d gateway(s)\n", k, len(reg.List(k)))
	}
}
