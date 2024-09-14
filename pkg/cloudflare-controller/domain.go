package cloudflarecontroller

import "strings"

type Domain struct {
	Name string
}

func (d Domain) IsSubDomainOf(target Domain) bool {
	currentLabels := strings.Split(strings.ToLower(d.Name), ".")
	targetLabels := strings.Split(strings.ToLower(target.Name), ".")
	if len(currentLabels) <= len(targetLabels) {
		return false
	}
	for i := 1; i <= len(targetLabels); i++ {
		if currentLabels[len(currentLabels)-i] != targetLabels[len(targetLabels)-i] {
			return false
		}
	}
	return true
}
