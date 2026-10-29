#!/usr/bin/env node
// Follis pilot rehearsal: builds the whole organization through the public
// API on an empty xchats instance, checks every piece landed, plays a day of
// WhatsApp customers (through the simulator — the same ingest path as a real
// WhatsApp number) and of staff answering them, then captures screenshots and
// writes a PASS/FAIL report. See README.md for what each phase covers.
//
// Preconditions (run.sh does all of this): backend on API_URL with an EMPTY
// database, the Vite frontend on UI_URL proxying to it.
//
//   node scenarios/follis/simulate.mjs [--llm=mock|real]
import { createRequire } from 'node:module'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const HERE = path.dirname(fileURLToPath(import.meta.url))
const REPO = path.resolve(HERE, '..', '..')
const require = createRequire(path.join(REPO, 'frontend', 'package.json'))
const { chromium } = require('@playwright/test')

const API = process.env.API_URL || 'http://localhost:8080'
const UI = process.env.UI_URL || 'http://localhost:5173'
const LLM = (process.argv.find((a) => a.startsWith('--llm=')) || '--llm=mock').slice(6)
const OUT = path.join(HERE, 'out')
const SHOTS = path.join(OUT, 'screenshots')
const MEDIA = path.join(HERE, 'media', 'out')
mkdirSync(SHOTS, { recursive: true })

const readJSON = (p) => JSON.parse(readFileSync(path.join(HERE, p), 'utf8'))
const ORG = readJSON('data/org.json')
const KB = readJSON('data/kb.json')
const CUSTOMERS = readJSON('data/customers.json').customers
const MANIFEST = JSON.parse(readFileSync(path.join(MEDIA, 'manifest.json'), 'utf8'))

// Prices after the staged Draft change is approved — what customers must see.
const PRICE = Object.fromEntries(KB.products.map((p) => [p.ref, p.price]))
PRICE[KB.draft_change.ref] = KB.draft_change.price
const ZONE_COST = Object.fromEntries(KB.zones.map((z) => [z.ref, z.delivery_cost]))

// ---------------------------------------------------------------------------
// results

const results = []
const findings = []
let phase = 'setup'
function check(name, ok, detail = '', { soft = false } = {}) {
  const status = ok ? 'PASS' : soft ? 'WARN' : 'FAIL'
  results.push({ phase, name, status, detail })
  const mark = { PASS: '✅', WARN: '⚠️ ', FAIL: '❌' }[status]
  console.log(`${mark} [${phase}] ${name}${detail && !ok ? ` — ${detail}` : ''}`)
  return ok
}
const log = (msg) => console.log(`   ${msg}`)

// ---------------------------------------------------------------------------
// API client: one cookie session per person, exactly like a browser login.

class Session {
  constructor(label) {
    this.label = label
    this.cookie = ''
  }
  async req(method, url, body, { form, headers = {} } = {}) {
    const init = { method, headers: { ...headers } }
    if (this.cookie) init.headers.Cookie = this.cookie
    if (form) init.body = form
    else if (body !== undefined) {
      init.headers['Content-Type'] = 'application/json'
      init.body = JSON.stringify(body)
    }
    const res = await fetch(`${API}/xchats/api/v1${url}`, init)
    const set = res.headers.getSetCookie?.() || []
    for (const c of set) if (c.startsWith('xchats_session=')) this.cookie = c.split(';')[0]
    const text = await res.text()
    let json = null
    try {
      json = JSON.parse(text)
    } catch {
      /* binary or empty */
    }
    return { status: res.status, ok: res.ok, json, payload: json?.payload, text, headers: res.headers }
  }
  get = (u) => this.req('GET', u)
  post = (u, b, o) => this.req('POST', u, b, o)
  put = (u, b) => this.req('PUT', u, b)
  patch = (u, b) => this.req('PATCH', u, b)
  // Sign-in is rate limited per IP (5/min, internal/httpapi/server.go), and
  // this script signs a whole team in from one machine — wait it out the way
  // a person would retry, instead of failing the run.
  async login(email, password) {
    for (let attempt = 0; ; attempt++) {
      const r = await this.post('/auth/login', { email, password })
      if (r.status !== 429 || attempt === 10) {
        this.user = r.payload?.user
        return r
      }
      await new Promise((res) => setTimeout(res, 13_000))
    }
  }
}

async function must(promise, what) {
  const r = await promise
  if (!r.ok) throw new Error(`${what}: HTTP ${r.status} ${r.json?.message || r.text.slice(0, 200)}`)
  return r
}

function fileForm(file, mime) {
  const form = new FormData()
  form.append('file', new Blob([readFileSync(file)], { type: mime }), path.basename(file))
  return form
}
const mimeOf = (f) => (f.endsWith('.mp4') ? 'video/mp4' : 'image/jpeg')

// ---------------------------------------------------------------------------
// screenshots: a real browser per person, Russian UI, logged in by API.

