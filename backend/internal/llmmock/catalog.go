package llmmock

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// catalogReply is customerResponse's answer for an ordinary shop-kb@v7
// organization (any org that is neither of the two hard-coded demo verticals
// above it): instead of a fixed greeting, it reads the catalog straight out of
// the rendered prompt — the ТОВАРЫ ДОСТУПНЫЕ / ТОВАРЫ НЕДОСТУПНЫЕ / ТЕМЫ /
// ЗОНЫ ДОСТАВКИ / BUSINESS FACTS blocks aiprompt.RenderPromptV7 writes — and
// answers the customer's last message the way the frame asks a real model to:
// exact values only as {{token}} placeholders copied from those blocks, media
// only as "<table>.<ref>.<column>" refs copied from them, Kazakh in → Kazakh
// out, escalation with the frame's holding phrase when the KB has no answer.
// Every token it emits is one the prompt itself listed, so the response always
// passes aiprompt.ValidateResponseV7 and the backend substitutes the real
// stored values — which is the point: a scenario run in mock-externals mode
// still exercises the KB → prompt → placeholder → substitution → media-send
// path end to end, just with templated wording instead of a model's.
//
// ok is false when the prompt carries no product block at all (not a v7
// customer prompt) or the message matches nothing this mock understands;
// customerResponse then keeps its generic greeting.
type mockReply struct {
	Text      string
	Lang      string
	Media     []string
	Escalate  bool
	Reason    string
	GapReason string
}

type promptProduct struct {
	Ref        string
	Name       string
	NameText   string // lowercased name + brand, for the strong match
	BodyText   string // lowercased description + advantages + best_for, for the weak match
	Summary    string // first sentence of the description, customer-facing prose
	Status     string // availability_status as rendered: в наличии | предзаказ | под заказ
	Note       string
	PriceToken string
	MediaRefs  map[string]string // column → "products.<ref>.<column>"
}

type promptTopic struct {
	Slug  string
	Title string
	Body  string
	Text  string // lowercased slug + title + body
}

type promptZone struct {
	Ref   string
	Name  string
	Level string
}

type promptCatalog struct {
	Products    []promptProduct
	Unavailable []string
	Topics      []promptTopic
	Zones       []promptZone
	Facts       map[string]bool
	History     []string
	Message     string
}

const (
	sectionProducts    = "\nТОВАРЫ ДОСТУПНЫЕ — "
	sectionUnavailable = "\nТОВАРЫ НЕДОСТУПНЫЕ — "
	sectionTariffs     = "\nТАРИФЫ — "
	sectionTopics      = "\nТЕМЫ:\n"
	sectionZones       = "\nЗОНЫ ДОСТАВКИ — "
	sectionFacts       = "\nBUSINESS FACTS:\n"
	sectionHistory     = "История переписки"
	messageMarker      = "Клиент пишет: "

	// holdingRU/holdingKK are the v7 frame's own rule-1 holding reply: it must
	// open by admitting the information is missing, then hand the question on.
	holdingRU = "Сейчас у меня нет этой информации — передаю ваш вопрос менеджеру и вернусь с точным ответом."
	holdingKK = "Қазір менде бұл ақпарат жоқ — сұрағыңызды менеджерге жіберемін де, нақты жауаппен ораламын."
)

var factTokenRE = regexp.MustCompile(`^\{\{[a-z0-9_.\-]+\}\}`)

// section returns the text between the LAST start marker and the first end
// marker after it (or the end of text) — the frame's instructions mention the
// same block names earlier in prose, so only the final occurrence is the block.
func section(text, start string, ends ...string) string {
	i := strings.LastIndex(text, start)
	if i < 0 {
		return ""
	}
	rest := text[i+len(start):]
	cut := len(rest)
	for _, e := range ends {
		if j := strings.Index(rest, e); j >= 0 && j < cut {
			cut = j
		}
	}
	return rest[:cut]
}

