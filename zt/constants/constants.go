/*
	Copyright NetFoundry Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

package constants

import "time"

const (
	HanzoZTOrg            = "hanzozt"
	ZT                    = "zt"
	ZROK                  = "zrok"
	CaddyOrg              = "caddyserver"
	Caddy                 = "caddy"
	ZT_CONTROLLER         = "zt-controller"
	ZT_ROUTER             = "zt-router"
	ZT_TUNNEL             = "zt-tunnel"
	ZT_EDGE_TUNNEL        = "zt-edge-tunnel"
	ZT_EDGE_TUNNEL_GITHUB = "zt-tunnel-sdk-c"
)

// Config Template Constants
const (
	DefaultRouterListenerBindPort = "10080"
	DefaultGetSessionTimeout      = 60 * time.Second

	DefaultRouterPort = "3022"

	DefaultCtrlBindAddress    = "0.0.0.0"
	DefaultCtrlAdvertisedPort = "6262"

	DefaultCtrlDatabaseFile = "db/ctrl.db"

	DefaultCtrlEdgeBindAddress    = "0.0.0.0"
	DefaultCtrlEdgeAdvertisedPort = "1280"

	DefaultEdgeRouterCsrC  = "US"
	DefaultEdgeRouterCsrST = ""
	DefaultEdgeRouterCsrL  = ""
	DefaultEdgeRouterCsrO  = "Hanzo AI"
	DefaultEdgeRouterCsrOU = "ZT"
)

// Env Var Constants
const (
	HomeVarName        = "ZT_HOME"
	HomeVarDescription = "base dirname used to construct paths"

	NetworkNameVarName        = "ZT_NETWORK_NAME"
	NetworkNameVarDescription = "base filename used to construct paths"

	PkiCtrlCertVarName                               = "ZT_PKI_CTRL_CERT"
	PkiCtrlCertVarDescription                        = "Path to the controller's default identity client cert"
	PkiCtrlServerCertVarName                         = "ZT_PKI_CTRL_SERVER_CERT"
	PkiCtrlServerCertVarDescription                  = "Path to the controller's default identity server cert, including partial chain"
	PkiCtrlKeyVarName                                = "ZT_PKI_CTRL_KEY"
	PkiCtrlKeyVarDescription                         = "Path to the controller's default identity private key"
	PkiCtrlCAVarName                                 = "ZT_PKI_CTRL_CA"
	PkiCtrlCAVarDescription                          = "Path to the controller's bundle of trusted root CAs"
	CtrlBindAddressVarName                           = "ZT_CTRL_BIND_ADDRESS"
	CtrlBindAddressVarDescription                    = "The address where the controller will listen for router control plane connections"
	CtrlAdvertisedAddressVarName                     = "ZT_CTRL_ADVERTISED_ADDRESS"
	CtrlAdvertisedAddressVarDescription              = "The address routers will use to connect to the controller"
	CtrlAdvertisedPortVarName                        = "ZT_CTRL_ADVERTISED_PORT"
	CtrlAdvertisedPortVarDescription                 = "TCP port routers will use to connect to the controller"
	CtrlConsoleLocationVarName                       = "ZT_CONSOLE_LOCATION"
	CtrlConsoleLocationVarDescription                = "The filesystem path to controller's web console static HTML files"
	CtrlEdgeBindAddressVarName                       = "ZT_CTRL_EDGE_BIND_ADDRESS"
	CtrlEdgeBindAddressVarDescription                = "The address where the controller will listen for edge API connections"
	CtrlEdgeAdvertisedAddressVarName                 = "ZT_CTRL_EDGE_ADVERTISED_ADDRESS"
	CtrlEdgeAdvertisedAddressVarDescription          = "The controller's edge API address"
	CtrlEdgeAltAdvertisedAddressVarName              = "ZT_CTRL_EDGE_ALT_ADVERTISED_ADDRESS"
	CtrlEdgeAltAdvertisedAddressVarDescription       = "The controller's edge API alternative address"
	CtrlEdgeAdvertisedPortVarName                    = "ZT_CTRL_EDGE_ADVERTISED_PORT"
	CtrlEdgeAdvertisedPortVarDescription             = "TCP port of the controller's edge API"
	CtrlDatabaseFileVarName                          = "ZT_CTRL_DATABASE_FILE"
	CtrlDatabaseFileVarDescription                   = "Path to the controller's database file"
	PkiSignerCertVarName                             = "ZT_PKI_SIGNER_CERT"
	PkiSignerCertVarDescription                      = "Path to the controller's edge signer CA cert"
	PkiSignerKeyVarName                              = "ZT_PKI_SIGNER_KEY"
	PkiSignerKeyVarDescription                       = "Path to the controller's edge signer CA key"
	CtrlEdgeIdentityEnrollmentDurationVarName        = "ZT_EDGE_IDENTITY_ENROLLMENT_DURATION"
	CtrlEdgeIdentityEnrollmentDurationVarDescription = "The identity enrollment duration in minutes"
	CtrlEdgeRouterEnrollmentDurationVarName          = "ZT_ROUTER_ENROLLMENT_DURATION"
	CtrlEdgeRouterEnrollmentDurationVarDescription   = "The router enrollment duration in minutes"
	CtrlPkiEdgeCertVarName                           = "ZT_PKI_EDGE_CERT"
	CtrlPkiEdgeCertVarDescription                    = "Path to the controller's web identity client certificate"
	CtrlPkiEdgeServerCertVarName                     = "ZT_PKI_EDGE_SERVER_CERT"
	CtrlPkiEdgeServerCertVarDescription              = "Path to the controller's web identity server certificate, including partial chain"
	CtrlPkiEdgeKeyVarName                            = "ZT_PKI_EDGE_KEY"
	CtrlPkiEdgeKeyVarDescription                     = "Path to the controller's web identity private key"
	CtrlPkiEdgeCAVarName                             = "ZT_PKI_EDGE_CA"
	CtrlPkiEdgeCAVarDescription                      = "Path to the controller's web identity root CA cert"
	PkiAltServerCertVarName                          = "ZT_PKI_ALT_SERVER_CERT"
	PkiAltServerCertVarDescription                   = "Path to the controller's default identity alternative server certificate; requires ZT_PKI_ALT_SERVER_KEY"
	PkiAltServerKeyVarName                           = "ZT_PKI_ALT_SERVER_KEY"
	PkiAltServerKeyVarDescription                    = "Path to the controller's default identity alternative private key. Requires ZT_PKI_ALT_SERVER_CERT"
	RouterNameVarName                                = "ZT_ROUTER_NAME"
	RouterNameVarDescription                         = "A filename prefix for the router's key and certs"
	RouterPortVarName                                = "ZT_ROUTER_PORT"
	RouterPortVarDescription                         = "TCP port where the router listens for edge connections from endpoint identities"
	RouterIdentityCertVarName                        = "ZT_ROUTER_IDENTITY_CERT"
	RouterIdentityCertVarDescription                 = "Path to the router's client certificate"
	RouterIdentityServerCertVarName                  = "ZT_ROUTER_IDENTITY_SERVER_CERT"
	RouterIdentityServerCertVarDescription           = "Path to the router's server certificate"
	RouterIdentityKeyVarName                         = "ZT_ROUTER_IDENTITY_KEY"
	RouterIdentityKeyVarDescription                  = "Path to the router's private key"
	RouterIdentityCAVarName                          = "ZT_ROUTER_IDENTITY_CA"
	RouterIdentityCAVarDescription                   = "Path to the router's bundle of trusted root CA certs"
	RouterIPOverrideVarName                          = "ZT_ROUTER_IP_OVERRIDE"
	RouterIPOverrideVarDescription                   = "Additional IP SAN of the router"
	RouterAdvertisedAddressVarName                   = "ZT_ROUTER_ADVERTISED_ADDRESS"
	RouterAdvertisedAddressVarDescription            = "The router's advertised address and DNS SAN"
	RouterListenerBindPortVarName                    = "ZT_ROUTER_LISTENER_BIND_PORT"
	RouterListenerBindPortVarDescription             = "TCP port where the router will listen for and advertise links to other routers"
	RouterResolverVarName                            = "ZT_ROUTER_TPROXY_RESOLVER"
	RouterResolverVarDescription                     = "The bind URI to listen for DNS requests in tproxy mode"
	RouterDnsSvcIpRangeVarName                       = "ZT_ROUTER_DNS_IP_RANGE"
	RouterDnsSvcIpRangeVarDescription                = "The CIDR range to use for ZT DNS in tproxy mode"
	RouterCsrCVarName                                = "ZT_ROUTER_CSR_C"
	RouterCsrCVarDescription                         = "The country (C) to use for router CSRs"
	RouterCsrSTVarName                               = "ZT_ROUTER_CSR_ST"
	RouterCsrSTVarDescription                        = "The state/province (ST) to use for router CSRs"
	RouterCsrLVarName                                = "ZT_ROUTER_CSR_L"
	RouterCsrLVarDescription                         = "The locality (L) to use for router CSRs"
	RouterCsrOVarName                                = "ZT_ROUTER_CSR_O"
	RouterCsrOVarDescription                         = "The organization (O) to use for router CSRs"
	RouterCsrOUVarName                               = "ZT_ROUTER_CSR_OU"
	RouterCsrOUVarDescription                        = "The organization unit to use for router CSRs"
	RouterCsrSansDnsVarName                          = "ZT_ROUTER_CSR_SANS_DNS"
	RouterCsrSansDnsVarDescription                   = "Additional DNS SAN of the router"

	CliNetworkIdVarName        = "ZT_CLI_NETWORK_ID"
	CliNetworkIdVarDescription = "Necessary when using the CLI over a ztfied transport"
)
