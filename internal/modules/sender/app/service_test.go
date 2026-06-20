package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
)

type fakeStorage struct {
	mu      sync.Mutex
	domains map[string]senderdomain.SenderDomain
	records map[string][]senderdomain.DNSRecord
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{
		domains: make(map[string]senderdomain.SenderDomain),
		records: make(map[string][]senderdomain.DNSRecord),
	}
}

type fakeDomainReadRepo struct {
	store *fakeStorage
}

func newFakeDomainReadRepo(store *fakeStorage) *fakeDomainReadRepo {
	return &fakeDomainReadRepo{store: store}
}

func (r *fakeDomainReadRepo) FindByID(_ context.Context, workspaceID, domainID string) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	sd, ok := r.store.domains[domainID]
	if !ok || sd.WorkspaceID != workspaceID {
		return nil, nil, senderdomain.ErrDomainNotFound
	}
	recs := r.store.records[domainID]
	out := make([]senderdomain.DNSRecord, len(recs))
	copy(out, recs)
	return &sd, out, nil
}

func (r *fakeDomainReadRepo) FindByDomain(_ context.Context, workspaceID, normalizedDomain string) (*senderdomain.SenderDomain, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	for _, sd := range r.store.domains {
		if sd.WorkspaceID == workspaceID && sd.Domain == normalizedDomain {
			return &sd, nil
		}
	}
	return nil, senderdomain.ErrDomainNotFound
}

func (r *fakeDomainReadRepo) ListByWorkspace(_ context.Context, workspaceID string) ([]senderdomain.SenderDomain, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	var result []senderdomain.SenderDomain
	for _, sd := range r.store.domains {
		if sd.WorkspaceID == workspaceID {
			result = append(result, sd)
		}
	}
	return result, nil
}

type fakeDomainWriteRepo struct {
	store *fakeStorage
}

func newFakeDomainWriteRepo(store *fakeStorage) *fakeDomainWriteRepo {
	return &fakeDomainWriteRepo{store: store}
}

func (w *fakeDomainWriteRepo) Create(_ context.Context, sd senderdomain.SenderDomain, records []senderdomain.DNSRecord) error {
	w.store.mu.Lock()
	defer w.store.mu.Unlock()
	if _, exists := w.store.domains[sd.ID]; exists {
		return senderdomain.ErrDomainConflict
	}
	for _, existing := range w.store.domains {
		if existing.WorkspaceID == sd.WorkspaceID && existing.Domain == sd.Domain {
			return senderdomain.ErrDomainConflict
		}
	}
	w.store.domains[sd.ID] = sd
	recs := make([]senderdomain.DNSRecord, len(records))
	copy(recs, records)
	w.store.records[sd.ID] = recs
	return nil
}

func (w *fakeDomainWriteRepo) UpdateDomain(_ context.Context, sd senderdomain.SenderDomain) error {
	w.store.mu.Lock()
	defer w.store.mu.Unlock()
	if _, ok := w.store.domains[sd.ID]; !ok {
		return senderdomain.ErrDomainNotFound
	}
	w.store.domains[sd.ID] = sd
	return nil
}

func (w *fakeDomainWriteRepo) ReplaceDNSRecordStatuses(_ context.Context, senderDomainID string, records []senderdomain.DNSRecord) error {
	w.store.mu.Lock()
	defer w.store.mu.Unlock()
	recs := make([]senderdomain.DNSRecord, len(records))
	copy(recs, records)
	w.store.records[senderDomainID] = recs
	return nil
}

func (w *fakeDomainWriteRepo) FindByID(_ context.Context, workspaceID, domainID string) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	w.store.mu.Lock()
	defer w.store.mu.Unlock()
	sd, ok := w.store.domains[domainID]
	if !ok || sd.WorkspaceID != workspaceID {
		return nil, nil, senderdomain.ErrDomainNotFound
	}
	recs := w.store.records[domainID]
	out := make([]senderdomain.DNSRecord, len(recs))
	copy(out, recs)
	return &sd, out, nil
}

