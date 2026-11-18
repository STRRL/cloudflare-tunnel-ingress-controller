package cloudflarecontroller

import (
	"reflect"
	"strings"
	"testing"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go"
	"github.com/go-logr/logr"
)

const WhateverTunnelId = "whatever"
const WhateverTunnelDomain = "whatever.cfargotunnel.com"

func mustRenderTXTContent(tunnelName string) string {
	content, err := renderTXTContent(tunnelName)
	if err != nil {
		panic(err)
	}
	return content
}

func Test_syncDNSRecord(t *testing.T) {
	type args struct {
		logger              logr.Logger
		exposures           []exposure.Exposure
		existedCNAMERecords []cloudflare.DNSRecord
		existedTXTRecords   []cloudflare.DNSRecord
		tunnelId            string
		tunnelName          string
	}
	var tests = []struct {
		name       string
		args       args
		wantCreate []DNSOperationCreate
		wantUpdate []DNSOperationUpdate
		wantDelete []DNSOperationDelete
		wantErr    bool
	}{
		{
			name: "noop",
			args: args{
				logger:              logr.Discard(),
				exposures:           nil,
				existedCNAMERecords: nil,
				existedTXTRecords:   nil,
				tunnelId:            WhateverTunnelId,
			},
			wantCreate: nil,
			wantUpdate: nil,
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "create new exposure",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:      "test.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     false,
					},
				},
				existedCNAMERecords: nil,
				existedTXTRecords:   nil,
				tunnelId:            WhateverTunnelId,
				tunnelName:          "tunnel-in-test",
			},
			wantCreate: []DNSOperationCreate{
				{
					Hostname: "test.example.com",
					Type:     "CNAME",
					Content:  WhateverTunnelDomain,
				},
				{
					Hostname: "_ctic_managed.test.example.com",
					Type:     "TXT",
					Content:  mustRenderTXTContent("tunnel-in-test"),
				},
			},
			wantUpdate: nil,
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "ignore deleted exposure",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:      "test.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     true,
					},
					{
						Hostname:      "test2.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     false,
					},
				},
				existedCNAMERecords: nil,
				existedTXTRecords:   nil,
				tunnelId:            WhateverTunnelId,
				tunnelName:          "tunnel-in-test",
			},
			wantCreate: []DNSOperationCreate{
				{
					Hostname: "test2.example.com",
					Type:     "CNAME",
					Content:  WhateverTunnelDomain,
				},
				{
					Hostname: "_ctic_managed.test2.example.com",
					Type:     "TXT",
					Content:  mustRenderTXTContent("tunnel-in-test"),
				},
			},
			wantUpdate: nil,
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "only delete managed record with matching TXT",
			args: args{
				logger:    logr.Discard(),
				exposures: nil,
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: "another.example.com",
					},
					{
						Name:    "test2.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
				},
				existedTXTRecords: []cloudflare.DNSRecord{
					{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("tunnel-in-test"),
					},
				},
				tunnelId:   WhateverTunnelId,
				tunnelName: "tunnel-in-test",
			},
			wantCreate: nil,
			wantUpdate: nil,
			wantDelete: []DNSOperationDelete{
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: "another.example.com",
					},
				},
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("tunnel-in-test"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "update existing exposure and create TXT",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:      "test.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     false,
					},
				},
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "A",
						Content: "1.2.3.4",
					},
				},
				existedTXTRecords: nil,
				tunnelId:          WhateverTunnelId,
				tunnelName:        "tunnel-in-test",
			},
			wantCreate: []DNSOperationCreate{
				{
					Hostname: "_ctic_managed.test.example.com",
					Type:     "TXT",
					Content:  mustRenderTXTContent("tunnel-in-test"),
				},
			},
			wantUpdate: []DNSOperationUpdate{
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "test.example.com",
						Type:    "A",
						Content: "1.2.3.4",
					},
					Type:    "CNAME",
					Content: WhateverTunnelDomain,
				},
			},
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "delete unused exposure with TXT",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:      "test.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     true,
					},
				},
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
				},
				existedTXTRecords: []cloudflare.DNSRecord{
					{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("tunnel-in-test"),
					},
				},
				tunnelId:   WhateverTunnelId,
				tunnelName: "tunnel-in-test",
			},
			wantCreate: nil,
			wantUpdate: nil,
			wantDelete: []DNSOperationDelete{
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
				},
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("tunnel-in-test"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "always update existing record with TXT",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:      "test.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     false,
					},
				},
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
				},
				existedTXTRecords: []cloudflare.DNSRecord{
					{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("tunnel-in-test"),
					},
				},
				tunnelId:   WhateverTunnelId,
				tunnelName: "tunnel-in-test",
			},
			wantCreate: nil,
			wantUpdate: []DNSOperationUpdate{
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
					Type:    "CNAME",
					Content: WhateverTunnelDomain,
				},
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("tunnel-in-test"),
					},
					Type:    "TXT",
					Content: mustRenderTXTContent("tunnel-in-test"),
				},
			},
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "rewrites unquoted ownership TXT to RFC 1035 quoted form",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:      "test.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     false,
					},
				},
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
				},
				existedTXTRecords: []cloudflare.DNSRecord{
					{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: `{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"tunnel-in-test"}`,
					},
				},
				tunnelId:   WhateverTunnelId,
				tunnelName: "tunnel-in-test",
			},
			wantCreate: nil,
			wantUpdate: []DNSOperationUpdate{
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
					Type:    "CNAME",
					Content: WhateverTunnelDomain,
				},
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: `{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"tunnel-in-test"}`,
					},
					Type:    "TXT",
					Content: mustRenderTXTContent("tunnel-in-test"),
				},
			},
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "do not delete CNAME managed by different tunnel",
			args: args{
				logger:    logr.Discard(),
				exposures: nil,
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: "another-tunnel.cfargotunnel.com",
					},
				},
				existedTXTRecords: []cloudflare.DNSRecord{
					{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("different-tunnel"),
					},
				},
				tunnelId:   "current-tunnel-id",
				tunnelName: "current-tunnel",
			},
			wantCreate: nil,
			wantUpdate: nil,
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "exposure with DNS management disabled creates no records",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:             "test.example.com",
						ServiceTarget:        "http://10.0.0.1:233",
						PathPrefix:           "/",
						IsDeleted:            false,
						DisableDNSManagement: true,
					},
				},
				existedCNAMERecords: nil,
				existedTXTRecords:   nil,
				tunnelId:            WhateverTunnelId,
				tunnelName:          "tunnel-in-test",
			},
			wantCreate: nil,
			wantUpdate: nil,
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "exposure with DNS management disabled relinquishes owned records",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:             "test.example.com",
						ServiceTarget:        "http://10.0.0.1:233",
						PathPrefix:           "/",
						IsDeleted:            false,
						DisableDNSManagement: true,
					},
				},
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
				},
				existedTXTRecords: []cloudflare.DNSRecord{
					{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("tunnel-in-test"),
					},
				},
				tunnelId:   WhateverTunnelId,
				tunnelName: "tunnel-in-test",
			},
			wantCreate: nil,
			wantUpdate: nil,
			wantDelete: []DNSOperationDelete{
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
				},
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("tunnel-in-test"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "exposure with DNS management disabled keeps a repointed CNAME and only drops the ownership TXT",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:             "test.example.com",
						ServiceTarget:        "http://10.0.0.1:233",
						PathPrefix:           "/",
						IsDeleted:            false,
						DisableDNSManagement: true,
					},
				},
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: "load-balancer.example.net",
					},
				},
				existedTXTRecords: []cloudflare.DNSRecord{
					{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("tunnel-in-test"),
					},
				},
				tunnelId:   WhateverTunnelId,
				tunnelName: "tunnel-in-test",
			},
			wantCreate: nil,
			wantUpdate: nil,
			wantDelete: []DNSOperationDelete{
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("tunnel-in-test"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "exposure with DNS management disabled ignores records owned by another tunnel",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:             "test.example.com",
						ServiceTarget:        "http://10.0.0.1:233",
						PathPrefix:           "/",
						IsDeleted:            false,
						DisableDNSManagement: true,
					},
				},
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: "other-tunnel.cfargotunnel.com",
					},
				},
				existedTXTRecords: []cloudflare.DNSRecord{
					{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("another-tunnel"),
					},
				},
				tunnelId:   WhateverTunnelId,
				tunnelName: "tunnel-in-test",
			},
			wantCreate: nil,
			wantUpdate: nil,
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "wildcard hostname uses wildcard-safe TXT name",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:      "*.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     false,
					},
				},
				existedCNAMERecords: nil,
				existedTXTRecords:   nil,
				tunnelId:            WhateverTunnelId,
				tunnelName:          "tunnel-in-test",
			},
			wantCreate: []DNSOperationCreate{
				{
					Hostname: "*.example.com",
					Type:     "CNAME",
					Content:  WhateverTunnelDomain,
				},
				{
					Hostname: "_ctic_managed._wildcard.example.com",
					Type:     "TXT",
					Content:  mustRenderTXTContent("tunnel-in-test"),
				},
			},
			wantUpdate: nil,
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "migrates unquoted asterisk TXT name to quoted _wildcard name",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:      "*.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     false,
					},
				},
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "*.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
				},
				existedTXTRecords: []cloudflare.DNSRecord{
					{
						Name:    "_ctic_managed.*.example.com",
						Type:    "TXT",
						Content: `{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"tunnel-in-test"}`,
					},
				},
				tunnelId:   WhateverTunnelId,
				tunnelName: "tunnel-in-test",
			},
			wantCreate: []DNSOperationCreate{
				{
					Hostname: "_ctic_managed._wildcard.example.com",
					Type:     "TXT",
					Content:  mustRenderTXTContent("tunnel-in-test"),
				},
			},
			wantUpdate: []DNSOperationUpdate{
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "*.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
					Type:    "CNAME",
					Content: WhateverTunnelDomain,
				},
			},
			wantDelete: []DNSOperationDelete{
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "_ctic_managed.*.example.com",
						Type:    "TXT",
						Content: `{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"tunnel-in-test"}`,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "deduplicates wildcard migration operations for multiple paths",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:      "*.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/api",
						IsDeleted:     false,
					},
					{
						Hostname:      "*.example.com",
						ServiceTarget: "http://10.0.0.2:233",
						PathPrefix:    "/",
						IsDeleted:     false,
					},
				},
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						ID:      "wildcard-cname",
						Name:    "*.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
				},
				existedTXTRecords: []cloudflare.DNSRecord{
					{
						ID:      "legacy-wildcard-txt",
						Name:    "_ctic_managed.*.example.com",
						Type:    "TXT",
						Content: `{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"tunnel-in-test"}`,
					},
				},
				tunnelId:   WhateverTunnelId,
				tunnelName: "tunnel-in-test",
			},
			wantCreate: []DNSOperationCreate{
				{
					Hostname: "_ctic_managed._wildcard.example.com",
					Type:     "TXT",
					Content:  mustRenderTXTContent("tunnel-in-test"),
				},
			},
			wantUpdate: []DNSOperationUpdate{
				{
					OldRecord: cloudflare.DNSRecord{
						ID:      "wildcard-cname",
						Name:    "*.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
					Type:    "CNAME",
					Content: WhateverTunnelDomain,
				},
			},
			wantDelete: []DNSOperationDelete{
				{
					OldRecord: cloudflare.DNSRecord{
						ID:      "legacy-wildcard-txt",
						Name:    "_ctic_managed.*.example.com",
						Type:    "TXT",
						Content: `{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"tunnel-in-test"}`,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "unquoted legacy TXT content still proves ownership",
			args: args{
				logger:    logr.Discard(),
				exposures: nil,
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
				},
				existedTXTRecords: []cloudflare.DNSRecord{
					{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: `{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"tunnel-in-test"}`,
					},
				},
				tunnelId:   WhateverTunnelId,
				tunnelName: "tunnel-in-test",
			},
			wantCreate: nil,
			wantUpdate: nil,
			wantDelete: []DNSOperationDelete{
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
				},
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: `{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"tunnel-in-test"}`,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "deletes wildcard CNAME using wildcard-safe TXT name",
			args: args{
				logger:    logr.Discard(),
				exposures: nil,
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "*.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
				},
				existedTXTRecords: []cloudflare.DNSRecord{
					{
						Name:    "_ctic_managed._wildcard.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("tunnel-in-test"),
					},
				},
				tunnelId:   WhateverTunnelId,
				tunnelName: "tunnel-in-test",
			},
			wantCreate: nil,
			wantUpdate: nil,
			wantDelete: []DNSOperationDelete{
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "*.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
				},
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "_ctic_managed._wildcard.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("tunnel-in-test"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "wildcard and literal wildcard hostname get distinct TXT names",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:      "*.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     false,
					},
					{
						Hostname:      "wildcard.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     false,
					},
				},
				existedCNAMERecords: nil,
				existedTXTRecords:   nil,
				tunnelId:            WhateverTunnelId,
				tunnelName:          "tunnel-in-test",
			},
			wantCreate: []DNSOperationCreate{
				{
					Hostname: "*.example.com",
					Type:     "CNAME",
					Content:  WhateverTunnelDomain,
				},
				{
					Hostname: "_ctic_managed._wildcard.example.com",
					Type:     "TXT",
					Content:  mustRenderTXTContent("tunnel-in-test"),
				},
				{
					Hostname: "wildcard.example.com",
					Type:     "CNAME",
					Content:  WhateverTunnelDomain,
				},
				{
					Hostname: "_ctic_managed.wildcard.example.com",
					Type:     "TXT",
					Content:  mustRenderTXTContent("tunnel-in-test"),
				},
			},
			wantUpdate: nil,
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "wildcard exposure with DNS management disabled relinquishes the legacy asterisk TXT",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:             "*.example.com",
						ServiceTarget:        "http://10.0.0.1:233",
						PathPrefix:           "/",
						IsDeleted:            false,
						DisableDNSManagement: true,
					},
				},
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "*.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
				},
				existedTXTRecords: []cloudflare.DNSRecord{
					{
						Name:    "_ctic_managed.*.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("tunnel-in-test"),
					},
				},
				tunnelId:   WhateverTunnelId,
				tunnelName: "tunnel-in-test",
			},
			wantCreate: nil,
			wantUpdate: nil,
			wantDelete: []DNSOperationDelete{
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "*.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
					},
				},
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "_ctic_managed.*.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("tunnel-in-test"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "legacy asterisk TXT owned by another tunnel is left untouched",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:      "*.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     false,
					},
				},
				existedCNAMERecords: nil,
				existedTXTRecords: []cloudflare.DNSRecord{
					{
						Name:    "_ctic_managed.*.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("another-tunnel"),
					},
				},
				tunnelId:   WhateverTunnelId,
				tunnelName: "tunnel-in-test",
			},
			wantCreate: []DNSOperationCreate{
				{
					Hostname: "*.example.com",
					Type:     "CNAME",
					Content:  WhateverTunnelDomain,
				},
				{
					Hostname: "_ctic_managed._wildcard.example.com",
					Type:     "TXT",
					Content:  mustRenderTXTContent("tunnel-in-test"),
				},
			},
			wantUpdate: nil,
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "hostname exposed on several paths yields one operation per record",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:      "test.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/api",
						IsDeleted:     false,
					},
					{
						Hostname:      "test.example.com",
						ServiceTarget: "http://10.0.0.2:233",
						PathPrefix:    "/",
						IsDeleted:     false,
					},
				},
				existedCNAMERecords: nil,
				existedTXTRecords:   nil,
				tunnelId:            WhateverTunnelId,
				tunnelName:          "tunnel-in-test",
			},
			wantCreate: []DNSOperationCreate{
				{
					Hostname: "test.example.com",
					Type:     "CNAME",
					Content:  WhateverTunnelDomain,
				},
				{
					Hostname: "_ctic_managed.test.example.com",
					Type:     "TXT",
					Content:  mustRenderTXTContent("tunnel-in-test"),
				},
			},
			wantUpdate: nil,
			wantDelete: nil,
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCreate, gotUpdate, gotDelete, err := syncDNSRecord(
				tt.args.logger,
				tt.args.exposures,
				tt.args.existedCNAMERecords,
				tt.args.existedTXTRecords,
				tt.args.tunnelId,
				tt.args.tunnelName,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("syncDNSRecord() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotCreate, tt.wantCreate) {
				t.Errorf("syncDNSRecord() gotCreate = %v, want %v", gotCreate, tt.wantCreate)
			}
			if !reflect.DeepEqual(gotUpdate, tt.wantUpdate) {
				t.Errorf("syncDNSRecord() gotUpdate = %v, want %v", gotUpdate, tt.wantUpdate)
			}
			if !reflect.DeepEqual(gotDelete, tt.wantDelete) {
				t.Errorf("syncDNSRecord() gotDelete = %v, want %v", gotDelete, tt.wantDelete)
			}
		})
	}
}

