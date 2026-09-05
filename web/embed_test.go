package web_test

import (
	"io/fs"
	"testing"

	"github.com/coraltele/com.coraltele.aiorchestrator/web"
)

func TestUIFS_EmbedsShells(t *testing.T) {
	need := []string{
		"index.html",
		"admin/index.html",
		"admin/engines.html",
		"admin/profiles.html",
		"admin/settings.html",
		"admin/bindings.html",
		"admin/flows.html",
		"admin/flows-builder.html",
		"admin/flow-builder.js",
		"admin/pin.html",
		"admin/admin.js",
		"supervisor/index.html",
		"supervisor/supervisor.js",
		"chat/index.html",
		"chat/chat.js",
		"shared/api.js",
		"shared/styles.css",
	}
	for _, name := range need {
		b, err := fs.ReadFile(web.UIFS, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(b) == 0 {
			t.Fatalf("%s empty", name)
		}
	}
}