func (w *fakeDomainWriteRepo) FindByDomain(_ context.Context, workspaceID, normalizedDomain string) (*senderdomain.SenderDomain, error) {
	w.store.mu.Lock()
	defer w.store.mu.Unlock()
	for _, sd := range w.store.domains {
		if sd.WorkspaceID == workspaceID && sd.Domain == normalizedDomain {
			return &sd, nil
		}
	}
	return nil, senderdomain.ErrDomainNotFound
}

func (w *fakeDomainWriteRepo) ListByWorkspace(_ context.Context, workspaceID string) ([]senderdomain.SenderDomain, error) {
	w.store.mu.Lock()
	defer w.store.mu.Unlock()
	var result []senderdomain.SenderDomain
	for _, sd := range w.store.domains {
		if sd.WorkspaceID == workspaceID {
			result = append(result, sd)
		}
	}
	return result, nil
}

type fakeDNSResolver struct {
	txtResults   map[string][]string
	txtErrors    map[string]error
	cnameResults map[string]string
	cnameErrors  map[string]error
}

func newFakeDNSResolver() *fakeDNSResolver {
	return &fakeDNSResolver{
		txtResults:   make(map[string][]string),
		txtErrors:    make(map[string]error),
		cnameResults: make(map[string]string),
		cnameErrors:  make(map[string]error),
	}
}

func (r *fakeDNSResolver) LookupTXT(_ context.Context, host string) ([]string, error) {
	if err, ok := r.txtErrors[host]; ok {
		return nil, err
	}
	if val, ok := r.txtResults[host]; ok {
		return val, nil
	}
	return nil, errors.New("no such host")
}

func (r *fakeDNSResolver) LookupCNAME(_ context.Context, host string) (string, error) {
	if err, ok := r.cnameErrors[host]; ok {
		return "", err
	}
	if val, ok := r.cnameResults[host]; ok {
		return val, nil
	}
	return "", errors.New("no such host")
}

type fakeAccessChecker struct {
	deny bool
}

func (a *fakeAccessChecker) RequirePermission(_ context.Context, _, _, _ string) error {
	if a.deny {
		return senderdomain.ErrManageDenied
	}
	return nil
}

func idGen() (string, error) {
	return "test-id", nil
}

func newTestService(store *fakeStorage, resolver *fakeDNSResolver, accessChecker *fakeAccessChecker) *Service {
	if store == nil {
		store = newFakeStorage()
	}
	if resolver == nil {
		resolver = newFakeDNSResolver()
	}
	if accessChecker == nil {
		accessChecker = &fakeAccessChecker{}
	}
	return NewService(Options{
		DomainsRead:   newFakeDomainReadRepo(store),
		DomainsWrite:  newFakeDomainWriteRepo(store),
		DNSResolver:   resolver,
		AccessChecker: accessChecker,
		IDGen:         idGen,
	})
}

