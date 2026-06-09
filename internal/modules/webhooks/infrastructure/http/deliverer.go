package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
)

var defaultDialer = &net.Dialer{
	Timeout:   10 * time.Second,
	KeepAlive: 30 * time.Second,
}

const (
	userAgent     = "sendflow-webhook/1.0"
	clientTimeout = 10 * time.Second
	maxBodySize   = 1 << 20
)

type Deliverer struct {
	client *http.Client
	signer func(payload []byte, timestamp, secret string) string
}

func NewDeliverer() *Deliverer {
	return &Deliverer{
		client: &http.Client{
			Timeout: clientTimeout,
			Transport: &http.Transport{
				DisableKeepAlives: false,
				DialContext:       dialContext,
			},
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		signer: SignPayload,
	}
}

func dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q: %w", addr, err)
	}
	if ip := net.ParseIP(host); ip != nil {
		if domain.IsPrivateIP(ip) {
			return nil, errors.New("dial to private IP address blocked")
		}
		return defaultDialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
	}
	ips, err := defaultDialer.Resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("DNS resolution failed for %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("DNS returned no addresses for %q", host)
	}
	var firstPublic net.IP
	for _, ip := range ips {
		if domain.IsPrivateIP(ip.IP) {
			return nil, fmt.Errorf("dial to host %q resolved to private IP %s, blocked", host, ip.IP)
		}
		if firstPublic == nil {
			firstPublic = ip.IP
		}
	}
	return defaultDialer.DialContext(ctx, network, net.JoinHostPort(firstPublic.String(), port))
}

func NewDelivererWithClient(client *http.Client, signer func(payload []byte, timestamp, secret string) string) *Deliverer {
	return &Deliverer{client: client, signer: signer}
}

func SignPayload(payload []byte, timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func (d *Deliverer) Deliver(ctx context.Context, req ports.DeliveryHTTPRequest) (ports.DeliveryHTTPResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.URL, strings.NewReader(string(req.Body)))
	if err != nil {
		return ports.DeliveryHTTPResponse{
			StatusCode: 0,
			Error:      "failed to create request: " + err.Error(),
		}, nil
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", userAgent)
	httpReq.Header.Set(req.SignatureHeader, req.SignatureValue)
	httpReq.Header.Set(req.TimestampHeader, req.TimestampValue)
	httpReq.Header.Set(req.EventIDHeader, req.EventIDValue)

	start := time.Now()
	resp, err := d.client.Do(httpReq)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return ports.DeliveryHTTPResponse{
			StatusCode: 0,
			Error:      "request failed: " + err.Error(),
			DurationMs: duration,
		}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))

	headers := make(map[string]string)
	for k := range resp.Header {
		if len(resp.Header[k]) > 0 {
			headers[k] = resp.Header.Get(k)
		}
	}

	return ports.DeliveryHTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       string(body),
		DurationMs: duration,
	}, nil
}
