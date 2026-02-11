package cloudflarecontroller

import (
	"reflect"
	"testing"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go"
	"github.com/go-logr/logr"
)

const WhateverTunnelId = "whatever"
const WhateverTunnelDomain = "whatever.cfargotunnel.com"

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
					Content:  `{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"tunnel-in-test"}`,
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
					Content:  `{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"tunnel-in-test"}`,
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
						Content: "another.example.com",
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
					Content:  `{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"tunnel-in-test"}`,
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
					Content: `{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"tunnel-in-test"}`,
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
						Content: `{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"different-tunnel"}`,
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
						Content: `{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"tunnel-in-test"}`,
					},
				},
				tunnelName: "tunnel-in-test",
			},
			wantDelete: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDelete := migrateLegacyDNSRecords(
				tt.args.logger,
				tt.args.exposures,
				tt.args.existedCNAMERecords,
				tt.args.existedTXTRecords,
				tt.args.tunnelName,
			)
			if !reflect.DeepEqual(gotDelete, tt.wantDelete) {
				t.Errorf("migrateLegacyDNSRecords() = %v, want %v", gotDelete, tt.wantDelete)
			}
		})
	}
}

func Test_renderTXTContent(t *testing.T) {
	result := renderTXTContent("my-tunnel")
	expected := `{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"my-tunnel"}`
	if result != expected {
		t.Errorf("renderTXTContent() = %v, want %v", result, expected)
	}
}

func Test_parseTXTContent(t *testing.T) {
	content := `{"controller":"strrl.dev/cloudflare-tunnel-ingress-controller","tunnel":"my-tunnel"}`
	result, err := parseTXTContent(content)
	if err != nil {
		t.Errorf("parseTXTContent() error = %v", err)
		return
	}
	if result.Controller != ControllerIdentifier {
		t.Errorf("parseTXTContent() Controller = %v, want %v", result.Controller, ControllerIdentifier)
	}
	if result.Tunnel != "my-tunnel" {
		t.Errorf("parseTXTContent() Tunnel = %v, want %v", result.Tunnel, "my-tunnel")
	}
}