func TestCreateSenderDomainSuccess(t *testing.T) {
	svc := newTestService(nil, nil, nil)

	result, err := svc.CreateSenderDomain(context.Background(), "ws-1", "user-1", "example.com", "", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Domain.ID != "test-id" {
		t.Errorf("expected domain ID 'test-id', got %q", result.Domain.ID)
	}
	if result.Domain.Domain != "example.com" {
		t.Errorf("expected domain 'example.com', got %q", result.Domain.Domain)
	}
	if result.Domain.Status != senderdomain.SenderDomainStatusPendingVerification {
		t.Errorf("expected status pending, got %s", result.Domain.Status)
	}
	if len(result.Records) != 5 {
		t.Errorf("expected 5 records, got %d", len(result.Records))
	}
	if result.Readiness.Ready {
		t.Error("new domain should not be ready")
	}
}

func TestCreateSenderDomainDenied(t *testing.T) {
	svc := newTestService(nil, nil, &fakeAccessChecker{deny: true})

	_, err := svc.CreateSenderDomain(context.Background(), "ws-1", "user-1", "example.com", "", time.Now())
	if !errors.Is(err, senderdomain.ErrManageDenied) {
		t.Fatalf("expected ErrManageDenied, got %v", err)
	}
}

func TestCreateSenderDomainDuplicate(t *testing.T) {
	svc := newTestService(nil, nil, nil)

	_, err := svc.CreateSenderDomain(context.Background(), "ws-1", "user-1", "example.com", "", time.Now())
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err = svc.CreateSenderDomain(context.Background(), "ws-1", "user-1", "example.com", "", time.Now())
	if !errors.Is(err, senderdomain.ErrDomainConflict) {
		t.Fatalf("expected ErrDomainConflict, got %v", err)
	}
}

func TestVerifyDNSMatches(t *testing.T) {
	store := newFakeStorage()
	resolver := newFakeDNSResolver()
	svc := newTestService(store, resolver, nil)

	now := time.Now()
	result, err := svc.CreateSenderDomain(context.Background(), "ws-1", "user-1", "example.com", "", now)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	resolver.txtResults["example.com"] = []string{"v=spf1 include:amazonses.com ~all"}
	resolver.txtResults["_dmarc.example.com"] = []string{"v=DMARC1; p=none"}
	resolver.cnameResults["s1._domainkey.example.com"] = "s1.dkim.amazonses.com"
	resolver.cnameResults["s2._domainkey.example.com"] = "s2.dkim.amazonses.com"
	resolver.cnameResults["s3._domainkey.example.com"] = "s3.dkim.amazonses.com"

	result, err = svc.RefreshSenderDomainDNSStatus(context.Background(), "ws-1", result.Domain.ID, "user-1", now)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Domain.Status != senderdomain.SenderDomainStatusVerified {
		t.Errorf("expected verified, got %s", result.Domain.Status)
	}
	if !result.Readiness.Ready {
		t.Error("expected readiness true")
	}
}

func TestVerifyDNSMissingRecord(t *testing.T) {
	store := newFakeStorage()
	resolver := newFakeDNSResolver()
	svc := newTestService(store, resolver, nil)

	now := time.Now()
	result, err := svc.CreateSenderDomain(context.Background(), "ws-1", "user-1", "example.com", "", now)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	resolver.txtResults["example.com"] = []string{"v=spf1 include:amazonses.com ~all"}
	resolver.txtResults["_dmarc.example.com"] = []string{"v=DMARC1; p=none"}
	resolver.cnameResults["s1._domainkey.example.com"] = "s1.dkim.amazonses.com"
	resolver.cnameResults["s2._domainkey.example.com"] = "s2.dkim.amazonses.com"

	result, err = svc.RefreshSenderDomainDNSStatus(context.Background(), "ws-1", result.Domain.ID, "user-1", now)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Domain.Status != senderdomain.SenderDomainStatusPendingVerification {
		t.Errorf("expected pending_verification due to missing record, got %s", result.Domain.Status)
	}
	if result.Readiness.Ready {
		t.Error("expected readiness false due to missing record")
	}

	for _, rec := range result.Records {
		if rec.Host == "s3._domainkey.example.com" && rec.Status != senderdomain.DNSRecordStatusMissing {
			t.Errorf("expected s3 to be missing, got %s", rec.Status)
		}
	}
}

func TestVerifyDNSMismatch(t *testing.T) {
	store := newFakeStorage()
	resolver := newFakeDNSResolver()
	svc := newTestService(store, resolver, nil)

	now := time.Now()
	result, err := svc.CreateSenderDomain(context.Background(), "ws-1", "user-1", "example.com", "", now)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	resolver.txtResults["example.com"] = []string{"v=spf1 include:amazonses.com ~all"}
	resolver.txtResults["_dmarc.example.com"] = []string{"v=DMARC1; p=quarantine"}
	resolver.cnameResults["s1._domainkey.example.com"] = "s1.dkim.amazonses.com"
	resolver.cnameResults["s2._domainkey.example.com"] = "s2.dkim.amazonses.com"
	resolver.cnameResults["s3._domainkey.example.com"] = "s3.dkim.amazonses.com"

	result, err = svc.RefreshSenderDomainDNSStatus(context.Background(), "ws-1", result.Domain.ID, "user-1", now)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Domain.Status != senderdomain.SenderDomainStatusPendingVerification {
		t.Errorf("expected pending_verification due to mismatch, got %s", result.Domain.Status)
	}

	for _, rec := range result.Records {
		if rec.Host == "_dmarc.example.com" && rec.Status != senderdomain.DNSRecordStatusMismatch {
			t.Errorf("expected dmarc to be mismatch, got %s", rec.Status)
		}
	}
}

func TestVerifyDNSWithExtraTXTRecords(t *testing.T) {
	store := newFakeStorage()
	resolver := newFakeDNSResolver()
	svc := newTestService(store, resolver, nil)

	now := time.Now()
	result, err := svc.CreateSenderDomain(context.Background(), "ws-1", "user-1", "example.com", "", now)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	resolver.txtResults["example.com"] = []string{
		"google-site-verification=abc123",
		"v=spf1 include:amazonses.com ~all",
	}
	resolver.txtResults["_dmarc.example.com"] = []string{"v=DMARC1; p=none"}
	resolver.cnameResults["s1._domainkey.example.com"] = "s1.dkim.amazonses.com"
	resolver.cnameResults["s2._domainkey.example.com"] = "s2.dkim.amazonses.com"
	resolver.cnameResults["s3._domainkey.example.com"] = "s3.dkim.amazonses.com"

	result, err = svc.RefreshSenderDomainDNSStatus(context.Background(), "ws-1", result.Domain.ID, "user-1", now)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Domain.Status != senderdomain.SenderDomainStatusVerified {
		t.Errorf("expected verified, got %s", result.Domain.Status)
	}
	if !result.Readiness.Ready {
		t.Error("expected readiness true")
	}
}

func TestVerifyClearsFailureReason(t *testing.T) {
	store := newFakeStorage()
	resolver := newFakeDNSResolver()
	svc := newTestService(store, resolver, nil)

	now := time.Now()
	result, err := svc.CreateSenderDomain(context.Background(), "ws-1", "user-1", "example.com", "", now)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	resolver.txtResults["example.com"] = []string{"wrong-spf-value"}
	resolver.txtResults["_dmarc.example.com"] = []string{"v=DMARC1; p=none"}
	resolver.cnameResults["s1._domainkey.example.com"] = "s1.dkim.amazonses.com"
	resolver.cnameResults["s2._domainkey.example.com"] = "s2.dkim.amazonses.com"
	resolver.cnameResults["s3._domainkey.example.com"] = "wrong.dkim.example.com"

	result, err = svc.RefreshSenderDomainDNSStatus(context.Background(), "ws-1", result.Domain.ID, "user-1", now)
	if err != nil {
		t.Fatalf("first verify: %v", err)
	}

	for _, rec := range result.Records {
		if rec.Host == "example.com" && rec.Status != senderdomain.DNSRecordStatusMismatch {
			t.Errorf("expected SPF mismatch, got %s", rec.Status)
		}
		if rec.Host == "s3._domainkey.example.com" && rec.Status != senderdomain.DNSRecordStatusMismatch {
			t.Errorf("expected s3 CNAME mismatch, got %s", rec.Status)
		}
	}

	resolver.txtResults["example.com"] = []string{"v=spf1 include:amazonses.com ~all"}
	resolver.cnameResults["s3._domainkey.example.com"] = "s3.dkim.amazonses.com"

	result, err = svc.RefreshSenderDomainDNSStatus(context.Background(), "ws-1", result.Domain.ID, "user-1", now)
	if err != nil {
		t.Fatalf("second verify: %v", err)
	}
	if result.Domain.Status != senderdomain.SenderDomainStatusVerified {
		t.Errorf("expected verified after fix, got %s", result.Domain.Status)
	}

	for _, rec := range result.Records {
		if rec.Status != senderdomain.DNSRecordStatusVerified {
			t.Errorf("record %s (%s) expected verified, got %s with reason %q", rec.Host, rec.RecordType, rec.Status, rec.FailureReason)
		}
		if rec.FailureReason != "" {
			t.Errorf("record %s (%s) has stale failure reason: %q", rec.Host, rec.RecordType, rec.FailureReason)
		}
	}
}

func TestVerifyRejectsDisabledDomain(t *testing.T) {
	store := newFakeStorage()
	svc := newTestService(store, nil, nil)

	now := time.Now()
	result, err := svc.CreateSenderDomain(context.Background(), "ws-1", "user-1", "example.com", "", now)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = svc.DisableSenderDomain(context.Background(), "ws-1", result.Domain.ID, "user-1", now)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}

	_, err = svc.RefreshSenderDomainDNSStatus(context.Background(), "ws-1", result.Domain.ID, "user-1", now)
	if !errors.Is(err, senderdomain.ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition, got %v", err)
	}
}

