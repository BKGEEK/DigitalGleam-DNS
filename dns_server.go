package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/miekg/dns"
)

func StartDNSServer(cfg *Config) {
	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) { handleDNSRequest(w, r, cfg) })
	addr := fmt.Sprintf(":%d", cfg.DNS.Port)
	fmt.Printf("🔌 DNS 服务正在 UDP %d 端口启动...\n", cfg.DNS.Port)
	if err := dns.ListenAndServe(addr, "udp", nil); err != nil {
		log.Fatalf("DNS 服务启动失败: %v", err)
	}
}

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg, cfg *Config) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	for _, q := range r.Question {
		answers, found := resolveQuestion(q, cfg)
		if !found {
			m.Rcode = dns.RcodeNameError
			continue
		}
		m.Answer = append(m.Answer, answers...)
	}

	_ = w.WriteMsg(m)
}

func resolveQuestion(q dns.Question, cfg *Config) ([]dns.RR, bool) {
	queryName := strings.TrimSuffix(q.Name, ".")
	zone, host, ok := splitZoneHost(queryName)
	if !ok {
		return nil, false
	}

	records, err := loadDNSRecords(zone, host)
	if err != nil || len(records) == 0 {
		return nil, false
	}

	var answers []dns.RR
	switch q.Qtype {
	case dns.TypeA:
		answers = append(answers, buildARecords(q.Name, cfg.DNS.TTL, records)...)
	case dns.TypeAAAA:
		answers = append(answers, buildAAAARecords(q.Name, cfg.DNS.TTL, records)...)
	case dns.TypeCNAME:
		answers = append(answers, buildCNAMERecords(q.Name, cfg.DNS.TTL, records)...)
	case dns.TypeMX:
		answers = append(answers, buildMXRecords(q.Name, cfg.DNS.TTL, records)...)
	case dns.TypeNS:
		answers = append(answers, buildNSRecords(q.Name, cfg.DNS.TTL, records)...)
	case dns.TypeTXT:
		answers = append(answers, buildTXTRecords(q.Name, cfg.DNS.TTL, records)...)
	case dns.TypeSOA:
		answers = append(answers, buildSOARecord(q.Name, cfg.DNS.TTL, zone, records)...)
	case dns.TypeSRV:
		answers = append(answers, buildSRVRecords(q.Name, cfg.DNS.TTL, records)...)
	case dns.TypePTR:
		answers = append(answers, buildPTRRecords(q.Name, cfg.DNS.TTL, records)...)
	default:
		return nil, false
	}

	if len(answers) == 0 {
		return nil, false
	}
	return answers, true
}

func splitZoneHost(queryName string) (string, string, bool) {
	parts := strings.Split(queryName, ".")
	if len(parts) < 2 {
		return "", "", false
	}
	zone := strings.Join(parts[len(parts)-2:], ".") + "."
	host := strings.Join(parts[:len(parts)-2], ".")
	if host == "" {
		host = "@"
	}
	return zone, host, true
}

type dnsRecord struct {
	RecordType string
	Value      string
	TTL        uint32
	Priority   int
}

func loadDNSRecords(zone, host string) ([]dnsRecord, error) {
	rows, err := db.Query("SELECT record_type, value, ttl, priority FROM dns_records WHERE zone = ? AND host = ?", zone, host)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []dnsRecord
	for rows.Next() {
		var record dnsRecord
		if err := rows.Scan(&record.RecordType, &record.Value, &record.TTL, &record.Priority); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func buildARecords(name string, defaultTTL uint32, records []dnsRecord) []dns.RR {
	var answers []dns.RR
	for _, record := range records {
		if strings.EqualFold(record.RecordType, "A") {
			ip := net.ParseIP(record.Value).To4()
			if ip == nil {
				continue
			}
			answers = append(answers, &dns.A{
				Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttlOrDefault(record.TTL, defaultTTL)},
				A:   ip,
			})
		}
	}
	return answers
}

func buildAAAARecords(name string, defaultTTL uint32, records []dnsRecord) []dns.RR {
	var answers []dns.RR
	for _, record := range records {
		if strings.EqualFold(record.RecordType, "AAAA") {
			ip := net.ParseIP(record.Value)
			if ip == nil || ip.To16() == nil || ip.To4() != nil {
				continue
			}
			answers = append(answers, &dns.AAAA{
				Hdr:  dns.RR_Header{Name: name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: ttlOrDefault(record.TTL, defaultTTL)},
				AAAA: ip,
			})
		}
	}
	return answers
}

func buildCNAMERecords(name string, defaultTTL uint32, records []dnsRecord) []dns.RR {
	var answers []dns.RR
	for _, record := range records {
		if strings.EqualFold(record.RecordType, "CNAME") {
			target := dns.Fqdn(record.Value)
			answers = append(answers, &dns.CNAME{
				Hdr:    dns.RR_Header{Name: name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: ttlOrDefault(record.TTL, defaultTTL)},
				Target: target,
			})
		}
	}
	return answers
}

func buildMXRecords(name string, defaultTTL uint32, records []dnsRecord) []dns.RR {
	var answers []dns.RR
	for _, record := range records {
		if strings.EqualFold(record.RecordType, "MX") {
			priority := uint16(record.Priority)
			if priority == 0 {
				priority = 10
			}
			answers = append(answers, &dns.MX{
				Hdr:        dns.RR_Header{Name: name, Rrtype: dns.TypeMX, Class: dns.ClassINET, Ttl: ttlOrDefault(record.TTL, defaultTTL)},
				Preference: priority,
				Mx:         dns.Fqdn(record.Value),
			})
		}
	}
	return answers
}

func buildNSRecords(name string, defaultTTL uint32, records []dnsRecord) []dns.RR {
	var answers []dns.RR
	for _, record := range records {
		if strings.EqualFold(record.RecordType, "NS") {
			answers = append(answers, &dns.NS{
				Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: ttlOrDefault(record.TTL, defaultTTL)},
				Ns:  dns.Fqdn(record.Value),
			})
		}
	}
	return answers
}

