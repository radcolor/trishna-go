package telegram

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gotd/td/tg"
)

func TestIsOwner(t *testing.T) {
	service := NewService(Config{OwnerUserIDs: []int64{123, 456}}, nil, testLogger())
	if !service.isOwner(123) {
		t.Fatal("expected owner")
	}
	if service.isOwner(789) {
		t.Fatal("expected non-owner")
	}
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		botUsername string
		want        string
		wantOK      bool
	}{
		{name: "plain", text: "/ping", botUsername: "trishna_bot", want: "ping", wantOK: true},
		{name: "mention", text: "/ping@trishna_bot", botUsername: "trishna_bot", want: "ping", wantOK: true},
		{name: "mention case", text: "/PING@Trishna_Bot", botUsername: "trishna_bot", want: "ping", wantOK: true},
		{name: "args", text: "/ping now", botUsername: "trishna_bot", want: "ping", wantOK: true},
		{name: "wrong mention", text: "/ping@other_bot", botUsername: "trishna_bot", wantOK: false},
		{name: "not command", text: "ping", botUsername: "trishna_bot", wantOK: false},
		{name: "empty", text: "", botUsername: "trishna_bot", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseCommand(tt.text, tt.botUsername)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("command = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMessageSenderUserID(t *testing.T) {
	msg := &tg.Message{
		PeerID: &tg.PeerChat{ChatID: 99},
	}
	msg.SetFromID(&tg.PeerUser{UserID: 42})

	userID, ok := messageSenderUserID(msg)
	if !ok {
		t.Fatal("expected user id")
	}
	if userID != 42 {
		t.Fatalf("user id = %d", userID)
	}
}

func TestMessageSenderUserIDFallbackPrivatePeer(t *testing.T) {
	userID, ok := messageSenderUserID(&tg.Message{
		PeerID: &tg.PeerUser{UserID: 42},
	})
	if !ok {
		t.Fatal("expected user id")
	}
	if userID != 42 {
		t.Fatalf("user id = %d", userID)
	}
}

func TestRunOwnerPingSendsMessage(t *testing.T) {
	api := newFakeTelegramAPI(t, []fakeUpdate{
		messageUpdate(100, 42, 99, "/ping"),
	})
	defer api.close()

	service := NewService(Config{
		Token:        fakeToken,
		OwnerUserIDs: []int64{42},
		APIBaseURL:   api.URL(),
		Transport:    TransportBotAPI,
	}, nil, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := runService(ctx, service)

	select {
	case sent := <-api.sentMessages:
		cancel()
		if sent.chatID != 99 {
			t.Fatalf("chat id = %d", sent.chatID)
		}
		if sent.text != "jihyooo ❤️" {
			t.Fatalf("text = %q", sent.text)
		}
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("timed out waiting for sendMessage")
	}

	if err := waitService(errCh); err != nil {
		t.Fatalf("run service: %v", err)
	}
}

func TestRunOwnerStatusSendsMessage(t *testing.T) {
	api := newFakeTelegramAPI(t, []fakeUpdate{
		messageUpdate(100, 42, 99, "/status"),
	})
	defer api.close()

	service := NewService(Config{
		Token:        fakeToken,
		OwnerUserIDs: []int64{42},
		APIBaseURL:   api.URL(),
		Transport:    TransportBotAPI,
	}, nil, testLogger())
	service.SetStatusHandler(func(context.Context) string {
		return "**status ok**"
	})
	service.SetHTMLStatusHandler(func(context.Context) string {
		return "<b>status ok</b>"
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := runService(ctx, service)

	select {
	case sent := <-api.sentMessages:
		cancel()
		if sent.chatID != 99 {
			t.Fatalf("chat id = %d", sent.chatID)
		}
		if sent.text != "<b>status ok</b>" {
			t.Fatalf("text = %q", sent.text)
		}
		if sent.parseMode != "HTML" {
			t.Fatalf("parse mode = %q", sent.parseMode)
		}
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("timed out waiting for sendMessage")
	}

	if err := waitService(errCh); err != nil {
		t.Fatalf("run service: %v", err)
	}
}

func TestRunOwnerTGNetSendsNetworkReport(t *testing.T) {
	api := newFakeTelegramAPI(t, []fakeUpdate{
		messageUpdate(100, 42, 99, "/tgnet"),
	})
	defer api.close()

	ipServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "203.0.113.7")
	}))
	defer ipServer.Close()
	oldPublicIPCheckURL := publicIPCheckURL
	publicIPCheckURL = ipServer.URL
	defer func() { publicIPCheckURL = oldPublicIPCheckURL }()

	service := NewService(Config{
		Token:          fakeToken,
		OwnerUserIDs:   []int64{42},
		APIBaseURL:     api.URL(),
		Transport:      TransportBotAPI,
		TGNetRevealIPs: true,
	}, nil, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := runService(ctx, service)

	select {
	case sent := <-api.sentMessages:
		cancel()
		if sent.chatID != 99 {
			t.Fatalf("chat id = %d", sent.chatID)
		}
		if sent.parseMode != "HTML" {
			t.Fatalf("parse mode = %q", sent.parseMode)
		}
		if !strings.Contains(sent.text, "<b>Telegram Network</b>") {
			t.Fatalf("missing network header: %q", sent.text)
		}
		if !strings.Contains(sent.text, "Public IP:") || !strings.Contains(sent.text, "203.0.113.7") {
			t.Fatalf("missing public ip: %q", sent.text)
		}
		if strings.Contains(strings.ToLower(sent.text), "secret") {
			t.Fatalf("leaked secret label/value: %q", sent.text)
		}
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("timed out waiting for sendMessage")
	}

	if err := waitService(errCh); err != nil {
		t.Fatalf("run service: %v", err)
	}
}

func TestRunOwnerTGNetHidesIPsByDefault(t *testing.T) {
	api := newFakeTelegramAPI(t, []fakeUpdate{
		messageUpdate(100, 42, 99, "/tgnet"),
	})
	defer api.close()

	ipServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("public IP endpoint should not be called by default")
		_, _ = io.WriteString(w, "203.0.113.7")
	}))
	defer ipServer.Close()
	oldPublicIPCheckURL := publicIPCheckURL
	publicIPCheckURL = ipServer.URL
	defer func() { publicIPCheckURL = oldPublicIPCheckURL }()

	service := NewService(Config{
		Token:        fakeToken,
		OwnerUserIDs: []int64{42},
		APIBaseURL:   api.URL(),
		Transport:    TransportBotAPI,
	}, nil, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := runService(ctx, service)

	select {
	case sent := <-api.sentMessages:
		cancel()
		if !strings.Contains(sent.text, "Public IP:    hidden") {
			t.Fatalf("public ip not hidden: %q", sent.text)
		}
		if strings.Contains(sent.text, "203.0.113.7") {
			t.Fatalf("leaked public ip: %q", sent.text)
		}
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("timed out waiting for sendMessage")
	}

	if err := waitService(errCh); err != nil {
		t.Fatalf("run service: %v", err)
	}
}

