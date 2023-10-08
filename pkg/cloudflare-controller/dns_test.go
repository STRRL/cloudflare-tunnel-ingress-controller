package cloudflarecontroller

import (
	"reflect"
	"testing"

	"github.com/cloudflare/cloudflare-go"
	"github.com/oliverbaehler/cloudflare-tunnel-ingress-controller/pkg/exposure"
)

const WhateverTunnelId = "whatever"
const WhateverTunnelDomain = "whatever.cfargotunnel.com"

func Test_syncDNSRecord(t *testing.T) {
	type args struct {
		exposures      []exposure.Exposure
		existedRecords []cloudflare.DNSRecord
		tunnelId       string
		tunnelName     string
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
				exposures:      nil,
				existedRecords: nil,
				tunnelId:       WhateverTunnelId,
			},
			wantCreate: nil,
			wantUpdate: nil,
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "create new exposure",
			args: args{
				exposures: []exposure.Exposure{
					{
						Hostname:      "test.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     false,
					},
				},
				existedRecords: nil,
				tunnelId:       WhateverTunnelId,
				tunnelName:     "tunnel-in-test",
			},
			wantCreate: []DNSOperationCreate{
				{
					Hostname: "test.example.com",
					Type:     "CNAME",
					Content:  WhateverTunnelDomain,
					Comment:  "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [tunnel-in-test]",
				},
			},
			wantUpdate: nil,
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "ignore deleted exposure",
			args: args{
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
				existedRecords: nil,
				tunnelId:       WhateverTunnelId,
				tunnelName:     "tunnel-in-test",
			},
			wantCreate: []DNSOperationCreate{
				{
					Hostname: "test2.example.com",
					Type:     "CNAME",
					Content:  WhateverTunnelDomain,
					Comment:  "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [tunnel-in-test]",
				},
			},
			wantUpdate: nil,
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "only delete managed record",
			args: args{
				exposures: nil,
				existedRecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: "another.example.com",
						Comment: "not a managed record",
					},
					{
						Name:    "test2.example.com",
						Type:    "A",
						Content: "1.2.3.4",
						Comment: "",
					},
				},
				tunnelId:   "",
				tunnelName: "",
			},
			wantCreate: nil,
			wantUpdate: nil,
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "update existed exposure",
			args: args{
				exposures: []exposure.Exposure{
					{
						Hostname:      "test.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     false,
					},
				},
				existedRecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "A",
						Content: "1.2.3.4",
						Comment: "",
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
						Type:    "A",
						Content: "1.2.3.4",
						Comment: "",
					},
					Type:    "CNAME",
					Content: WhateverTunnelDomain,
					Comment: "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [tunnel-in-test]",
				},
			},
			wantDelete: nil,
			wantErr:    false,
		},
		{
			name: "delete unused exposure",
			args: args{
				exposures: []exposure.Exposure{
					{
						Hostname:      "test.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     true,
					},
				},
				existedRecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "A",
						Content: "1.2.3.4",
						Comment: "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [tunnel-in-test]",
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
						Type:    "A",
						Content: "1.2.3.4",
						Comment: "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [tunnel-in-test]",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "always update existed record",
			args: args{
				exposures: []exposure.Exposure{
					{
						Hostname:      "test.example.com",
						ServiceTarget: "http://10.0.0.1:233",
						PathPrefix:    "/",
						IsDeleted:     false,
					},
				},
				existedRecords: []cloudflare.DNSRecord{
					{
						Name:    "test.example.com",
						Type:    "CNAME",
						Content: WhateverTunnelDomain,
						Comment: "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [tunnel-in-test]",
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
						Comment: "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [tunnel-in-test]",
					},
					Type:    "CNAME",
					Content: WhateverTunnelDomain,
					Comment: "managed by strrl.dev/cloudflare-tunnel-ingress-controller, tunnel [tunnel-in-test]",
				},
			},
			wantDelete: nil,
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCreate, gotUpdate, gotDelete, err := syncDNSRecord(tt.args.exposures, tt.args.existedRecords, tt.args.tunnelId, tt.args.tunnelName)
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
