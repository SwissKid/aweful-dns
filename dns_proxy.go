package main

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/miekg/dns"
)

type DNSProxy struct {
	Cache         *Cache
	domains       map[string]interface{}
	servers       map[string]interface{}
	defaultServer string
}

func (proxy *DNSProxy) getResponse(requestMsg *dns.Msg, internal_mask string, external_mask string) (*dns.Msg, error) {
	responseMsg := new(dns.Msg)
	if len(requestMsg.Question) > 0 {
		question := requestMsg.Question[0]

		dnsServer := proxy.getIPFromConfigs(question.Name, proxy.servers)
		if dnsServer == "" {
			dnsServer = proxy.defaultServer
		}

		switch question.Qtype {
		case dns.TypeA:
			answer, err := proxy.processTypeA(dnsServer, &question, requestMsg, internal_mask, external_mask)
			if err != nil {
				return responseMsg, err
			}
			responseMsg.Answer = append(responseMsg.Answer, *answer)

		default:
			answer, err := proxy.processOtherTypes(dnsServer, &question, requestMsg)
			if err != nil {
				return responseMsg, err
			}
			responseMsg.Answer = append(responseMsg.Answer, *answer)
		}
	}

	return responseMsg, nil
}

func (proxy *DNSProxy) processOtherTypes(dnsServer string, q *dns.Question, requestMsg *dns.Msg) (*dns.RR, error) {
	queryMsg := new(dns.Msg)
	requestMsg.CopyTo(queryMsg)
	queryMsg.Question = []dns.Question{*q}

	msg, err := lookup(dnsServer, queryMsg)
	if err != nil {
		return nil, err
	}

	if len(msg.Answer) > 0 {
		return &msg.Answer[0], nil
	}
	return nil, fmt.Errorf("not found")
}

func (proxy *DNSProxy) processTypeA(dnsServer string, q *dns.Question, requestMsg *dns.Msg, internal_mask string, external_mask string) (*dns.RR, error) {
	ip := proxy.getIPFromConfigs(q.Name, proxy.domains)
	cacheMsg, found := proxy.Cache.Get(q.Name)

	if ip == "" && !found {
		queryMsg := new(dns.Msg)
		requestMsg.CopyTo(queryMsg)
		queryMsg.Question = []dns.Question{*q}
		team_pattern := regexp.MustCompile(`\.team\d+\.`)
		if team_pattern.MatchString((queryMsg.Question[0].Name)) {
			queryMsg.Question[0].Name = team_pattern.ReplaceAllString(queryMsg.Question[0].Name, ".")
		}

		msg, err := lookup(dnsServer, queryMsg)
		if err != nil {
			return nil, err
		}

		if len(msg.Answer) > 0 {
			// proxy.Cache.Set(q.Name, &msg.Answer[len(msg.Answer)-1])
			// var pre_answer dns.A
			pre_answer := strings.Split(msg.Answer[len(msg.Answer)-1].String(), "\t")
			better_data := pre_answer[len(pre_answer)-1]
			new_addr := strings.Replace(better_data, internal_mask, external_mask, -1)
			fmt.Println("chris test", new_addr)
			answer, err := dns.NewRR(fmt.Sprintf("%s A %s", q.Name, new_addr))
			if err != nil {
				return nil, err
			}
			return &answer, nil
		}

	} else if found {
		return cacheMsg.(*dns.RR), nil
	} else if ip != "" {

		answer, err := dns.NewRR(fmt.Sprintf("%s A %s", q.Name, ip))
		if err != nil {
			return nil, err
		}
		return &answer, nil
	}
	return nil, fmt.Errorf("not found")
}

func (dnsProxy *DNSProxy) getIPFromConfigs(domain string, configs map[string]interface{}) string {

	for k, v := range configs {
		match, _ := regexp.MatchString(k+"\\.", domain)
		if match {
			return v.(string)
		}
	}
	return ""
}

func GetOutboundIP() (net.IP, error) {

	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP, nil
}

func lookup(server string, m *dns.Msg) (*dns.Msg, error) {
	dnsClient := new(dns.Client)
	dnsClient.Net = "udp"
	response, _, err := dnsClient.Exchange(m, server)
	if err != nil {
		return nil, err
	}

	return response, nil
}
