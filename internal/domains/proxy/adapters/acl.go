package adapters

import (
	"net"
)

type ACLMatcher struct {
	allowNets []*net.IPNet
	denyNets  []*net.IPNet
}

func NewACLMatcher(allowCIDRs, denyCIDRs []string) (*ACLMatcher, error) {
	a := &ACLMatcher{}
	for _, cidr := range allowCIDRs {
		if cidr == "" {
			continue
		}
		_, net, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}
		a.allowNets = append(a.allowNets, net)
	}
	for _, cidr := range denyCIDRs {
		if cidr == "" {
			continue
		}
		_, net, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}
		a.denyNets = append(a.denyNets, net)
	}
	return a, nil
}

func (a *ACLMatcher) Allow(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}

	for _, net := range a.denyNets {
		if net.Contains(parsed) {
			return false
		}
	}

	if len(a.allowNets) == 0 {
		return true
	}

	for _, net := range a.allowNets {
		if net.Contains(parsed) {
			return true
		}
	}

	return false
}