// blockFields splits one "key: value" block into its fields, keeping the
// first value of a repeated key.
func blockFields(block string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(block, "\n") {
		k, v, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if _, seen := out[k]; !seen {
			out[k] = strings.TrimSpace(v)
		}
	}
	return out
}

func parseCatalog(prompt string) *promptCatalog {
	products := section(prompt, sectionProducts, sectionUnavailable, sectionTariffs, sectionTopics)
	if !strings.Contains(products, "product: ") {
		return nil
	}
	cat := &promptCatalog{Facts: map[string]bool{}}
	for _, block := range strings.Split(products, "\n\n") {
		block = strings.TrimSpace(block)
		if !strings.HasPrefix(block, "product: ") {
			continue
		}
		f := blockFields(block)
		p := promptProduct{
			Ref: f["product"], Name: f["name"], Status: f["availability_status"], Note: f["availability_note"],
			NameText:  normalize(f["name"] + " " + f["brand"]),
			BodyText:  normalize(f["description"] + " " + f["advantages"] + " " + f["best_for"]),
			Summary:   firstSentence(f["description"], 160),
			MediaRefs: map[string]string{},
		}
		if tok := f["price_placeholder"]; factTokenRE.MatchString(tok) {
			p.PriceToken = tok
		}
		for k, v := range f {
			if col, ok := strings.CutSuffix(k, "_ref"); ok && strings.HasPrefix(v, "products.") {
				p.MediaRefs[col] = v
			}
		}
		cat.Products = append(cat.Products, p)
	}
	for _, line := range strings.Split(section(prompt, sectionUnavailable, sectionTariffs, sectionTopics), "\n") {
		if name, ok := strings.CutPrefix(strings.TrimSpace(line), "- "); ok {
			cat.Unavailable = append(cat.Unavailable, name)
		}
	}
	for _, block := range strings.Split(section(prompt, sectionTopics, sectionZones, sectionFacts), "\n\n") {
		block = strings.TrimSpace(block)
		if !strings.HasPrefix(block, "topic: ") {
			continue
		}
		f := blockFields(block)
		cat.Topics = append(cat.Topics, promptTopic{
			Slug: f["topic"], Title: f["title"], Body: f["body"],
			Text: normalize(f["topic"] + " " + f["title"] + " " + f["body"]),
		})
	}
	for _, line := range strings.Split(section(prompt, sectionZones, sectionFacts), "\n") {
		parts := strings.Split(line, " | ")
		if len(parts) < 3 || !strings.HasPrefix(parts[2], "zone_level: ") {
			continue
		}
		cat.Zones = append(cat.Zones, promptZone{
			Ref: strings.TrimSpace(parts[0]), Name: strings.TrimSpace(parts[1]),
			Level: strings.TrimPrefix(parts[2], "zone_level: "),
		})
	}
	for _, line := range strings.Split(section(prompt, sectionFacts, sectionHistory), "\n") {
		if tok := factTokenRE.FindString(strings.TrimSpace(line)); tok != "" {
			cat.Facts[tok] = true
		}
	}
	if i := strings.LastIndex(prompt, messageMarker); i >= 0 {
		msg := prompt[i+len(messageMarker):]
		if j := strings.Index(msg, "\n\n"); j >= 0 {
			msg = msg[:j]
		}
		cat.Message = strings.TrimSpace(msg)
		history := section(prompt[:i], sectionHistory)
		for _, line := range strings.Split(history, "\n") {
			for _, label := range []string{"Клиент: ", "Ассистент: "} {
				if s, ok := strings.CutPrefix(line, label); ok {
					cat.History = append(cat.History, s)
				}
			}
		}
	}
	return cat
}

// normalize lowercases s and folds ё into е, so stems match either spelling.
func normalize(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "ё", "е")
}

func firstSentence(s string, max int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, ".!?"); i >= 0 {
		s = s[:i+1]
	}
	if utf8.RuneCountInString(s) > max {
		r := []rune(s)
		s = strings.TrimSpace(string(r[:max])) + "…"
	}
	return s
}

