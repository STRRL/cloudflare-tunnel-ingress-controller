---
title: Architecture
description: Understand how Kubernetes Ingress state becomes Cloudflare Tunnel routes, DNS records, and running cloudflared connectors.
---

Cloudflare Tunnel Ingress Controller connects two control planes. Kubernetes Ingress resources describe which Services should be exposed, while Cloudflare holds the public DNS records and tunnel routing configuration. The controller continuously translates the Kubernetes view into the Cloudflare view.

Traffic does not pass through the controller itself. The controller manages configuration. The `cloudflared` connectors carry traffic from Cloudflare into the cluster.

## From Ingress to tunnel route

`IngressController` watches Kubernetes Ingress resources and selects those assigned to its Ingress class. When one changes, the controller reads all controlled Ingress resources again. This full view matters because the Cloudflare tunnel configuration is one ordered list of ingress rules, rather than one independent object per Kubernetes Ingress.

Each host and path is transformed into an `Exposure`. An Exposure is the internal boundary between Kubernetes and Cloudflare. It contains the public hostname, path prefix, and a Service target such as `http://my-service.default.svc.cluster.local:8080`. It also carries origin options derived from annotations.

`TunnelClient` turns the active Exposures into Cloudflare tunnel ingress rules. It orders specific hostnames before wildcard hostnames and longer paths before shorter paths, then adds a final rule that returns HTTP 404. If the resulting rule list differs from the current remote tunnel configuration, the client updates Cloudflare.

This produces two related flows:

```text
Configuration:
Ingress -> IngressController -> Exposure -> TunnelClient -> Cloudflare tunnel rules

Traffic:
Client -> Cloudflare edge -> tunnel -> cloudflared -> Kubernetes Service
```

The separation is intentional. Ingress reconciliation can update routes without placing the controller in the request path. `cloudflared` maintains outbound tunnel connections to Cloudflare and forwards each request to the Service target selected by the matching tunnel rule.

See the [Ingress reference](/reference/ingress/) for supported route behavior and validation rules.

## DNS and ownership

For each active hostname, the controller normally manages two Cloudflare DNS records:

* A proxied CNAME from the public hostname to `<tunnel-id>.cfargotunnel.com`
* A TXT record named `_ctic_managed.<hostname>` that identifies this controller and tunnel

The CNAME sends public traffic toward the tunnel. The TXT record records ownership. Ownership lets the controller remove records when an Exposure disappears without treating every CNAME in the zone as its own. A matching ownership record is required before normal cleanup deletes a CNAME.

The `disable-dns-management` annotation changes only the DNS side of reconciliation. The Exposure still becomes a tunnel ingress rule, but the controller stops creating or updating its CNAME and ownership TXT records. This allows another system, such as external-dns or a Cloudflare Load Balancer, to manage how the hostname reaches the tunnel. It also allows the hostname to live outside the Cloudflare zones visible to the controller.

If DNS was previously managed by this controller, enabling the annotation relinquishes that ownership. The controller removes its TXT record. It removes the CNAME only when the record still points to this tunnel, preserving a CNAME that another system has already repointed.

See the [Ingress annotations reference](/reference/ingress-annotations/) for annotation syntax and related origin settings.

## Keeping cloudflared running

`ControlledCloudflaredConnector` is a separate reconciliation loop. After this controller instance becomes the elected leader, the loop runs every 10 seconds. Each pass fetches the tunnel token and compares the managed Kubernetes Secret and Deployment with the desired connector configuration.

The loop creates the connector resources when they do not exist. When they drift, it updates settings such as the image, replica count, command, token Secret version, and pod customization. Kubernetes then rolls out the resulting Deployment changes.

The managed Deployment runs `cloudflared tunnel run` with the tunnel token from the Secret. Those connector pods establish the tunnel connections that carry traffic. Rechecking every 10 seconds makes the connector Deployment self healing even when it is changed independently of an Ingress event.

Connector settings belong in configuration rather than this explanation. See [Controller Configuration](/reference/controller-configuration/) and [Helm Values](/reference/helm-values/) for the available controls.