func buildTXTRecords(name string, defaultTTL uint32, records []dnsRecord) []dns.RR {
	var answers []dns.RR
	for _, record := range records {
		if strings.EqualFold(record.RecordType, "TXT") {
			answers = append(answers, &dns.TXT{
				Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: ttlOrDefault(record.TTL, defaultTTL)},
				Txt: []string{record.Value},
			})
		}
	}
	return answers
}

func buildSOARecord(name string, defaultTTL uint32, zone string, records []dnsRecord) []dns.RR {
	for _, record := range records {
		if strings.EqualFold(record.RecordType, "SOA") {
			fields := strings.Fields(record.Value)
			if len(fields) < 7 {
				return nil
			}
			serial, _ := strconv.ParseUint(fields[2], 10, 32)
			refresh, _ := strconv.ParseUint(fields[3], 10, 32)
			retry, _ := strconv.ParseUint(fields[4], 10, 32)
			expire, _ := strconv.ParseUint(fields[5], 10, 32)
			minimum, _ := strconv.ParseUint(fields[6], 10, 32)
			return []dns.RR{&dns.SOA{
				Hdr:     dns.RR_Header{Name: dns.Fqdn(zone), Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: ttlOrDefault(record.TTL, defaultTTL)},
				Ns:      dns.Fqdn(fields[0]),
				Mbox:    dns.Fqdn(fields[1]),
				Serial:  uint32(serial),
				Refresh: uint32(refresh),
				Retry:   uint32(retry),
				Expire:  uint32(expire),
				Minttl:  uint32(minimum),
			}}
		}
	}
	return nil
}

func buildSRVRecords(name string, defaultTTL uint32, records []dnsRecord) []dns.RR {
	var answers []dns.RR
	for _, record := range records {
		if strings.EqualFold(record.RecordType, "SRV") {
			fields := strings.Fields(record.Value)
			if len(fields) < 4 {
				continue
			}
			priority, _ := strconv.Atoi(fields[0])
			weight, _ := strconv.Atoi(fields[1])
			port, _ := strconv.Atoi(fields[2])
			answers = append(answers, &dns.SRV{
				Hdr:      dns.RR_Header{Name: name, Rrtype: dns.TypeSRV, Class: dns.ClassINET, Ttl: ttlOrDefault(record.TTL, defaultTTL)},
				Priority: uint16(priority),
				Weight:   uint16(weight),
				Port:     uint16(port),
				Target:   dns.Fqdn(fields[3]),
			})
		}
	}
	return answers
}

func buildPTRRecords(name string, defaultTTL uint32, records []dnsRecord) []dns.RR {
	var answers []dns.RR
	for _, record := range records {
		if strings.EqualFold(record.RecordType, "PTR") {
			answers = append(answers, &dns.PTR{
				Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: ttlOrDefault(record.TTL, defaultTTL)},
				Ptr: dns.Fqdn(record.Value),
			})
		}
	}
	return answers
}

func ttlOrDefault(ttl uint32, defaultTTL uint32) uint32 {
	if ttl > 0 {
		return ttl
	}
	return defaultTTL
}
