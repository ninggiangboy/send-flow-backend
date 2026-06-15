package realtime

import (
	"os"
	"testing"
)

func TestWorkspaceChannel(t *testing.T) {
	ch := WorkspaceChannel("ws-1")
	want := "sendflow:notifications:workspace:ws-1"
	if ch != want {
		t.Errorf("WorkspaceChannel = %q, want %q", ch, want)
	}
}

func TestUserChannel(t *testing.T) {
	ch := UserChannel("user-1")
	want := "sendflow:notifications:user:user-1"
	if ch != want {
		t.Errorf("UserChannel = %q, want %q", ch, want)
	}
}

func TestIntegrationBackplane_PublishSubscribe(t *testing.T) {
	if os.Getenv("REDIS_INTEGRATION_TEST") == "" {
		t.Skip("set REDIS_INTEGRATION_TEST=1 to run Redis integration tests")
	}

	client := newTestRedisClient(t)
	backplane := NewBackplane(client, newTestLogger(t))

	ctx := t.Context()
	ev := Event{
		EventID:         "evt-1",
		EventType:       "notification.message.queued.v1",
		MessageID:       "msg-1",
		WorkspaceID:     "ws-1",
		RecipientUserID: "user-1",
		Type:            "welcome_email",
		Status:          "queued",
		OccurredAt:      mustParseTime("2026-06-15T12:00:00Z"),
	}

	subCh, unsub, err := backplane.Subscribe(ctx, WorkspaceChannel("ws-1"))
	if err != nil {
		t.Fatal(err)
	}
	defer unsub()

	if err := backplane.Publish(ctx, ev); err != nil {
		t.Fatal(err)
	}

	select {
	case received := <-subCh:
		if received.EventID != "evt-1" {
			t.Errorf("received EventID = %q, want evt-1", received.EventID)
		}
		if received.RecipientUserID != "user-1" {
			t.Errorf("received RecipientUserID = %q, want user-1", received.RecipientUserID)
		}
	default:
		t.Fatal("expected to receive event on workspace channel")
	}
}

func TestIntegrationBackplane_PublishToBothChannels(t *testing.T) {
	if os.Getenv("REDIS_INTEGRATION_TEST") == "" {
		t.Skip("set REDIS_INTEGRATION_TEST=1 to run Redis integration tests")
	}

	client := newTestRedisClient(t)
	backplane := NewBackplane(client, newTestLogger(t))

	ctx := t.Context()
	ev := Event{
		EventID:         "evt-2",
		EventType:       "notification.message.sent.v1",
		MessageID:       "msg-2",
		WorkspaceID:     "ws-2",
		RecipientUserID: "user-2",
		Type:            "invitation_email",
		Status:          "sent",
		OccurredAt:      mustParseTime("2026-06-15T12:00:00Z"),
	}

	wsCh, wsUnsub, err := backplane.Subscribe(ctx, WorkspaceChannel("ws-2"))
	if err != nil {
		t.Fatal(err)
	}
	defer wsUnsub()

	userCh, userUnsub, err := backplane.Subscribe(ctx, UserChannel("user-2"))
	if err != nil {
		t.Fatal(err)
	}
	defer userUnsub()

	if err := backplane.Publish(ctx, ev); err != nil {
		t.Fatal(err)
	}

	select {
	case <-wsCh:
	default:
		t.Error("expected event on workspace channel")
	}

	select {
	case <-userCh:
	default:
		t.Error("expected event on user channel")
	}
}
