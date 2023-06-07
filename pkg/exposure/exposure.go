package exposure

// Exposure is the minimal information for exposing a service.
type Exposure struct {
	// Hostname is the domain name to expose the service, eg. hello.strrl.dev
	Hostname string
	// ServiceTarget is the url of the service to expose, eg. http://10.109.94.106:9117
	ServiceTarget string
	// PathPrefix is the path prefix to expose the service, eg. /hello
	PathPrefix string
}