func TestDisableSuccess(t *testing.T) {
	store := newFakeStorage()
	svc := newTestService(store, nil, nil)

	now := time.Now()
	result, err := svc.CreateSenderDomain(context.Background(), "ws-1", "user-1", "example.com", "", now)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	result, err = svc.DisableSenderDomain(context.Background(), "ws-1", result.Domain.ID, "user-1", now)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if result.Domain.Status != senderdomain.SenderDomainStatusDisabled {
		t.Errorf("expected disabled, got %s", result.Domain.Status)
	}
	if result.Domain.DisabledAt == nil {
		t.Error("expected DisabledAt to be set")
	}
}

func TestDisableRepeatedConflict(t *testing.T) {
	store := newFakeStorage()
	svc := newTestService(store, nil, nil)

	now := time.Now()
	result, err := svc.CreateSenderDomain(context.Background(), "ws-1", "user-1", "example.com", "", now)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = svc.DisableSenderDomain(context.Background(), "ws-1", result.Domain.ID, "user-1", now)
	if err != nil {
		t.Fatalf("first disable: %v", err)
	}

	_, err = svc.DisableSenderDomain(context.Background(), "ws-1", result.Domain.ID, "user-1", now)
	if !errors.Is(err, senderdomain.ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition on second disable, got %v", err)
	}
}

