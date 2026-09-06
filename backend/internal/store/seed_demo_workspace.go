package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Fixed IDs make the complete workspace fixture idempotent and make its rows
// unmistakably synthetic when an operator inspects a development database.
var (
	demoWhatsAppAccountID      = uuid.MustParse("10000000-0000-4000-8000-000000000001")
	demoTelegramAccountID      = uuid.MustParse("10000000-0000-4000-8000-000000000002")
	demoInstagramAccountID     = uuid.MustParse("10000000-0000-4000-8000-000000000003")
	demoMessengerAccountID     = uuid.MustParse("10000000-0000-4000-8000-000000000004")
	demoWhatsAppCloudAccountID = uuid.MustParse("10000000-0000-4000-8000-000000000005")
)

func demoAccountIDForChannel(channel string) uuid.UUID {
	switch channel {
	case "whatsapp":
		return demoWhatsAppAccountID
	case "telegram":
		return demoTelegramAccountID
	case "instagram":
		return demoInstagramAccountID
	case "messenger":
		return demoMessengerAccountID
	case "whatsapp_cloud":
		return demoWhatsAppCloudAccountID
	default:
		return uuid.Nil
	}
}

// SeedDemoWorkspace adds the channel dashboard, realistic customer inbox,
// grounded reply drafts, and private KB-assistant history used by the product
// tour. It intentionally refuses to mix fake accounts into an organization
// that already has a real channel account.
func (s *Store) SeedDemoWorkspace(ctx context.Context, orgID, adminUserID uuid.UUID) (bool, error) {
	var realAccounts int
	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM inbox_accounts_v
		WHERE organization_id = $1 AND channel <> 'simulator'
		  AND id NOT IN ($2, $3, $4, $5, $6)`,
		orgID, demoWhatsAppAccountID, demoTelegramAccountID, demoInstagramAccountID,
		demoMessengerAccountID, demoWhatsAppCloudAccountID).Scan(&realAccounts); err != nil {
		return false, err
	}
	if realAccounts > 0 {
		return false, nil
	}

	if err := s.seedDemoChannelAccounts(ctx, orgID); err != nil {
		return false, err
	}
	if err := s.seedDemoInbox(ctx, adminUserID); err != nil {
		return false, err
	}
	if err := s.seedDemoAssistantHistory(ctx, orgID, adminUserID); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) seedDemoChannelAccounts(ctx context.Context, orgID uuid.UUID) error {
	now := time.Now().UTC()
	if _, err := s.SeedAccount(ctx, Account{
		ID:                 demoWhatsAppAccountID,
		OrganizationID:     uuid.NullUUID{UUID: orgID, Valid: true},
		DisplayName:        "Qazan Home · Sales",
		ExternalAccountRef: "77007007070@s.whatsapp.net",
		ExternalHandle:     "77007007070",
		ConnectionState:    "connected",
	}); err != nil {
		return fmt.Errorf("seed demo WhatsApp account: %w", err)
	}
	if _, err := s.db.Exec(ctx, `UPDATE wa_accounts SET last_live_event_at = $2 WHERE id = $1`,
		demoWhatsAppAccountID, now.Add(-25*time.Second)); err != nil {
		return err
	}

	if _, err := s.db.Exec(ctx, `
		INSERT INTO tg_accounts
			(id, organization_id, display_name, bot_id, bot_username, connection_state,
			 webhook_url, webhook_registered_at, webhook_last_checked_at, last_live_event_at)
		VALUES ($1, $2, 'Qazan Home · Telegram', 7007007070, 'qazan_home_demo_bot', 'connected',
			'https://demo.invalid/webhooks/telegram', $3, $3, $3)
		ON CONFLICT (id) DO UPDATE SET
			organization_id = excluded.organization_id,
			display_name = excluded.display_name,
			bot_username = excluded.bot_username,
			connection_state = excluded.connection_state,
			webhook_url = excluded.webhook_url,
			webhook_registered_at = excluded.webhook_registered_at,
			webhook_last_checked_at = excluded.webhook_last_checked_at,
			webhook_last_error = '', last_live_event_at = excluded.last_live_event_at,
			deleted_at = NULL`, demoTelegramAccountID, orgID, now.Add(-40*time.Second)); err != nil {
		return fmt.Errorf("seed demo Telegram account: %w", err)
	}

	type metaAccount struct {
		id, channel, externalID, name, handle, webhook, meta string
	}
	accounts := []metaAccount{
		{demoInstagramAccountID.String(), "instagram", "ig-qazan-home-demo", "Qazan Home · Instagram", "@qazan.home", "https://demo.invalid/webhooks/meta", `{"business_id":"demo-business","ig_user_id":"ig-qazan-home-demo"}`},
		{demoMessengerAccountID.String(), "messenger", "page-qazan-home-demo", "Qazan Home · Messenger", "Qazan Home", "https://demo.invalid/webhooks/meta", `{"business_id":"demo-business","page_id":"page-qazan-home-demo"}`},
		{demoWhatsAppCloudAccountID.String(), "whatsapp_cloud", "phone-qazan-home-demo", "Qazan Home · WhatsApp Cloud", "+7 700 700 70 70", "https://demo.invalid/webhooks/meta", `{"business_id":"demo-business","waba_id":"demo-waba","registered":true}`},
	}
	for i, account := range accounts {
		if _, err := s.db.Exec(ctx, `
			INSERT INTO channel_accounts
				(id, organization_id, channel, external_account_id, display_name, handle,
				 connection_state, webhook_url, webhook_registered_at, webhook_last_checked_at,
				 last_live_event_at, provider_meta)
			VALUES ($1, $2, $3, $4, $5, $6, 'connected', $7, $8, $8, $8, $9)
			ON CONFLICT (id) DO UPDATE SET
				organization_id = excluded.organization_id,
				channel = excluded.channel,
				external_account_id = excluded.external_account_id,
				display_name = excluded.display_name,
				handle = excluded.handle,
				connection_state = excluded.connection_state,
				webhook_url = excluded.webhook_url,
				webhook_registered_at = excluded.webhook_registered_at,
				webhook_last_checked_at = excluded.webhook_last_checked_at,
				webhook_last_error = '', last_live_event_at = excluded.last_live_event_at,
				provider_meta = excluded.provider_meta, deleted_at = NULL`,
			account.id, orgID, account.channel, account.externalID, account.name, account.handle,
			account.webhook, now.Add(-time.Duration(i+1)*time.Minute), account.meta); err != nil {
			return fmt.Errorf("seed demo %s account: %w", account.channel, err)
		}
	}
	return nil
}

func (s *Store) seedDemoInbox(ctx context.Context, adminUserID uuid.UUID) error {
	now := time.Now().UTC()

	waChats := []struct {
		jid, phone, name, message, reply string
		offset                           time.Duration
	}{
		{
			jid: "77771234567@s.whatsapp.net", phone: "+7 777 123 4567", name: "Алия Мухамеджанова",
			message: "Здравствуйте! Aurora Barista Pro сейчас есть в наличии? Можно оплатить через Kaspi?",
			reply:   "Здравствуйте, Алия! Кофемашина Aurora Barista Pro есть в наличии по цене 289 900 ₸. Оплатить можно картой, через Kaspi или наличными при получении. Помочь оформить заказ?",
			offset:  3 * time.Minute,
		},
		{
			jid: "77023337711@s.whatsapp.net", phone: "+7 702 333 7711", name: "Елена Васильева",
			message: "Когда снова будет аэрофритюрница Crisp Air 5L? И сколько стоит доставка в Алматы?",
			reply:   "Здравствуйте, Елена! Crisp Air 5L сейчас временно нет в наличии, поэтому точную дату поступления обещать не буду. Доставка по Алматы стоит 5 000 ₸ и обычно занимает один день. Могу передать менеджеру просьбу сообщить о поступлении.",
			offset:  18 * time.Minute,
		},
	}
	for i, chat := range waChats {
		res, err := s.UpsertInbound(ctx, InboundUpsert{
			AccountID: demoWhatsAppAccountID, PhoneJID: chat.jid, RemoteJID: chat.jid,
			PhoneNumber: chat.phone, PushName: chat.name, Direction: "in", SenderKind: "contact",
			ExternalMessageID: fmt.Sprintf("demo-wa-%d", i+1), MessageKind: "text",
			Body: chat.message, Preview: chat.message, Source: "demo_seed", MessageTS: now.Add(-chat.offset),
		})
		if err != nil {
			return fmt.Errorf("seed demo WhatsApp conversation: %w", err)
		}
		if err := s.seedDemoDraft(ctx, "whatsapp", res.ChatID, res.MessageID, chat.reply); err != nil {
			return err
		}
		if _, err := s.db.Exec(ctx, `UPDATE wa_chats SET assignee_user_id = $2, ai_summary = $3 WHERE id = $1`,
			res.ChatID, adminUserID, "Покупатель выбирает кухонную технику и уточняет наличие, оплату или доставку."); err != nil {
			return err
		}
	}

	tg, err := s.IngestTelegramInbound(ctx, TgInbound{
		AccountID: demoTelegramAccountID, UpdateID: 700001, TelegramChatID: 7019876543,
		TelegramUserID: 7019876543, Username: "daniyar_serik", FirstName: "Данияр", LastName: "Сериков",
		DisplayName: "Данияр Сериков", TelegramMessageID: 501, MessageKind: "text",
		Body:    "Посоветуйте мощный блендер до 60 000 ₸ для смузи.",
		Preview: "Посоветуйте мощный блендер до 60 000 ₸ для смузи.", MessageTS: now.Add(-8 * time.Minute),
	})
	if err != nil {
		return fmt.Errorf("seed demo Telegram conversation: %w", err)
	}
	if err := s.seedDemoDraft(ctx, "telegram", tg.ChatID, tg.MessageID,
		"Данияр, под ваш бюджет подходит Pulse 1200 за 59 900 ₸. Это стационарный блендер мощностью 1200 Вт со стеклянной чашей, шестью скоростями и импульсным режимом; сейчас он есть в наличии."); err != nil {
		return err
	}
	if _, err := s.db.Exec(ctx, `UPDATE tg_chats SET assignee_user_id = $2 WHERE id = $1`, tg.ChatID, adminUserID); err != nil {
		return err
	}

	metaChats := []struct {
		accountID                           uuid.UUID
		channel, externalID, handle, name   string
		threadID, messageID, message, reply string
		offset                              time.Duration
	}{
		{
			accountID: demoInstagramAccountID, channel: "instagram", externalID: "ig_kamila_n", handle: "kamila_style", name: "Камила Нурланова",
			threadID: "demo-ig-thread-kamila", messageID: "demo-ig-1", offset: 12 * time.Minute,
			message: "У тостера Sage Toast 2 есть гарантия?", reply: "Да, Камила. На Sage Toast 2 действует гарантия производителя; для обращения понадобится чек или подтверждение заказа. Тостер сейчас есть в наличии по цене 39 900 ₸.",
		},
		{
			accountID: demoMessengerAccountID, channel: "messenger", externalID: "psid_bauyrzhan_a", handle: "bauyr_kz", name: "Бауыржан Ахметов",
			threadID: "demo-fb-thread-bauyrzhan", messageID: "demo-fb-1", offset: 15 * time.Minute,
			message: "Нужно четыре кофемашины для кафе. Есть оптовая цена?", reply: "Бауыржан, Aurora Barista Pro есть в наличии по цене 289 900 ₸ за штуку. Подтверждённой оптовой цены в базе пока нет, поэтому передам запрос менеджеру для индивидуального расчёта на четыре кофемашины.",
		},
	}
	for _, chat := range metaChats {
		res, err := s.IngestChannelInbound(ctx, ChannelInbound{
			AccountID: chat.accountID, ExternalContactID: chat.externalID, ContactHandle: chat.handle,
			ContactDisplayName: chat.name, ExternalThreadID: chat.threadID, Direction: "in", SenderKind: "contact",
			ExternalMessageID: chat.messageID, MessageKind: "text", Body: chat.message,
			Preview: chat.message, Source: "demo_seed", MessageTS: now.Add(-chat.offset),
		})
		if err != nil {
			return fmt.Errorf("seed demo %s conversation: %w", chat.channel, err)
		}
		if err := s.seedDemoDraft(ctx, chat.channel, res.ChatID, res.MessageID, chat.reply); err != nil {
			return err
		}
		if _, err := s.db.Exec(ctx, `UPDATE channel_chats SET assignee_user_id = $2 WHERE id = $1`, res.ChatID, adminUserID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) seedDemoDraft(ctx context.Context, channel string, chatID, messageID uuid.UUID, reply string) error {
	id := uuid.NewSHA1(uuid.NameSpaceURL, []byte("xchats-demo-draft:"+channel+":"+chatID.String()))
	_, err := s.db.Exec(ctx, `
		INSERT INTO ai_drafts
			(id, chat_id, trigger_message_id, option_ordinal, draft_text, context_state,
			 confidence, escalate, draft_state, reply_language, channel)
		VALUES ($1, $2, $3, 1, $4, 'full', 0.96, FALSE, 'suggested', 'ru', $5)
		ON CONFLICT (id) DO UPDATE SET
			trigger_message_id = excluded.trigger_message_id,
			draft_text = excluded.draft_text,
			context_state = excluded.context_state,
			confidence = excluded.confidence,
			escalate = excluded.escalate,
			draft_state = excluded.draft_state,
			reply_language = excluded.reply_language,
			channel = excluded.channel,
			updated_at = strftime('%Y-%m-%d %H:%M:%f','now')`,
		id, chatID, messageID, reply, channel)
	if err != nil {
		return fmt.Errorf("seed demo AI draft: %w", err)
	}
	return nil
}

func (s *Store) seedDemoAssistantHistory(ctx context.Context, orgID, adminUserID uuid.UUID) error {
	now := time.Now().UTC()
	type message struct {
		role, content string
	}
	conversations := []struct {
		id      uuid.UUID
		title   string
		updated time.Time
		turns   []message
	}{
		{
			id: uuid.MustParse("20000000-0000-4000-8000-000000000001"), title: "Что сейчас есть в наличии?", updated: now.Add(-2 * time.Minute),
			turns: []message{
				{role: "user", content: "Какая кухонная техника сейчас есть в наличии и сколько она стоит?"},
				{role: "assistant", content: "Сейчас в наличии четыре позиции:\n\n- Кофемашина Aurora Barista Pro — 289 900 ₸\n- Блендер Pulse 1200 — 59 900 ₸\n- Чайник Ember 1.7 — 34 900 ₸\n- Тостер Sage Toast 2 — 39 900 ₸\n\nАэрофритюрница Crisp Air 5L временно недоступна. Все цены взяты из рабочей базы знаний."},
			},
		},
		{
			id: uuid.MustParse("20000000-0000-4000-8000-000000000002"), title: "Изменения в черновике", updated: now.Add(-35 * time.Minute),
			turns: []message{
				{role: "user", content: "Что изменится после публикации текущего черновика?"},
				{role: "assistant", content: "Черновик снижает цену тостера Sage Toast 2 с 39 900 ₸ до 34 900 ₸, добавляет осеннее предложение и новую зону доставки в Тараз. До ручной публикации эти изменения не используются в ответах клиентам."},
			},
		},
		{
			id: uuid.MustParse("20000000-0000-4000-8000-000000000003"), title: "Доставка по Казахстану", updated: now.Add(-90 * time.Minute),
			turns: []message{
				{role: "user", content: "Какие условия доставки по Казахстану?"},
				{role: "assistant", content: "По Алматы доставка стоит 5 000 ₸ и занимает один день. В другие города Казахстана — 10 000 ₸ и 3–4 дня. Заказы от 20 000 ₸ доставляются бесплатно; минимальная сумма заказа — 5 000 ₸."},
			},
		},
	}

	for _, conversation := range conversations {
		if _, err := s.db.Exec(ctx, `
			INSERT INTO chat_conversations (id, organization_id, user_id, title, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5)
			ON CONFLICT (id) DO UPDATE SET title = excluded.title, updated_at = excluded.updated_at`,
			conversation.id, orgID, adminUserID, conversation.title, conversation.updated); err != nil {
			return fmt.Errorf("seed demo assistant conversation: %w", err)
		}
		for i, turn := range conversation.turns {
			messageID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("xchats-demo-chat:%s:%d", conversation.id, i+1)))
			if _, err := s.db.Exec(ctx, `
				INSERT INTO chat_messages (id, conversation_id, seq, role, content, metadata, created_at)
				VALUES ($1, $2, $3, $4, $5, '{}', $6)
				ON CONFLICT (id) DO UPDATE SET role = excluded.role, content = excluded.content, metadata = excluded.metadata`,
				messageID, conversation.id, i+1, turn.role, turn.content, conversation.updated.Add(time.Duration(i)*time.Second)); err != nil {
				return fmt.Errorf("seed demo assistant message: %w", err)
			}
		}
	}
	return nil
}
