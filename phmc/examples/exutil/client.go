package exutil

import (
	hmc "github.com/IBM/infra-go-sdk/phmc"
)

// NewClient builds a RestClient with TLS verification disabled and the
// DebugTransport installed.  Call it after parsing flags:
//
//	restClient := exutil.NewClient(*hmcIP, *debug, *debugFull)
//
// When both debug and debugFull are false the transport still wraps the
// default transport, but only emits a single summary line per request.
func NewClient(hmcIP string, debug, debugFull bool) *hmc.RestClient {
	c := hmc.NewRestClient(hmcIP).WithTLSInsecure()
	c.WithTransport(&DebugTransport{
		Inner:     c.HTTPTransport(),
		Debug:     debug || debugFull,
		DebugFull: debugFull,
	})
	return c
}