let browser
const shots = []
// A browser for one person, signed in with the cookie of their API session
// (a second sign-in would only burn the login rate limit).
async function personPage(sess) {
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1, locale: 'ru-RU' })
  await ctx.addInitScript(() => {
    try {
      localStorage.setItem('locale', 'ru')
    } catch {
      /* ignore */
    }
  })
  if (sess) {
    const [name, value] = sess.cookie.split('=')
    await ctx.addCookies([{ name, value, url: UI, httpOnly: true, sameSite: 'Lax' }])
  }
  return ctx.newPage()
}
async function settle(page, ms = 1200) {
  await page.waitForLoadState('networkidle', { timeout: 15_000 }).catch(() => {})
  await page.waitForTimeout(ms)
}
async function shot(page, name, caption) {
  const file = path.join(SHOTS, `${name}.png`)
  await page.screenshot({ path: file })
  shots.push({ name, file: path.relative(OUT, file), caption })
  log(`📸 ${name} — ${caption}`)
}
async function tryClick(locator, timeout = 4000) {
  try {
    await locator.first().click({ timeout })
    return true
  } catch {
    return false
  }
}

// ---------------------------------------------------------------------------
// phase 1: organization, staff, knowledge base

const sessions = {}
const users = {}
const materials = {} // ref -> {images:[{id,file}], videos:[{id,file}]}

async function setup() {
  phase = 'setup'
  const admin = (sessions.admin = new Session('admin'))
  const b = ORG.bootstrap_admin
  let r = await admin.login(b.email, b.default_password)
  if (!r.ok) r = await admin.login(b.email, b.new_password)
  if (!r.ok) throw new Error(`bootstrap admin login failed: ${r.status}`)
  if (r.payload.user.must_change_password) {
    let pw = await admin.post('/auth/password', { current_password: b.default_password, new_password: b.new_password })
    if (pw.status === 429) {
      await new Promise((res) => setTimeout(res, 13_000))
      pw = await admin.post('/auth/password', { current_password: b.default_password, new_password: b.new_password })
    }
    if (!pw.ok) throw new Error(`change bootstrap password: HTTP ${pw.status}`)
    log('bootstrap admin password rotated (forced first-login change)')
  }

  await must(admin.put('/organization', { name: ORG.organization.name, timezone: ORG.organization.timezone }), 'rename organization')
  log(`organization renamed to ${ORG.organization.name}`)

  for (const s of ORG.staff) {
    const c = await admin.post('/users', { email: s.email, password: s.password, name: s.name })
    if (c.status === 409) log(`${s.email} already exists`)
    else if (!c.ok) throw new Error(`create ${s.email}: ${c.status} ${c.json?.message}`)
  }
  const list = await must(admin.get('/users?page_size=100'), 'list users')
  for (const u of list.payload.items) users[u.email] = u
  for (const s of ORG.staff.filter((s) => s.role === 'admin')) {
    await must(admin.put(`/users/${users[s.email].id}/role`, { role: 'admin' }), `promote ${s.email}`)
  }
  log(`${ORG.staff.length} staff created (${ORG.staff.filter((s) => s.role === 'admin').length} admins)`)

  for (const s of ORG.staff) {
    const sess = (sessions[s.key] = new Session(s.key))
    const lr = await sess.login(s.email, s.password)
    if (!lr.ok) throw new Error(`${s.email} cannot sign in: HTTP ${lr.status}`)
    s.id = users[s.email].id
  }

  // The owner fills the knowledge base, as they would from the UI.
  const owner = sessions.erlan
  const tags = {}
  for (const t of ORG.tags) {
    const c = await must(owner.post('/crm/tags', t), `tag ${t.name}`)
    tags[t.name] = c.payload.id
  }
  ORG.tagIds = tags

  await must(owner.patch('/kb/config', KB.assistant), 'assistant config')
  await must(owner.patch('/kb/contacts', KB.contacts), 'contacts')
  await must(owner.patch('/kb/policies', KB.policies), 'policies')
  for (const t of KB.topics) await must(owner.post('/kb/topics', t), `topic ${t.slug}`)
  for (const z of KB.zones) await must(owner.post('/kb/zones', { ...z, sales_status: 'active' }), `zone ${z.ref}`)

  let uploaded = 0
  for (const p of KB.products) {
    const m = (materials[p.ref] = { images: [], videos: [] })
    for (const kind of ['images', 'videos']) {
      for (const file of MANIFEST[p.ref]?.[kind] || []) {
        const full = path.join(MEDIA, file)
        const u = await must(owner.post('/kb/materials', undefined, { form: fileForm(full, mimeOf(file)) }), `upload ${file}`)
        m[kind].push({ id: u.payload.material_id, file: full })
        uploaded++
      }
    }
    const { media, icon, ...fields } = p
    await must(
      owner.post('/kb/products', {
        ...fields,
        sales_status: 'active',
        gallery_images: m.images.map((x) => x.id),
        demo_videos: m.videos.map((x) => x.id),
      }),
      `product ${p.ref}`,
    )
  }
  log(`knowledge base filled: ${KB.products.length} products, ${KB.topics.length} topics, ${KB.zones.length} zones, ${uploaded} media files`)
}