// stopStems are message words that say nothing about WHICH product is meant —
// question words, politeness, and the intent words detected separately below.
var stopStems = map[string]bool{
	"здрав": true, "добры": true, "добро": true, "подск": true, "пожал": true, "можно": true,
	"нужен": true, "нужна": true, "нужно": true, "интер": true, "хочу": true, "хотел": true,
	"сколь": true, "стоит": true, "стоим": true, "цена": true, "цены": true, "ценам": true,
	"есть": true, "налич": true, "какой": true, "какая": true, "какие": true, "какую": true,
	"фото": true, "видео": true, "покаж": true, "скинь": true, "пришл": true, "отпра": true,
	"спаси": true, "магаз": true, "вашем": true, "вашег": true, "ваша": true, "ваши": true,
	"будет": true, "этот": true, "этого": true, "такой": true, "тоже": true, "также": true,
	"очень": true, "доста": true, "сделк": true, "заказ": true, "купит": true, "возьм": true,
	"канша": true, "қанша": true, "турад": true, "тұрад": true, "керек": true, "салем": true,
	"сәлем": true, "рахме": true, "сурет": true, "бейне": true, "бағас": true, "багас": true,
	"сізде": true, "сизде": true, "бар": true, "жібер": true, "жибер": true, "маған": true,
	"маган": true, "алайы": true, "аламы": true, "здрас": true, "добри": true, "вечер": true,
	"утро": true, "день": true, "прошу": true, "новый": true, "новая": true,
	"давай": true, "отлич": true, "оформ": true, "хорош": true, "ладно": true, "понят": true,
	"тогда": true, "может": true, "скину": true, "тамаш": true, "жақсы": true, "жаксы": true,
	"работ": true, "выгля": true, "подой": true, "смотр": true, "посмо": true, "показ": true,
	"увиде": true, "нему": true, "него": true, "қалай": true, "калай": true,
}

