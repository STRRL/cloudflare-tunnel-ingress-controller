---
title: I Built a Cloudflare Tunnel Ingress Controller So I Could Access My Homelab's Emby Server from outside
date: 2025-10-27
authors:
  - strrl
tags:
  - story
  - cloudflare
  - kubernetes
  - homelab
excerpt: The story behind Cloudflare Tunnel Ingress Controller.
---

## My Homelab Journey

I'm a homelab enthusiast. At one point, I had a 5-machine Kubernetes cluster running in my home. Like many homelab users, I faced a common challenge: I didn't have a public IP address, but I needed to access my services from outside my home network, particularly my Emby media server.

## Discovering Cloudflare Tunnel

My initial solution was to manually configure Cloudflare Tunnel for each service I wanted to expose. It worked beautifully. The tunnels were reliable, secure, and didn't require me to open ports on my router or deal with dynamic DNS.

## The Aha Moment

As I worked more with both Cloudflare Tunnel and Kubernetes, I realized something interesting: **Cloudflare Tunnel's concept maps almost 1:1 with Kubernetes Ingress**. Both are about routing external traffic to internal services. The similarity was striking.

Having previously worked on [Chaos Mesh](https://github.com/chaos-mesh/chaos-mesh), I had extensive experience with Kubernetes controllers. I started thinking: wouldn't it be great to have a controller that automatically manages Cloudflare Tunnels based on Kubernetes Ingress resources?

## The Official Project's Status

I looked at the official [cloudflare/cloudflare-ingress-controller](https://github.com/cloudflare/cloudflare-ingress-controller) project, but it had become inactive. Unfortunately, the project was eventually archived.

## Starting Fresh

After consulting with Jintao from the ingress-nginx team, I decided to start this project from scratch. With my controller development experience from Chaos Mesh, the development went smoothly.

The project quickly matured and became stable. Version 0.0.15, 0.0.18 have been running reliably for a long time, serving users who face similar challenges with their homelabs and Kubernetes clusters.

## Design Philosophy: Simplicity First

From the beginning, I had a clear design goal: **make it simple and work out of the box**.

The only prerequisite? Know Kubernetes. That's it.

Kubernetes has elegantly designed abstractions for traffic routing - Ingress and Gateway API. These resources clearly express the intent: "I want to expose this service to the outside world with these rules." If we can provide a simple implementation that just works, without requiring deep knowledge of cloud providers, load balancers, or networking complexities, it becomes incredibly valuable for small and medium-sized teams.

As DevOps practitioners, we're always learning new tools and technologies. After exploring countless products on the market, we all eventually crave simplicity. Many teams already understand Kubernetes concepts and know how to write an Ingress manifest. With this controller, you define your Ingress the same way you always have. The controller handles the Cloudflare Tunnel magic behind the scenes.

This simplicity is especially powerful for:

- **Homelab users** who want Kubernetes-native traffic management
- **Small teams** who don't have dedicated DevOps engineers
- **Startups** that need to move fast without infrastructure complexity
- **Anyone** who prefers declarative configuration over manual setup

The goal isn't to build the most feature-rich controller with every possible option. It's to solve one problem really well: automatically managing Cloudflare Tunnels based on Kubernetes resources you already understand.

## See It In Action

If this concept makes sense to you, or if you're still not quite sure what it can do, here's a video showing how quickly and smoothly it works.

Watch how you can bootstrap a local Kubernetes cluster, configure any web service (in this case, Kubernetes Dashboard), and expose it to the internet through Cloudflare Tunnel Ingress Controller - all without a public IP address and without a cloud provider load balancer:

[![Less than 4 minutes! Bootstrap a Kubernetes Cluster and Expose Kubernetes Dashboard to the Internet.](https://markdown-videos.vercel.app/youtube/e-ARlEnS4zQ)](http://www.youtube.com/watch?v=e-ARlEnS4zQ "Less than 4 minutes! Bootstrap a Kubernetes Cluster and Expose Kubernetes Dashboard to the Internet.")

The entire process takes less than 4 minutes. That's the kind of simplicity I'm talking about.

## 1000 Stars and Beyond

Recently, the project reached 1000 stars on GitHub, which was both humbling and exciting. This milestone made me realize there's a real community of users who find this project valuable.

What's interesting is that during this time, I barely updated the project. It's been running at versions 0.0.15 and 0.0.18 for a long time - old and stable, just working quietly in the background. Sometimes the best code is the code that just works and doesn't need constant updates. But reaching 1000 stars made me realize it's time to invest more energy into making it even better for this growing community.

## What's Next

With this growing community, I want to focus on:

1. **Improving Documentation**: Making it easier for new users to get started and reducing the learning curve

2. **Enhanced Observability**: Adding traffic monitoring and metrics to help users understand their tunnel usage

3. **Gateway API Support**: Exploring integration with the Kubernetes Gateway API as it becomes more mature

4. **Zero Trust Integration**: Investigating how to integrate with Cloudflare Zero Trust authentication for enhanced security

## The Vision

This project started from a personal need - a homelab enthusiast wanting to access Emby from outside. It evolved into something that helps thousands of users solve similar problems. That's what makes open source beautiful.

If you're running a homelab or managing Kubernetes clusters without public IPs, I hope this project makes your life easier. And if you have ideas or suggestions, I'd love to hear from you on [GitHub](https://github.com/strrl/cloudflare-tunnel-ingress-controller).

## How Are You Using It?

If you've made it this far, you might be interested in learning how others are using this project. Or perhaps you'd like to share your own use case?

I've created a discussion thread where users share their stories, configurations, and creative solutions: [**Show and Tell: How are you using cloudflare-tunnel-ingress-controller?**](https://github.com/STRRL/cloudflare-tunnel-ingress-controller/issues/193)

Whether you're running a personal homelab, managing a production cluster, or doing something completely unique - I'd love to hear about it. Your story might inspire others or help someone solve a similar problem.

---

_This is the beginning of a new chapter for this project. Stay tuned for more updates as we explore new possibilities together._
