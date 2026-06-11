package provider

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/ports"
)

type SESVerifier struct {
	httpClient *http.Client
	certCache  sync.Map
}

func NewSESVerifier() *SESVerifier {
	return &SESVerifier{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type snsMessage struct {
	Type             string `json:"Type"`
	MessageID        string `json:"MessageId"`
	TopicArn         string `json:"TopicArn"`
	Message          string `json:"Message"`
	Subject          string `json:"Subject,omitempty"`
	SubscribeURL     string `json:"SubscribeURL,omitempty"`
	Timestamp        string `json:"Timestamp"`
	SignatureVersion string `json:"SignatureVersion"`
	Signature        string `json:"Signature"`
	SigningCertURL   string `json:"SigningCertURL"`
	Token            string `json:"Token,omitempty"`
}

func (v *SESVerifier) Verify(ctx context.Context, input ports.VerifyInput) error {
	var msgType string
	for k, vals := range input.Headers {
		if strings.EqualFold(k, "x-amz-sns-message-type") {
			if len(vals) > 0 {
				msgType = vals[0]
			}
			break
		}
	}
	if msgType == "" {
		return domain.ErrInvalidSignature
	}

	msg, err := v.parseAndVerifySignature(ctx, input.RawBody)
	if err != nil {
		return err
	}

	if msgType != msg.Type {
		return domain.ErrInvalidSignature
	}

	switch msgType {
	case "SubscriptionConfirmation", "UnsubscribeConfirmation":
		return v.handleConfirmation(ctx, msg)
	case "Notification":
		return nil
	default:
		return domain.ErrInvalidSignature
	}
}

func (v *SESVerifier) parseAndVerifySignature(ctx context.Context, rawBody []byte) (*snsMessage, error) {
	var msg snsMessage
	if err := json.Unmarshal(rawBody, &msg); err != nil {
		return nil, domain.ErrInvalidSignature
	}

	if msg.SignatureVersion != "1" && msg.SignatureVersion != "2" {
		return nil, domain.ErrInvalidSignature
	}

	certURL, err := url.Parse(msg.SigningCertURL)
	if err != nil {
		return nil, domain.ErrInvalidSignature
	}
	if certURL.Scheme != "https" {
		return nil, domain.ErrInvalidSignature
	}
	if !isSNSHost(certURL.Host) {
		return nil, domain.ErrInvalidSignature
	}

	stringToSign, err := buildSNSStringToSign(&msg)
	if err != nil {
		return nil, domain.ErrInvalidSignature
	}

	cert, err := v.getCert(ctx, msg.SigningCertURL)
	if err != nil {
		return nil, domain.ErrTemporarilyUnavailable
	}

	sig, err := base64.StdEncoding.DecodeString(msg.Signature)
	if err != nil {
		return nil, domain.ErrInvalidSignature
	}

	var hash []byte
	var hashFunc crypto.Hash
	if msg.SignatureVersion == "1" {
		h := sha1.Sum([]byte(stringToSign))
		hash = h[:]
		hashFunc = crypto.SHA1
	} else {
		h := sha256.Sum256([]byte(stringToSign))
		hash = h[:]
		hashFunc = crypto.SHA256
	}

	if err := rsa.VerifyPKCS1v15(cert.PublicKey.(*rsa.PublicKey), hashFunc, hash, sig); err != nil {
		return nil, domain.ErrInvalidSignature
	}

	return &msg, nil
}

func (v *SESVerifier) handleConfirmation(ctx context.Context, msg *snsMessage) error {
	if msg.SubscribeURL == "" {
		return nil
	}

	u, err := url.Parse(msg.SubscribeURL)
	if err != nil {
		return domain.ErrInvalidSignature
	}
	if u.Scheme != "https" {
		return domain.ErrInvalidSignature
	}
	if !isSNSHost(u.Host) {
		return domain.ErrInvalidSignature
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, msg.SubscribeURL, nil)
	if err != nil {
		return nil
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil
	}
	resp.Body.Close()
	return nil
}

func buildSNSStringToSign(msg *snsMessage) (string, error) {
	var b strings.Builder
	switch msg.Type {
	case "Notification":
		b.WriteString("Message\n")
		b.WriteString(msg.Message)
		b.WriteString("\nMessageId\n")
		b.WriteString(msg.MessageID)
		if msg.Subject != "" {
			b.WriteString("\nSubject\n")
			b.WriteString(msg.Subject)
		}
		b.WriteString("\nTimestamp\n")
		b.WriteString(msg.Timestamp)
		b.WriteString("\nTopicArn\n")
		b.WriteString(msg.TopicArn)
		b.WriteString("\nType\n")
		b.WriteString(msg.Type)
	case "SubscriptionConfirmation", "UnsubscribeConfirmation":
		b.WriteString("Message\n")
		b.WriteString(msg.Message)
		b.WriteString("\nMessageId\n")
		b.WriteString(msg.MessageID)
		b.WriteString("\nSubscribeURL\n")
		b.WriteString(msg.SubscribeURL)
		b.WriteString("\nTimestamp\n")
		b.WriteString(msg.Timestamp)
		b.WriteString("\nToken\n")
		b.WriteString(msg.Token)
		b.WriteString("\nTopicArn\n")
		b.WriteString(msg.TopicArn)
		b.WriteString("\nType\n")
		b.WriteString(msg.Type)
	default:
		return "", fmt.Errorf("unknown SNS message type: %s", msg.Type)
	}
	return b.String(), nil
}

func isSNSHost(host string) bool {
	if !strings.HasPrefix(host, "sns.") {
		return false
	}
	return strings.HasSuffix(host, ".amazonaws.com") || strings.HasSuffix(host, ".amazonaws.com.cn")
}

func (v *SESVerifier) getCert(ctx context.Context, certURL string) (*x509.Certificate, error) {
	if cached, ok := v.certCache.Load(certURL); ok {
		return cached.(*x509.Certificate), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, certURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	pemBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}

	if _, ok := cert.PublicKey.(*rsa.PublicKey); !ok {
		return nil, fmt.Errorf("certificate does not contain RSA public key")
	}

	v.certCache.Store(certURL, cert)
	return cert, nil
}

type sesNotification struct {
	NotificationType string               `json:"notificationType"`
	Mail             sesMail              `json:"mail"`
	Bounce           *sesBounce           `json:"bounce,omitempty"`
	Complaint        *sesComplaint        `json:"complaint,omitempty"`
	Delivery         *sesDelivery         `json:"delivery,omitempty"`
	Open             *sesOpen             `json:"open,omitempty"`
	Click            *sesClick            `json:"click,omitempty"`
	Send             *sesSend             `json:"send,omitempty"`
	Reject           *sesReject           `json:"reject,omitempty"`
	RenderingFailure *sesRenderingFailure `json:"renderingFailure,omitempty"`
}

type sesMail struct {
	Timestamp        time.Time              `json:"timestamp"`
	MessageID        string                 `json:"messageId"`
	Source           string                 `json:"source"`
	Destination      []string               `json:"destination"`
	HeadersTruncated bool                   `json:"headersTruncated"`
	Headers          []map[string]string    `json:"headers"`
	CommonHeaders    map[string]interface{} `json:"commonHeaders"`
	Tags             map[string][]string    `json:"tags"`
}

type sesBounce struct {
	BounceType        string         `json:"bounceType"`
	BounceSubType     string         `json:"bounceSubType"`
	BouncedRecipients []sesRecipient `json:"bouncedRecipients"`
	Timestamp         time.Time      `json:"timestamp"`
	FeedbackID        string         `json:"feedbackId"`
}

type sesComplaint struct {
	ComplaintRecipients []sesRecipient `json:"complainedRecipients"`
	Timestamp           time.Time      `json:"timestamp"`
	FeedbackID          string         `json:"feedbackId"`
	ComplaintSubType    string         `json:"complaintSubType"`
}

type sesDelivery struct {
	Timestamp      time.Time `json:"timestamp"`
	ProcessingTime int       `json:"processingTimeMillis"`
	Recipients     []string  `json:"recipients"`
	SmtpResponse   string    `json:"smtpResponse"`
	ReportingMTA   string    `json:"reportingMTA"`
}

type sesOpen struct {
	Timestamp time.Time `json:"timestamp"`
	IPAddress string    `json:"ipAddress"`
	UserAgent string    `json:"userAgent"`
	Link      string    `json:"link"`
}

type sesClick struct {
	Timestamp time.Time `json:"timestamp"`
	IPAddress string    `json:"ipAddress"`
	UserAgent string    `json:"userAgent"`
	Link      string    `json:"link"`
}

type sesSend struct {
	Timestamp time.Time `json:"timestamp"`
}

type sesReject struct {
	Reason string `json:"reason"`
}

type sesRenderingFailure struct {
	TemplateName string `json:"templateName"`
	ErrorMessage string `json:"errorMessage"`
}

type sesRecipient struct {
	EmailAddress   string `json:"emailAddress"`
	Status         string `json:"status,omitempty"`
	DiagnosticCode string `json:"diagnosticCode,omitempty"`
}

type SESNormalizer struct{}

func (n *SESNormalizer) Normalize(_ context.Context, input ports.NormalizeInput) (*ports.NormalizedProviderEventInput, error) {
	rawBody := input.RawBody

	var sns snsMessage
	if err := json.Unmarshal(rawBody, &sns); err == nil && sns.Type == "Notification" && sns.Message != "" {
		rawBody = []byte(sns.Message)
	}

	var notif sesNotification
	if err := json.Unmarshal(rawBody, &notif); err != nil {
		return nil, fmt.Errorf("%w: failed to parse SES notification: %v", domain.ErrPayloadInvalid, err)
	}

	eventType, err := mapSESNotificationType(notif.NotificationType)
	if err != nil {
		return nil, err
	}

	providerEventID := notif.Mail.MessageID
	if notif.Bounce != nil && notif.Bounce.FeedbackID != "" {
		providerEventID = notif.Bounce.FeedbackID
	} else if notif.Complaint != nil && notif.Complaint.FeedbackID != "" {
		providerEventID = notif.Complaint.FeedbackID
	} else if notif.Delivery != nil {
		providerEventID = notif.Mail.MessageID + ":delivery"
	} else if notif.Open != nil {
		providerEventID = notif.Mail.MessageID + fmt.Sprintf(":open:%d", notif.Open.Timestamp.UnixNano())
	} else if notif.Click != nil {
		providerEventID = notif.Mail.MessageID + fmt.Sprintf(":click:%d", notif.Click.Timestamp.UnixNano())
	}

	occurredAt := notif.Mail.Timestamp.UTC()
	if notif.Bounce != nil && !notif.Bounce.Timestamp.IsZero() {
		occurredAt = notif.Bounce.Timestamp.UTC()
	} else if notif.Complaint != nil && !notif.Complaint.Timestamp.IsZero() {
		occurredAt = notif.Complaint.Timestamp.UTC()
	} else if notif.Delivery != nil && !notif.Delivery.Timestamp.IsZero() {
		occurredAt = notif.Delivery.Timestamp.UTC()
	} else if notif.Open != nil && !notif.Open.Timestamp.IsZero() {
		occurredAt = notif.Open.Timestamp.UTC()
	} else if notif.Click != nil && !notif.Click.Timestamp.IsZero() {
		occurredAt = notif.Click.Timestamp.UTC()
	}

	return &ports.NormalizedProviderEventInput{
		ProviderEventID:   providerEventID,
		ProviderMessageID: notif.Mail.MessageID,
		EventType:         eventType,
		OccurredAt:        occurredAt,
		PayloadJSON:       input.RawBody,
	}, nil
}

func mapSESNotificationType(notificationType string) (string, error) {
	switch notificationType {
	case "Bounce":
		return domain.EventTypeBounced, nil
	case "Complaint":
		return domain.EventTypeComplained, nil
	case "Delivery":
		return domain.EventTypeDelivered, nil
	case "Send":
		return domain.EventTypeAccepted, nil
	case "Open":
		return domain.EventTypeOpened, nil
	case "Click":
		return domain.EventTypeClicked, nil
	case "Reject":
		return domain.EventTypeRejected, nil
	case "RenderingFailure":
		return domain.EventTypeRenderingFailed, nil
	default:
		return "", fmt.Errorf("%w: unknown SES notification type: %s", domain.ErrPayloadInvalid, notificationType)
	}
}