// stems returns the message's content-word stems: the first five runes of
// every word of four or more runes, minus stopStems. Five runes is enough to
// survive Russian and Kazakh case endings on a product noun (домкрат,
// домкрата, домкраттың → "домкр") while staying specific.
func stems(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, w := range strings.FieldsFunc(normalize(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		r := []rune(w)
		if len(r) < 4 {
			continue
		}
		if len(r) > 5 {
			r = r[:5]
		}
		st := string(r)
		if stopStems[st] || seen[st] {
			continue
		}
		seen[st] = true
		out = append(out, st)
	}
	return out
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// kazakhStems are words a Kazakh speaker types even without Kazakh letters
// ("Домкрат канша турады?") — the frame's own rule 7 example is exactly that.
var kazakhStems = []string{
	"канша", "қанша", "турады", "тұрады", "бағас", "багас", "сурет", "бейне", "керек", "кажет", "қажет",
	"рахмет", "салем", "сәлем", "сиздер", "сіздер", "бар ма", "барма", "жеткіз", "жеткиз", "жибер", "жібер",
	"кашан", "қашан", "кайда", "қайда", "ашыла", "дукен", "дүкен", "алайын", "аламын", "болад", "бола ма",
	"маган", "маған", "жок па", "жоқ па", "иә", "жақсы", "жаксы",
}

func isKazakh(msg string) bool {
	if strings.ContainsAny(msg, "әғқңөұүһіӘҒҚҢӨҰҮҺІ") {
		return true
	}
	return containsAny(normalize(msg), kazakhStems...)
}

type intents struct {
	price, photo, video, delivery, address, hours, payment, warranty, buy, thanks, greeting, human, wholesale bool
}

func detectIntents(msg string) intents {
	m := normalize(msg)
	return intents{
		price:     containsAny(m, "сколько", "цена", "цену", "цены", "стоит", "стоимост", "почем", "прайс", "канша", "қанша", "баға", "бага", "турады", "тұрады"),
		photo:     containsAny(m, "фото", "фотк", "сурет", "покаж", "картинк", "как выгляд"),
		video:     containsAny(m, "видео", "бейне", "ролик", "обзор"),
		delivery:  containsAny(m, "доставк", "доставит", "доставля", "привез", "жеткіз", "жеткиз", "отправите в", "отправить в"),
		address:   containsAny(m, "адрес", "где вы", "где наход", "где магазин", "как доехать", "как найти", "мекенжай", "кайда", "қайда"),
		hours:     containsAny(m, "открыт", "откроет", "открое", "график", "режим", "часы работ", "время работ", "во сколько", "ашыла", "ашасыз", "жұмыс уақыт", "жумыс уакыт", "кашан", "қашан"),
		payment:   containsAny(m, "рассроч", "kaspi", "каспи", "оплат", "кредит", "бөліп", "болип", "наличными"),
		warranty:  containsAny(m, "гарант", "кепілд", "кепилд"),
		buy:       containsAny(m, "беру", "возьму", "оформ", "заказыва", "покупаю", "аламын", "алайын", "давайте его", "забронир"),
		thanks:    containsAny(m, "спасибо", "благодар", "рахмет"),
		greeting:  containsAny(m, "здравств", "привет", "добрый", "доброе", "салем", "сәлем", "салам", "сәлеметсіз"),
		human:     containsAny(m, "менеджер", "оператор", "живой человек", "позвоните мне", "перезвоните", "хабарлас"),
		wholesale: containsAny(m, "оптом", "оптов", "опт ", "для сто", "для автосервис", "для нашего сто", "юрлиц", "счет на оплат", "счёт на оплат", "кп ", "коммерческое предлож", "көтерме", "котерме"),
	}
}

// productMatches ranks the available products against the message's stems:
// a stem found in the name or brand weighs 3, in the description 1, and only
// products with at least one name hit are candidates. Several products tied
// on the best score are all returned (up to 3) so the reply can list them —
// "домкрат" alone matches every jack in the shop, and listing them is the
// helpful answer.
func productMatches(cat *promptCatalog, text string) []promptProduct {
	out, _ := rankProducts(cat, text)
	return out
}

// rankProducts is productMatches plus the best candidate's name-hit count,
// which catalogReply weighs against an unavailable product's own match.
func rankProducts(cat *promptCatalog, text string) ([]promptProduct, int) {
	sts := stems(text)
	if len(sts) == 0 {
		return nil, 0
	}
	type scored struct {
		p     promptProduct
		score int
	}
	var best []scored
	top, topHits := 0, 0
	for _, p := range cat.Products {
		nameHits, score := 0, 0
		for _, st := range sts {
			switch {
			case strings.Contains(p.NameText, st):
				nameHits++
				score += 3
			case strings.Contains(p.BodyText, st):
				score++
			}
		}
		if nameHits == 0 {
			continue
		}
		switch {
		case score > top:
			top, topHits = score, nameHits
			best = []scored{{p, score}}
		case score == top:
			best = append(best, scored{p, score})
		}
	}
	var out []promptProduct
	for i, s := range best {
		if i == 3 {
			break
		}
		out = append(out, s.p)
	}
	return out, topHits
}

// unavailableMatch returns the unavailable product sharing the most stems
// with the message, and how many — "ножничный подъёмник" names the
// unavailable scissor lift twice but the available two-post lift only once.
func unavailableMatch(cat *promptCatalog, text string) (string, int) {
	sts := stems(text)
	best, top := "", 0
	for _, name := range cat.Unavailable {
		n, hits := normalize(name), 0
		for _, st := range sts {
			if strings.Contains(n, st) {
				hits++
			}
		}
		if hits > top {
			best, top = name, hits
		}
	}
	return best, top
}

// alternativeFor suggests an available product sharing a name stem with an
// unavailable one (Подъёмник двухстоечный → Подъёмник ножничный).
func alternativeFor(cat *promptCatalog, name string) *promptProduct {
	for _, st := range stems(name) {
		for i := range cat.Products {
			if strings.Contains(cat.Products[i].NameText, st) {
				return &cat.Products[i]
			}
		}
	}
	return nil
}

func topicMatch(cat *promptCatalog, keys ...string) *promptTopic {
	for _, k := range keys {
		for i := range cat.Topics {
			if strings.Contains(cat.Topics[i].Text, k) {
				return &cat.Topics[i]
			}
		}
	}
	return nil
}

// topicByStems is the fallback when no intent keyword picked a topic: the
// topic sharing the most content stems with the message, if any.
func topicByStems(cat *promptCatalog, text string) *promptTopic {
	var best *promptTopic
	top := 0
	for i := range cat.Topics {
		n := 0
		for _, st := range stems(text) {
			if strings.Contains(cat.Topics[i].Text, st) {
				n++
			}
		}
		if n > top {
			top, best = n, &cat.Topics[i]
		}
	}
	return best
}

func topicSnippet(t *promptTopic) string {
	body := strings.TrimSpace(t.Body)
	var parts []string
	for len(parts) < 2 && body != "" {
		s := firstSentence(body, 220)
		parts = append(parts, s)
		body = strings.TrimSpace(strings.TrimPrefix(body, strings.TrimSuffix(s, "…")))
	}
	return strings.Join(parts, " ")
}

// lastProduct finds the product the conversation was last about, so a
// follow-up like "а видео есть?" still knows which product it means.
func lastProduct(cat *promptCatalog) []promptProduct {
	for i := len(cat.History) - 1; i >= 0; i-- {
		if m := productMatches(cat, cat.History[i]); len(m) > 0 {
			return m[:1]
		}
	}
	return nil
}

func statusPhrase(p promptProduct, kk bool) string {
	switch p.Status {
	case "предзаказ":
		if kk {
			return "алдын ала тапсырыспен қолжетімді"
		}
		return "доступен по предзаказу"
	case "под заказ":
		if kk {
			return "тапсырыс бойынша әкелінеді"
		}
		return "доступен под заказ"
	default:
		if kk {
			return "қоймада бар"
		}
		return "в наличии"
	}
}

func priceLine(p promptProduct, kk bool) string {
	if p.PriceToken == "" {
		return "«" + p.Name + "»"
	}
	if kk {
		return "«" + p.Name + "» бағасы — " + p.PriceToken
	}
	return "«" + p.Name + "» — " + p.PriceToken
}

func (c *promptCatalog) fact(tok string) bool { return c.Facts[tok] }

func catalogReply(prompt string) (mockReply, bool) {
	cat := parseCatalog(prompt)
	if cat == nil || cat.Message == "" {
		return mockReply{}, false
	}
	kk := isKazakh(cat.Message)
	lang := "ru"
	if kk {
		lang = "kk"
	}
	pick := func(ru, kz string) string {
		if kk {
			return kz
		}
		return ru
	}
	in := detectIntents(cat.Message)
	reply := func(text string, media ...string) (mockReply, bool) {
		if media == nil {
			media = []string{}
		}
		return mockReply{Text: text, Lang: lang, Media: media}, true
	}
	escalate := func(reason, gap, extra string) (mockReply, bool) {
		text := pick(holdingRU, holdingKK)
		if extra != "" {
			text += " " + extra
		}
		return mockReply{Text: text, Lang: lang, Media: []string{}, Escalate: true, Reason: reason, GapReason: gap}, true
	}

	// 1. Requests a person must handle: wholesale terms, invoices, a call back.
	if in.wholesale {
		return escalate("оптовый запрос / условия для автосервиса — нужен менеджер", "unsupported_request",
			pick("Менеджер подготовит для вас индивидуальное предложение.", "Менеджер сізге жеке ұсыныс дайындайды."))
	}
	if in.human {
		return escalate("клиент просит связаться с менеджером", "human_requested", "")
	}

	// A message naming nothing (no content stems: "а видео есть?", "беру")
	// continues the product the conversation was last about; one that names
	// something the catalog lacks must NOT borrow it — that is rule 1d below.
	products, nameHits := rankProducts(cat, cat.Message)
	unavailable, unavailableHits := unavailableMatch(cat, cat.Message)
	if unavailableHits <= nameHits {
		unavailable = ""
	}
	if len(products) == 0 && len(stems(cat.Message)) == 0 && (in.photo || in.video || in.buy || in.price) {
		products = lastProduct(cat)
	}

	// 2. Delivery: the most specific zone the message names, else the city zone.
	if in.delivery {
		var zone *promptZone
		for i := range cat.Zones {
			for _, st := range stems(cat.Zones[i].Name) {
				if strings.Contains(normalize(cat.Message), st) {
					zone = &cat.Zones[i]
					break
				}
			}
			if zone != nil {
				break
			}
		}
		if zone == nil {
			for i := range cat.Zones {
				if cat.Zones[i].Level == "city" {
					zone = &cat.Zones[i]
					break
				}
			}
		}
		var text string
		switch {
		case zone != nil && cat.fact("{{delivery."+zone.Ref+".delivery_cost}}"):
			cost := "{{delivery." + zone.Ref + ".delivery_cost}}"
			days := "{{delivery." + zone.Ref + ".delivery_in_days}}"
			text = pick("Доставка — "+zone.Name+": "+cost+", срок — "+days+" дня.",
				zone.Name+" бойынша жеткізу: "+cost+", мерзімі — "+days+" күн.")
		case zone != nil && cat.fact("{{delivery."+zone.Ref+".delivery_available}}"):
			text = pick(zone.Name+": {{delivery."+zone.Ref+".delivery_available}}.",
				zone.Name+": {{delivery."+zone.Ref+".delivery_available}}.")
		case cat.fact("{{policy.main.delivery_cost}}") && cat.fact("{{policy.main.delivery_in_days}}"):
			text = pick("Доставка — {{policy.main.delivery_cost}}, срок — {{policy.main.delivery_in_days}} дня.",
				"Жеткізу — {{policy.main.delivery_cost}}, мерзімі — {{policy.main.delivery_in_days}} күн.")
		default:
			return escalate("нет данных о доставке в базе знаний", "missing_field", "")
		}
		if len(products) == 1 {
			text = priceLine(products[0], kk) + ". " + text
		}
		return reply(text)
	}

	// 3. A known-but-unavailable product (frame rule 1a): say so, never escalate.
	if name := unavailable; name != "" {
		text := pick("К сожалению, «"+name+"» сейчас нет в наличии.", "Өкінішке қарай, «"+name+"» қазір қолжетімді емес.")
		if alt := alternativeFor(cat, name); alt != nil {
			text += pick(" Могу предложить: "+priceLine(*alt, false)+" ("+statusPhrase(*alt, false)+").",
				" Оның орнына ұсына аламын: "+priceLine(*alt, true)+" ("+statusPhrase(*alt, true)+").")
		}
		return reply(text)
	}

	// 4. One or more catalog products.
	if len(products) > 1 && !in.photo && !in.video {
		lines := []string{pick("Вот что есть по вашему запросу:", "Сұрауыңыз бойынша бізде мыналар бар:")}
		for _, p := range products {
			lines = append(lines, "• "+priceLine(p, kk)+" ("+statusPhrase(p, kk)+")")
		}
		lines = append(lines, pick("Какой вариант вам ближе? Могу отправить фото.", "Қай нұсқа ыңғайлы? Суретін жібере аламын."))
		return reply(strings.Join(lines, "\n"))
	}
	if len(products) > 0 {
		p := products[0]
		switch {
		case in.video:
			if ref, ok := p.MediaRefs["demo_videos"]; ok {
				return reply(pick("Отправляю видео-обзор: "+priceLine(p, false)+".", "Бейнешолуын жіберемін: "+priceLine(p, true)+"."), ref)
			}
			text := pick("Видео по «"+p.Name+"» пока нет.", "«"+p.Name+"» бойынша бейне әзірге жоқ.")
			if ref := photoRef(p); ref != "" {
				return reply(text+pick(" Отправляю фото.", " Суретін жіберемін."), ref)
			}
			return reply(text)
		case in.photo:
			var media []string
			for _, q := range products {
				if ref := photoRef(q); ref != "" && len(media) < 2 {
					media = append(media, ref)
				}
			}
			if len(media) == 0 {
				return reply(pick("Фото «"+p.Name+"» пока нет, но могу подробно рассказать о нём.", "«"+p.Name+"» суреті әзірге жоқ, бірақ толық айтып бере аламын."))
			}
			return reply(pick("Отправляю фото: "+priceLine(p, false)+".", "Суреттерін жіберемін: "+priceLine(p, true)+"."), media...)
		case in.buy:
			text := pick("Отлично! "+priceLine(p, false)+", "+statusPhrase(p, false)+". Напишите, пожалуйста, ваше имя и удобное время — подготовим заказ.",
				"Тамаша! "+priceLine(p, true)+", "+statusPhrase(p, true)+". Тапсырысты дайындау үшін атыңызды және ыңғайлы уақытты жазыңыз.")
			return reply(text)
		default:
			text := priceLine(p, kk)
			if p.Status != "" && p.Status != "в наличии" {
				text += ", " + statusPhrase(p, kk)
			}
			text += "."
			if p.Note != "" {
				text += " " + p.Note
			}
			if p.Summary != "" && !kk {
				text += " " + p.Summary
			}
			if photoRef(p) != "" {
				text += pick(" Хотите, отправлю фото?", " Суретін жіберейін бе?")
			}
			return reply(text)
		}
	}

	// 5. Store facts that live in topics: address, opening/hours, payment, warranty.
	var t *promptTopic
	switch {
	case in.address:
		t = topicMatch(cat, "адрес", "как добраться", "наход")
	case in.hours:
		t = topicMatch(cat, "открыт", "график", "режим", "часы")
	case in.payment:
		t = topicMatch(cat, "рассроч", "оплат", "kaspi", "каспи")
	case in.warranty:
		t = topicMatch(cat, "гарант")
	}
	if t == nil && !in.price && !in.photo && !in.video && !in.greeting && !in.thanks {
		t = topicByStems(cat, cat.Message)
	}
	if t != nil {
		text := topicSnippet(t)
		if kk {
			text = "Ақпарат: " + text
		}
		if in.hours && cat.fact("{{contact.main.working_hours}}") {
			text += pick(" Часы работы: {{contact.main.working_hours}}.", " Жұмыс уақыты: {{contact.main.working_hours}}.")
		}
		return reply(text)
	}
	if in.hours && cat.fact("{{contact.main.working_hours}}") {
		return reply(pick("Часы работы: {{contact.main.working_hours}}.", "Жұмыс уақыты: {{contact.main.working_hours}}."))
	}

	// 6. A price for something the catalog does not carry (frame rule 1d).
	if in.price || in.photo || in.video {
		return escalate("товар не найден в каталоге", "missing_entity", "")
	}
	if in.thanks {
		return reply(pick("Спасибо, что написали! Если появятся вопросы — пишите, всегда поможем.", "Рахмет! Тағы сұрақтарыңыз болса, жазыңыз — әрдайым көмектесеміз."))
	}
	if in.greeting {
		return reply(pick("Здравствуйте! Подскажу по инструменту и оборудованию: цены, наличие, фото и видео. Что вас интересует?",
			"Сәлеметсіз бе! Құрал-саймандар мен жабдықтар бойынша көмектесемін: бағасы, бары, суреті мен бейнесі. Сізге не керек?"))
	}
	return mockReply{}, false
}

func photoRef(p promptProduct) string {
	if ref, ok := p.MediaRefs["gallery_images"]; ok {
		return ref
	}
	return p.MediaRefs["featured_image"]
}