func Test_migrateLegacyDNSRecords(t *testing.T) {
	type args struct {
		logger              logr.Logger
		exposures           []exposure.Exposure
		existedCNAMERecords []cloudflare.DNSRecord
		existedTXTRecords   []cloudflare.DNSRecord
		tunnelName          string
	}
	tests := []struct {
		name       string
		args       args
		wantDelete []DNSOperationDelete
	}{
		{
			name: "delete legacy comment-based record without TXT",
			args: args{
				logger:    logr.Discard(),
				exposures: nil,
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
						Comment: "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [tunnel-in-test]",
					},
				},
				existedTXTRecords: nil,
				tunnelName:        "tunnel-in-test",
			},
			wantDelete: []DNSOperationDelete{
				{
					OldRecord: cloudflare.DNSRecord{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
						Comment: "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [tunnel-in-test]",
					},
				},
			},
		},
		{
			name: "do not delete legacy record from different tunnel",
			args: args{
				logger:    logr.Discard(),
				exposures: nil,
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: "other-tunnel.cfargotunnel.com",
						Comment: "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [other-tunnel]",
					},
				},
				existedTXTRecords: nil,
				tunnelName:        "tunnel-in-test",
			},
			wantDelete: nil,
		},
		{
			name: "skip legacy record still in active exposures",
			args: args{
				logger: logr.Discard(),
				exposures: []exposure.Exposure{
					{
						Hostname:      "test.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     false,
					},
				},
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
						Comment: "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [tunnel-in-test]",
					},
				},
				existedTXTRecords: nil,
				tunnelName:        "tunnel-in-test",
			},
			wantDelete: nil,
		},
		{
			name: "skip record already tracked by TXT",
			args: args{
				logger:    logr.Discard(),
				exposures: nil,
				existedCNAMERecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
						Comment: "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [tunnel-in-test]",
					},
				},
				existedTXTRecords: []cloudflare.DNSRecord{
					{
						Name:    "_ctic_managed.test.example.com",
						Type:    "TXT",
						Content: mustRenderTXTContent("tunnel-in-test"),
					},
				},
				tunnelName: "tunnel-in-test",
			},
			wantDelete: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDelete, err := migrateLegacyDNSRecords(
				tt.args.logger,
				tt.args.exposures,
				tt.args.existedCNAMERecords,
				tt.args.existedTXTRecords,
				tt.args.tunnelName,
			)
			if err != nil {
				t.Errorf("migrateLegacyDNSRecords() unexpected error: %v", err)
			}
			if !reflect.DeepEqual(gotDelete, tt.wantDelete) {
				t.Errorf("migrateLegacyDNSRecords() = %v, want %v", gotDelete, tt.wantDelete)
			}
		})
	}
}

