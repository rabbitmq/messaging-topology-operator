---
title: Star Certificate Support
authors:
  - "@acedres"
reviewers:
  - "@janedoe"
creation-date: 2022-01-05
last-updated: 2022-01-05
status: provisional ~~|implementable|implemented|deferred|rejected|withdrawn|replaced~~
see-also:
  - "/docs/proposals/20190101-we-heard-you-like-proposals.md"
  - "/docs/proposals/20190102-everyone-gets-a-proposal.md"
replaces:
  - "/docs/proposals/20181231-replaced-proposal.md"
superseded-by:
  - "/docs/proposals/20190104-superceding-proposal.md"
---

# Star Certificate Support
## Table of Contents

* [Star Certificate Support](#star-certificate-support)
  * [Table of Contents](#table-of-contents)
  * [Glossary](#glossary)
  * [Summary](#summary)
  * [Motivation](#motivation)
    * [Goals](#goals)
    * [Non-Goals/Future Work](#non-goalsfuture-work)
  * [Proposal](#proposal)
    * [User Stories](#user-stories)
      * [Internal Kubernetes Domain name is non-default](#internal-kubernetes-domain-name-is-non-default)
      * [RabbitMQ is exposed via Ingress](#rabbitmq-is-exposed-via-ingress)
    * [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
    * [Risks and Mitigations](#risks-and-mitigations)
  * [Alternatives](#alternatives)
  * [Upgrade Strategy](#upgrade-strategy)
  * [Additional Details](#additional-details)
    * [Test Plan [optional]](#test-plan-optional)
    * [Graduation Criteria [optional]](#graduation-criteria-optional)
    * [Version Skew Strategy [optional]](#version-skew-strategy-optional)
  * [Implementation History](#implementation-history)

Created by [gh-md-toc](https://github.com/ekalinin/github-markdown-toc)

## Glossary

Refer to the [Cluster API Book Glossary](https://cluster-api.sigs.k8s.io/reference/glossary.html).

## Summary

RabbitMQ is a critical component in some infrastructures. As a Data Service, it is expected to meet
security standards and interact using secure channels with other services or applications. It is
a common practice to use, exclusively, AMQP over TLS (AMQPs) for communication with RabbitMQ.
The Messaging Topology Operator must be able to function correctly in such environments. This
proposal outlines scenarios to consider and potential solutions to enable the Topology Operator
to function in mentioned environments.

## Motivation

The current implementation in the Topology Operator to communicate over a TLS channel with RabbitMQ
does not function in certain platform configurations. Ideally, users should have the flexibility to chose
how to configure their platform, and the Topology Operator should work in reasonable configurations.
It is almost impossible to support every possible scenario or platform configuration, however, users
should have the flexibility to "hint" or configure the Topology Operator, so that it functions correctly
in their platform configuration.

### Goals

- Design and document a solution to address the current limitation regarding secure communication.
- Outline possible scenarios and configurations to support.
- The proposed solution should work, regardless of how the certificate is provisioned.
- Provide a flexible solution that works in a variety on platform configurations.

### Non-Goals/Future Work

- Support for specific Service Meshes.
- A solution that only works if Cert Manager is present.

## Proposal

The Messaging Topology Operator constructs the URL to connect to RabbitMQ using the following pieces of
information:

- The service name from the `RabbitmqClusterReference`, as both match. 
- The namespace name from the `RabbitmqClusterReference` (with some allow-list).
- The protocol (HTTP or HTTPs) and port number from the Service's Ports.

The above pieces of information are combined to construct the URL, such that it results in 
`{protocol}://{service-name}.{namespace}.svc:{port}`. For example: `https://myrabbit.default.svc:15671`.

A user will configure an environment variable, in the Messaging Topology Operator Deployment, specifying
a domain name to append to the above URI. For example, an env var like `MESSAGING_DOMAIN_NAME`, with value
`.example.com` (or `example.com`), will be appended, so that the resulting URL to connect to RabbitMQ
would look like `https://myrabbit.default.svc.example.com:15671`. This will allow for client validation
when RabbitMQ presents a certificate valid for `*.example.com`.

![](../images/start-cert-internal-domain.jpg "Start Certificate Support with internal cluster domain")

In certain scenarios, the above solution won't be sufficient. For example, RabbitMQ can be exposed via
TCP Ingress to applications external to Kubernetes. In this scenario, RabbitMQ is provisioned with a
certificate to match the FQDN used to connect to the Ingress, for example `rabbitmq-example.my-domain.com`.
In this scenario, it is not possible to determine the URL from the pieces of information described above.
In order to satisfy this scenario, the Topology Operator needs the URL as input, per RabbitMQ instance.
The proposed solution is to use an annotation in `RabbitmqCluster`, with the URL to connect to as value.
For example, the annotation `messaging.rabbitmq.com/override-url` with value `"https://myrabbit.example.com"`.

![](../images/start-cert-support.jpg "Start Certificate Support in Messaging Topology Operator")

### User Stories

- Internal Kubernetes domain is non-default
- RabbitMQ is exposed via Ingress

#### Internal Kubernetes Domain name is non-default

Kubernetes internal domain [can be customised][kubelet-config] in the kubelet e.g. `.myk8s.local`.
In this setup, a start certificate, with SAN or CN `*.myk8s.local`, could be issued to a
RabbitMQ service. Any application connecting to an Operator-deployed RabbitMQ cluster should
trust the certificate (given that the certificate is signed by a trusted CA) presented by RabbitMQ.

```gherkin
Background:
  Given that RabbitMQ is deployed with the Cluster Operator
  And RabbitMQ only uses protocols over TLS
  And the certificate presented by RabbitMQ "rmq-cert" is signed by a trusted CA
  And RabbitMQ is accessible within Kubernetes via `myrabbit.default.svc.cluster.local`
  And RabbitMQ is accessible externally via Ingress with FQDN `myrabbit.example.com`

Scenario: RabbitMQ presents a star certificate for K8s cluster domain
  Given the certificate "rmq-cert" is valid for '*.default.svc.cluster.local'
  When an application connects to RabbitMQ within Kubernetes
  Then the application validates the certificate
  And creates a secure channel to communicate with RabbitMQ
```

#### RabbitMQ is exposed via Ingress

In addition to the internal domain, services in Kubernetes can be accessed from outside Kubernetes
(off-platform access or Ingress). Applications accessing Kubernetes via Ingress do not have access
to Kubernetes internal DNS service, therefore, are unable to resolve DNS requests for Kubernetes internal
domain name.

In this setup, the services running inside Kubernetes must present a certificate with SAN or CN matching
the hostname/FQDN presented by the Ingress e.g. `myrabbit-service.example.com`. It would be possible for
a certificate to include both internal and external hostnames/FQDNs in the SANs, however, this scenario
focuses on certificates with only the external hostname/FQDN in the SAN.

```gherkin
Background:
  Given that RabbitMQ is deployed with the Cluster Operator
  And RabbitMQ only uses protocols over TLS
  And the certificate presented by RabbitMQ "rmq-cert" is signed by a trusted CA
  And RabbitMQ is accessible within Kubernetes via `myrabbit.default.svc.cluster.local`
  And RabbitMQ is accessible externally via Ingress via `myrabbit.example.com`

Scenario: RabbitMQ presents a star certificate for Ingress domain name
  Given the certificate is valid for '*.example.com'
  When an application connects to RabbitMQ via Ingress
  Then the application validates the certificate
  And creates a secure channel to communicate with RabbitMQ
```

### Implementation Details/Notes/Constraints

The "override URL" annotation is a text input. This annotation could be used to make the Topology Operator
target any RabbitMQ service, accessible by the Topology Operator Pod. The Topology Operator should always
consume the credentials from the `RabbitmqCluster` reference. Attempting to inject credentials in the URL
should not be permitted, and such credentials should be ignored. For example
`https://bad_actor:password123@some-rabbit.example.com:15671` is a valid URL, however, the Topology Operator
should not use those credentials, but sanitise the input, drop the credentials part, and connect to RabbitMQ
using the credentials provided in the default user Secret.

By consuming an environment variable and the value of an annotation, we are consuming user input. This input
must be validated and sanitised before it's used in the `RabbitHole` Client.

### Risks and Mitigations

This proposal does not add any new fields to any of the Specs of the CRDs owned by the Topology Operator. There
are no maintenance risks on that regards.

There aren't any foreseeable upgrade risks because this proposal adds an env var and an annotation to the
current workflow. If used in previous versions, those simply won't have any effect. It is important to keep
in mind that default behaviour should not be changed.

## Alternatives

An alternative solution is to "fix it in the docs", and be more restrictive about the Topology Operator
expectations or requirements on the certificate presented by RabbitMQ. However, this alternative, although
cheap, it might not work for some users, given that control over the certificate generation is not always
controlled by the same persona who uses `RabbitmqCluster` and topology objects.

Another alternative could be to add an environment variable to disable certificate verification. This
alternative simply workarounds the problem, whilst reducing significantly the security and trust in the
Topology Operator. This feature could be useful in certain scenarios, for example, in a local development
environment, with self-signed certs. However, such behaviour should not be part of this proposal.

## Upgrade Strategy

When testing this change, ensure the following requirements are met:

- [ ] Default behaviour does not change.
- [ ] Upgrade of Topology Operator does not cause a RabbitMQ rolling restart.

## Additional Details

### Test Plan [optional]

**Note:** *Section not required until targeted at a release.*

Consider the following in developing a test plan for this enhancement:
- Will there be e2e and integration tests, in addition to unit tests?
- How will it be tested in isolation vs with other components?

No need to outline all of the test cases, just the general strategy.
Anything that would count as tricky in the implementation and anything particularly challenging to test should be called out.

All code is expected to have adequate tests (eventually with coverage expectations).
Please adhere to the [Kubernetes testing guidelines][testing-guidelines] when drafting this test plan.

### Graduation Criteria [optional]

<!--
**Note:** *Section not required until targeted at a release.*

Define graduation milestones.

These may be defined in terms of API maturity, or as something else. Initial proposal should keep
this high-level with a focus on what signals will be looked at to determine graduation.

Consider the following in developing the graduation criteria for this enhancement:
- [Maturity levels (`alpha`, `beta`, `stable`)][maturity-levels]
- [Deprecation policy][deprecation-policy]

Clearly define what graduation means by either linking to the [API doc definition](https://kubernetes.io/docs/concepts/overview/kubernetes-api/#api-versioning),
or by redefining what graduation means.

In general, we try to use the same stages (alpha, beta, GA), regardless how the functionality is accessed.

[maturity-levels]: https://git.k8s.io/community/contributors/devel/sig-architecture/api_changes.md#alpha-beta-and-stable-versions
[deprecation-policy]: https://kubernetes.io/docs/reference/using-api/deprecation-policy/
-->

Not applicable. The change is arguably a bug fix.

### Version Skew Strategy [optional]

<!--
If applicable, how will the component handle version skew with other components? What are the guarantees? Make sure
this is in the test plan.

Consider the following in developing a version skew strategy for this enhancement:
- Does this enhancement involve coordinating behavior in the control plane and in the kubelet? How does an n-2 kubelet without this feature available behave when this feature is used?
- Will any other components on the node change? For example, changes to CSI, CRI or CNI may require updating that component before the kubelet.
-->

Not applicable. This section could be removed before the proposal is accepted.

## Implementation History

- [ ] MM/DD/YYYY: Proposed idea in an issue or [community meeting]
- [ ] MM/DD/YYYY: Compile a Google Doc following the CAEP template (link here)
- [ ] MM/DD/YYYY: First round of feedback from community
- [ ] MM/DD/YYYY: Present proposal at a [community meeting]
- [ ] MM/DD/YYYY: Open proposal PR

<!-- Links -->
[community meeting]: https://docs.google.com/document/d/1Ys-DOR5UsgbMEeciuG0HOgDQc8kZsaWIWJeKJ1-UfbY
[kubelet-config]: https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/#options
[testing-guidelines]: https://git.k8s.io/community/contributors/devel/sig-testing/testing.md
