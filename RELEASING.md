# Releasing ZT

ZT ships one image, `ghcr.io/hanzozt/zt`, for `linux/amd64` and `linux/arm64`.
It is built the one way every repo in the estate is built: `/hanzo.yml` declares
the image and the gate, and `.github/workflows/cicd.yml` hands it to
`hanzoai/ci`, whose build door compiles each architecture natively in-cluster
and joins the two under one index.

- A push to `main` runs the gate (`test:` in `hanzo.yml`) and publishes the next
  patch after the highest of `version:` and what the registry already holds.
- A `v*` tag publishes exactly that version. Tag only a commit whose gate is green.

Before tagging, the gate should pass locally:

```bash
go vet ./...
go test ./...
```

The image runs either role from its environment; `zt controller --help` and
`zt router --help` list the variables each one reads.