func Test_renderTXTContent(t *testing.T) {
	result, err := renderTXTContent("my-tunnel")
	if err != nil {
		t.Fatalf("renderTXTContent() unexpected error: %v", err)
	}
	expected := `"{\"controller\":\"strrl.dev/cloudflare-tunnel-ingress-controller\",\"tunnel\":\"my-tunnel\"}"`
	if result != expected {
		t.Errorf("renderTXTContent() = %v, want %v", result, expected)
	}
}

func Test_parseTXTContent(t *testing.T) {
	payloads := []string{
		`{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"my-tunnel"}`,
		`"{\"controller\":\"strrl.dev/cloudflare-tunnel-ingress-controller\",\"tunnel\":\"my-tunnel\"}"`,
	}
	for _, content := range payloads {
		result, err := parseTXTContent(content)
		if err != nil {
			t.Errorf("parseTXTContent(%q) error = %v", content, err)
			continue
		}
		if result.Controller != ControllerIdentifier {
			t.Errorf("parseTXTContent(%q) Controller = %v, want %v", content, result.Controller, ControllerIdentifier)
		}
		if result.Tunnel != "my-tunnel" {
			t.Errorf("parseTXTContent(%q) Tunnel = %v, want %v", content, result.Tunnel, "my-tunnel")
		}
	}
}

