package provider

import (
	"context"
	"errors"
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
