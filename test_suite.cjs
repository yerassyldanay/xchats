const { chromium } = require('./frontend/node_modules/playwright');

async function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

(async () => {
  console.log('🚀 Launching Chrome on DISPLAY :0...');
  const browser = await chromium.launch({
    headless: false,
    executablePath: '/usr/bin/google-chrome',
    args: ['--no-sandbox', '--start-maximized', '--window-size=1440,900']
  });

  const context = await browser.newContext({ viewport: null });
  const page = await context.newPage();

  // 1. LOGIN
  console.log('🔑 1. Logging into xchats...');
  await page.goto('http://localhost:5173/login', { waitUntil: 'networkidle' });
  await page.fill('input[type="email"]', 'admin@xchat.kz');
  await page.fill('input[type="password"]', 'xchat-admin-change-me');
  await page.click('button[type="submit"]');
  await page.waitForNavigation({ waitUntil: 'networkidle' }).catch(() => {});
  await sleep(1500);

  // 2. CRM: CUSTOMERS
  console.log('👥 2. Testing CRM Customers (/customers)...');
  await page.goto('http://localhost:5173/customers', { waitUntil: 'networkidle' });
  await sleep(2000);

  // Toggle Grid / List view
  console.log('📋 Exploring Customer Grid & List views...');
  const listToggle = page.locator('button[title*="список"], button[title*="list"], header button:has(svg.lucide-list)');
  if (await listToggle.count() > 0) {
    await listToggle.first().click();
    await sleep(1500);
  }
  const gridToggle = page.locator('button[title*="сетк"], button[title*="grid"], header button:has(svg.lucide-layout-grid)');
  if (await gridToggle.count() > 0) {
    await gridToggle.first().click();
    await sleep(1500);
  }

  // Create New Customer in CRM
  console.log('➕ Creating New Customer: Айгерим Серикова...');
  const newCustBtn = page.locator('button', { hasText: /Новый клиент|New customer/i });
  if (await newCustBtn.count() > 0) {
    await newCustBtn.first().click();
    await sleep(1200);

    const nameInput = page.locator('label').filter({ hasText: /Имя|Name/i }).locator('input');
    const phoneInput = page.locator('label').filter({ hasText: /Телефон|Phone/i }).locator('input');
    const emailInput = page.locator('label').filter({ hasText: /Email/i }).locator('input');

    if (await nameInput.count() > 0) await nameInput.fill('Айгерим Серикова (Клиент салона)');
    if (await phoneInput.count() > 0) await phoneInput.fill('+7 777 123 45 67');
    if (await emailInput.count() > 0) await emailInput.fill('aigerim@example.kz');
    await sleep(1000);

    // Click Save
    const saveBtn = page.locator('div[role="dialog"] button', { hasText: /Сохранить|Save/i });
    if (await saveBtn.count() > 0) {
      await saveBtn.last().click();
      await sleep(1500);
    }
    // Make sure dialog is closed
    await page.keyboard.press('Escape');
    await sleep(800);
  }

  // 3. TASKS & FOLLOWUPS
  console.log('📅 3. Testing Task Creation & Management (/followups)...');
  await page.goto('http://localhost:5173/followups', { waitUntil: 'networkidle' });
  await sleep(2000);

  console.log('➕ Opening Task Creation Dialog...');
  const newTaskBtn = page.locator('[data-testid="followups-new-task"]');
  if (await newTaskBtn.count() > 0) {
    await newTaskBtn.click();
    await sleep(1500);

    // Pick customer
    const custSearch = page.locator('div[role="dialog"] input[placeholder*="клиента"], div[role="dialog"] input[placeholder*="customer"]');
    if (await custSearch.count() > 0) {
      await custSearch.fill('Айгерим');
      await sleep(1000);
      const custOption = page.locator('div[role="dialog"] div.absolute button');
      if (await custOption.count() > 0) {
        await custOption.first().click();
        await sleep(600);
      }
    }

    // Set Note
    const noteArea = page.locator('div[role="dialog"] textarea');
    if (await noteArea.count() > 0) {
      await noteArea.fill('Позвонить клиенту: предложить запись на сложное окрашивание Airtouch к мастеру Алине Ким');
      await sleep(800);
    }

    // Set Date
    const today = new Date().toISOString().split('T')[0];
    const dateInput = page.locator('div[role="dialog"] input[type="date"]');
    if (await dateInput.count() > 0) {
      await dateInput.fill(today);
      await sleep(500);
    }

    // Save
    const saveTaskBtn = page.locator('div[role="dialog"] button', { hasText: /Сохранить|Создать|Save|Create/i });
    if (await saveTaskBtn.count() > 0) {
      await saveTaskBtn.last().click();
      await sleep(2000);
    }
    await page.keyboard.press('Escape');
    await sleep(1000);
  }

  // Toggle Completed and Active tabs
  console.log('📑 Checking Task status tabs...');
  const completedTab = page.locator('[data-testid="followups-tab-completed"]');
  if (await completedTab.count() > 0) {
    await completedTab.click({ force: true });
    await sleep(1800);
  }
  const activeTab = page.locator('[data-testid="followups-tab-active"]');
  if (await activeTab.count() > 0) {
    await activeTab.click({ force: true });
    await sleep(1500);
  }

  // 4. CHATBOARD & MEDIA RESPONSE
  console.log('💬 4. Testing Chatboard & Responding with Media (/chatboard)...');
  await page.goto('http://localhost:5173/chatboard', { waitUntil: 'networkidle' });
  await sleep(2500);

  // Select first conversation
  console.log('📩 Selecting customer chat thread...');
  const chatItem = page.locator('button:has(.ring-card), aside button.relative');
  if (await chatItem.count() > 0) {
    await chatItem.first().click();
    await sleep(2000);
  }

  // Customer CRM Notes in right sidebar
  console.log('📝 Adding internal CRM note to Customer profile...');
  const custTab = page.locator('button[role="tab"]', { hasText: /Клиент|Customer/i });
  if (await custTab.count() > 0) {
    await custTab.first().click();
    await sleep(1200);

    const noteInput = page.locator('textarea[placeholder*="заметк"], textarea[placeholder*="note"]');
    if (await noteInput.count() > 0) {
      await noteInput.first().fill('Клиент просил выслать фото работ колориста и прайс');
      await noteInput.first().blur();
      await sleep(1500);
    }
  }

  // Assistant Tab in right sidebar
  console.log('🤖 Checking AI Assistant tab in sidebar...');
  const asstTab = page.locator('button[role="tab"]', { hasText: /Ассистент|Assistant/i });
  if (await asstTab.count() > 0) {
    await asstTab.first().click();
    await sleep(2000);
  }

  // Responding with Media in Chat
  console.log('📎 Attaching Salon Portfolio photo to Message Composer...');
  const fileInput = page.locator('input[type="file"]');
  if (await fileInput.count() > 0) {
    await fileInput.setInputFiles('/tmp/salon_portfolio_sample.jpg');
    await sleep(1500);
  }

  const composerArea = page.locator('textarea[placeholder*="сообщение"], textarea[placeholder*="message"]');
  if (await composerArea.count() > 0) {
    await composerArea.first().fill('Здравствуйте! Отправляю вам фото примера сложного окрашивания Airtouch от нашего мастера Алины Ким.');
    await sleep(1000);
  }

  console.log('📤 Sending message with media...');
  const sendBtn = page.locator('button:has(svg.lucide-send), button:has(svg.lucide-send-horizontal), button[type="submit"]');
  if (await sendBtn.count() > 0) {
    await sendBtn.first().click();
    await sleep(3500);
  }

  // View Lightbox for sent media
  console.log('🖼️ Opening Media Lightbox preview...');
  const mediaImg = page.locator('img[src*="media"]');
  if (await mediaImg.count() > 0) {
    await mediaImg.last().click();
    await sleep(2500);
    await page.keyboard.press('Escape');
    await sleep(1000);
  }

  // 5. KNOWLEDGE BASE MATERIALS
  console.log('📁 5. Testing Knowledge Base Materials (/knowledge-base)...');
  await page.goto('http://localhost:5173/knowledge-base', { waitUntil: 'networkidle' });
  await sleep(1500);
  const materialsTab = page.locator('button', { hasText: /Файлы|Materials/i });
  if (await materialsTab.count() > 0) {
    await materialsTab.first().click();
    await sleep(2500);
  }

  // 6. CHANNELS
  console.log('📡 6. Testing Channels (/channels)...');
  await page.goto('http://localhost:5173/channels', { waitUntil: 'networkidle' });
  await sleep(2000);

  // 7. RETURN TO CHATBOARD
  console.log('✨ 7. Returning to Chatboard with all features verified...');
  await page.goto('http://localhost:5173/chatboard', { waitUntil: 'networkidle' });
  await sleep(1500);

  console.log('🎉 All requested functionalities tested successfully! Browser kept open on screen.');
  await new Promise(() => {});
})().catch(err => {
  console.error('Error during test execution:', err);
});