func Test_managedTXTRecordName(t *testing.T) {
	tests := []struct {
		hostname string
		want     string
	}{
		{hostname: "test.example.com", want: "_ctic_managed.test.example.com"},
		{hostname: "example.com", want: "_ctic_managed.example.com"},
		{hostname: "wildcard.example.com", want: "_ctic_managed.wildcard.example.com"},
		{hostname: "*.example.com", want: "_ctic_managed._wildcard.example.com"},
		{hostname: "*.foo.example.com", want: "_ctic_managed._wildcard.foo.example.com"},
	}
	for _, tt := range tests {
		if got := managedTXTRecordName(tt.hostname); got != tt.want {
			t.Errorf("managedTXTRecordName(%q) = %q, want %q", tt.hostname, got, tt.want)
		}
	}
}

func Test_quoteTXTContent(t *testing.T) {
	quoted := quoteTXTContent(`{"controller":"example","tunnel":"t"}`)
	expected := `"{\"controller\":\"example\",\"tunnel\":\"t\"}"`
	if quoted != expected {
		t.Errorf("quoteTXTContent() = %q, want %q", quoted, expected)
	}
	if got := unquoteTXTContent(quoted); got != `{"controller":"example","tunnel":"t"}` {
		t.Errorf("unquoteTXTContent() = %q, want original JSON", got)
	}
	if got := unquoteTXTContent(`{"controller":"example","tunnel":"t"}`); got != `{"controller":"example","tunnel":"t"}` {
		t.Errorf("unquoteTXTContent() unquoted input = %q", got)
	}
}

