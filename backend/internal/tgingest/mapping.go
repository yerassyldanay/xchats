package tgingest

import "github.com/yerassyldanay/xchats/backend/internal/telegram"

// MessageKind maps a Telegram message to our message_kind vocabulary,
// mirroring the WhatsApp names so the inbox renders both the same way.
func MessageKind(m *telegram.Message) string {
	switch m.MediaKind() {
	case "image":
		if m.Sticker != nil {
			return "stickerMessage"
		}
		return "imageMessage"
	case "video":
		return "videoMessage"
	case "audio":
		return "audioMessage"
	case "document":
		return "documentMessage"
	case "sticker":
		return "stickerMessage"
	}
	return "conversation"
}

// Preview is the chat-list line. Text (or a caption) wins; a caption-less
// attachment falls back to the same placeholders WhatsApp uses.
func Preview(m *telegram.Message) string {
	if body := firstNonEmpty(m.Text, m.Caption); body != "" {
		return truncatePreview(body)
	}
	switch m.MediaKind() {
	case "image":
		return "📷 Фото"
	case "video":
		return "🎥 Видео"
	case "audio":
		return "🎙 Аудио"
	case "document":
		return "📄 Документ"
	case "sticker":
		return "🌟 Стикер"
	}
	return ""
}

// truncatePreview mirrors internal/httpapi's own preview() truncation rule
// (120 runes) — duplicated rather than imported: tgingest must not depend on
// httpapi (httpapi already depends on tgingest, via the webhook handler).
func truncatePreview(text string) string {
	if r := []rune(text); len(r) > 120 {
		return string(r[:120])
	}
	return text
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
