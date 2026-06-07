package domain

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeDomainValid(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Example.COM", "example.com"},
		{"  example.com  ", "example.com"},
		{"sub.domain.com", "sub.domain.com"},
		{"a.b.c.d.e", "a.b.c.d.e"},
	}
	for _, tc := range tests {
		got, err := NormalizeDomain(tc.input)
		if err != nil {
			t.Errorf("NormalizeDomain(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.expected {
			t.Errorf("NormalizeDomain(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestNormalizeDomainInvalid(t *testing.T) {
	tests := []struct {
		input string
	}{
		{""},
		{" "},
		{"http://example.com"},
		{"https://example.com"},
		{"example.com/path"},
		{"example.com?query=1"},
		{"example.com:8080"},
		{"*.example.com"},
		{"example"},
		{string(make([]byte, 254))},
	}
	for _, tc := range tests {
		_, err := NormalizeDomain(tc.input)
		if err == nil {
			t.Errorf("NormalizeDomain(%q) expected error, got nil", tc.input)
		}
	}
}

func TestValidateProvider(t *testing.T) {
	provider, err := ValidateProvider("ses")
	if err != nil {
		t.Fatalf("ValidateProvider(ses) unexpected error: %v", err)
	}
	if provider != ProviderSES {
		t.Fatalf("ValidateProvider(ses) = %q, want %q", provider, ProviderSES)
	}

	provider, err = ValidateProvider("")
	if err != nil {
		t.Fatalf("ValidateProvider('') unexpected error: %v", err)
	}
	if provider != ProviderSES {
		t.Fatalf("ValidateProvider('') = %q, want %q", provider, ProviderSES)
	}

	_, err = ValidateProvider("sendgrid")
	if err == nil {
		t.Fatal("ValidateProvider(sendgrid) expected error")
	}
}

func TestGenerateDNSRecords(t *testing.T) {
	records := GenerateDNSRecords("dom-1", "example.com", time.Now())

	if len(records) != 5 {
		t.Fatalf("expected 5 records, got %d", len(records))
	}

	if records[0].RecordType != DNSRecordTypeTXT {
		t.Errorf("expected record 0 to be TXT, got %s", records[0].RecordType)
	}
	if records[0].Host != "example.com" {
		t.Errorf("expected SPF host 'example.com', got %q", records[0].Host)
	}
	if records[0].ExpectedValue != "v=spf1 include:amazonses.com ~all" {
		t.Errorf("unexpected SPF value: %q", records[0].ExpectedValue)
	}

	if records[1].RecordType != DNSRecordTypeTXT {
		t.Errorf("expected record 1 to be TXT, got %s", records[1].RecordType)
	}
	if records[1].Host != "_dmarc.example.com" {
		t.Errorf("expected DMARC host '_dmarc.example.com', got %q", records[1].Host)
	}
	if records[1].ExpectedValue != "v=DMARC1; p=none" {
		t.Errorf("unexpected DMARC value: %q", records[1].ExpectedValue)
	}

	for i := 2; i < 5; i++ {
		if records[i].RecordType != DNSRecordTypeCNAME {
			t.Errorf("expected record %d to be CNAME, got %s", i, records[i].RecordType)
		}
		if records[i].Status != DNSRecordStatusPending {
			t.Errorf("expected record %d status pending, got %s", i, records[i].Status)
		}
	}

	ids := make(map[string]bool)
	for _, rec := range records {
		if rec.ID == "" {
			t.Error("expected non-empty DNS record ID")
		}
		if !strings.HasPrefix(rec.ID, "dom-1-dns-") {
			t.Errorf("expected ID %q to have prefix 'dom-1-dns-'", rec.ID)
		}
		if ids[rec.ID] {
			t.Errorf("duplicate DNS record ID: %q", rec.ID)
		}
		ids[rec.ID] = true
	}
}

func TestStateTransitions(t *testing.T) {
	sd := SenderDomain{Status: SenderDomainStatusPendingVerification}
	if !sd.CanTransitionTo(SenderDomainStatusVerified) {
		t.Error("pending should allow verified")
	}
	if !sd.CanTransitionTo(SenderDomainStatusDisabled) {
		t.Error("pending should allow disabled")
	}

	sd = SenderDomain{Status: SenderDomainStatusVerified}
	if !sd.CanTransitionTo(SenderDomainStatusDisabled) {
		t.Error("verified should allow disabled")
	}
	if !sd.CanTransitionTo(SenderDomainStatusPendingVerification) {
		t.Error("verified should allow pending_verification")
	}

	sd = SenderDomain{Status: SenderDomainStatusDisabled}
	if sd.CanTransitionTo(SenderDomainStatusVerified) {
		t.Error("disabled should NOT allow verified")
	}
	if sd.CanTransitionTo(SenderDomainStatusPendingVerification) {
		t.Error("disabled should NOT allow pending_verification")
	}
	if sd.CanTransitionTo(SenderDomainStatusDisabled) {
		t.Error("disabled should NOT allow disabled")
	}
}

func TestIsReady(t *testing.T) {
	now := time.Now()

	verifiedRecords := []DNSRecord{
		{Status: DNSRecordStatusVerified},
		{Status: DNSRecordStatusVerified},
	}

	sd := SenderDomain{Status: SenderDomainStatusVerified}
	if !sd.IsReady(verifiedRecords) {
		t.Error("verified domain with verified records should be ready")
	}

	sd = SenderDomain{Status: SenderDomainStatusPendingVerification}
	if sd.IsReady(verifiedRecords) {
		t.Error("pending domain should not be ready")
	}

	sd = SenderDomain{Status: SenderDomainStatusDisabled, DisabledAt: &now}
	if sd.IsReady(verifiedRecords) {
		t.Error("disabled domain should not be ready")
	}

	missingRecord := []DNSRecord{
		{Status: DNSRecordStatusVerified},
		{Status: DNSRecordStatusMissing},
	}
	sd = SenderDomain{Status: SenderDomainStatusVerified}
	if sd.IsReady(missingRecord) {
		t.Error("verified domain with missing record should not be ready")
	}
}

func TestFlattenTXT(t *testing.T) {
	result := FlattenTXT([]string{"v=spf1", "include:amazonses.com", "~all"})
	if result != "v=spf1include:amazonses.com~all" {
		t.Errorf("unexpected flatten result: %q", result)
	}
}

func TestNormalizeCNAME(t *testing.T) {
	if got := NormalizeCNAME("s1.dkim.amazonses.com."); got != "s1.dkim.amazonses.com" {
		t.Errorf("expected without trailing dot, got %q", got)
	}
	if got := NormalizeCNAME("s1.dkim.amazonses.com"); got != "s1.dkim.amazonses.com" {
		t.Errorf("expected unchanged, got %q", got)
	}
}
