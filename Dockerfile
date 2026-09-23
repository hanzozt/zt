# One image, two roles: `zt controller` and `zt router`, both configured from
# the environment (`zt controller --help`, `zt router --help`).
FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=0.0.0
ARG REVISION=local
RUN CGO_ENABLED=0 go build -trimpath -o /zt \
      -ldflags "-s -w -X github.com/hanzozt/zt/v2/common/version.Version=v${VERSION#v} -X github.com/hanzozt/zt/v2/common/version.Revision=${REVISION}" \
      ./zt \
 && mkdir /state

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /zt /usr/local/bin/zt
COPY --from=build --chown=65532:65532 /state /state
ENV ZT_HOME=/state
USER 65532:65532
WORKDIR /state
ENTRYPOINT ["/usr/local/bin/zt"]