// The store manager stages an opening-price change in the Draft; the owner
// reviews it in the UI and publishes it — the human-review loop for KB edits.
async function draftReview() {
  phase = 'draft'
  const dc = KB.draft_change
  const product = KB.products.find((p) => p.ref === dc.ref)
  const { media, icon, ...fields } = product
  const m = materials[dc.ref]
  await must(
    sessions.aigerim.post('/playground/draft/products', {
      ...fields,
      price: dc.price,
      sales_status: 'active',
      gallery_images: m.images.map((x) => x.id),
      demo_videos: m.videos.map((x) => x.id),
    }),
    'stage draft price change',
  )
  const live = await sessions.erlan.get('/kb')
  check('Staged change does not touch the live price', live.payload.products.find((p) => p.ref === dc.ref)?.price === product.price)

  const page = await personPage(sessions.erlan)
  await page.goto(`${UI}/draft`, { waitUntil: 'domcontentloaded' })
  await settle(page)
  await tryClick(page.getByRole('button', { name: /^Товары/ }))
  await settle(page, 800)
  await shot(page, '06-draft-review', `Черновик: управляющая предложила цену к открытию (${product.price} → ${dc.price}); владелец проверяет до публикации`)
  await page.context().close()

  const a = await sessions.erlan.post('/playground/draft/approve', {})
  check('Owner publishes the Draft', a.ok, `HTTP ${a.status} ${a.json?.message || ''}`)
  const after = await sessions.erlan.get('/kb')
  check('Published price is live', after.payload.products.find((p) => p.ref === dc.ref)?.price === dc.price)
}

// ---------------------------------------------------------------------------
// phase 2: verify organization, staff and knowledge base

async function verify() {
  phase = 'verify-org'
  const owner = sessions.erlan
  const org = await owner.get('/organization')
  check('Organization is named Follis', org.payload?.name === ORG.organization.name, JSON.stringify(org.payload))
  check('Organization timezone is Asia/Almaty', org.payload?.timezone === ORG.organization.timezone)

  phase = 'verify-staff'
  const list = await owner.get('/users?page_size=100')
  const byEmail = Object.fromEntries(list.payload.items.map((u) => [u.email, u]))
  check(`Team has ${ORG.staff.length} Follis staff + the setup admin`, list.payload.total === ORG.staff.length + 1, `total=${list.payload.total}`)
  for (const s of ORG.staff) {
    const u = byEmail[s.email]
    check(`${s.name} — ${s.role === 'admin' ? 'администратор' : 'сотрудник'}, имя совпадает`, u && u.role === s.role && u.name === s.name, JSON.stringify(u))
    const me = await sessions[s.key].get('/me')
    check(`${s.name} can sign in and lands in Follis`, me.ok && me.payload.organization?.name === 'Follis')
  }
  const member = sessions.nurlan
  const r1 = await member.post('/users', { email: 'intruder@follis.test', password: 'whatever-123', name: 'x' })
  check('A member cannot add staff (403)', r1.status === 403, `HTTP ${r1.status}`)
  const r2 = await member.put('/organization', { name: 'Hacked' })
  check('A member cannot rename the organization (403)', r2.status === 403, `HTTP ${r2.status}`)
  const r3 = await member.get('/settings/integrations')
  check('A member cannot open AI/integration settings (403)', r3.status === 403, `HTTP ${r3.status}`)
  const r4 = await member.get('/kb')
  check('A member can read the knowledge base', r4.ok)

  phase = 'verify-kb'
  const kb = (await owner.get('/kb')).payload
  const cfg = kb.config
  check('Assistant persona, mission, guardrails, language policy saved', ['persona', 'mission', 'guardrails', 'language_policy'].every((k) => cfg[k] === KB.assistant[k]))
  const contacts = kb.contacts[0] || {}
  check('Contacts saved (address, phone, WhatsApp, hours)', ['address', 'phone', 'whatsapp', 'working_hours'].every((k) => contacts[k] === KB.contacts[k]), JSON.stringify(contacts).slice(0, 200))
  const policies = kb.policies[0] || {}
  check('Policies saved (installment, warranty, returns, free delivery)', ['installment', 'warranty', 'return_period_in_days', 'free_delivery_from'].every((k) => policies[k] === KB.policies[k]))
  check(`All ${KB.topics.length} topics saved`, KB.topics.every((t) => kb.topics.some((x) => x.slug === t.slug && x.body_md === t.body_md)))
  check(`All ${KB.zones.length} delivery zones saved`, KB.zones.every((z) => kb.zones.some((x) => x.ref === z.ref && x.delivery_cost === z.delivery_cost)))
  check(`All ${KB.products.length} products saved`, kb.products.length === KB.products.length, `got ${kb.products.length}`)

  let fieldMismatches = []
  let mediaMismatches = []
  for (const p of KB.products) {
    const got = kb.products.find((x) => x.ref === p.ref)
    if (!got) {
      fieldMismatches.push(`${p.ref}: missing`)
      continue
    }
    for (const k of ['name', 'category', 'brand', 'description', 'availability_status']) {
      if ((got[k] || '') !== (p[k] || (k === 'availability_status' ? 'in_stock' : ''))) fieldMismatches.push(`${p.ref}.${k}`)
    }
    if (got.price !== PRICE[p.ref]) fieldMismatches.push(`${p.ref}.price ${got.price} != ${PRICE[p.ref]}`)
    const want = MANIFEST[p.ref]
    if (got.gallery_images.length !== want.images.length) mediaMismatches.push(`${p.ref} photos ${got.gallery_images.length}/${want.images.length}`)
    if (got.demo_videos.length !== want.videos.length) mediaMismatches.push(`${p.ref} videos ${got.demo_videos.length}/${want.videos.length}`)
  }
  check('Every product: name, category, brand, description, status and price match the catalog', fieldMismatches.length === 0, fieldMismatches.join('; '))
  const nImages = KB.products.reduce((n, p) => n + MANIFEST[p.ref].images.length, 0)
  const nVideos = KB.products.reduce((n, p) => n + MANIFEST[p.ref].videos.length, 0)
  check(`Photos (${nImages}) and videos (${nVideos}) attached to the right products`, mediaMismatches.length === 0, mediaMismatches.join('; '))

  let served = 0
  const broken = []
  for (const [ref, m] of Object.entries(materials)) {
    for (const x of [...m.images, ...m.videos]) {
      const r = await owner.get(`/kb/materials/${x.id}/content`)
      const type = r.headers.get('content-type') || ''
      const size = readFileSync(x.file).length
      if (r.ok && type.startsWith(mimeOf(x.file)) && Number(r.headers.get('content-length') || r.text.length) > 0) served++
      else broken.push(`${ref}/${path.basename(x.file)} HTTP ${r.status} ${type} (${size} bytes on disk)`)
    }
  }
  check(`Every uploaded file is served back with the right type (${served}/${nImages + nVideos})`, broken.length === 0, broken.join('; '))

  phase = 'verify-prompt'
  const prompt = (await owner.get('/kb/prompt')).payload
  check('AI prompt builds from the knowledge base', prompt.status === 'ok' && prompt.prompt_ref === 'shop-kb@v7', `${prompt.status} ${prompt.prompt_ref}`)
  const text = prompt.rendered_text
  const available = KB.products.filter((p) => p.availability_status !== 'unavailable')
  const unseen = available.filter((p) => !text.includes(`product: ${p.ref}\n`) || !text.includes(`price_placeholder: {{product.${p.ref}.price}}`))
  check(`AI sees all ${available.length} available products with a price placeholder`, unseen.length === 0, unseen.map((p) => p.ref).join(', '))
  const noPhoto = KB.products.filter((p) => MANIFEST[p.ref].images.length && p.availability_status !== 'unavailable' && !text.includes(`gallery_images_ref: products.${p.ref}.gallery_images`))
  const noVideo = KB.products.filter((p) => MANIFEST[p.ref].videos.length && !text.includes(`demo_videos_ref: products.${p.ref}.demo_videos`))
  check('AI can offer every product photo and video', noPhoto.length + noVideo.length === 0, [...noPhoto, ...noVideo].map((p) => p.ref).join(', '))
  const gone = KB.products.filter((p) => p.availability_status === 'unavailable')
  check('Unavailable products are listed by name only — no price, no media', gone.every((p) => text.includes(`- ${p.name}`) && !text.includes(`product: ${p.ref}\n`)))
  check('AI sees every topic (address, opening, payment, warranty, installation, wholesale)', KB.topics.every((t) => text.includes(`topic: ${t.slug}`)))
  check('AI sees every delivery zone and its cost placeholder', KB.zones.every((z) => text.includes(`{{delivery.${z.ref}.delivery_cost}}`)))
  check('Raw prices never appear in the prompt (placeholders only)', !Object.values(PRICE).some((v) => text.includes(v)))
  const policyProse = ['prepayment', 'installment', 'warranty'].filter((k) => !text.includes(KB.policies[k]))
  const addressOnlyViaTopic = !text.includes(`Адрес: ${KB.contacts.address}`)
  if (policyProse.length || addressOnlyViaTopic) {
    findings.push({
      title: 'Contacts address and payment/warranty policy text never reach the AI prompt',
      detail:
        `shop-kb@v7 renders no %%DESCRIPTIONS%% slot (backend/aiprompt/prompt.go renderPromptWithSchema; frames/shop-kb-v7-ru.txt), so contacts.address and policies.${policyProse.join('/')} are saved but invisible to the model. ` +
        'Customers asking "где вы?" or "есть рассрочка?" only get an answer because this scenario repeats that text in topics.',
    })
  }
  check('Address and payment/warranty texts reach the AI only through topics (contacts/policies fields are not rendered)', !(policyProse.length || addressOnlyViaTopic), 'see findings', { soft: true })
}