func Test_quoteTXTContentSplitsLongPayload(t *testing.T) {
	payload := strings.Repeat("a", txtCharacterStringLimit) + strings.Repeat("b", 10)
	quoted := quoteTXTContent(payload)
	expected := `"` + strings.Repeat("a", txtCharacterStringLimit) + `" "` + strings.Repeat("b", 10) + `"`
	if quoted != expected {
		t.Errorf("quoteTXTContent() = %q, want %q", quoted, expected)
	}
	if got := unquoteTXTContent(quoted); got != payload {
		t.Errorf("unquoteTXTContent() = %q, want the original payload", got)
	}
}

// Cloudflare splits stored TXT values into multiple character-strings, so the
// ownership record must still be recognised after a round trip through the API.
func Test_txtContentEqualAcrossCloudflareSplitting(t *testing.T) {
	expected := mustRenderTXTContent(strings.Repeat("long-tunnel-name", 20))
	payload := unquoteTXTContent(expected)
	var split strings.Builder
	for start := 0; start < len(payload); start += 100 {
		if start > 0 {
			split.WriteString(" ")
		}
		split.WriteString(quoteTXTContent(payload[start:min(start+100, len(payload))]))
	}
	if !txtContentEqual(split.String(), expected) {
		t.Errorf("txtContentEqual() = false for re-split content %q", split.String())
	}
}

func Test_unquoteTXTContentMalformedInput(t *testing.T) {
	tests := []string{
		`"unterminated`,
		`"quoted" then bare`,
		`"`,
	}
	for _, content := range tests {
		if got := unquoteTXTContent(content); got != content {
			t.Errorf("unquoteTXTContent(%q) = %q, want the input unchanged", content, got)
		}
	}
}
