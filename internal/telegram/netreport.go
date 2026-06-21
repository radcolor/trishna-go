package telegram

import (
	"context"
	"fmt"
	stdhtml "html"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gotd/td/mtproxy"
)

const (
	networkCommandName      = "tgnet"
	defaultPublicIPCheckURL = "https://api.ipify.org?format=text"
)

var publicIPCheckURL = defaultPublicIPCheckURL

func (s *Service) networkReportHTML(ctx context.Context) string {
	var b strings.Builder
	writeHTMLReportSection(&b, "Telegram Network", s.networkLines(ctx))
	writeHTMLReportSection(&b, "Privacy Verdict", s.privacyLines())
	return b.String()
}

func (s *Service) networkLines(ctx context.Context) []string {
	health := s.Health()
	lines := []string{
		fmt.Sprintf("Service:      %s", serviceState(health.Running)),
		fmt.Sprintf("Detail:       %s", fallback(health.Detail, "n/a")),
		fmt.Sprintf("Transport:    %s", fallback(s.cfg.Transport, "n/a")),
	}
	if health.LastOK != nil {
		lines = append(lines, fmt.Sprintf("Last OK:      %s ago", time.Since(*health.LastOK).Round(time.Second)))
	}
	if health.LastError != "" {
		lines = append(lines, fmt.Sprintf("Last Error:   %s", health.LastError))
	}

	switch s.cfg.Transport {
	case TransportMTProto:
		lines = append(lines,
			fmt.Sprintf("App ID:       %s", configuredInt(s.cfg.MTProto.AppID)),
			fmt.Sprintf("Session:      %s", s.sessionStatus()),
		)
		if s.cfg.MTProto.ProxyEnabled {
			lines = append(lines,
				"MTProxy:      enabled",
				fmt.Sprintf("Proxy Host:   %s", fallback(s.cfg.MTProto.ProxyHost, "missing")),
				fmt.Sprintf("Proxy Port:   %d", s.cfg.MTProto.ProxyPort),
				fmt.Sprintf("Proxy DNS:    %s", resolveHost(ctx, s.cfg.MTProto.ProxyHost)),
				fmt.Sprintf("Secret Type:  %s", mtproxySecretType(s.cfg.MTProto.ProxySecret)),
			)
		} else {
			lines = append(lines, "MTProxy:      disabled (direct Telegram DC connection)")
		}
	case TransportBotAPI:
		lines = append(lines,
			"Bot API:      enabled",
			fmt.Sprintf("API Base:     %s", fallback(s.cfg.APIBaseURL, "https://api.telegram.org")),
		)
	default:
		lines = append(lines, "Transport:    unknown")
	}

	lines = append(lines,
		fmt.Sprintf("Local IP:     %s", localOutboundIP()),
		fmt.Sprintf("Public IP:    %s", publicIP(ctx)),
	)
	return lines
}

func (s *Service) privacyLines() []string {
	switch s.cfg.Transport {
	case TransportMTProto:
		if s.cfg.MTProto.ProxyEnabled {
			return []string{
				"Network:      ISP sees MTProxy host/port, not Telegram DC.",
				"Telegram IP:  Telegram should see proxy IP, not home IP.",
				"Proxy Owner:  Can see your IP and timing; not message plaintext.",
				"Messages:     Bot chats are not end-to-end encrypted.",
				"Access:       Telegram servers and this bot process can read bot chats.",
				"MTProxy:      Changes network path only; not chat encryption.",
			}
		}
		return []string{
			"Network:      Direct MTProto to Telegram DC.",
			"ISP:          Can see Telegram DC IP and timing.",
			"Messages:     Bot chats are not end-to-end encrypted.",
			"Access:       Telegram servers and this bot process can read bot chats.",
		}
	case TransportBotAPI:
		return []string{
			"Network:      HTTPS Bot API.",
			"ISP:          Can see api.telegram.org unless routed by VPN/proxy.",
			"Messages:     Bot chats are not end-to-end encrypted.",
			"Access:       Telegram servers and this bot process can read bot chats.",
		}
	default:
		return []string{"Network:      Telegram disabled or config invalid."}
	}
}

func (s *Service) sessionStatus() string {
	path := s.cfg.MTProto.SessionPath
	if path == "" {
		return "missing"
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return path + " (not created yet)"
		}
		return path + " (" + err.Error() + ")"
	}
	return fmt.Sprintf("%s (%s)", path, info.Mode().Perm())
}

func writeHTMLReportSection(b *strings.Builder, title string, lines []string) {
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	b.WriteString("<b>")
	b.WriteString(stdhtml.EscapeString(title))
	b.WriteString("</b>\n<pre>")
	b.WriteString(stdhtml.EscapeString(strings.Join(lines, "\n")))
	b.WriteString("</pre>")
}

func serviceState(running bool) string {
	if running {
		return "running"
	}
	return "stopped"
}

func fallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func configuredInt(value int) string {
	if value <= 0 {
		return "missing"
	}
	return fmt.Sprintf("%d", value)
}

func mtproxySecretType(raw []byte) string {
	secret, err := mtproxy.ParseSecret(raw)
	if err != nil {
		return "invalid"
	}
	switch secret.Type {
	case mtproxy.Simple:
		return "simple"
	case mtproxy.Secured:
		return "secured"
	case mtproxy.TLS:
		if secret.CloakHost != "" {
			return "tls cloak " + secret.CloakHost
		}
		return "tls"
	default:
		return "unknown"
	}
}

func resolveHost(ctx context.Context, host string) string {
	if host == "" {
		return "missing"
	}
	resolver := net.Resolver{}
	lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ips, err := resolver.LookupHost(lookupCtx, host)
	if err != nil {
		return "error: " + err.Error()
	}
	if len(ips) == 0 {
		return "no records"
	}
	return strings.Join(ips, ", ")
}

func localOutboundIP() string {
	conn, err := net.DialTimeout("udp", "1.1.1.1:80", 2*time.Second)
	if err != nil {
		return "error: " + err.Error()
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil {
		return "unknown"
	}
	return addr.IP.String()
}

func publicIP(ctx context.Context) string {
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, publicIPCheckURL, nil)
	if err != nil {
		return "error: " + err.Error()
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "error: " + err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf("http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		return "error: " + err.Error()
	}
	value := strings.TrimSpace(string(body))
	if value == "" {
		return "empty"
	}
	return value + " (normal HTTPS egress, not MTProxy)"
}