func TestRunNonOwnerPingIsIgnored(t *testing.T) {
	api := newFakeTelegramAPI(t, []fakeUpdate{
		messageUpdate(100, 7, 99, "/ping"),
	})
	defer api.close()

	service := NewService(Config{
		Token:        fakeToken,
		OwnerUserIDs: []int64{42},
		APIBaseURL:   api.URL(),
		Transport:    TransportBotAPI,
	}, nil, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := runService(ctx, service)

	select {
	case <-api.afterScript:
		cancel()
	case sent := <-api.sentMessages:
		cancel()
		t.Fatalf("unexpected sendMessage: %+v", sent)
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("timed out waiting for updates")
	}

	if err := waitService(errCh); err != nil {
		t.Fatalf("run service: %v", err)
	}
}

func TestRunOwnerWrongChatIsIgnored(t *testing.T) {
	api := newFakeTelegramAPI(t, []fakeUpdate{
		messageUpdate(100, 42, 99, "/ping"),
	})
	defer api.close()

	service := NewService(Config{
		Token:          fakeToken,
		OwnerUserIDs:   []int64{42},
		AllowedChatIDs: []int64{100},
		APIBaseURL:     api.URL(),
		Transport:      TransportBotAPI,
	}, nil, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := runService(ctx, service)

	select {
	case <-api.afterScript:
		cancel()
	case sent := <-api.sentMessages:
		cancel()
		t.Fatalf("unexpected sendMessage: %+v", sent)
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("timed out waiting for updates")
	}

	if err := waitService(errCh); err != nil {
		t.Fatalf("run service: %v", err)
	}
}

func TestRunStopsOnCanceledContext(t *testing.T) {
	api := newFakeTelegramAPI(t, nil)
	defer api.close()

	service := NewService(Config{
		Token:        fakeToken,
		OwnerUserIDs: []int64{42},
		APIBaseURL:   api.URL(),
		Transport:    TransportBotAPI,
	}, nil, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	errCh := runService(ctx, service)

	select {
	case <-api.afterScript:
		cancel()
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("timed out waiting for poll")
	}

	if err := waitService(errCh); err != nil {
		t.Fatalf("run service: %v", err)
	}
}

func TestRecordStoppedRedactsTelegramSecrets(t *testing.T) {
	service := NewService(Config{
		Token: "123456:secret-token",
		MTProto: MTProtoConfig{
			AppHash:     "apphash-secret",
			SessionPath: "/tmp/trishna/mtproto-session.json",
			ProxySecret: mustDecodeHex(t,
				validTLSMTProxySecretHex,
			),
		},
	}, nil, testLogger())

	err := fmt.Errorf("token 123456:secret-token hash apphash-secret proxy %s session /tmp/trishna/mtproto-session.json mtproto-session.json", validTLSMTProxySecretHex)
	service.recordStopped("stopped", err)

	lastError := service.Health().LastError
	for _, leaked := range []string{"123456:secret-token", "apphash-secret", validTLSMTProxySecretHex, "mtproto-session.json"} {
		if strings.Contains(lastError, leaked) {
			t.Fatalf("last error leaked %q: %q", leaked, lastError)
		}
	}
}

func TestEnsureSecureSessionPathRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target-session.json")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(dir, "mtproto-session.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	err := ensureSecureSessionPath(link)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink error, got %v", err)
	}
}

func TestEnsureSecureSessionPathRepairsPermissions(t *testing.T) {
	sessionDir := filepath.Join(t.TempDir(), "telegram")
	if err := os.Mkdir(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	sessionPath := filepath.Join(sessionDir, "mtproto-session.json")
	if err := os.WriteFile(sessionPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	if err := ensureSecureSessionPath(sessionPath); err != nil {
		t.Fatalf("ensure session: %v", err)
	}
	sessionInfo, err := os.Stat(sessionPath)
	if err != nil {
		t.Fatalf("stat session: %v", err)
	}
	if got := sessionInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("session mode = %o", got)
	}
	dirInfo, err := os.Stat(sessionDir)
	if err != nil {
		t.Fatalf("stat session dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("session dir mode = %o", got)
	}
}

const fakeToken = "123:test"

func mustDecodeHex(t *testing.T, value string) []byte {
	t.Helper()
	bytes, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("decode hex: %v", err)
	}
	return bytes
}

type fakeTelegramAPI struct {
	t *testing.T

	server *httptest.Server
	script []fakeUpdate

	mu              sync.Mutex
	getUpdatesCalls int
	afterScript     chan struct{}
	afterScriptOnce sync.Once
	sentMessages    chan sentMessage
}

type fakeUpdate struct {
	updateID int64
	userID   int64
	chatID   int64
	text     string
}

type sentMessage struct {
	chatID    int64
	text      string
	parseMode string
}

func newFakeTelegramAPI(t *testing.T, script []fakeUpdate) *fakeTelegramAPI {
	t.Helper()

	api := &fakeTelegramAPI{
		t:            t,
		script:       append([]fakeUpdate(nil), script...),
		afterScript:  make(chan struct{}),
		sentMessages: make(chan sentMessage, 4),
	}
	api.server = httptest.NewServer(http.HandlerFunc(api.handle))
	return api
}

func (a *fakeTelegramAPI) URL() string {
	return a.server.URL
}

func (a *fakeTelegramAPI) close() {
	a.server.Close()
}

func (a *fakeTelegramAPI) handle(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		a.t.Errorf("parse multipart form: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	method := strings.TrimPrefix(r.URL.Path, "/bot"+fakeToken+"/")
	switch method {
	case "getMe":
		writeTelegramOK(w, `{"id":123,"is_bot":true,"first_name":"Trishna","username":"trishna_bot"}`)
	case "getUpdates":
		a.handleGetUpdates(w, r)
	case "sendMessage":
		chatID, err := strconv.ParseInt(r.FormValue("chat_id"), 10, 64)
		if err != nil {
			a.t.Errorf("parse chat_id: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.sentMessages <- sentMessage{chatID: chatID, text: r.FormValue("text"), parseMode: r.FormValue("parse_mode")}
		writeTelegramOK(w, messageJSON(200, 123, chatID, r.FormValue("text")))
	default:
		a.t.Errorf("unexpected method %q", method)
		http.NotFound(w, r)
	}
}

func (a *fakeTelegramAPI) handleGetUpdates(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	a.getUpdatesCalls++
	call := a.getUpdatesCalls
	a.mu.Unlock()

	if call == 1 {
		writeTelegramOK(w, `[]`)
		return
	}

	index := call - 2
	if index >= 0 && index < len(a.script) {
		update := a.script[index]
		writeTelegramOK(w, fmt.Sprintf(`[%s]`, updateJSON(update)))
		return
	}

	a.afterScriptOnce.Do(func() { close(a.afterScript) })
	select {
	case <-r.Context().Done():
		return
	case <-time.After(20 * time.Millisecond):
		writeTelegramOK(w, `[]`)
	}
}

func messageUpdate(updateID, userID, chatID int64, text string) fakeUpdate {
	return fakeUpdate{updateID: updateID, userID: userID, chatID: chatID, text: text}
}

func updateJSON(update fakeUpdate) string {
	return fmt.Sprintf(`{"update_id":%d,"message":%s}`, update.updateID, messageJSON(1, update.userID, update.chatID, update.text))
}

func messageJSON(messageID, userID, chatID int64, text string) string {
	return fmt.Sprintf(`{"message_id":%d,"date":1,"from":{"id":%d,"is_bot":false,"first_name":"Owner"},"chat":{"id":%d,"type":"private"},"text":%q}`, messageID, userID, chatID, text)
}

func writeTelegramOK(w http.ResponseWriter, result string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"ok":true,"result":%s}`, result)
}

func runService(ctx context.Context, service *Service) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- service.Run(ctx)
	}()
	return errCh
}

func waitService(errCh <-chan error) error {
	select {
	case err := <-errCh:
		return err
	case <-time.After(2 * time.Second):
		return fmt.Errorf("timed out waiting for service stop")
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
