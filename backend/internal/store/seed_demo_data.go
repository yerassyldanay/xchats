package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	purecampaign "github.com/yerassyldanay/xchats/backend/campaign"
)

// SeedDemoCRM inserts demo CRM customers, notes, tags, follow-ups, campaign templates,
// and simulator campaigns. Idempotent: safe to run multiple times.
func (s *Store) SeedDemoCRM(ctx context.Context, orgID, adminUserID uuid.UUID) error {
	var count int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM crm_customers WHERE organization_id = $1`, orgID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil // already seeded
	}

	actor := uuid.NullUUID{UUID: adminUserID, Valid: adminUserID != uuid.Nil}

	// 1. Get lifecycle statuses
	statuses, err := s.ListStatuses(ctx, orgID)
	if err != nil {
		return err
	}
	var newStatusID, inProgStatusID, waitingStatusID, wonStatusID uuid.UUID
	for _, st := range statuses {
		switch st.Slug {
		case "new":
			newStatusID = st.ID
		case "in_progress":
			inProgStatusID = st.ID
		case "waiting":
			waitingStatusID = st.ID
		case "won":
			wonStatusID = st.ID
		}
	}
	if newStatusID == uuid.Nil && len(statuses) > 0 {
		newStatusID = statuses[0].ID
	}
	if inProgStatusID == uuid.Nil {
		inProgStatusID = newStatusID
	}
	if waitingStatusID == uuid.Nil {
		waitingStatusID = newStatusID
	}
	if wonStatusID == uuid.Nil {
		wonStatusID = newStatusID
	}

	// Tag helper cache
	tagCache := make(map[string]uuid.UUID)
	getTagID := func(tagName string) (uuid.UUID, error) {
		if id, ok := tagCache[tagName]; ok {
			return id, nil
		}
		t, err := s.CreateTag(ctx, orgID, CustomerTag{Name: tagName, Slug: strings.ToLower(tagName), Color: "#64748b"})
		if err == nil {
			tagCache[tagName] = t.ID
			return t.ID, nil
		}
		// lookup if duplicate
		tags, listErr := s.ListTags(ctx, orgID)
		if listErr == nil {
			for _, tag := range tags {
				if tag.Name == tagName {
					tagCache[tagName] = tag.ID
					return tag.ID, nil
				}
			}
		}
		return uuid.Nil, err
	}

	// 2. Customers definition
	type demoCustomer struct {
		Name     string
		Phone    string
		Email    string
		StatusID uuid.UUID
		Tags     []string
		Notes    []string
		Channel  string
		ExtID    string
		Username string
	}

	customersData := []demoCustomer{
		{
			Name: "Алия Мухамеджанова", Phone: "+7 777 123 4567", Email: "aliya.m@example.kz",
			StatusID: newStatusID, Tags: []string{"VIP", "Постоянный", "Алматы"},
			Notes: []string{
				"Выбирает кофемашину для небольшой студии и уточняет условия оптовой скидки.",
				"Предпочитает общение в WhatsApp в первой половине дня.",
			},
			Channel: "whatsapp", ExtID: "77771234567@s.whatsapp.net",
		},
		{
			Name: "Данияр Сериков", Phone: "+7 701 987 6543", Email: "daniyar.s@example.kz",
			StatusID: inProgStatusID, Tags: []string{"Новый лид", "Астана"},
			Notes:   []string{"Запрос на расчет индивидуального тарифа по доставке."},
			Channel: "telegram", ExtID: "7019876543", Username: "daniyar_serik",
		},
		{
			Name: "Камила Нурланова", Phone: "+7 705 555 8899", Email: "kamila.n@example.kz",
			StatusID: waitingStatusID, Tags: []string{"В процессе", "Срочно"},
			Notes:   []string{"Уточняет статус возврата товара по гарантии."},
			Channel: "instagram", ExtID: "ig_kamila_n", Username: "kamila_style",
		},
		{
			Name: "Бауыржан Ахметов", Phone: "+7 775 444 1122", Email: "bauyrzhan.a@example.kz",
			StatusID: inProgStatusID, Tags: []string{"Партнер", "Шымкент"},
			Notes:   []string{"Запросил коммерческое предложение на технику для кухни нового кафе."},
			Channel: "messenger", ExtID: "psid_bauyrzhan_a", Username: "bauyr_kz",
		},
		{
			Name: "Елена Васильева", Phone: "+7 702 333 7711", Email: "elena.v@example.com",
			StatusID: wonStatusID, Tags: []string{"Успешно", "Архив"},
			Notes:   []string{"Заказ успешно доставлен и оплачен. Клиент оставил положительный отзыв."},
			Channel: "whatsapp", ExtID: "77023337711@s.whatsapp.net",
		},
	}

	createdCustomers := make([]Customer, 0, len(customersData))
	for _, cd := range customersData {
		name := cd.Name
		phone := cd.Phone
		email := cd.Email
		statNull := uuid.NullUUID{UUID: cd.StatusID, Valid: cd.StatusID != uuid.Nil}
		cust, err := s.CreateCustomer(ctx, orgID, CustomerPatch{
			DisplayName:    &name,
			Phone:          &phone,
			Email:          &email,
			StatusID:       &statNull,
			AssigneeUserID: &actor,
		}, actor)
		if err != nil {
			return fmt.Errorf("seed customer %s: %w", cd.Name, err)
		}

		for _, tag := range cd.Tags {
			if tagID, err := getTagID(tag); err == nil && tagID != uuid.Nil {
				_ = s.AddCustomerTag(ctx, orgID, cust.ID, tagID, actor)
			}
		}
		for _, note := range cd.Notes {
			_, _ = s.AddCustomerNote(ctx, orgID, cust.ID, actor, note)
		}
		if cd.Channel != "" {
			accountID := demoAccountIDForChannel(cd.Channel)
			// contact_id is required but deliberately has no FK: use a stable
			// placeholder until the corresponding seeded conversation is ingested.
			// ResolveCustomerForContact finds this row by account/external identity
			// and replaces the placeholder with the real transport contact id.
			contactID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("xchats-demo-contact:"+cd.Channel+":"+cd.ExtID))
			if _, err := s.db.Exec(ctx, `
				INSERT INTO crm_customer_identities (organization_id, customer_id, channel, account_id, contact_id, external_id, username, phone, display_name)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
				orgID, cust.ID, cd.Channel, accountID, contactID, cd.ExtID, cd.Username, cd.Phone, cd.Name); err != nil {
				return fmt.Errorf("seed identity for %s: %w", cd.Name, err)
			}
		}
		createdCustomers = append(createdCustomers, cust)
	}

	// 3. Follow-ups
	now := time.Now().UTC()
	type demoFollowup struct {
		CustIdx   int
		Action    string
		DayOffset int
		Minute    int
		Note      string
		Completed bool
	}

	fuList := []demoFollowup{
		{CustIdx: 0, Action: FollowupActionCall, DayOffset: -1, Minute: 660, Note: "Позвонить по поводу согласования скидки на оптовый заказ"},
		{CustIdx: 1, Action: FollowupActionMessage, DayOffset: 0, Minute: 780, Note: "Отправить ссылку на обновленный каталог кухонной техники в WhatsApp"},
		{CustIdx: 2, Action: FollowupActionCall, DayOffset: 0, Minute: 1020, Note: "Уточнить реквизиты для выставления счета по безналичному расчету"},
		{CustIdx: 3, Action: FollowupActionMeeting, DayOffset: 1, Minute: 690, Note: "Презентация условий оптовой программы для региональных представителей"},
		{CustIdx: 4, Action: FollowupActionOther, DayOffset: 2, Minute: 960, Note: "Проверить трек-номер отправки заказа через службу доставки СДЭК"},
		{CustIdx: 0, Action: FollowupActionCall, DayOffset: 3, Minute: 840, Note: "Контрольный звонок после доставки первой тестовой партии"},
		{CustIdx: 1, Action: FollowupActionMessage, DayOffset: 5, Minute: 600, Note: "Запросить обратную связь и предложить участие в программе лояльности"},
		{CustIdx: 0, Action: FollowupActionMessage, DayOffset: -2, Minute: 720, Note: "Отправить чек об оплате на почту", Completed: true},
	}

	for _, fu := range fuList {
		if fu.CustIdx >= len(createdCustomers) {
			continue
		}
		cust := createdCustomers[fu.CustIdx]
		targetDate := now.AddDate(0, 0, fu.DayOffset)
		dueDateStr := targetDate.Format("2006-01-02")
		dueAt := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), fu.Minute/60, fu.Minute%60, 0, 0, time.UTC)
		minute := fu.Minute

		created, err := s.CreateFollowup(ctx, orgID, FollowupInput{
			CustomerID:     cust.ID,
			Action:         fu.Action,
			DueDate:        dueDateStr,
			DueAt:          dueAt,
			DueMinute:      &minute,
			Note:           fu.Note,
			AssigneeUserID: actor,
		}, actor)
		if err != nil {
			return fmt.Errorf("seed followup: %w", err)
		}
		if fu.Completed {
			_, _ = s.SetFollowupState(ctx, orgID, created.ID, FollowupCompleted, actor)
		}
	}

	// 4. Campaign Templates
	templates := []CampaignTemplate{
		{
			OrganizationID: orgID,
			Name:           "Скидка 15% на первый заказ",
			MessageBody:    "Здравствуйте, {{name}}! 👋 Спасибо за интерес к Qazan Home. Дарим скидку 15% на первую покупку кухонной техники по промокоду WELCOME15.",
			CreatedBy:      adminUserID,
		},
		{
			OrganizationID: orgID,
			Name:           "Напоминание о доставке",
			MessageBody:    "Добрый день, {{name}}! Ваш заказ кухонной техники запланирован к доставке завтра в {{time}}. Если время неудобно, пожалуйста, сообщите нам.",
			CreatedBy:      adminUserID,
		},
		{
			OrganizationID: orgID,
			Name:           "Техника недели",
			MessageBody:    "Приветствуем, {{name}}! 🔥 Только на этой неделе действуют специальные цены на кофемашины, блендеры и тостеры. Ответьте на сообщение — поможем выбрать модель.",
			CreatedBy:      adminUserID,
		},
	}
	for _, t := range templates {
		_, _ = s.CreateCampaignTemplate(ctx, t)
	}

	// 5. Simulator Account & Campaigns
	simAccount, err := s.GetOrCreateSimulatorAccount(ctx, orgID)
	if err == nil {
		camp1, err := s.CreateCampaign(ctx, Campaign{
			OrganizationID: orgID,
			AccountID:      simAccount.ID,
			Name:           "Осенние скидки на кофемашины",
			Channel:        purecampaign.ChannelSimulator,
			MessageBody:    "Здравствуйте, {{name}}! ☕ В Qazan Home стартовала осенняя распродажа кофемашин Aurora со скидкой 20%. Подробности в каталоге или у нашего менеджера.",
			CreatedBy:      adminUserID,
		})
		if err == nil {
			_ = s.ReplaceCampaignRecipients(ctx, camp1.ID, []CampaignRecipientInput{
				{NormalizedIdentity: "+77771234567", Name: "Алия", RawInput: "+77771234567, Алия"},
				{NormalizedIdentity: "+77019876543", Name: "Данияр", RawInput: "+77019876543, Данияр"},
				{NormalizedIdentity: "+77055558899", Name: "Камила", RawInput: "+77055558899, Камила"},
				{NormalizedIdentity: "+77754441122", Name: "Бауыржан", RawInput: "+77754441122, Бауыржан"},
				{NormalizedIdentity: "+77023337711", Name: "Елена", RawInput: "+77023337711, Елена"},
			})
			// Keep the example stable and safe: it looks like a real campaign
			// with delivery progress, but it is paused so no background worker
			// can send anything when a seeded instance starts.
			_, _ = s.db.Exec(ctx, `
				UPDATE campaigns SET status = 'paused', started_at = $2 WHERE id = $1`,
				camp1.ID, now.Add(-2*time.Hour))
			for _, phone := range []string{"+77771234567", "+77019876543", "+77055558899"} {
				_, _ = s.db.Exec(ctx, `UPDATE campaign_recipients SET status = 'sent', attempts = 1 WHERE campaign_id = $1 AND normalized_identity = $2`, camp1.ID, phone)
			}
			_, _ = s.db.Exec(ctx, `
				UPDATE campaign_recipients SET status = 'failed', attempts = 1, failure_reason = 'Demo: номер временно недоступен'
				WHERE campaign_id = $1 AND normalized_identity = '+77754441122'`, camp1.ID)
		}

		camp2, err := s.CreateCampaign(ctx, Campaign{
			OrganizationID: orgID,
			AccountID:      simAccount.ID,
			Name:           "Опрос качества обслуживания клиентов",
			Channel:        purecampaign.ChannelSimulator,
			MessageBody:    "Добрый день, {{name}}! 🌟 Пожалуйста, оцените качество последней консультации от 1 до 5. Ваше мнение помогает нам становиться лучше!",
			CreatedBy:      adminUserID,
		})
		if err == nil {
			_ = s.ReplaceCampaignRecipients(ctx, camp2.ID, []CampaignRecipientInput{
				{NormalizedIdentity: "+77771234567", Name: "Алия", RawInput: "+77771234567, Алия"},
				{NormalizedIdentity: "+77019876543", Name: "Данияр", RawInput: "+77019876543, Данияр"},
				{NormalizedIdentity: "+77055558899", Name: "Камила", RawInput: "+77055558899, Камила"},
			})
		}
	}

	return nil
}
