package provider

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/ports"
)

func TestSESVerifier_MissingHeader(t *testing.T) {
	v := NewSESVerifier()
	err := v.Verify(context.Background(), ports.VerifyInput{
		Headers: map[string][]string{},
		RawBody: []byte(`{}`),
	})
	if !errors.Is(err, domain.ErrInvalidSignature) {
		t.Errorf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestSESVerifier_UnknownMessageType(t *testing.T) {
	v := NewSESVerifier()
	err := v.Verify(context.Background(), ports.VerifyInput{
		Headers: map[string][]string{
			"x-amz-sns-message-type": {"UnknownType"},
		},
		RawBody: []byte(`{"Type":"UnknownType","SignatureVersion":"1","SigningCertURL":"https://sns.us-east-1.amazonaws.com/cert.pem","Signature":"dGVzdA==","MessageId":"id","TopicArn":"arn","Timestamp":"2024-01-01T00:00:00Z","Message":"hello"}`),
	})
	if !errors.Is(err, domain.ErrInvalidSignature) {
		t.Errorf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestSESVerifier_InvalidJSON(t *testing.T) {
	v := NewSESVerifier()
	err := v.Verify(context.Background(), ports.VerifyInput{
		Headers: map[string][]string{
			"x-amz-sns-message-type": {"Notification"},
		},
		RawBody: []byte(`not json`),
	})
	if !errors.Is(err, domain.ErrInvalidSignature) {
		t.Errorf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestSESVerifier_InvalidSignatureVersion(t *testing.T) {
	v := NewSESVerifier()
	err := v.Verify(context.Background(), ports.VerifyInput{
		Headers: map[string][]string{
			"x-amz-sns-message-type": {"Notification"},
		},
		RawBody: []byte(`{"Type":"Notification","SignatureVersion":"3","SigningCertURL":"https://sns.us-east-1.amazonaws.com/cert.pem","Signature":"dGVzdA==","MessageId":"id","TopicArn":"arn","Timestamp":"2024-01-01T00:00:00Z","Message":"hello"}`),
	})
	if !errors.Is(err, domain.ErrInvalidSignature) {
		t.Errorf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestSESVerifier_NonHTTPSCertURL(t *testing.T) {
	v := NewSESVerifier()
	err := v.Verify(context.Background(), ports.VerifyInput{
		Headers: map[string][]string{
			"x-amz-sns-message-type": {"Notification"},
		},
		RawBody: []byte(`{"Type":"Notification","SignatureVersion":"1","SigningCertURL":"http://sns.us-east-1.amazonaws.com/cert.pem","Signature":"dGVzdA==","MessageId":"id","TopicArn":"arn","Timestamp":"2024-01-01T00:00:00Z","Message":"hello"}`),
	})
	if !errors.Is(err, domain.ErrInvalidSignature) {
		t.Errorf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestSESVerifier_NonSNSHostCertURL(t *testing.T) {
	v := NewSESVerifier()
	err := v.Verify(context.Background(), ports.VerifyInput{
		Headers: map[string][]string{
			"x-amz-sns-message-type": {"Notification"},
		},
		RawBody: []byte(`{"Type":"Notification","SignatureVersion":"1","SigningCertURL":"https://evil.com/cert.pem","Signature":"dGVzdA==","MessageId":"id","TopicArn":"arn","Timestamp":"2024-01-01T00:00:00Z","Message":"hello"}`),
	})
	if !errors.Is(err, domain.ErrInvalidSignature) {
		t.Errorf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestSESVerifier_EmptyBody(t *testing.T) {
	v := NewSESVerifier()
	err := v.Verify(context.Background(), ports.VerifyInput{
		Headers: map[string][]string{
			"x-amz-sns-message-type": {"Notification"},
		},
		RawBody: []byte(``),
	})
	if !errors.Is(err, domain.ErrInvalidSignature) {
		t.Errorf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestSESVerifier_XAmzSnsMessageTypeCaseInsensitive(t *testing.T) {
	v := NewSESVerifier()
	err := v.Verify(context.Background(), ports.VerifyInput{
		Headers: map[string][]string{
			"X-Amz-Sns-Message-Type": {"Notification"},
		},
		RawBody: []byte(`{"Type":"Notification","SignatureVersion":"1","SigningCertURL":"https://sns.us-east-1.amazonaws.com/cert.pem","Signature":"dGVzdA==","MessageId":"id","TopicArn":"arn","Timestamp":"2024-01-01T00:00:00Z","Message":"hello"}`),
	})
	if !errors.Is(err, domain.ErrTemporarilyUnavailable) {
		t.Errorf("expected ErrTemporarilyUnavailable (header found, cert fetch fails), got %v", err)
	}
}

func TestSESVerifier_ConfirmationSNSSubscribeURLAllowed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	v := NewSESVerifier()
	msg := &snsMessage{
		Type:             "SubscriptionConfirmation",
		MessageID:        "id",
		TopicArn:         "arn:aws:sns:us-east-1:123:topic",
		Message:          "hello",
		SubscribeURL:     "https://sns.us-east-1.amazonaws.com/confirmation-url",
		Timestamp:        "2024-01-01T00:00:00Z",
		Token:            "token",
		SignatureVersion: "1",
		Signature:        "dGVzdA==",
		SigningCertURL:   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	err := v.handleConfirmation(ctx, msg)
	// Errors from the HTTP GET are explicitly swallowed; the test verifies
	// that the SNS host constraint passes (no ErrInvalidSignature).
	if err != nil {
		t.Errorf("expected no error for valid SNS SubscribeURL, got %v", err)
	}
}

func TestSESVerifier_ConfirmationNonHTTPSSubscribeURL(t *testing.T) {
	v := NewSESVerifier()
	msg := &snsMessage{
		Type:             "SubscriptionConfirmation",
		MessageID:        "id",
		TopicArn:         "arn:aws:sns:us-east-1:123:topic",
		Message:          "hello",
		SubscribeURL:     "http://sns.us-east-1.amazonaws.com/confirmation-url",
		Timestamp:        "2024-01-01T00:00:00Z",
		Token:            "token",
		SignatureVersion: "1",
		Signature:        "dGVzdA==",
		SigningCertURL:   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	err := v.handleConfirmation(context.Background(), msg)
	if !errors.Is(err, domain.ErrInvalidSignature) {
		t.Errorf("expected ErrInvalidSignature for non-HTTPS SubscribeURL, got %v", err)
	}
}

func TestSESVerifier_ConfirmationNonSNSSubscribeURL(t *testing.T) {
	v := NewSESVerifier()
	msg := &snsMessage{
		Type:             "SubscriptionConfirmation",
		MessageID:        "id",
		TopicArn:         "arn:aws:sns:us-east-1:123:topic",
		Message:          "hello",
		SubscribeURL:     "https://evil.com/confirmation-url",
		Timestamp:        "2024-01-01T00:00:00Z",
		Token:            "token",
		SignatureVersion: "1",
		Signature:        "dGVzdA==",
		SigningCertURL:   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	err := v.handleConfirmation(context.Background(), msg)
	if !errors.Is(err, domain.ErrInvalidSignature) {
		t.Errorf("expected ErrInvalidSignature for non-SNS SubscribeURL, got %v", err)
	}
}

func TestSESVerifier_ConfirmationMissingSubscribeURL(t *testing.T) {
	v := NewSESVerifier()
	msg := &snsMessage{
		Type:             "SubscriptionConfirmation",
		MessageID:        "id",
		TopicArn:         "arn:aws:sns:us-east-1:123:topic",
		Message:          "hello",
		SubscribeURL:     "",
		Timestamp:        "2024-01-01T00:00:00Z",
		Token:            "token",
		SignatureVersion: "1",
		Signature:        "dGVzdA==",
		SigningCertURL:   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	err := v.handleConfirmation(context.Background(), msg)
	if err != nil {
		t.Errorf("expected no error for empty SubscribeURL, got %v", err)
	}
}

func TestSESVerifier_ChinaRegionCertURL(t *testing.T) {
	v := NewSESVerifier()
	err := v.Verify(context.Background(), ports.VerifyInput{
		Headers: map[string][]string{
			"x-amz-sns-message-type": {"Notification"},
		},
		RawBody: []byte(`{"Type":"Notification","SignatureVersion":"1","SigningCertURL":"https://sns.cn-north-1.amazonaws.com.cn/cert.pem","Signature":"dGVzdA==","MessageId":"id","TopicArn":"arn","Timestamp":"2024-01-01T00:00:00Z","Message":"hello"}`),
	})
	if !errors.Is(err, domain.ErrTemporarilyUnavailable) {
		t.Errorf("expected ErrTemporarilyUnavailable (amazonaws.com.cn passes host check, cert fetch fails), got %v", err)
	}
}

func TestBuildSNSStringToSign_Notification(t *testing.T) {
	msg := &snsMessage{
		Type:             "Notification",
		MessageID:        "msg-id",
		TopicArn:         "arn:aws:sns:us-east-1:123:topic",
		Subject:          "subject",
		Message:          "message body",
		Timestamp:        "2024-01-01T00:00:00Z",
		SignatureVersion: "1",
		Signature:        "sig",
		SigningCertURL:   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	result, err := buildSNSStringToSign(msg)
	if err != nil {
		t.Fatal(err)
	}
	expected := "Message\nmessage body\nMessageId\nmsg-id\nSubject\nsubject\nTimestamp\n2024-01-01T00:00:00Z\nTopicArn\narn:aws:sns:us-east-1:123:topic\nType\nNotification"
	if result != expected {
		t.Errorf("unexpected string-to-sign:\ngot:\n%q\nwant:\n%q", result, expected)
	}
}

func TestBuildSNSStringToSign_SubscriptionConfirmation(t *testing.T) {
	msg := &snsMessage{
		Type:             "SubscriptionConfirmation",
		MessageID:        "msg-id",
		TopicArn:         "arn:aws:sns:us-east-1:123:topic",
		Message:          "message body",
		SubscribeURL:     "https://sns.us-east-1.amazonaws.com/confirm",
		Timestamp:        "2024-01-01T00:00:00Z",
		Token:            "token123",
		SignatureVersion: "1",
		Signature:        "sig",
		SigningCertURL:   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	result, err := buildSNSStringToSign(msg)
	if err != nil {
		t.Fatal(err)
	}
	expected := "Message\nmessage body\nMessageId\nmsg-id\nSubscribeURL\nhttps://sns.us-east-1.amazonaws.com/confirm\nTimestamp\n2024-01-01T00:00:00Z\nToken\ntoken123\nTopicArn\narn:aws:sns:us-east-1:123:topic\nType\nSubscriptionConfirmation"
	if result != expected {
		t.Errorf("unexpected string-to-sign:\ngot:\n%q\nwant:\n%q", result, expected)
	}
}

func TestSESVerifier_VerificationFailsOnCertFetch(t *testing.T) {
	v := NewSESVerifier()
	body := `{"Type":"Notification","SignatureVersion":"1","SigningCertURL":"https://sns.us-east-1.amazonaws.com/cert.pem","Signature":"dGVzdA==","MessageId":"id","TopicArn":"arn","Timestamp":"2024-01-01T00:00:00Z","Message":"hello"}`
	err := v.Verify(context.Background(), ports.VerifyInput{
		Headers: map[string][]string{
			"x-amz-sns-message-type": {"Notification"},
		},
		RawBody: []byte(body),
	})
	if !errors.Is(err, domain.ErrTemporarilyUnavailable) {
		t.Errorf("expected ErrTemporarilyUnavailable (cert fetch fails), got %v", err)
	}
}

// Test that SubscribeURL without valid URL parses returns ErrInvalidSignature.
func TestSESVerifier_ConfirmationInvalidSubscribeURL(t *testing.T) {
	v := NewSESVerifier()
	msg := &snsMessage{
		Type:             "SubscriptionConfirmation",
		MessageID:        "id",
		TopicArn:         "arn:aws:sns:us-east-1:123:topic",
		Message:          "hello",
		SubscribeURL:     string([]byte{0x7f}),
		Timestamp:        "2024-01-01T00:00:00Z",
		Token:            "token",
		SignatureVersion: "1",
		Signature:        "dGVzdA==",
		SigningCertURL:   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	err := v.handleConfirmation(context.Background(), msg)
	if !errors.Is(err, domain.ErrInvalidSignature) {
		t.Errorf("expected ErrInvalidSignature for unparseable SubscribeURL, got %v", err)
	}
}

func TestSESVerifier_UnsubscribeConfirmation(t *testing.T) {
	v := NewSESVerifier()
	ctx := context.Background()
	body := `{"Type":"UnsubscribeConfirmation","MessageId":"id","Token":"token","TopicArn":"arn:aws:sns:us-east-1:123:topic","Message":"unsubscribed","SubscribeURL":"https://sns.us-east-1.amazonaws.com/confirm","Timestamp":"2024-01-01T00:00:00Z","SignatureVersion":"1","Signature":"dGVzdA==","SigningCertURL":"https://sns.us-east-1.amazonaws.com/cert.pem"}`
	err := v.Verify(ctx, ports.VerifyInput{
		Headers: map[string][]string{
			"x-amz-sns-message-type": {"UnsubscribeConfirmation"},
		},
		RawBody: []byte(body),
	})
	if !errors.Is(err, domain.ErrTemporarilyUnavailable) {
		t.Errorf("expected ErrTemporarilyUnavailable (valid SNS message, cert fetch fails), got %v", err)
	}
}

func TestMapSESNotificationType_Bounce(t *testing.T) {
	eventType, err := mapSESNotificationType("Bounce")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eventType != domain.EventTypeBounced {
		t.Errorf("expected %q, got %q", domain.EventTypeBounced, eventType)
	}
}

func TestMapSESNotificationType_Complaint(t *testing.T) {
	eventType, err := mapSESNotificationType("Complaint")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eventType != domain.EventTypeComplained {
		t.Errorf("expected %q, got %q", domain.EventTypeComplained, eventType)
	}
}

func TestMapSESNotificationType_Delivery(t *testing.T) {
	eventType, err := mapSESNotificationType("Delivery")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eventType != domain.EventTypeDelivered {
		t.Errorf("expected %q, got %q", domain.EventTypeDelivered, eventType)
	}
}

func TestMapSESNotificationType_Open(t *testing.T) {
	eventType, err := mapSESNotificationType("Open")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eventType != domain.EventTypeOpened {
		t.Errorf("expected %q, got %q", domain.EventTypeOpened, eventType)
	}
}

func TestMapSESNotificationType_Click(t *testing.T) {
	eventType, err := mapSESNotificationType("Click")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eventType != domain.EventTypeClicked {
		t.Errorf("expected %q, got %q", domain.EventTypeClicked, eventType)
	}
}

func TestMapSESNotificationType_Unknown(t *testing.T) {
	_, err := mapSESNotificationType("UnknownType")
	if err == nil {
		t.Fatal("expected error for unknown SES notification type")
	}
}

func TestSESNormalizer_NormalizeBounce(t *testing.T) {
	n := &SESNormalizer{}
	input := ports.NormalizeInput{
		RawBody: []byte(`{
			"Type": "Notification",
			"MessageId": "msg-id",
			"TopicArn": "arn:aws:sns:us-east-1:123:topic",
			"Message": "{\"notificationType\":\"Bounce\",\"bounce\":{\"bounceType\":\"Permanent\",\"bounceSubType\":\"General\",\"bouncedRecipients\":[{\"emailAddress\":\"bounce@example.com\",\"status\":\"5.1.1\",\"diagnosticCode\":\"smtp; 550 5.1.1 user unknown\"}],\"timestamp\":\"2024-01-01T00:00:00.000Z\",\"feedbackId\":\"fb-id-001\"},\"mail\":{\"timestamp\":\"2024-01-01T00:00:00.000Z\",\"messageId\":\"msg-id\",\"source\":\"sender@example.com\",\"destination\":[\"bounce@example.com\"]}}",
			"Timestamp": "2024-01-01T00:00:00Z",
			"SignatureVersion": "1",
			"Signature": "dGVzdA==",
			"SigningCertURL": "https://sns.us-east-1.amazonaws.com/cert.pem"
		}`),
	}
	result, err := n.Normalize(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EventType != domain.EventTypeBounced {
		t.Errorf("expected event type %q, got %q", domain.EventTypeBounced, result.EventType)
	}
	if result.ProviderEventID != "fb-id-001" {
		t.Errorf("expected provider event ID fb-id-001, got %q", result.ProviderEventID)
	}
	if result.ProviderMessageID != "msg-id" {
		t.Errorf("expected provider message ID msg-id, got %q", result.ProviderMessageID)
	}
}

func TestSESNormalizer_NormalizeDelivery(t *testing.T) {
	n := &SESNormalizer{}
	input := ports.NormalizeInput{
		RawBody: []byte(`{
			"Type": "Notification",
			"MessageId": "msg-id",
			"TopicArn": "arn:aws:sns:us-east-1:123:topic",
			"Message": "{\"notificationType\":\"Delivery\",\"delivery\":{\"timestamp\":\"2024-01-01T00:00:00.000Z\",\"processingTimeMillis\":1234,\"recipients\":[\"delivered@example.com\"],\"smtpResponse\":\"250 ok\",\"reportingMTA\":\"mta.example.com\"},\"mail\":{\"timestamp\":\"2024-01-01T00:00:00.000Z\",\"messageId\":\"msg-id\",\"source\":\"sender@example.com\",\"destination\":[\"delivered@example.com\"]}}",
			"Timestamp": "2024-01-01T00:00:00Z",
			"SignatureVersion": "1",
			"Signature": "dGVzdA==",
			"SigningCertURL": "https://sns.us-east-1.amazonaws.com/cert.pem"
		}`),
	}
	result, err := n.Normalize(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EventType != domain.EventTypeDelivered {
		t.Errorf("expected event type %q, got %q", domain.EventTypeDelivered, result.EventType)
	}
}

func TestSESNormalizer_NormalizeComplaint(t *testing.T) {
	n := &SESNormalizer{}
	input := ports.NormalizeInput{
		RawBody: []byte(`{
			"Type": "Notification",
			"MessageId": "msg-id",
			"TopicArn": "arn:aws:sns:us-east-1:123:topic",
			"Message": "{\"notificationType\":\"Complaint\",\"complaint\":{\"complainedRecipients\":[{\"emailAddress\":\"complaint@example.com\"}],\"timestamp\":\"2024-01-01T00:00:00.000Z\",\"feedbackId\":\"fb-complaint-001\"},\"mail\":{\"timestamp\":\"2024-01-01T00:00:00.000Z\",\"messageId\":\"msg-id\",\"source\":\"sender@example.com\",\"destination\":[\"complaint@example.com\"]}}",
			"Timestamp": "2024-01-01T00:00:00Z",
			"SignatureVersion": "1",
			"Signature": "dGVzdA==",
			"SigningCertURL": "https://sns.us-east-1.amazonaws.com/cert.pem"
		}`),
	}
	result, err := n.Normalize(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EventType != domain.EventTypeComplained {
		t.Errorf("expected event type %q, got %q", domain.EventTypeComplained, result.EventType)
	}
}

func TestSESNormalizer_NormalizeOpen(t *testing.T) {
	n := &SESNormalizer{}
	input := ports.NormalizeInput{
		RawBody: []byte(`{
			"Type": "Notification",
			"MessageId": "msg-id",
			"TopicArn": "arn:aws:sns:us-east-1:123:topic",
			"Message": "{\"notificationType\":\"Open\",\"open\":{\"timestamp\":\"2024-01-01T00:00:00.000Z\",\"ipAddress\":\"1.2.3.4\",\"userAgent\":\"Mozilla/5.0\"},\"mail\":{\"timestamp\":\"2024-01-01T00:00:00.000Z\",\"messageId\":\"msg-id\",\"source\":\"sender@example.com\",\"destination\":[\"opened@example.com\"]}}",
			"Timestamp": "2024-01-01T00:00:00Z",
			"SignatureVersion": "1",
			"Signature": "dGVzdA==",
			"SigningCertURL": "https://sns.us-east-1.amazonaws.com/cert.pem"
		}`),
	}
	result, err := n.Normalize(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EventType != domain.EventTypeOpened {
		t.Errorf("expected event type %q, got %q", domain.EventTypeOpened, result.EventType)
	}
}

func TestSESVerifier_MultiValueHeader(t *testing.T) {
	v := NewSESVerifier()
	err := v.Verify(context.Background(), ports.VerifyInput{
		Headers: map[string][]string{
			"x-amz-sns-message-type": {"Notification", "extra-value"},
		},
		RawBody: []byte(`{"Type":"Notification","SignatureVersion":"1","SigningCertURL":"https://sns.us-east-1.amazonaws.com/cert.pem","Signature":"dGVzdA==","MessageId":"id","TopicArn":"arn","Timestamp":"2024-01-01T00:00:00Z","Message":"hello"}`),
	})
	if !errors.Is(err, domain.ErrTemporarilyUnavailable) {
		t.Errorf("expected ErrTemporarilyUnavailable (first value used, cert fetch fails), got %v", err)
	}
}

func TestBuildSNSStringToSign_UnsubscribeConfirmation(t *testing.T) {
	msg := &snsMessage{
		Type:             "UnsubscribeConfirmation",
		MessageID:        "msg-id",
		TopicArn:         "arn:aws:sns:us-east-1:123:topic",
		Message:          "unsubscribed",
		SubscribeURL:     "https://sns.us-east-1.amazonaws.com/unsub",
		Timestamp:        "2024-01-01T00:00:00Z",
		Token:            "token456",
		SignatureVersion: "1",
		Signature:        "sig",
		SigningCertURL:   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	result, err := buildSNSStringToSign(msg)
	if err != nil {
		t.Fatal(err)
	}
	expected := "Message\nunsubscribed\nMessageId\nmsg-id\nSubscribeURL\nhttps://sns.us-east-1.amazonaws.com/unsub\nTimestamp\n2024-01-01T00:00:00Z\nToken\ntoken456\nTopicArn\narn:aws:sns:us-east-1:123:topic\nType\nUnsubscribeConfirmation"
	if result != expected {
		t.Errorf("unexpected string-to-sign:\ngot:\n%q\nwant:\n%q", result, expected)
	}
}

func TestBuildSNSStringToSign_NotificationWithoutSubject(t *testing.T) {
	msg := &snsMessage{
		Type:             "Notification",
		MessageID:        "msg-id",
		TopicArn:         "arn:aws:sns:us-east-1:123:topic",
		Message:          "body",
		Timestamp:        "2024-01-01T00:00:00Z",
		SignatureVersion: "1",
		Signature:        "sig",
		SigningCertURL:   "https://sns.us-east-1.amazonaws.com/cert.pem",
	}
	result, err := buildSNSStringToSign(msg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result, "\nSubject\n") {
		t.Errorf("expected Subject to be omitted from Notification string-to-sign when not present, got:\n%q", result)
	}
}

func TestIsSNSHost(t *testing.T) {
	tests := []struct {
		host string
		ok   bool
	}{
		{"sns.us-east-1.amazonaws.com", true},
		{"sns.cn-north-1.amazonaws.com.cn", true},
		{"sns.eu-west-2.amazonaws.com", true},
		{"notsns.us-east-1.amazonaws.com", false},
		{"sns.evil.com", false},
		{"evil.sns.us-east-1.amazonaws.com", false},
	}
	for _, tc := range tests {
		got := isSNSHost(tc.host)
		if got != tc.ok {
			t.Errorf("host %q: got %v, want %v", tc.host, got, tc.ok)
		}
	}
}
