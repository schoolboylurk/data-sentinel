package auth

import (
	"os"

	"github.com/permitio/permit-golang/pkg/config"
	"github.com/permitio/permit-golang/pkg/permit"
)

// PermitClient is the enforcement client you use to do runtime checks.
var PermitClient *permit.Client

// InitPermit initializes the Permit.io SDK using your env vars.
func InitPermit() {
	cfg := config.
		NewConfigBuilder(os.Getenv("PERMIT_API_KEY")).
		WithPdpUrl(os.Getenv("PERMIT_PDP_URL")).
		Build()

	PermitClient = permit.New(cfg)
}
