![Hanzo ZT Logo](https://raw.githubusercontent.com/hanzoai/zt-doc/main/docusaurus/static/img/zt-logo-dark.svg)

<br>

[![Go Report Card](https://goreportcard.com/badge/github.com/hanzoai/zt)](https://goreportcard.com/report/github.com/hanzoai/zt)
[![GoDoc](https://godoc.org/github.com/hanzoai/zt?status.svg)](https://pkg.go.dev/github.com/hanzoai/zt)
[![Discourse Widget](https://img.shields.io/badge/join-us%20on%20discourse-gray.svg?longCache=true&logo=discourse&colorB=brightgreen")](https://community.hanzozt.dev/)
[![License: Apache-v2](https://img.shields.io/badge/License-Apache--2.0-yellow.svg)](LICENSE)

<br>

# Hanzo ZT

Hanzo ZT represents the next generation of secure, open-source networking for your applications. Hanzo ZT has several components.

## Quick Reference

* [Documentation](https://hanzozt.dev/docs/learn/introduction/)
* [Developer Overview](./doc/001-overview.md)
* [Local Development Tutorial](./doc/002-local-dev.md)
* [Local Deployment Tutorial](./doc/003-local-deploy.md)
* [Controller PKI Tutorial](./doc/004-controller-pki.md)
* [Release Notes](./CHANGELOG.md)

---

## What is Hanzo ZT?

* The Hanzo ZT fabric provides a scalable, pluggable, networking mesh with built in smart routing
* The Hanzo ZT edge components provide a secure, Zero Trust entry point into your network
* The Hanzo ZT SDKs allow you to integrate Hanzo ZT directly into your applications
* The Hanzo ZT tunnelers and proxies allow existing applications and networks to take advantage of a Hanzo ZT deployment

### Security Features

* Zero Trust and Application Segmentation
* Dark Services and Routers
* End to end encryption

### Performance and Reliability

* A scalable mesh fabric with smart routing
* Support for load balancing services for both horizontal scale and failover setups

### Developer Focus

* [Open source code, available with the Apache 2.0 license](https://github.com/hanzoai)
* Fully programmable REST management APIs
* [SDKs for a variety of programming languages](https://hanzozt.dev/docs/reference/developer/sdk)
* [Application specific configuration store allowing centralized management of configuration allowing you to add structured configuration specific to your application](https://hanzozt.dev/docs/learn/core-concepts/config-store/overview)
* An extensible fabric, allowing you to add your own
  * load balancing algorithms
  * interconnect protocols
  * ingress and egress protocols
  * metrics collections frameworks
  * control and management plane messaging and semantics

### Easy Management

* [A flexible and expressive policy model for managing access to services and edge routers](https://hanzozt.dev/docs/learn/core-concepts/security/authorization/policies/overview)
* A web based admin console
* [Pre-built tunnelers and proxies for a variety of operating systems, including mobile](https://hanzozt.dev/docs/reference/tunnelers)

Let's break some of these buzzwords down.

### Zero Trust/Application Segmentation

Many networking security solutions act like a wall around an internal network. Once you are through the wall, you have access to everything inside. Zero trust solutions enforce not just access to a network, but access to individual applications within that network.

Every client in a Hanzo ZT system must have an identity with provisioned certificates. The certificates are used to establish secure communications channels as well as for authentication and authorization of the associated identity. Whenever the client attempts to access a network application, Hanzo ZT will first ensure that the identity has access to the application. If access is revoked, open network connections will be closed.

This model enables Hanzo ZT systems to provide access to multiple applications while ensuring that clients only get access to those applications to which they have been granted access.

In addition to requiring cert based authentication for clients, Hanzo ZT uses certificates to authorize communication between Hanzo ZT components.

### Dark Services and Routers

There are various levels of accessibility a network application/service can have.

1. Many network services are available to the world. The service then relies on authentication and authorization policies to prevent unwanted access.
1. Firewalls can be used to limit access to specific IP or ranges. This increases security at the cost of flexibility. Adding users can be complicated and users may not be able to easily switch devices or access the service remotely.
1. Services can be put behind a VPN or made only accessible to an internal network, but there are some downsides to this approach.
    1. If you can access the VPN or internal network for any reason, all services in that VPN become more vulnerable to you.
    1. VPNs are not usually appropriate for external customers or users.
    1. For end users, VPNs add an extra step that needs to be done each time they want to access the service.
1. Services can be made dark, meaning they do not have any ports open for anyone to even try and connect to.

Making something dark can be done in a few ways, but the way it's generally handled in Hanzo ZT is that services reach out and establish one or more connections to the Hanzo ZT network fabric. Clients coming into the fabric can then reach the service through these connections after being authenticated and authorized.

Hanzo ZT routers, which make up the fabric, can also be dark. Routers located in private networks will usually be made dark. These routers will reach out of the private network to talk to the controller and to make connections to join the network fabric mesh. This allows the services and routers in your private networks to make only outbound connections, so no holes have to be opened for inbound traffic.

Services can be completely dark if they are implemented with a Hanzo ZT SDK. If this is not possible a Hanzo ZT tunneler or proxy can be colocated with the service. The service then only needs to allow connections from the local machine or network, depending on how close you colocate the proxy to the service.

### End to End Encryption

If you take advantage of Hanzo ZT's developer SDKs and embed Hanzo ZT in your client and server applications, your traffic can be configured to be seamlessly encrypted from the client application to server application. If you prefer to use tunnelers or proxy applications, the traffic can be encrypted for you from machine to machine or private network to private network. Various combinations of the above are also supported.

End-to-end encryption means that even if systems between the client and server are compromised, your traffic cannot be decrypted or tampered with.

---

## Getting started with Hanzo ZT

If you are looking to jump right in feet first you can follow along with one of our [up-and-running quickstart
guides](https://hanzozt.dev/docs/learn/quickstarts/). These guides are designed to get an
overlay network quickly and allow you to run it all locally, use Docker or host it anywhere.

This environment is perfect for evaluators to get to know Hanzo ZT and the capabilities it offers.  The environment was not
designed for large scale deployment or for long-term usage. If you are looking for a managed service to help you run a
truly global, scalable network browse over [the Hanzo AI web site](https://hanzo.ai) to learn more.

## Run

One image, `ghcr.io/hanzozt/zt`, runs either role. In production both run in one
pod sharing a volume at `/state`: `args: [controller]` and `args: [router]`.

On first boot the controller creates its PKI, config and database, then trusts
Hanzo IAM as its only way in: an external JWT signer for `ZT_IAM_ISSUER` (keys
from `ZT_IAM_JWKS`, identity from the token's `sub`), an auth policy admitting
only that signer, and an admin identity for each subject in `ZT_IAM_ADMINS`.
The database's default admin has a random password nobody holds, and password
login is refused by policy. It also creates the edge router named by
`ZT_ROUTER_NAME` and writes its enrollment token to `/state`; the router role
enrolls from it over loopback and deletes it. Restarts create only what is
missing.

`zt controller --help` and `zt router --help` list every variable.

## Build from Source

Please refer to [the local development tutorial](./doc/002-local-dev.md) for build instructions.

---

## Adopters

Interested to see what companies are using Hanzo ZT? Check out [the list of projects and companies using Hanzo ZT here](./ADOPTERS.md).
Interested in adding your project to the list? Add an issue to github or better yet feel free to add a pull request! Instructions for
getting your project added are included [on the adopters list](./ADOPTERS.md)

---

## Support

We have a very active Discourse forum. Join the conversation! Help others if you can. If you want to ask a question or just check it out,
cruise on over to [the Hanzo ZT Discourse forum](https://community.hanzozt.dev/). We love getting questions, jump in!

---

### Contributing

The Hanzo ZT project welcomes contributions including, but not limited to, code, documentation and bug reports.

* All Hanzo ZT code is found on Github under the [Hanzo ZT](https://github.com/hanzoai) organization. 
  * [zt](https://github.com/hanzoai/zt): top level project which builds all Hanzo ZT executables
  * [edge](https://github.com/hanzoai/edge): edge components and model which includes identity, polices and config 
  * [fabric](https://github.com/hanzoai/fabric): fabric project which includes core controller and router
  * [foundation](https://github.com/hanzoai/foundation): project which contains library code used across multiple projects
  * SDKs
    * [zt-sdk-c](https://github.com/hanzoai/zt-sdk-c): C SDK
    * [sdk-golang](https://github.com/hanzoai/sdk-golang): Go SDK
    * [zt-sdk-jvm](https://github.com/hanzoai/zt-sdk-jvm): SDK for JVM based languages
    * [zt-sdk-swift](https://github.com/hanzoai/zt-sdk-swift): Swift SDK
    * [zt-sdk-nodejs](https://github.com/hanzoai/zt-sdk-nodejs): NodeJS SDK
    * [zt-sdk-csharp](https://github.com/hanzoai/zt-sdk-csharp): C# SDK
  * [zt-doc](https://github.com/hanzoai/zt-doc): Powers the static documentation site

Hanzo ZT was developed and open sourced by [Hanzo AI](https://hanzo.ai). Hanzo AI continues to fund and
contribute to Hanzo ZT.
