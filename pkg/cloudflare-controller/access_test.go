package cloudflarecontroller

import (
	"reflect"
	"testing"

	"github.com/STRRL/cloudflare-tunnel-ingress-controller/pkg/exposure"
	"github.com/cloudflare/cloudflare-go"
)

func Test_accessAppName(t *testing.T) {
	tests := []struct {
		name       string
		tunnelName string
		hostname   string
		want       string
	}{
		{
			name:       "basic",
			tunnelName: "k8s-tunnel",
			hostname:   "grafana.twiechert.de",
			want:       "ctic:k8s-tunnel:grafana.twiechert.de",
		},
		{
			name:       "subdomain",
			tunnelName: "my-tunnel",
			hostname:   "app.sub.example.com",
			want:       "ctic:my-tunnel:app.sub.example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := accessAppName(tt.tunnelName, tt.hostname)
			if got != tt.want {
				t.Errorf("accessAppName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isOwnedAccessApp(t *testing.T) {
	tests := []struct {
		name       string
		app        cloudflare.AccessApplication
		tunnelName string
		want       bool
	}{
		{
			name:       "owned app",
			app:        cloudflare.AccessApplication{Name: "ctic:k8s-tunnel:grafana.example.com"},
			tunnelName: "k8s-tunnel",
			want:       true,
		},
		{
			name:       "different tunnel",
			app:        cloudflare.AccessApplication{Name: "ctic:other-tunnel:grafana.example.com"},
			tunnelName: "k8s-tunnel",
			want:       false,
		},
		{
			name:       "not owned",
			app:        cloudflare.AccessApplication{Name: "my-manual-app"},
			tunnelName: "k8s-tunnel",
			want:       false,
		},
		{
			name:       "prefix collision",
			app:        cloudflare.AccessApplication{Name: "ctic:k8s-tunnel-extended:grafana.example.com"},
			tunnelName: "k8s-tunnel",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOwnedAccessApp(tt.app, tt.tunnelName)
			if got != tt.want {
				t.Errorf("isOwnedAccessApp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hostnameFromAccessAppName(t *testing.T) {
	got := hostnameFromAccessAppName("ctic:k8s-tunnel:grafana.example.com", "k8s-tunnel")
	want := "grafana.example.com"
	if got != want {
		t.Errorf("hostnameFromAccessAppName() = %v, want %v", got, want)
	}
}

func Test_desiredAccessApps(t *testing.T) {
	tests := []struct {
		name      string
		exposures []exposure.Exposure
		want      map[string]desiredAccessApp
	}{
		{
			name:      "empty exposures",
			exposures: nil,
			want:      map[string]desiredAccessApp{},
		},
		{
			name: "no access groups",
			exposures: []exposure.Exposure{
				{Hostname: "app.example.com", ServiceTarget: "http://svc:80"},
			},
			want: map[string]desiredAccessApp{},
		},
		{
			name: "single exposure with allowed groups",
			exposures: []exposure.Exposure{
				{
					Hostname:              "app.example.com",
					AllowedAccessGroupIDs: []string{"group-1", "group-2"},
				},
			},
			want: map[string]desiredAccessApp{
				"app.example.com": {
					AllowedGroupIDs: []string{"group-1", "group-2"},
					DeniedGroupIDs:  nil,
				},
			},
		},
		{
			name: "single exposure with both allowed and denied groups",
			exposures: []exposure.Exposure{
				{
					Hostname:              "app.example.com",
					AllowedAccessGroupIDs: []string{"group-1"},
					DeniedAccessGroupIDs:  []string{"group-deny"},
				},
			},
			want: map[string]desiredAccessApp{
				"app.example.com": {
					AllowedGroupIDs: []string{"group-1"},
					DeniedGroupIDs:  []string{"group-deny"},
				},
			},
		},
		{
			name: "deduplicate same hostname",
			exposures: []exposure.Exposure{
				{
					Hostname:              "app.example.com",
					PathPrefix:            "/",
					AllowedAccessGroupIDs: []string{"group-1", "group-2"},
				},
				{
					Hostname:              "app.example.com",
					PathPrefix:            "/api",
					AllowedAccessGroupIDs: []string{"group-2", "group-3"},
				},
			},
			want: map[string]desiredAccessApp{
				"app.example.com": {
					AllowedGroupIDs: []string{"group-1", "group-2", "group-3"},
					DeniedGroupIDs:  nil,
				},
			},
		},
		{
			name: "skip deleted exposures",
			exposures: []exposure.Exposure{
				{
					Hostname:              "app.example.com",
					IsDeleted:             true,
					AllowedAccessGroupIDs: []string{"group-1"},
				},
			},
			want: map[string]desiredAccessApp{},
		},
		{
			name: "deny-only is omitted (no allowed groups)",
			exposures: []exposure.Exposure{
				{
					Hostname:             "app.example.com",
					DeniedAccessGroupIDs: []string{"group-deny"},
				},
			},
			want: map[string]desiredAccessApp{},
		},
		{
			name: "bypass creates app without groups",
			exposures: []exposure.Exposure{
				{
					Hostname:     "public.example.com",
					AccessBypass: true,
				},
			},
			want: map[string]desiredAccessApp{
				"public.example.com": {
					Bypass: true,
				},
			},
		},
		{
			name: "session duration and auto-redirect propagated",
			exposures: []exposure.Exposure{
				{
					Hostname:              "app.example.com",
					AllowedAccessGroupIDs: []string{"group-1"},
					AccessSessionDuration: "1h",
					AccessAutoRedirect:    boolPtr(true),
				},
			},
			want: map[string]desiredAccessApp{
				"app.example.com": {
					AllowedGroupIDs: []string{"group-1"},
					SessionDuration: "1h",
					AutoRedirect:    boolPtr(true),
				},
			},
		},
		{
			name: "multiple hostnames",
			exposures: []exposure.Exposure{
				{
					Hostname:              "app1.example.com",
					AllowedAccessGroupIDs: []string{"group-a"},
				},
				{
					Hostname:              "app2.example.com",
					AllowedAccessGroupIDs: []string{"group-b"},
					DeniedAccessGroupIDs:  []string{"group-c"},
				},
			},
			want: map[string]desiredAccessApp{
				"app1.example.com": {
					AllowedGroupIDs: []string{"group-a"},
					DeniedGroupIDs:  nil,
				},
				"app2.example.com": {
					AllowedGroupIDs: []string{"group-b"},
					DeniedGroupIDs:  []string{"group-c"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := desiredAccessApps(tt.exposures)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("desiredAccessApps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_accessGroupIncludeRules(t *testing.T) {
	tests := []struct {
		name     string
		groupIDs []string
		want     []interface{}
	}{
		{
			name:     "single group",
			groupIDs: []string{"abc123"},
			want: []interface{}{
				cloudflare.AccessGroupAccessGroup{
					Group: struct {
						ID string `json:"id"`
					}{ID: "abc123"},
				},
			},
		},
		{
			name:     "multiple groups",
			groupIDs: []string{"group-1", "group-2"},
			want: []interface{}{
				cloudflare.AccessGroupAccessGroup{
					Group: struct {
						ID string `json:"id"`
					}{ID: "group-1"},
				},
				cloudflare.AccessGroupAccessGroup{
					Group: struct {
						ID string `json:"id"`
					}{ID: "group-2"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := accessGroupIncludeRules(tt.groupIDs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("accessGroupIncludeRules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_resolveGroupNames(t *testing.T) {
	lookup := map[string]string{"admin": "id-1", "viewer": "id-2"}

	got := resolveGroupNames([]string{"admin", "viewer", "unknown"}, lookup)
	want := []string{"id-1", "id-2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("resolveGroupNames() = %v, want %v", got, want)
	}
}

func boolPtr(b bool) *bool { return &b }

func Test_unionStrings(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want []string
	}{
		{
			name: "both empty",
			a:    nil,
			b:    nil,
			want: nil,
		},
		{
			name: "a empty",
			a:    nil,
			b:    []string{"x"},
			want: []string{"x"},
		},
		{
			name: "no overlap",
			a:    []string{"a", "b"},
			b:    []string{"c"},
			want: []string{"a", "b", "c"},
		},
		{
			name: "with overlap",
			a:    []string{"a", "b"},
			b:    []string{"b", "c"},
			want: []string{"a", "b", "c"},
		},
		{
			name: "fully overlapping",
			a:    []string{"a", "b"},
			b:    []string{"a", "b"},
			want: []string{"a", "b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unionStrings(tt.a, tt.b)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unionStrings() = %v, want %v", got, tt.want)
			}
		})
	}
}
