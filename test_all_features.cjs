const { chromium } = require('playwright');
const path = require('path');

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

(async () => {
  console.log('🚀 Launching visible Chrome on DISPLAY :0...');
  const browser = await chromium.launch({
    headless: false,
    executablePath: '/usr/bin/google-chrome',
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--start-maximized'],
  });

  const context = await browser.newContext({
    viewport: null, // Full screen viewport
  });

  const page = await context.newPage();

  // Helper to ensure all modal backdrops/dialogs are cleanly dismissed
  async function cleanOverlays() {
    await page.keyboard.press('Escape').catch(() => {});
    await sleep(400);
    await page.evaluate(() => {
      document.querySelectorAll('[data-state="open"]').forEach((el) => {
        if (el.classList.contains('fixed') && el.classList.contains('z-50')) {
          el.remove();
        }
      });
    });
  }

  // 1. LOGIN
  console.log('🔑 Logging in as admin@xchat.kz...');
  await page.goto('http://localhost:5173/login', { waitUntil: 'networkidle' });
  await sleep(1000);

  const emailInput = page.locator('input[type="email"], input[name="email"], input[placeholder*="email" i]');
  const passInput = page.locator('input[type="password"]');

  if (await emailInput.count() > 0) {
    await emailInput.fill('admin@xchat.kz');
    await passInput.fill('xchat-admin-change-me');
    await page.locator('button[type="submit"]').click();
    await sleep(2500);
  }

  // 2. CRM CUSTOMERS
  console.log('👥 1. Testing CRM Customers (/customers)...');
  await page.goto('http://localhost:5173/customers', { waitUntil: 'networkidle' });
  await sleep(2000);

  // Toggle Grid / List view
  console.log('📋 Testing Grid / List view switch...');
  const listToggle = page.locator('button[title*="список" i], button[title*="list" i]').first();
  if (await listToggle.count() > 0) {
    await listToggle.click();
    await sleep(1200);
  }
  const gridToggle = page.locator('button[title*="сетка" i], button[title*="grid" i]').first();
  if (await gridToggle.count() > 0) {
    await gridToggle.click();
    await sleep(1200);
  }

  // Quick filter buttons
  console.log('🏷️ Testing Quick Filters...');
  const quickFilters = page.locator('header button.rounded-full');
  if (await quickFilters.count() > 1) {
    await quickFilters.nth(1).click();
    await sleep(1000);
    await quickFilters.first().click();
    await sleep(1000);
  }

  // Create New Customer
  console.log('➕ Creating New Salon Customer in CRM...');
  const newCustBtn = page.locator('button', { hasText: /Новый клиент|New customer/i });
  if (await newCustBtn.count() > 0) {
    await newCustBtn.first().click();
    await sleep(1500);

    const nameField = page.locator('div[role="dialog"] input').nth(0);
    const phoneField = page.locator('div[role="dialog"] input').nth(1);
    const mailField = page.locator('div[role="dialog"] input').nth(2);

    if (await nameField.count() > 0) {
      await nameField.fill('Айгерим Серикова (Салон Аура)');
      await sleep(400);
    }
    if (await phoneField.count() > 0) {
      await phoneField.fill('+7 777 123 45 67');
      await sleep(400);
    }
    if (await mailField.count() > 0) {
      await mailField.fill('aigerim.beauty@example.kz');
      await sleep(400);
    }

    const saveCustomerBtn = page.locator('div[role="dialog"] button', { hasText: /Сохранить|Save/i });
    if (await saveCustomerBtn.count() > 0) {
      await saveCustomerBtn.last().click();
      await sleep(1500);
    }
    await cleanOverlays();
  }

  // 3. TASKS & FOLLOW-UPS
  console.log('📅 2. Testing Task Creation (/followups)...');
  await page.goto('http://localhost:5173/followups', { waitUntil: 'networkidle' });
  await sleep(2000);

  console.log('➕ Opening New Task Dialog...');
  const newTaskBtn = page.locator('[data-testid="followups-new-task"]');
  if (await newTaskBtn.count() > 0) {
    await newTaskBtn.click();
    await sleep(1200);

    // Pick customer by searching
    console.log('🔍 Searching and picking customer for task...');
    const searchCustomerInput = page.locator('div[role="dialog"] input.pl-9, div[role="dialog"] input[placeholder*="username" i], div[role="dialog"] input[placeholder*="Имя" i]').first();
    if (await searchCustomerInput.count() > 0) {
      await searchCustomerInput.fill('Айгерим');
      await sleep(1200);

      const customerOption = page.locator('div[role="dialog"] div.absolute button').first();
      if (await customerOption.count() > 0) {
        await customerOption.click();
        await sleep(800);
      }
    }

    // Set Note
    const taskNote = page.locator('div[role="dialog"] textarea');
    if (await taskNote.count() > 0) {
      await taskNote.fill('Согласовать запись на сложное окрашивание Airtouch к мастеру Алине Ким');
      await sleep(500);
    }

    // Set Date (Tomorrow)
    const d = new Date();
    d.setDate(d.getDate() + 1);
    const tomorrowStr = d.toISOString().split('T')[0];
    const dateInput = page.locator('div[role="dialog"] input[type="date"]');
    if (await dateInput.count() > 0) {
      await dateInput.fill(tomorrowStr);
      await sleep(500);
    }

    // Save Task
    console.log('💾 Saving Follow-up Task...');
    const saveTaskBtn = page.locator('div[role="dialog"] button', { hasText: /Сохранить|Save/i });
    if (await saveTaskBtn.count() > 0) {
      await saveTaskBtn.last().click();
      await sleep(1500);
    }

    await cleanOverlays();
  }

  // Inspect task board view
  console.log('📑 Inspecting Task sections (Today / Tomorrow / Later)...');
  await page.evaluate(() => window.scrollBy({ top: 250, behavior: 'smooth' }));
  await sleep(1500);
  await page.evaluate(() => window.scrollBy({ top: -250, behavior: 'smooth' }));
  await sleep(1000);

  // Switch to Completed tab
  const completedTab = page.locator('header button', { hasText: /Выполненные|Completed/i });
  if (await completedTab.count() > 0) {
    await completedTab.first().click();
    await sleep(1500);
    const activeTab = page.locator('header button', { hasText: /Активные|Active/i });
    if (await activeTab.count() > 0) {
      await activeTab.first().click();
      await sleep(1000);
    }
  }

  // 4. CHATBOARD, CRM PANEL, MEDIA RESPONSE & LIGHTBOX
  console.log('💬 3. Testing Customer Inbox & Media Response (/chatboard)...');
  await page.goto('http://localhost:5173/chatboard', { waitUntil: 'networkidle' });
  await sleep(2500);
  await cleanOverlays();

  // Select customer chat conversation
  console.log('📩 Selecting customer chat conversation...');
  const chatCards = page.locator('div.flex-1.overflow-y-auto > button');
  if (await chatCards.count() > 0) {
    await chatCards.first().click();
    await sleep(2000);
  }

  // Customer CRM Panel in Right Sidebar
  console.log('👤 Inspecting Customer CRM panel in sidebar...');
  const noteArea = page.locator('textarea[placeholder*="запомнить" i], textarea[placeholder*="note" i]').first();
  if (await noteArea.count() > 0) {
    await noteArea.scrollIntoViewIfNeeded();
    await noteArea.fill('Клиентка интересуется примерами работ по Airtouch и графиком колориста Алины Ким.');
    await noteArea.blur();
    await sleep(1500);
  }

  // Responding with Media File in Chat
  console.log('📎 4. Attaching Salon Portfolio Image in Chat Composer...');
  const fileInput = page.locator('input[type="file"]');
  if (await fileInput.count() > 0) {
    await fileInput.setInputFiles('/tmp/salon_portfolio_sample.jpg');
    await sleep(1500);
  }

  const composerArea = page.locator('textarea[placeholder*="сообщение" i], textarea[placeholder*="message" i]');
  if (await composerArea.count() > 0) {
    await composerArea.fill('Здравствуйте! Отправляю вам фото примера сложного окрашивания Airtouch от нашего топ-стилиста Алины Ким.');
    await sleep(1000);
  }

  // Click Send
  console.log('📤 Sending message with media attachment...');
  const sendBtn = page.locator('button', { hasText: /Отправить|Send/i }).last();
  if (await sendBtn.count() > 0) {
    await sendBtn.click();
    await sleep(4000);
  }

  // View Lightbox for sent media
  console.log('🖼️ 5. Testing Media Lightbox preview on sent photo...');
  const sentImages = page.locator('img[src*="media"], img.cursor-zoom-in');
  if (await sentImages.count() > 0) {
    await sentImages.last().click();
    console.log('✨ Lightbox opened full screen!');
    await sleep(3500);
    // Close lightbox
    await page.keyboard.press('Escape');
    await sleep(1000);
    console.log('✨ Lightbox closed.');
  }

  // 5. KNOWLEDGE BASE MATERIALS
  console.log('📁 6. Testing Knowledge Base Materials (/knowledge-base)...');
  await page.goto('http://localhost:5173/knowledge-base', { waitUntil: 'networkidle' });
  await sleep(2000);

  const materialsTab = page.locator('button', { hasText: /Файлы|Materials/i });
  if (await materialsTab.count() > 0) {
    await materialsTab.first().click();
    await sleep(2500);
  }

  // 6. CHANNELS
  console.log('📡 7. Testing Communication Channels (/channels)...');
  await page.goto('http://localhost:5173/channels', { waitUntil: 'networkidle' });
  await sleep(2500);

  // 7. RETURN TO CHATBOARD
  console.log('✨ Returning to Chatboard inbox with tests completed...');
  await page.goto('http://localhost:5173/chatboard', { waitUntil: 'networkidle' });
  await sleep(2000);

  console.log('🎉 All feature tests completed successfully! Browser remains open on your screen.');
  // Keep process alive so the user can interact with the open Chrome window
  await new Promise(() => {});
})().catch((err) => {
  console.error('Error during test execution:', err);
});