// ---------------------------------------------------------------------------
// phase 3 + 4: customers write on WhatsApp, staff answer

function expand(s) {
  return s.replace(/\{price:([^}]+)\}/g, (_, ref) => PRICE[ref]).replace(/\{zone:([^}]+)\}/g, (_, ref) => ZONE_COST[ref])
}
const staffOf = (key) => ORG.staff.find((s) => s.key === key)
const conv = {} // customer key -> { chatId, transcript: [] }

async function uploadForChat(sess, kindRef) {
  const [kind, ref] = kindRef.split(':')
  const list = materials[ref][kind === 'video' ? 'videos' : 'images']
  const ids = []
  for (const x of list.slice(0, kind === 'video' ? 1 : 2)) {
    const u = await must(sess.post('/media', undefined, { form: fileForm(x.file, mimeOf(x.file)) }), `upload chat media ${ref}`)
    ids.push(u.payload.media_id)
  }
  return ids
}

const e0 = (turn) => turn.expect || {}
const rejections = {}
function noteEngineRejection(code, detail, label, text, lang) {
  const r = (rejections[code + detail] ||= { code, detail, turns: [], kazakh: 0 })
  r.turns.push(`${label} («${text}»)`)
  if (lang === 'kk') r.kazakh++
}

async function customerTurn(c, turn, idx, { leavePending = false } = {}) {
  const admin = sessions.admin // the simulator plays the customer's phone
  const staff = staffOf(c.staff)
  const staffSess = sessions[c.staff]
  const label = `${c.name} #${idx + 1}`
  const started = Date.now()
  const r = await admin.post('/simulator/messages', { contact_ref: c.phone, conversation_ref: c.phone, text: turn.text, wait_for_response: true })
  if (!check(`${label}: message delivered and AI drafted a reply`, r.ok && r.payload?.draft?.text, `HTTP ${r.status} ${r.json?.message || ''}`)) return
  const draft = r.payload.draft
  const chatId = r.payload.conversation_id
  const t = (conv[c.key] ||= { chatId, transcript: [] })
  t.transcript.push({ from: 'customer', text: turn.text })
  t.transcript.push({ from: 'ai-draft', text: draft.text, lang: draft.reply_language, escalate: draft.escalate, ms: Date.now() - started })
  log(`💬 ${c.name}: ${turn.text}`)
  log(`🤖 [${draft.reply_language}${draft.escalate ? ', escalate' : ''}] ${draft.text.replace(/\n/g, ' ')}`)

  if (draft.escalate) {
    // Why did it hand over? The draft's escalation_reason says whether the
    // model chose to, or the engine rejected the model's answer.
    const list = await staffSess.get(`/chats/${chatId}/ai-drafts`)
    const items = list.payload?.items || list.payload || []
    const reason = items.find((d) => d.id === draft.id)?.escalation_reason || ''
    t.transcript[t.transcript.length - 1].reason = reason
    if (reason.startsWith('engine_error')) {
      log(`⚙️  engine rejected the AI answer: ${reason}`)
      const m = reason.match(/Code:(\w+) Detail:(.*?)\}\]\s*$/)
      noteEngineRejection(m ? m[1] : 'engine_error', m ? m[2].trim() : reason, label, turn.text, e0(turn).lang)
    }
  }

  const e = turn.expect || {}
  const soft = LLM === 'real'
  if (e.lang) check(`${label}: replies in ${e.lang === 'kk' ? 'Kazakh' : 'Russian'}`, draft.reply_language === e.lang, `got ${draft.reply_language}: ${draft.text}`)
  check(`${label}: ${e.escalate ? 'hands over to a manager' : 'answers without escalating'}`, Boolean(draft.escalate) === Boolean(e.escalate), draft.text, { soft: soft && !e.escalate })
  check(`${label}: no unresolved {{placeholders}}`, !draft.text.includes('{{'), draft.text)
  for (const want of (e.contains || []).map(expand)) {
    check(`${label}: reply contains "${want}"`, draft.text.includes(want), draft.text, { soft })
  }

  // Staff side: take the chat on the first message, then review the draft.
  if (idx === 0) {
    const a = await staffSess.patch(`/chats/${chatId}/assignee`, { assignee_user_id: staff.id })
    check(`${label}: ${staff.name} takes the chat`, a.ok, `HTTP ${a.status} ${a.json?.message || ''}`)
  }
  if (leavePending) return
  if (draft.escalate) {
    const reply = turn.staff_reply || (e0(turn).lang === 'kk' ? 'Рахмет! Нақтылап, жауап беремін.' : 'Спасибо! Уточню и вернусь с ответом.')
    const s = await staffSess.post(`/chats/${chatId}/messages`, { text: reply })
    check(`${label}: ${staff.name} answers the escalation personally`, s.ok, `HTTP ${s.status} ${s.json?.message || ''}`)
    await staffSess.post(`/chats/${chatId}/ai-drafts/dismiss`, {})
    t.transcript.push({ from: 'staff', who: staff.name, text: reply })
    return
  }
  const body = {}
  if (turn.staff_attach) body.media_ids = await uploadForChat(staffSess, turn.staff_attach)
  const ap = await staffSess.post(`/ai-drafts/${draft.id}/approve`, body)
  check(`${label}: ${staff.name} approves and sends the AI reply${body.media_ids ? ' with the product media attached by hand' : ''}`, ap.ok, `HTTP ${ap.status} ${ap.json?.message || ''}`)
  t.transcript.push({ from: 'sent', who: staff.name, text: draft.text, media: turn.staff_attach || null })
}

