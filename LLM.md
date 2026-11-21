# zt

Hanzo's fork of OpenZiti: one image, `ghcr.io/hanzoai/zt`, runs either role
(`zt controller`, `zt router`) from its environment. `zt controller --help`
lists the variables. Releases: RELEASING.md.

## Controller listeners

The controller's `ctrl.yaml` is written once, at first boot, with one web
listener (`client-management`) that serves every API. Every boot then converges
what the environment adds to it:

- `ZT_CTRL_EDGE_ALT_ADVERTISED_ADDRESS`: more names, comma-separated, put on the
  server certificate (`zt/run/certnames`: same key, same CA, new serial).
- `ZT_CTRL_EDGE_CLIENT_PORT`: a second listener, `client`, on that port at the
  first of those names. It serves `edge-client` and `edge-oidc` alone
  (`zt/run/listener`): what an identity signs in with, since current SDKs
  authenticate against the OIDC provider. The management and fabric APIs stay on
  the first listener. It exists for clients
  that sign in with a certificate and so cannot cross a TLS-terminating proxy:
  they reach this name with the proxy out of the path, and the management API
  never is on it.

Neither variable set, nothing changes. Hanzo's production values and routes:
`hanzo-inc/universe` `infra/aws/k8s/zt.yaml` and `infra/aws/routes`.
