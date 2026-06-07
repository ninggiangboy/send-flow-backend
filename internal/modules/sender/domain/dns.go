package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	dkimSelectorCount = 3
)

func GenerateDNSRecords(senderDomainID, domain string, now time.Time) []DNSRecord {
	records := make([]DNSRecord, 0, 2+dkimSelectorCount)

	i := 0
	records = append(records, DNSRecord{
		ID:             DNSRecordID(senderDomainID, i),
		SenderDomainID: senderDomainID,
		RecordType:     DNSRecordTypeTXT,
		Host:           domain,
		ExpectedValue:  "v=spf1 include:amazonses.com ~all",
		Status:         DNSRecordStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	i++

	records = append(records, DNSRecord{
		ID:             DNSRecordID(senderDomainID, i),
		SenderDomainID: senderDomainID,
		RecordType:     DNSRecordTypeTXT,
		Host:           "_dmarc." + domain,
		ExpectedValue:  "v=DMARC1; p=none",
		Status:         DNSRecordStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	i++

	for j := range dkimSelectorCount {
		selector := fmt.Sprintf("s%d", j+1)
		records = append(records, DNSRecord{
			ID:             DNSRecordID(senderDomainID, i),
			SenderDomainID: senderDomainID,
			RecordType:     DNSRecordTypeCNAME,
			Host:           selector + "._domainkey." + domain,
			ExpectedValue:  selector + ".dkim.amazonses.com",
			Status:         DNSRecordStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
		i++
	}

	return records
}

func DNSRecordID(prefix string, index int) string {
	return prefix + "-dns-" + strconv.Itoa(index)
}

func NormalizeCNAME(value string) string {
	return strings.TrimSuffix(strings.TrimSpace(value), ".")
}

func FlattenTXT(values []string) string {
	var b strings.Builder
	for _, v := range values {
		b.WriteString(strings.TrimSpace(v))
	}
	return b.String()
}