func TestGetSenderReadinessNoUserID(t *testing.T) {
	store := newFakeStorage()
	svc := newTestService(store, nil, nil)

	now := time.Now()
	result, err := svc.CreateSenderDomain(context.Background(), "ws-1", "user-1", "example.com", "", now)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	readiness, err := svc.GetSenderReadiness(context.Background(), "ws-1", result.Domain.ID)
	if err != nil {
		t.Fatalf("GetSenderReadiness: %v", err)
	}
	if readiness.Ready {
		t.Error("pending domain should not be ready")
	}
}

func TestListSenderDomains(t *testing.T) {
	store := newFakeStorage()
	svc := newTestService(store, nil, nil)

	now := time.Now()
	_, err := svc.CreateSenderDomain(context.Background(), "ws-1", "user-1", "example.com", "", now)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	secondIDGen := func() (string, error) { return "test-id-2", nil }

	svc2 := NewService(Options{
		DomainsRead:   newFakeDomainReadRepo(store),
		DomainsWrite:  newFakeDomainWriteRepo(store),
		DNSResolver:   newFakeDNSResolver(),
		AccessChecker: &fakeAccessChecker{},
		IDGen:         secondIDGen,
	})
	_, err = svc2.CreateSenderDomain(context.Background(), "ws-1", "user-1", "test.org", "", now)
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	thirdIDGen := func() (string, error) { return "test-id-3", nil }
	svc3 := NewService(Options{
		DomainsRead:   newFakeDomainReadRepo(store),
		DomainsWrite:  newFakeDomainWriteRepo(store),
		DNSResolver:   newFakeDNSResolver(),
		AccessChecker: &fakeAccessChecker{},
		IDGen:         thirdIDGen,
	})
	_, err = svc3.CreateSenderDomain(context.Background(), "ws-2", "user-1", "other.com", "", now)
	if err != nil {
		t.Fatalf("create other workspace: %v", err)
	}

	results, err := svc.ListSenderDomains(context.Background(), "ws-1", "user-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 domains in ws-1, got %d", len(results))
	}
}