async function customers() {
  phase = 'customers'
  // Interleave customers turn by turn, like a real busy day in the inbox.
  const maxTurns = Math.max(...CUSTOMERS.map((c) => c.turns.length))
  for (let i = 0; i < maxTurns; i++) {
    for (const c of CUSTOMERS) {
      if (!c.turns[i]) continue
      const lastOfGulnar = c.key === 'gulnar' && i === c.turns.length - 1
      await customerTurn(c, c.turns[i], i, { leavePending: lastOfGulnar })
    }
  }

  // The AI chose media for "скиньте фото / видео" — does it reach the draft?
  const olga = conv.olga
  const drafts = await sessions.nurlan.get(`/chats/${olga.chatId}/ai-drafts`)
  const draftKeys = Object.keys(drafts.payload?.items?.[0] || drafts.payload?.[0] || {})
  const exposesMedia = draftKeys.some((k) => /media|attach/i.test(k))
  check('AI-chosen product photo/video travels with the draft', exposesMedia, 'drafts carry text only — staff attached media by hand', { soft: true })
  if (!exposesMedia) {
    findings.push({
      title: 'Photos and videos the AI picks are dropped before the draft',
      detail:
        'The model returns media_files_to_send (e.g. products.trolley-jack-3t.gallery_images); response.Engine validates it (backend/response/engine.go: aiprompt.ResolveSend) but GenerateResult has no media field and ai_drafts has no media column, ' +
        'so the draft says "Отправляю фото…" with nothing attached. In this run staff attached the files by hand via approve media_ids.',
    })
  }

  phase = 'staff-crm'
  for (const c of CUSTOMERS) {
    const t = conv[c.key]
    if (!t) continue
    const sess = sessions[c.staff]
    const chat = (await sess.get(`/chats/${t.chatId}`)).payload
    const customerId = chat?.customer_id
    if (!check(`${c.name}: chat is linked to a CRM customer`, Boolean(customerId))) continue
    t.customerId = customerId
    const up = await sess.patch(`/customers/${customerId}`, { display_name: c.name, phone: c.phone, assignee_user_id: staffOf(c.staff).id })
    check(`${c.name}: profile named, phone saved, owner set`, up.ok, `HTTP ${up.status} ${up.json?.message || ''}`)
    for (const tag of c.crm?.tags || []) {
      const tr = await sess.post(`/customers/${customerId}/tags`, { tag_id: ORG.tagIds[tag] })
      check(`${c.name}: tagged «${tag}»`, tr.ok, `HTTP ${tr.status} ${tr.json?.message || ''}`)
    }
    if (c.crm?.note) {
      const nr = await sess.post(`/customers/${customerId}/notes`, { body: c.crm.note })
      check(`${c.name}: note added`, nr.ok, `HTTP ${nr.status}`)
    }
    if (c.crm?.followup) {
      // Due "in N days" on Follis's own clock (Asia/Almaty, UTC+5): today's
      // tasks two hours from now, later ones at 10:00.
      const target = new Date(Date.now() + c.crm.followup.in_days * 86400_000)
      const dueDate = new Intl.DateTimeFormat('en-CA', { timeZone: ORG.organization.timezone }).format(target)
      const [hh, mm] = new Intl.DateTimeFormat('en-GB', { timeZone: ORG.organization.timezone, hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).format(new Date()).split(':').map(Number)
      const dueMinute = c.crm.followup.in_days === 0 ? Math.min(hh * 60 + mm + 120, 1439) : 600
      const pad = (n) => String(n).padStart(2, '0')
      const fr = await sess.post('/followups', {
        customer_id: customerId,
        conversation_id: t.chatId,
        channel: 'simulator',
        due_date: dueDate,
        due_minute: dueMinute,
        due_at: `${dueDate}T${pad(Math.floor(dueMinute / 60))}:${pad(dueMinute % 60)}:00+05:00`,
        action: c.crm.followup.kind,
        note: c.crm.followup.action,
        assignee_user_id: staffOf(c.staff).id,
      })
      check(`${c.name}: follow-up scheduled — ${c.crm.followup.action}`, fr.ok, `HTTP ${fr.status} ${fr.json?.message || ''}`)
    }
    if (c.crm?.resolve) {
      const sr = await sess.patch(`/chats/${t.chatId}/status`, { status: 'resolved' })
      check(`${c.name}: chat marked resolved`, sr.ok, `HTTP ${sr.status}`)
    }
  }

  for (const r of Object.values(rejections)) {
    findings.push({
      title: `Valid AI answers rejected by the ${r.code} check (${r.turns.length} replies)`,
      detail:
        `${r.detail}. The stored value is short (Shymkent delivery = "1" day), so the same digits inside product names such as «1/2"» count as a leaked fact (backend/aiprompt/contract.go validateFactContract → containsLiteral). ` +
        `The engine then falls back to the holding reply${r.kazakh ? `, in Russian even for the ${r.kazakh} Kazakh-speaking turn(s) (response.HoldingText has no Kazakh version)` : ''}. Affected: ${r.turns.join('; ')}.`,
    })
  }

  phase = 'verify-day'
  const chats = (await sessions.erlan.get('/chats?page_size=100')).payload.items
  check(`Inbox holds all ${CUSTOMERS.length} customer conversations`, chats.length === CUSTOMERS.length, `got ${chats.length}`)
  const assignedRight = CUSTOMERS.every((c) => chats.find((x) => x.id === conv[c.key]?.chatId)?.assignee_user_id === staffOf(c.staff).id)
  check('Every conversation is assigned to the right staff member', assignedRight)
  const msgs = (await sessions.nurlan.get(`/chats/${conv.olga.chatId}/messages?limit=50`)).payload
  const items = msgs.items || msgs
  const out = items.filter((m) => m.direction === 'out' || m.from_me)
  const withMedia = out.filter((m) => (m.media && (m.media.id || m.media.length)) || m.media_type || m.message_kind === 'image')
  check('Olga received the AI replies, including the product photo', out.length >= 3 && withMedia.length >= 1, `outbound=${out.length} with media=${withMedia.length}`)
  const failed = []
  let sent = 0
  for (const c of CUSTOMERS) {
    const page = (await sessions.erlan.get(`/chats/${conv[c.key].chatId}/messages?limit=100`)).payload
    for (const m of (page.items || page).filter((m) => m.direction === 'out')) {
      sent++
      if (m.status === 'failed') failed.push(`${c.name}: ${m.content.slice(0, 40)}`)
    }
  }
  check(`All ${sent} replies sent by staff were delivered (none failed)`, failed.length === 0, failed.join('; '))
  const custs = (await sessions.erlan.get('/customers?page_size=100')).payload.items
  check('Customers list shows every named customer', CUSTOMERS.every((c) => custs.some((x) => x.display_name === c.name)), custs.map((x) => x.display_name).join(', '))
  const buckets = (await sessions.erlan.get('/followups/buckets')).payload
  const planned = CUSTOMERS.filter((c) => c.crm?.followup).length
  log(`follow-up buckets: ${JSON.stringify(buckets).slice(0, 300)}`)
  const totalOpen = Object.values(buckets || {}).reduce((n, v) => n + (Array.isArray(v) ? v.length : typeof v === 'number' ? v : v?.items?.length || v?.count || 0), 0)
  check(`Follow-up board holds the ${planned} planned follow-ups`, totalOpen >= planned, `found ${totalOpen}`)
}

// ---------------------------------------------------------------------------
// phase 5: screenshots

async function screenshots() {
  phase = 'screenshots'
  let page = await personPage()
  await page.goto(`${UI}/login`, { waitUntil: 'domcontentloaded' })
  await settle(page)
  await shot(page, '01-login', 'Вход в xchats')
  await page.context().close()

  page = await personPage(sessions.erlan)
  await page.goto(`${UI}/home`, { waitUntil: 'domcontentloaded' })
  await settle(page, 2000)
  await shot(page, '02-dashboard', 'Главная владельца Follis: непрочитанные чаты, просроченные задачи, изменения в Черновике')

  await page.goto(`${UI}/settings`, { waitUntil: 'domcontentloaded' })
  await settle(page)
  await tryClick(page.getByRole('button', { name: 'Команда' }))
  await settle(page)
  await shot(page, '03-team', 'Команда Follis: 2 администратора и 4 сотрудника')

  await page.goto(`${UI}/knowledge-base`, { waitUntil: 'domcontentloaded' })
  await settle(page)
  await tryClick(page.getByRole('button', { name: /^Товары/ }))
  await settle(page, 1500)
  await shot(page, '04-kb-products', 'База знаний: товары с фото, ценами и наличием')
  await page.mouse.wheel(0, 1400)
  await settle(page, 800)
  await shot(page, '04b-kb-products-more', 'База знаний: оборудование для автосервиса, предзаказ и «нет в наличии»')
  await page.mouse.wheel(0, -1400)
  // A product's photos and demo video show in its edit dialog.
  const card = page.getByTestId('kb-product-card').filter({ hasText: 'Шиномонтажный' }).first()
  await card.scrollIntoViewIfNeeded().catch(() => {})
  await card.hover().catch(() => {})
  if (await tryClick(card.getByRole('button', { name: 'Изменить' }))) {
    const dialog = page.getByRole('dialog')
    await dialog.waitFor({ timeout: 10_000 }).catch(() => {})
    await settle(page, 1000)
    await dialog.getByText(/видео/i).last().scrollIntoViewIfNeeded().catch(() => {})
    await settle(page, 1200)
    await shot(page, '05-kb-product-video', 'Карточка товара: фото и видео-демонстрация шиномонтажного станка')
    await page.keyboard.press('Escape')
  }
  await page.goto(`${UI}/knowledge-base`, { waitUntil: 'domcontentloaded' })
  await settle(page)
  await shot(page, '05b-kb-overview', 'База знаний: роль ассистента, задача, ограничения и языковая политика')

  await page.context().close()

  // Staff views of the inbox.
  page = await personPage(sessions.nurlan)
  await page.goto(`${UI}/chatboard/${conv.olga.chatId}`, { waitUntil: 'domcontentloaded' })
  await settle(page, 2500)
  await shot(page, '08-inbox-photo-sent', 'Сотрудник (Нурлан): ответы ассистента отправлены, фото домкрата приложено вручную')
  await page.goto(`${UI}/home`, { waitUntil: 'domcontentloaded' })
  await settle(page, 1500)
  await shot(page, '16-member-view', 'Вид сотрудника: без раздела «Настройки» — только администраторы управляют командой и ИИ')
  await page.context().close()

  page = await personPage(sessions.asel)
  await page.goto(`${UI}/chatboard/${conv.gulnar.chatId}`, { waitUntil: 'domcontentloaded' })
  await settle(page, 2000)
  await tryClick(page.getByRole('tab', { name: /ИИ-помощник/ }))
  await settle(page, 1200)
  await shot(page, '09-inbox-ai-draft-kk', 'Оператор (Асель): черновик ответа на казахском ждёт проверки — без одобрения клиенту ничего не уходит')
  await page.goto(`${UI}/chatboard/${conv.dauren.chatId}`, { waitUntil: 'domcontentloaded' })
  await settle(page, 2000)
  await shot(page, '10-inbox-kk-latin', 'Клиент пишет по-казахски русскими буквами: ключ 1/2" попал под проверку утечки (см. выводы), Туркестан — ответ по-казахски')
  await page.context().close()

  page = await personPage(sessions.baurzhan)
  await page.goto(`${UI}/chatboard/${conv.marat.chatId}`, { waitUntil: 'domcontentloaded' })
  await settle(page, 2000)
  await shot(page, '11-inbox-escalation', 'Оптовый запрос СТО: ассистент передал менеджеру, специалист ответил сам')
  await tryClick(page.getByRole('tab', { name: /Клиент/ }))
  await settle(page, 1200)
  await shot(page, '12-customer-panel', 'Карточка клиента: теги «СТО», «Опт», заметка и следующий шаг')
  await page.context().close()

  page = await personPage(sessions.erlan)
  await page.goto(`${UI}/chatboard`, { waitUntil: 'domcontentloaded' })
  await settle(page, 2000)
  await shot(page, '13-inbox-all', 'Общий входящий: 10 клиентов за день, у каждого чата свой ответственный')
  await page.goto(`${UI}/customers`, { waitUntil: 'domcontentloaded' })
  await settle(page, 1500)
  await shot(page, '14-customers', 'Клиенты: имена, телефоны, теги и ответственные')
  await page.goto(`${UI}/followups`, { waitUntil: 'domcontentloaded' })
  await settle(page, 1500)
  await tryClick(page.getByRole('button', { name: 'Все', exact: true }))
  await settle(page, 1000)
  await shot(page, '15-followups', 'Задачи: перезвонить, выставить счёт, подтвердить самовывоз')

  // Last, so this extra test chat does not show up in the inbox shots above.
  // Simulator page: a fresh test conversation typed in the UI, RU then KK.
  await page.goto(`${UI}/simulator`, { waitUntil: 'domcontentloaded' })
  await settle(page)
  await tryClick(page.getByTestId('simulator-new-conversation'))
  for (const text of ['Здравствуйте! Сколько стоит набор инструментов на 108 предметов?', 'Сәлеметсіз бе! Балансировочный станок бағасы қанша?']) {
    await page.getByTestId('simulator-input').fill(text)
    await page.getByTestId('simulator-send').click()
    await page.getByTestId('simulator-loading').waitFor({ state: 'detached', timeout: 60_000 }).catch(() => {})
    await settle(page, 1500)
  }
  await shot(page, '07-simulator', 'Проверка ассистента: вопрос по-русски и по-казахски, цены подставлены из базы знаний')
  await page.context().close()
}

// ---------------------------------------------------------------------------
// report

function report(error) {
  const count = (s) => results.filter((r) => r.status === s).length
  const summary = { llm: LLM, pass: count('PASS'), warn: count('WARN'), fail: count('FAIL'), error: error ? String(error.stack || error) : null }
  const data = {
    generated_at: new Date().toISOString(),
    summary,
    organization: ORG.organization,
    staff: ORG.staff.map(({ password, ...s }) => s),
    catalog: { products: KB.products.length, topics: KB.topics.length, zones: KB.zones.length },
    results,
    findings,
    conversations: Object.fromEntries(Object.entries(conv).map(([k, v]) => [k, { name: CUSTOMERS.find((c) => c.key === k).name, ...v }])),
    screenshots: shots,
  }
  writeFileSync(path.join(OUT, 'results.json'), JSON.stringify(data, null, 2))

  const md = []
  md.push(`# Follis pilot rehearsal — ${data.generated_at.slice(0, 16).replace('T', ' ')} UTC`, '')
  md.push(`AI replies: **${LLM === 'real' ? 'real model' : 'mock (catalog-aware)'}** · ✅ ${summary.pass} passed · ⚠️ ${summary.warn} warnings · ❌ ${summary.fail} failed`, '')
  if (error) md.push('**Run aborted:**', '```', summary.error, '```', '')
  if (findings.length) {
    md.push('## Findings', '')
    for (const f of findings) md.push(`- **${f.title}.** ${f.detail}`)
    md.push('')
  }
  md.push('## Checks', '', '| Phase | Check | Result | Detail |', '|---|---|---|---|')
  for (const r of results) md.push(`| ${r.phase} | ${r.name.replace(/\|/g, '/')} | ${r.status} | ${r.status === 'PASS' ? '' : r.detail.replace(/\|/g, '/').replace(/\n/g, ' ').slice(0, 220)} |`)
  md.push('', '## Conversations', '')
  for (const [, v] of Object.entries(data.conversations)) {
    md.push(`### ${v.name}`, '')
    for (const t of v.transcript) {
      const who = { customer: '👤 Клиент', 'ai-draft': `🤖 Черновик ИИ [${t.lang}${t.escalate ? ', эскалация' : ''}]`, sent: `✉️ Отправлено (${t.who})${t.media ? ` + ${t.media}` : ''}`, staff: `🧑‍🔧 ${t.who}` }[t.from]
      if (t.from === 'sent') continue
      md.push(`- ${who}: ${t.text.replace(/\n/g, ' ')}`)
    }
    md.push('')
  }
  md.push('## Screenshots', '')
  for (const s of shots) md.push(`- \`${s.file}\` — ${s.caption}`)
  writeFileSync(path.join(OUT, 'REPORT.md'), md.join('\n'))
  console.log(`\nREPORT: ${path.join(OUT, 'REPORT.md')}\n✅ ${summary.pass}  ⚠️  ${summary.warn}  ❌ ${summary.fail}`)
  return summary
}

// ---------------------------------------------------------------------------

// Same fallback as frontend/scripts/capture-screenshots.mjs: some sandboxes
// pre-install Chromium at a fixed path whose revision differs from the
// checked-in @playwright/test.
const pinned = '/opt/pw-browsers/chromium'
let failure = null
try {
  browser = await chromium.launch(existsSync(pinned) ? { executablePath: pinned } : {})
  await setup()
  await draftReview()
  await verify()
  await customers()
  await screenshots()
} catch (err) {
  failure = err
  console.error('\n💥', err)
} finally {
  await browser?.close()
}
const summary = report(failure)
process.exit(failure || summary.fail ? 1 : 0)
